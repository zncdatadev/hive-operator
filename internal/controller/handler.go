package controller

import (
	"context"
	"fmt"
	stackv1alpha1 "github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
	"github.com/zncdata-labs/operator-go/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	roleGroupExtractor func(*stackv1alpha1.HiveMetastore, string, *stackv1alpha1.RoleGroupSpec,
		*runtime.Scheme) (*client.Object, error)) error {
	resources, err := r.extractResources(instance, roleGroupExtractor)
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

func (r *HiveMetastoreReconciler) getRoleGroupLabels(config *stackv1alpha1.ConfigRoleGroupSpec) map[string]string {
	additionalLabels := make(map[string]string)
	if configLabels := config.MatchLabels; configLabels != nil {
		for k, v := range config.MatchLabels {
			additionalLabels[k] = v
		}
	}
	return additionalLabels
}

func (r *HiveMetastoreReconciler) mergeLabels(mergeLabels *Map, instanceLabels map[string]string,
	roleGroup *stackv1alpha1.RoleGroupSpec) {
	mergeLabels.MapMerge(instanceLabels, true)
	mergeLabels.MapMerge(r.getRoleGroupLabels(roleGroup.Config), true)
}

func (r *HiveMetastoreReconciler) getServiceInfo(instanceSvc *stackv1alpha1.ServiceSpec,
	roleGroup *stackv1alpha1.RoleGroupSpec) (int32, corev1.ServiceType, map[string]string) {
	var targetSvc *stackv1alpha1.ServiceSpec
	if roleGroup != nil && roleGroup.Config != nil && roleGroup.Config.Service != nil {
		targetSvc = roleGroup.Config.Service
	}
	if targetSvc == nil {
		targetSvc = instanceSvc
	}
	return targetSvc.Port, targetSvc.Type, targetSvc.Annotations
}

func (r *HiveMetastoreReconciler) extractResources(instance *stackv1alpha1.HiveMetastore,
	roleGroupExtractor func(*stackv1alpha1.HiveMetastore, string, *stackv1alpha1.RoleGroupSpec,
		*runtime.Scheme) (*client.Object, error)) ([]*client.Object, error) {
	var resources []*client.Object
	if instance.Spec.RoleGroups != nil {
		for roleGroupName, roleGroup := range instance.Spec.RoleGroups {
			rsc, err := roleGroupExtractor(instance, roleGroupName, roleGroup, r.Scheme)
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

func (r *HiveMetastoreReconciler) extractPvcForRoleGroup(instance *stackv1alpha1.HiveMetastore, groupName string,
	roleGroup *stackv1alpha1.RoleGroupSpec, scheme *runtime.Scheme) (*client.Object, error) {

	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	var (
		storageClassName *string
		accessMode       []corev1.PersistentVolumeAccessMode
		storageSize      string
		volumeMode       *corev1.PersistentVolumeMode
	)
	if roleGroup != nil && roleGroup.Config != nil {
		if rgPersistence := roleGroup.Config.Persistence; rgPersistence != nil {
			if !rgPersistence.Enable {
				return nil, nil
			}
			storageClassName = rgPersistence.StorageClass
			accessMode = rgPersistence.AccessModes
			storageSize = rgPersistence.Size
			volumeMode = rgPersistence.VolumeMode
		} else {
			if instancePersistence := instance.Spec.Persistence; !instancePersistence.Enable {
				storageClassName = instancePersistence.StorageClass
				accessMode = instancePersistence.AccessModes
				storageSize = instancePersistence.Size
				volumeMode = instancePersistence.VolumeMode
			}
			return nil, nil
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
			AccessModes:      accessMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
			VolumeMode: volumeMode,
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

func (r *HiveMetastoreReconciler) extractServiceForRoleGroup(instance *stackv1alpha1.HiveMetastore, groupName string,
	roleGroup *stackv1alpha1.RoleGroupSpec, scheme *runtime.Scheme) (*client.Object, error) {
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

func (r *HiveMetastoreReconciler) extractConfigMapForRoleGroup(instance *stackv1alpha1.HiveMetastore, roleGroupName string,
	roleGroup *stackv1alpha1.RoleGroupSpec, schema *runtime.Scheme) (*client.Object, error) {
	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	var s3Cfg *stackv1alpha1.S3Spec
	if roleGroup != nil {
		if rgCfg := roleGroup.Config; rgCfg != nil {
			s3Cfg = rgCfg.S3
		}
	}
	if s3Cfg == nil {
		s3Cfg = instance.Spec.RoleConfig.S3
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
			"hive-site.xml": fmt.Sprintf(xmlFileTmp, s3Cfg.MaxConnect, s3Cfg.EnableSSL, s3Cfg.Endpoint,
				s3Cfg.PathStyleAccess),
		},
	}
	err := ctrl.SetControllerReference(instance, cm, schema)
	if err != nil {
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

func (r *HiveMetastoreReconciler) extractDeploymentForRoleGroup(instance *stackv1alpha1.HiveMetastore, roleGroupName string,
	roleGroup *stackv1alpha1.RoleGroupSpec, schema *runtime.Scheme) (*client.Object, error) {
	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	var (
		image       stackv1alpha1.ImageSpec
		securityCtx *corev1.PodSecurityContext
	)
	if roleGroup != nil && roleGroup.Config != nil {
		if rgImage := roleGroup.Config.Image; rgImage != nil {
			image = *rgImage
		} else {
			image = instance.Spec.Image
		}
		if rgSecurityCtx := roleGroup.Config.SecurityContext; rgSecurityCtx != nil {
			securityCtx = rgSecurityCtx
		} else {
			securityCtx = instance.Spec.SecurityContext
		}
	}

	hiveConfVolumeNameFunc := func() string { return instance.GetNameWithSuffix(roleGroupName + "-conf") }
	hiveDataVolumeNameFunc := func() string { return instance.GetNameWithSuffix(roleGroupName + "-data") }

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
							Name:            instance.GetNameWithSuffix("init"),
							Image:           "quay.io/plutoso/alpine-tools:latest",
							ImagePullPolicy: image.PullPolicy,
							Args: []string{
								"sh",
								"-c",
								"telnet" + " " + instance.Spec.RoleConfig.PostgresSecret.Host + " " + instance.Spec.RoleConfig.PostgresSecret.Port,
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
	err := ctrl.SetControllerReference(instance, dep, schema)
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

func (r *HiveMetastoreReconciler) extractSecretForRoleGroup(instance *stackv1alpha1.HiveMetastore, roleGroupName string,
	roleGroup *stackv1alpha1.RoleGroupSpec, schema *runtime.Scheme) (*client.Object, error) {
	var mergeLabels Map
	r.mergeLabels(&mergeLabels, instance.GetLabels(), roleGroup)

	// https://cwiki.apache.org/confluence/display/Hive/Setting+Up+Hive+with+Docker
	var (
		awsKeyId  string
		awsSecret string
		awsRegion string

		serviceOpts string
	)
	if roleGroup != nil && roleGroup.Config != nil {
		if rgS3 := roleGroup.Config.S3; rgS3 != nil {
			awsKeyId = rgS3.AccessKey
			awsSecret = rgS3.SecretKey
			awsRegion = rgS3.Region
		} else if roleCfg := instance.Spec.RoleConfig; roleCfg != nil {
			awsKeyId = roleCfg.S3.AccessKey
			awsSecret = roleCfg.S3.SecretKey
			awsRegion = roleCfg.S3.Region
		}
		var postgres *stackv1alpha1.PostgresSecretSpec
		if rgPg := roleGroup.Config.PostgresSecret; rgPg != nil {
			postgres = rgPg
		} else {
			postgres = instance.Spec.RoleConfig.PostgresSecret
		}
		serviceOptsTemp := "-Xmx1G -Djavax.jdo.option.ConnectionDriverName=org.postgresql.Driver\n" +
			"              -Djavax.jdo.option.ConnectionURL=jdbc:postgresql://%s:%s/%s\n" +
			"              -Djavax.jdo.option.ConnectionUserName=%s\n" +
			"              -Djavax.jdo.option.ConnectionPassword=%s"
		serviceOpts = fmt.Sprintf(serviceOptsTemp, postgres.Host, postgres.Port, postgres.DataBase, postgres.UserName,
			postgres.Password)
	}

	data := make(map[string][]byte)
	data["DB_DRIVER"] = []byte("postgres")
	data["AWS_ACCESS_KEY_ID"] = []byte(awsKeyId)
	data["AWS_SECRET_ACCESS_KEY"] = []byte(awsSecret)
	data["AWS_DEFAULT_REGION"] = []byte(awsRegion)
	data["SERVICE_NAME"] = []byte("metastore -hiveconf hive.root.logger=INFO,console")
	data["SERVICE_OPTS"] = []byte(serviceOpts)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetNameWithSuffix(roleGroupName),
			Namespace: instance.Namespace,
			Labels:    mergeLabels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	secretEx := client.Object(secret)
	return &secretEx, nil

}
