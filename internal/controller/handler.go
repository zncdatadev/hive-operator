package controller

import (
	"context"
	"fmt"
	stackv1alpha1 "github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
	opgo "github.com/zncdata-labs/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdata-labs/operator-go/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resource todo for dep, svc and so on
type Resource interface {
	extractResource()
}

func (r *HiveMetastoreReconciler) createOrUpdateResource(ctx context.Context, instance *stackv1alpha1.HiveMetastore,
	roleGroupExtractor func(*stackv1alpha1.HiveMetastore, context.Context, string, *stackv1alpha1.RoleConfigSpec,
		*runtime.Scheme) (*client.Object, error)) error {
	resources, err := r.extractResources(instance, ctx, roleGroupExtractor)
	if err != nil {
		return err
	}

	for _, rsc := range resources {
		if rsc == nil {
			continue
		}

		if err := CreateOrUpdate(ctx, r.Client, *rsc); err != nil {
			r.Log.Error(err, "Failed to create or update Resource", "resource", rsc)
			return err
		}
	}
	return nil
}

func (r *HiveMetastoreReconciler) fetchResource(ctx context.Context, obj client.Object,
	instance *stackv1alpha1.HiveMetastore) error {
	name := obj.GetName()
	kind := obj.GetObjectKind()
	if err := r.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: name}, obj); err != nil {
		opt := []any{"ns", instance.Namespace, "name", name, "kind", kind}
		if apierrors.IsNotFound(err) {
			r.Log.Error(err, "Fetch resource NotFound", opt...)
		} else {
			r.Log.Error(err, "Fetch resource occur some unknown err", opt...)
		}
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) getRoleGroupLabels(config *stackv1alpha1.RoleConfigSpec) map[string]string {
	additionalLabels := make(map[string]string)
	if configLabels := config.MatchLabels; configLabels != nil {
		for k, v := range config.MatchLabels {
			additionalLabels[k] = v
		}
	}
	return additionalLabels
}

func (r *HiveMetastoreReconciler) mergeLabels(mergeLabels *Map, instanceLabels map[string]string,
	roleGroup *stackv1alpha1.RoleConfigSpec) {
	mergeLabels.MapMerge(instanceLabels, true)
	mergeLabels.MapMerge(r.getRoleGroupLabels(roleGroup), true)
}

func (r *HiveMetastoreReconciler) getServiceInfo(instanceSvc *stackv1alpha1.ServiceSpec,
	roleGroup *stackv1alpha1.RoleConfigSpec) (int32, corev1.ServiceType, map[string]string) {
	var targetSvc = instanceSvc
	if roleGroup != nil && roleGroup.Service != nil {
		targetSvc = roleGroup.Service
	}
	return targetSvc.Port, targetSvc.Type, targetSvc.Annotations
}

func (r *HiveMetastoreReconciler) extractResources(instance *stackv1alpha1.HiveMetastore, ctx context.Context,
	roleGroupExtractor func(*stackv1alpha1.HiveMetastore, context.Context, string, *stackv1alpha1.RoleConfigSpec,
		*runtime.Scheme) (*client.Object, error)) ([]*client.Object, error) {
	var resources []*client.Object
	if instance.Spec.RoleGroups != nil {
		for roleGroupName, roleGroup := range instance.Spec.RoleGroups {
			rsc, err := roleGroupExtractor(instance, ctx, roleGroupName, roleGroup, r.Scheme)
			if err != nil {
				return nil, err
			}
			resources = append(resources, rsc)
		}
	}
	return resources, nil
}

func (r *HiveMetastoreReconciler) reconcilePvc(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	err := r.createOrUpdateResource(ctx, instance, r.extractPvcForRoleGroup)
	if err != nil {
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) extractPvcForRoleGroup(instance *stackv1alpha1.HiveMetastore, _ context.Context,
	groupName string, roleGroup *stackv1alpha1.RoleConfigSpec, scheme *runtime.Scheme) (*client.Object, error) {

	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	var (
		storageClassName = instance.Spec.RoleConfig.StorageClass
		storageSize      = instance.Spec.RoleConfig.StorageSize
		volumeMode       = corev1.PersistentVolumeFilesystem
	)
	if roleGroup != nil {
		if rgStorageClass := roleGroup.StorageClass; rgStorageClass != nil {
			storageClassName = rgStorageClass
		}
		if rgStorageSize := roleGroup.StorageSize; rgStorageSize != "" {
			storageSize = rgStorageSize
		}
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetNameWithSuffix(groupName),
			Namespace: instance.Namespace,
			Labels:    mergeLabels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: storageClassName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
			VolumeMode: &volumeMode,
		},
	}

	err := ctrl.SetControllerReference(instance, pvc, scheme)
	if err != nil {
		r.Log.Error(err, "Failed to set controller reference for pvc")
		return nil, errors.Wrap(err, "Failed to set controller reference for pvc")
	}
	pvcEx := client.Object(pvc)
	return &pvcEx, nil

}

func (r *HiveMetastoreReconciler) reconcileService(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	err := r.createOrUpdateResource(ctx, instance, r.extractServiceForRoleGroup)
	if err != nil {
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) extractServiceForRoleGroup(instance *stackv1alpha1.HiveMetastore, _ context.Context,
	groupName string, roleGroup *stackv1alpha1.RoleConfigSpec, scheme *runtime.Scheme) (*client.Object, error) {
	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	port, serviceType, annotations := r.getServiceInfo(instance.Spec.Service, roleGroup)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        groupName,
			Namespace:   instance.Namespace,
			Labels:      mergeLabels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     port,
					Name:     "http",
					Protocol: "TCP",
				},
			},
			Selector: mergeLabels,
			Type:     serviceType,
		},
	}

	err := ctrl.SetControllerReference(instance, svc, scheme)
	if err != nil {
		r.Log.Error(err, "Failed to set controller reference for service")
		return nil, errors.Wrap(err, "Failed to set controller reference for service")
	}
	svcEx := client.Object(svc)
	return &svcEx, nil
}

func (r *HiveMetastoreReconciler) reconcileConfigMap(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	err := r.createOrUpdateResource(ctx, instance, r.extractConfigMapForRoleGroup)
	if err != nil {
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) fetchS3(s3Bucket *opgo.S3Bucket, ctx context.Context,
	instance *stackv1alpha1.HiveMetastore) (*opgo.S3Connection, error) {
	// 1 - fetch exists s3-bucket by reference
	cliObj := client.Object(s3Bucket)
	if err := r.fetchResource(ctx, cliObj, instance); err != nil {
		return nil, err
	}
	//2 - fetch exist s3-collection by pre-fetch bucketName
	s3Collection := &opgo.S3Connection{
		ObjectMeta: metav1.ObjectMeta{Name: s3Bucket.Spec.Reference},
	}
	collCliObj := client.Object(s3Collection)
	if err := r.fetchResource(ctx, collCliObj, instance); err != nil {
		return nil, err
	}
	return s3Collection, nil
}

func (r *HiveMetastoreReconciler) fetchDb(database *opgo.Database, ctx context.Context,
	instance *stackv1alpha1.HiveMetastore) (*opgo.DatabaseConnection, error) {
	// 1 - fetch exists Database by reference
	cliObj := client.Object(database)
	if err := r.fetchResource(ctx, cliObj, instance); err != nil {
		return nil, err
	}
	//2 - fetch exist database collection by pre-fetch 'database.spec.name'
	dbCollection := &opgo.DatabaseConnection{
		ObjectMeta: metav1.ObjectMeta{Name: database.Spec.DatabaseName},
	}
	collCliObj := client.Object(dbCollection)
	if err := r.fetchResource(ctx, collCliObj, instance); err != nil {
		return nil, err
	}
	return dbCollection, nil
}

func (r *HiveMetastoreReconciler) extractConfigMapForRoleGroup(instance *stackv1alpha1.HiveMetastore, ctx context.Context,
	roleGroupName string, roleGroup *stackv1alpha1.RoleConfigSpec, schema *runtime.Scheme) (*client.Object, error) {
	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	var s3Cfg = instance.Spec.RoleConfig.Config.S3
	if roleGroup != nil {
		if rgCfg := roleGroup.Config; rgCfg != nil {
			s3Cfg = rgCfg.S3
		}
	}
	if s3Cfg == nil {
		return nil, nil
	}
	s3Bucket := &opgo.S3Bucket{
		ObjectMeta: metav1.ObjectMeta{Name: s3Cfg.Reference},
	}
	s3Collection, err := r.fetchS3(s3Bucket, ctx, instance)
	if err != nil {
		return nil, err
	}

	var xmlFileTmp = "" +
		"	 <?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n" +
		"    <?xml-stylesheet type=\"text/xsl\" href=\"configuration.xsl\"?>\n" +
		"    <configuration>\n" +
		"        <property>\n" +
		"            <name>fs.s3a.connection.maximum</name>\n" +
		"            <value>%d</value>\n" +
		"            <description>Controls the maximum number of simultaneous connections to S3.</description>\n" +
		"        </property>\n" +
		"        <property>\n" +
		"            <name>fs.s3a.connection.ssl.enabled</name>\n" +
		"            <value>%t</value>\n" +
		"            <description>Enables or disables SSL connections to S3.</description>\n" +
		"        </property>\n" +
		"        <property>\n" +
		"            <name>fs.s3a.endpoint</name>\n" +
		"            <value>%s</value>\n" +
		"            <description>AWS S3 endpoint to connect to. An up-to-date list is\n" +
		"                provided in the AWS Documentation: regions and endpoints. Without this\n" +
		"                property, the standard region (s3.amazonaws.com) is assumed.\n" +
		"            </description>\n" +
		"        </property>\n" +
		"        <property>\n" +
		"            <name>fs.s3a.path.style.access</name>\n" +
		"            <value>%t</value>\n" +
		"            <description>Enable S3 path style access ie disabling the default virtual hosting behaviour.\n" +
		"                Useful for S3A-compliant storage providers as it removes the need to set up DNS for\n" +
		"                virtual hosting.\n" +
		"            </description>\n" +
		"        </property>\n" +
		"        <property>\n" +
		"            <name>fs.s3a.impl</name>\n" +
		"            <value>org.apache.hadoop.fs.s3a.S3AFileSystem</value>\n" +
		"            <description>The implementation class of the S3A Filesystem</description>\n" +
		"        </property>\n" +
		"        <property>\n" +
		"            <name>fs.AbstractFileSystem.s3a.impl</name>\n" +
		"            <value>org.apache.hadoop.fs.s3a.S3A</value>\n" +
		"            <description>The implementation class of the S3A AbstractFileSystem.</description>\n" +
		"        </property>\n" +
		"    </configuration>"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetNameWithSuffix(roleGroupName),
			Namespace: instance.Namespace,
			Labels:    mergeLabels,
		},
		Data: map[string]string{
			"hive-site.xml": fmt.Sprintf(xmlFileTmp, s3Cfg.MaxConnect, s3Collection.Spec.SSL, s3Collection.Spec.Endpoint,
				s3Cfg.PathStyleAccess),
		},
	}

	if err := ctrl.SetControllerReference(instance, cm, schema); err != nil {
		r.Log.Error(err, "Failed to set controller reference for configmap")
		return nil, err
	}
	cmEx := client.Object(cm)
	return &cmEx, nil
}

func (r *HiveMetastoreReconciler) reconcileDeployment(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	err := r.createOrUpdateResource(ctx, instance, r.extractDeploymentForRoleGroup)
	if err != nil {
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) fetchDbForRoleGroup(instance *stackv1alpha1.HiveMetastore, ctx context.Context,
	roleGroup *stackv1alpha1.RoleConfigSpec) (*opgo.Database, *opgo.DatabaseConnection, error) {
	var db *stackv1alpha1.DatabaseSpec
	if roleGroup != nil && roleGroup.Config != nil {
		if rgDatabase := roleGroup.Config.Database; rgDatabase != nil {
			db = rgDatabase
		} else {
			db = instance.Spec.RoleConfig.Config.Database
		}
	}
	dbrsc := &opgo.Database{
		ObjectMeta: metav1.ObjectMeta{Name: db.Reference},
	}
	dbCollection, err := r.fetchDb(dbrsc, ctx, instance)
	if err != nil {
		return nil, nil, err
	}
	return dbrsc, dbCollection, nil
}

func (r *HiveMetastoreReconciler) extractDeploymentForRoleGroup(instance *stackv1alpha1.HiveMetastore, ctx context.Context,
	roleGroupName string, roleGroup *stackv1alpha1.RoleConfigSpec, schema *runtime.Scheme) (*client.Object, error) {
	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	var (
		image       stackv1alpha1.ImageSpec
		securityCtx *corev1.PodSecurityContext
	)
	if roleGroup != nil {
		if rgImage := roleGroup.Image; rgImage != nil {
			image = *rgImage
		} else {
			image = instance.Spec.Image
		}
		if rgSecurityCtx := roleGroup.SecurityContext; rgSecurityCtx != nil {
			securityCtx = rgSecurityCtx
		} else {
			securityCtx = instance.Spec.SecurityContext
		}
	}

	hiveConfVolumeNameFunc := func() string { return instance.GetNameWithSuffix(roleGroupName + "-conf") }
	hiveDataVolumeNameFunc := func() string { return instance.GetNameWithSuffix(roleGroupName + "-data") }
	var dbCollection *opgo.DatabaseConnection
	var err error
	if _, dbCollection, err = r.fetchDbForRoleGroup(instance, ctx, roleGroup); err != nil {
		return nil, errors.Wrap(err, "Fetch db collection for roleGroup err")
	}
	pg := dbCollection.Spec.Provider.Postgres
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetNameWithSuffix(roleGroupName),
			Namespace: instance.Namespace,
			Labels:    mergeLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &roleGroup.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: mergeLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: mergeLabels,
				},
				Spec: corev1.PodSpec{
					SecurityContext: securityCtx,
					Containers: []corev1.Container{
						{
							Name:            instance.Name,
							Image:           image.Repository + ":" + image.Tag,
							ImagePullPolicy: image.PullPolicy,
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: instance.GetNameWithSuffix(roleGroupName),
										},
									},
								},
							},
							Resources: *roleGroup.Config.Resources,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9083,
									Name:          "http",
									Protocol:      "TCP",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      hiveConfVolumeNameFunc(),
									MountPath: "/opt/hive/conf/hive-site.xml",
									SubPath:   "hive-site.xml",
								},
								{
									Name:      hiveDataVolumeNameFunc(),
									MountPath: "/opt/hive/data",
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:            instance.GetNameWithSuffix(roleGroupName + "-init"),
							Image:           "quay.io/plutoso/alpine-tools:latest",
							ImagePullPolicy: image.PullPolicy,
							Args: []string{
								"sh",
								"-c",
								fmt.Sprintf("telnet %s %d", pg.Host, pg.Port),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: hiveConfVolumeNameFunc(),
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: instance.GetNameWithSuffix(roleGroupName),
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "hive-site.xml",
											Path: "hive-site.xml",
										},
									},
								},
							},
						},
						{
							Name: hiveDataVolumeNameFunc(),
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: instance.GetNameWithSuffix(roleGroupName),
								},
							},
						},
					},
				},
			},
		},
	}

	CreateScheduler(instance, dep, roleGroup)

	err = ctrl.SetControllerReference(instance, dep, schema)
	if err != nil {
		r.Log.Error(err, "Failed to set controller reference for deployment")
		return nil, err
	}
	depEx := client.Object(dep)
	return &depEx, nil
}

func (r *HiveMetastoreReconciler) reconcileSecret(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	err := r.createOrUpdateResource(ctx, instance, r.extractSecretForRoleGroup)
	if err != nil {
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) extractSecretForRoleGroup(instance *stackv1alpha1.HiveMetastore, ctx context.Context,
	roleGroupName string, roleGroup *stackv1alpha1.RoleConfigSpec, schema *runtime.Scheme) (*client.Object, error) {
	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	// https://cwiki.apache.org/confluence/display/Hive/Setting+Up+Hive+with+Docker
	var (
		s3 *stackv1alpha1.S3Spec
		db *stackv1alpha1.DatabaseSpec
	)
	if roleCfg := instance.Spec.RoleConfig; roleCfg != nil {
		s3 = roleCfg.Config.S3
	}
	if roleGroup != nil && roleGroup.Config != nil {
		if rgS3 := roleGroup.Config.S3; rgS3 != nil {
			s3 = rgS3
		} else if roleCfg := instance.Spec.RoleConfig; roleCfg != nil {
			s3 = roleCfg.Config.S3
		}
		if rgPg := roleGroup.Config.Database; rgPg != nil {
			db = rgPg
		} else {
			db = instance.Spec.RoleConfig.Config.Database
		}
	}

	var (
		s3Collection *opgo.S3Connection
		dbrsc        *opgo.Database
		dbCollection *opgo.DatabaseConnection
		err          error
	)
	if s3 != nil {
		s3Bucket := &opgo.S3Bucket{
			ObjectMeta: metav1.ObjectMeta{Name: s3.Reference},
		}
		s3Collection, err = r.fetchS3(s3Bucket, ctx, instance)
		if err != nil {
			return nil, err
		}
	}
	if db != nil {
		dbrsc = &opgo.Database{
			ObjectMeta: metav1.ObjectMeta{Name: db.Reference},
		}
		dbCollection, err = r.fetchDb(dbrsc, ctx, instance)
		if err != nil {
			return nil, err
		}
	}
	if dbrsc == nil {
		return nil, errors.New(fmt.Sprintf("Role-group: %s should set a database config in role-group-config, "+
			"role-config or cluster-config", roleGroupName))
	}

	data := make(map[string][]byte)
	if err = r.makeDatabaseData(dbrsc, dbCollection, ctx, instance, &data); err != nil {
		return nil, err
	}
	r.makeS3Data(s3Collection, &data)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetNameWithSuffix(roleGroupName),
			Namespace: instance.Namespace,
			Labels:    mergeLabels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	if err := ctrl.SetControllerReference(instance, secret, schema); err != nil {
		r.Log.Error(err, "Failed to set controller reference for secret")
		return nil, err
	}
	secretEx := client.Object(secret)
	return &secretEx, nil
}

func (r *HiveMetastoreReconciler) makeDatabaseData(dbrsc *opgo.Database, dbCollection *opgo.DatabaseConnection,
	ctx context.Context, instance *stackv1alpha1.HiveMetastore, data *map[string][]byte) error {
	fetchUsrPassFunc := func() ([]string, error) {
		dbCredential := dbrsc.Spec.Credential
		if existSecretRef := dbCredential.ExistSecret; existSecretRef != "" {
			existCredentialSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: existSecretRef}}
			cliObj := client.Object(existCredentialSecret)
			if err := r.fetchResource(ctx, cliObj, instance); err != nil {
				return nil, err
			}
			credentialByteData := &existCredentialSecret.Data
			var username, passwd string
			if usr, err := DecodeBase64Data(credentialByteData, "username"); err != nil {
				return nil, err
			} else {
				username = *usr
			}
			if pass, err := DecodeBase64Data(credentialByteData, "password"); err != nil {
				return nil, err
			} else {
				passwd = *pass
			}
			return []string{username, passwd}, nil
		}
		return []string{dbCredential.Username, dbCredential.Password}, nil
	}

	var usrPass []string
	var err error
	if usrPass, err = fetchUsrPassFunc(); err != nil {
		return errors.Wrap(err, "Fetch username and password error")
	}
	serviceOptsTemp := "-Xmx1G -Djavax.jdo.option.ConnectionDriverName=org.postgresql.Driver\n" +
		"              -Djavax.jdo.option.ConnectionURL=jdbc:postgresql://%s:%d/%s\n" +
		"              -Djavax.jdo.option.ConnectionUserName=%s\n" +
		"              -Djavax.jdo.option.ConnectionPassword=%s"
	pg := dbCollection.Spec.Provider.Postgres
	serviceOpts := fmt.Sprintf(serviceOptsTemp, pg.Host, pg.Port, dbCollection.Name, usrPass[0], usrPass[1])
	(*data)["DB_DRIVER"] = []byte(pg.Driver)
	(*data)["SERVICE_NAME"] = []byte("metastore -hiveconf hive.root.logger=INFO,console")
	(*data)["SERVICE_OPTS"] = []byte(serviceOpts)
	return nil
}

func (r *HiveMetastoreReconciler) makeS3Data(s3Collection *opgo.S3Connection, data *map[string][]byte) {
	if s3Collection != nil {
		s3Credential := s3Collection.Spec.S3Credential
		(*data)["AWS_ACCESS_KEY_ID"] = []byte(s3Credential.AccessKey)
		(*data)["AWS_SECRET_ACCESS_KEY"] = []byte(s3Credential.SecretKey)
		(*data)["AWS_DEFAULT_REGION"] = []byte(s3Collection.Spec.Region)
	}
}
