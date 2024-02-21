package controller

import (
	"context"
	stackv1alpha1 "github.com/zncdata-labs/hive-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type DeploymentReconciler struct {
	*BaseRoleGroupResourceReconciler
}

func NewReconcileDeployment(
	client client.Client,
	schema *runtime.Scheme,
	cr *stackv1alpha1.HiveMetastore,
	roleName string,
	roleGroupName string,
	roleGroup *stackv1alpha1.RoleGroupSpec,
) *DeploymentReconciler {

	return &DeploymentReconciler{
		&BaseRoleGroupResourceReconciler{
			client:        client,
			scheme:        schema,
			cr:            cr,
			roleName:      roleName,
			roleGroupName: roleGroupName,
			roleGroup:     roleGroup,
		},
	}
}

func (r *DeploymentReconciler) RoleGroupConfig() *stackv1alpha1.ConfigSpec {
	return r.roleGroup.Config
}

func (r *DeploymentReconciler) Reconcile(ctx context.Context) (ctrl.Result, error) {
	log.Info("Reconciling Deployment")

	if res, err := r.apply(ctx); err != nil {
		return ctrl.Result{}, err
	} else if res.RequeueAfter > 0 {
		return res, nil
	}

	// Check if the pods are satisfied
	satisfied, err := r.CheckPodsSatisfied(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if satisfied {
		err = r.updateStatus(
			metav1.ConditionTrue,
			"DeploymentSatisfied",
			"Deployment is satisfied",
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	err = r.updateStatus(
		metav1.ConditionFalse,
		"DeploymentNotSatisfied",
		"Deployment is not satisfied",
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

func (r *DeploymentReconciler) hiveSiteMountName() string {
	return "hive-site"
}

func (r *DeploymentReconciler) log4jMountName() string {
	return "hive-log4j2"
}

func (r *DeploymentReconciler) metastoreConfigMapName() string {
	return MetastoreLog4jConfigMapName(r.cr, r.roleGroupName)
}

func (r *DeploymentReconciler) hiveDataMountName() string {
	return "warehouse"
}

// volumes returns the volumes for the deployment
func (r *DeploymentReconciler) volumes() []corev1.Volume {
	vs := []corev1.Volume{
		{
			Name: r.hiveSiteMountName(),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: HiveSiteSecretName(r.cr, r.roleGroupName),
					Items: []corev1.KeyToPath{
						{
							Key:  HiveSiteName,
							Path: HiveSiteName,
						},
					},
				},
			},
		},
	}

	if r.EnabledDataPVC() {
		vs = append(vs, corev1.Volume{
			Name: r.hiveDataMountName(),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: HiveDataPVCName(r.cr, r.roleGroupName),
				},
			},
		})
	}

	if r.EnabledLogging() {
		vs = append(vs, corev1.Volume{
			Name: r.log4jMountName(),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.metastoreConfigMapName(),
					},
					Items: []corev1.KeyToPath{
						{
							Key:  HiveMetastoreLog4jName,
							Path: HiveMetastoreLog4jName,
						},
					},
				},
			},
		})
	}

	return vs
}

func (r *DeploymentReconciler) make() (*appsv1.Deployment, error) {

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Name(),
			Namespace: r.NameSpace(),
			Labels:    r.GetLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r.roleGroup.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: r.GetLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: r.GetLabels(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						r.metastoreContainer(),
					},
					Volumes: r.volumes(),
				},
			},
		},
	}

	if r.RoleGroupConfig() != nil {
		if r.RoleGroupConfig().Affinity != nil {
			dep.Spec.Template.Spec.Affinity = r.RoleGroupConfig().Affinity
		}

		if r.RoleGroupConfig().Tolerations != nil {
			dep.Spec.Template.Spec.Tolerations = r.RoleGroupConfig().Tolerations
		}

		if r.RoleGroupConfig().NodeSelector != nil {
			dep.Spec.Template.Spec.NodeSelector = r.RoleGroupConfig().NodeSelector
		}
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(r.cr, dep, r.scheme); err != nil {
		return nil, err
	}

	return dep, nil
}

func (r *DeploymentReconciler) Image() *stackv1alpha1.ImageSpec {
	if r.cr.Spec.Image == nil {
		return &stackv1alpha1.ImageSpec{
			Repository: stackv1alpha1.ImageRepository,
			Tag:        stackv1alpha1.ImageTag,
			PullPolicy: stackv1alpha1.ImagePullPolicy,
		}
	}
	return r.cr.Spec.Image
}

func (r *DeploymentReconciler) EnabledDataPVC() bool {
	return r.RoleGroupConfig() != nil &&
		r.RoleGroupConfig().Resources != nil &&
		r.RoleGroupConfig().Resources.Storage != nil

}

func (r *DeploymentReconciler) EnabledEnvSecret() bool {
	return r.cr.Spec.ClusterConfig != nil &&
		(r.cr.Spec.ClusterConfig.S3Bucket != nil || r.cr.Spec.ClusterConfig.Database != nil)
}

func (r *DeploymentReconciler) EnabledLogging() bool {
	return r.RoleGroupConfig() != nil &&
		r.RoleGroupConfig().Logging != nil &&
		r.RoleGroupConfig().Logging.Metastore != nil
}

// volumeMounts returns the volume mounts for the container
func (r *DeploymentReconciler) volumeMounts() []corev1.VolumeMount {
	vms := []corev1.VolumeMount{
		{
			Name:      r.hiveSiteMountName(),
			MountPath: "/opt/hive/conf/hive-site.xml",
			SubPath:   "hive-site.xml",
		},
	}

	if r.EnabledDataPVC() {
		vms = append(vms, corev1.VolumeMount{
			Name:      r.hiveDataMountName(),
			MountPath: stackv1alpha1.WarehouseDir,
		})
	}

	if r.EnabledLogging() {
		vms = append(vms, corev1.VolumeMount{
			Name:      r.log4jMountName(),
			MountPath: "/opt/hive/conf/" + HiveMetastoreLog4jName,
			SubPath:   HiveMetastoreLog4jName,
		})
	}
	return vms
}

func (r *DeploymentReconciler) containerFromEnvSecret() []corev1.EnvFromSource {
	envs := make([]corev1.EnvFromSource, 0)
	if r.EnabledEnvSecret() {
		envs = append(
			envs,
			corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: HiveEnvSecretName(r.cr),
					},
				},
			},
		)
	}
	return envs
}

func (r *DeploymentReconciler) overrideEnv() []corev1.EnvVar {
	envs := make([]corev1.EnvVar, 0)
	if r.roleGroup.EnvOverrides != nil {
		for key, value := range r.roleGroup.EnvOverrides {
			if key != "" {
				envs = append(envs, corev1.EnvVar{
					Name:  key,
					Value: value,
				})
			}
		}
	}
	return envs
}

func (r *DeploymentReconciler) metastoreContainer() corev1.Container {

	image := r.Image()

	obj := corev1.Container{
		Name:            r.Name(),
		Image:           image.Repository + ":" + image.Tag,
		ImagePullPolicy: image.PullPolicy,
		Env: []corev1.EnvVar{
			{
				Name:  "SERVICE_NAME",
				Value: "metastore",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 9083,
				Protocol:      corev1.ProtocolTCP,
				Name:          "tcp",
			},
		},
		VolumeMounts: r.volumeMounts(),
	}

	obj.EnvFrom = r.containerFromEnvSecret()

	obj.Env = append(obj.Env, r.overrideEnv()...)

	return obj
}

func (r *DeploymentReconciler) apply(ctx context.Context) (ctrl.Result, error) {
	dep, err := r.make()

	if err != nil {
		return ctrl.Result{}, err
	}

	mutant, err := CreateOrUpdate(ctx, r.client, dep)
	if err != nil {
		return ctrl.Result{}, err
	}

	if mutant {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	return ctrl.Result{}, nil
}

func (r *DeploymentReconciler) CheckPodsSatisfied(ctx context.Context) (bool, error) {
	pods := corev1.PodList{}
	podListOptions := []client.ListOption{
		client.InNamespace(r.NameSpace()),
		client.MatchingLabels(r.GetLabels()),
	}
	err := r.client.List(ctx, &pods, podListOptions...)
	if err != nil {
		return false, err
	}

	return len(pods.Items) == int(r.roleGroup.Replicas), nil
}

func (r *DeploymentReconciler) updateStatus(status metav1.ConditionStatus, reason string, message string) error {
	apimeta.SetStatusCondition(&r.cr.Status.Conditions, metav1.Condition{
		Type:               stackv1alpha1.ConditionTypeClusterAvailable,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: r.cr.GetGeneration(),
	})

	return r.client.Status().Update(context.Background(), r.cr)
}
