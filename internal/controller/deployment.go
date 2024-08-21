package controller

import (
	"context"
	"maps"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeploymentReconciler struct {
	*BaseRoleGroupResourceReconciler
}

func NewReconcileDeployment(
	client client.Client,
	schema *runtime.Scheme,
	cr *hivev1alpha1.HiveMetastore,
	roleName string,
	roleGroupName string,
	roleGroup *hivev1alpha1.RoleGroupSpec,
	stop bool,
) *DeploymentReconciler {

	return &DeploymentReconciler{
		&BaseRoleGroupResourceReconciler{
			client:        client,
			scheme:        schema,
			cr:            cr,
			roleName:      roleName,
			roleGroupName: roleGroupName,
			roleGroup:     roleGroup,
			stop:          stop,
		},
	}
}

func (r *DeploymentReconciler) RoleGroupConfig() *hivev1alpha1.ConfigSpec {
	return r.roleGroup.Config
}

func (r *DeploymentReconciler) getTerminationGracePeriodSeconds() *int64 {
	if r.roleGroup.Config.GracefulShutdownTimeout != nil {
		if tiime, err := time.ParseDuration(*r.roleGroup.Config.GracefulShutdownTimeout); err == nil {
			seconds := int64(tiime.Seconds())
			return &seconds
		}
	}
	return nil
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

func (r *DeploymentReconciler) getPodTemplate() corev1.PodTemplateSpec {
	copyedPodTemplate := r.roleGroup.PodOverride.DeepCopy()
	podTemplate := corev1.PodTemplateSpec{}

	if copyedPodTemplate != nil {
		podTemplate = *copyedPodTemplate
	}

	if podTemplate.ObjectMeta.Labels == nil {
		podTemplate.ObjectMeta.Labels = make(map[string]string)
	}

	maps.Copy(podTemplate.ObjectMeta.Labels, r.GetLabels())

	podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, r.createContainer())

	podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes, r.createVolumes()...)

	seconds := r.getTerminationGracePeriodSeconds()
	if r.roleGroup.Config.GracefulShutdownTimeout != nil {
		podTemplate.Spec.TerminationGracePeriodSeconds = seconds
	}

	return podTemplate
}

func (r *DeploymentReconciler) metastoreConfigMapName() string {
	return MetastoreLog4jConfigMapName(r.cr, r.roleGroupName)
}

// volumes returns the volumes for the deployment
func (r *DeploymentReconciler) createVolumes() []corev1.Volume {
	vs := []corev1.Volume{
		{
			// config dir, writeable
			Name: hivev1alpha1.KubeDataConfigDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: func() *resource.Quantity { q := resource.MustParse("10Mi"); return &q }(),
				},
			},
		},
		{
			// log dir
			Name: hivev1alpha1.KubeDataLogDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: func() *resource.Quantity { q := resource.MustParse("30Mi"); return &q }(),
				},
			},
		},
		{
			// config mount dir, read only
			Name: hivev1alpha1.KubeDataConfigMountDirName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: func() *int32 { i := int32(0755); return &i }(),
					LocalObjectReference: corev1.LocalObjectReference{
						Name: HiveMetaStoreConfigMapName(r.cr, r.roleGroupName),
					},
				},
			},
		},
	}

	// log4j2 config dir, read only
	vs = append(vs, corev1.Volume{
		Name: hivev1alpha1.KubeDataLogConfigMountDirName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: r.metastoreConfigMapName(),
				},
			},
		},
	})
	//kerberos
	if IsKerberosEnabled(r.cr.Spec.ClusterConfig) {
		secretClass := r.cr.Spec.ClusterConfig.Authentication.Kerberos.SecretClass
		vs = append(vs, KrbVolume(secretClass, r.cr.Name))
	}
	// secret
	if IsS3Enable(r.cr.Spec.ClusterConfig) {
		secretClass := r.cr.Spec.ClusterConfig.S3Bucket.SecretClass
		vs = append(vs, S3Volume(secretClass))
	}
	return vs
}

func (r *DeploymentReconciler) command() []string {
	return []string{"/bin/bash", "-x", "-euo", "pipefail", "-c"}
}

func (r *DeploymentReconciler) Args() []string {
	var args []string
	// copy hive config from mount to writeable folder
	args = append(args, `echo copying {{.ConfigMountDir}} to {{.ConfigDir}}
cp -RL {{.ConfigMountDir}}/*.xml {{.ConfigDir}}/
	`)
	// copy hive log4j config from mount to writeable folder
	args = append(args, `echo copying {{.LogMountDir}}/{{.HiveLog4j2Properties}} to {{.ConfigDir}}/{{.HiveLog4j2Properties}}
cp -RL {{.LogMountDir}}/* {{.ConfigDir}}/
	`)
	// kerberos
	args = append(args, `{{ if .kerberosEnabled }}
{{- .kerberosScript -}}
{{- end }}
	`)
	// s3
	args = append(args, `{{ if .s3Enabled }}
{{- .s3Script -}}
{{- end }}`)
	//common bash trap functions
	args = append(args, COMMON_BASH_TRAP_FUNCTIONS)
	//remove vector shutdown file command
	args = append(args, RemoveVectorShutdownFileCommand("{{.LogDir}}"))
	//hive executer
	args = append(args, `prepare_signal_handlers
DB_TYPE="${DB_DRIVER:-derby}"
echo "hive db type is $DB_TYPE"
bin/start-metastore --config {{.ConfigDir}} --db-type $DB_TYPE --hive-bin-dir bin &
wait_for_termination $!
	`)
	//create vector shutdown file command
	args = append(args, CreateVectorShutdownFileCommand("{{.LogDir}}"))
	tmplate := strings.Join(args, "\n")

	var data = make(map[string]interface{})
	krbTemplateData := CreateKrbScriptData(r.cr.Spec.ClusterConfig)
	data["ConfigDir"] = hivev1alpha1.KubeDataConfigDir
	data["ConfigMountDir"] = hivev1alpha1.KubeDataConfigMountDir
	data["LogDir"] = hivev1alpha1.KubeDataLogDir
	data["LogMountDir"] = hivev1alpha1.KubeDataLogConfigMountDir
	data["HiveLog4j2Properties"] = HiveMetastoreLog4jName
	s3TemplateData := CreateS3ScriptData(r.cr.Spec.ClusterConfig)
	if len(krbTemplateData) > 0 {
		maps.Copy(data, krbTemplateData)
	}
	if len(s3TemplateData) > 0 {
		maps.Copy(data, s3TemplateData)
	}
	return ParseKerberosScript(tmplate, data)
}

func (r *DeploymentReconciler) replicas() int32 {
	return r.roleGroup.Replicas
}

func (r *DeploymentReconciler) make() (*appsv1.Deployment, error) {
	podTemplate := r.getPodTemplate()
	replicas := r.replicas()
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Name(),
			Namespace: r.NameSpace(),
			Labels:    r.GetLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: r.GetLabels(),
			},
			Template: podTemplate,
		},
	}

	if r.RoleGroupConfig() != nil {
		r.addAffinity(dep)

		if r.RoleGroupConfig().Tolerations != nil {
			dep.Spec.Template.Spec.Tolerations = r.RoleGroupConfig().Tolerations
		}

		if r.RoleGroupConfig().NodeSelector != nil {
			dep.Spec.Template.Spec.NodeSelector = r.RoleGroupConfig().NodeSelector
		}
	}

	// if vector is enabled, extend the workload
	if isVectorEnabled := IsVectorEnable(r.RoleGroupConfig().Logging); isVectorEnabled {
		ExtendWorkloadByVector(nil, dep, HiveMetaStoreConfigMapName(r.cr, r.roleGroupName))
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(r.cr, dep, r.scheme); err != nil {
		return nil, err
	}

	return dep, nil
}

func (r *DeploymentReconciler) addAffinity(dep *appsv1.Deployment) {
	var affinity *corev1.Affinity
	if r.RoleGroupConfig().Affinity != nil {
		affinity = r.RoleGroupConfig().Affinity
	} else {
		affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 70,
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey: corev1.LabelHostname,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									LabelCrName:    strings.ToLower(r.cr.Name),
									LabelComponent: r.roleName,
								},
							},
						},
					},
				},
			},
		}
	}
	dep.Spec.Template.Spec.Affinity = affinity
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
			Name:      hivev1alpha1.KubeDataConfigDirName,
			MountPath: hivev1alpha1.KubeDataConfigDir,
		},
		{
			Name:      hivev1alpha1.KubeDataLogDirName,
			MountPath: hivev1alpha1.KubeDataLogDir,
		},
		{
			Name:      hivev1alpha1.KubeDataConfigMountDirName,
			MountPath: hivev1alpha1.KubeDataConfigMountDir,
		},
	}

	// vms = append(vms, corev1.VolumeMount{
	// 	Name:      r.hiveDataMountName(),
	// 	MountPath: hivev1alpha1.WarehouseDir,
	// })

	vms = append(vms, corev1.VolumeMount{
		Name:      hivev1alpha1.KubeDataLogConfigMountDirName,
		MountPath: hivev1alpha1.KubeDataLogConfigMountDir,
	})

	if IsKerberosEnabled(r.cr.Spec.ClusterConfig) {
		vms = append(vms, KrbVolumeMount())
	}

	if IsS3Enable(r.cr.Spec.ClusterConfig) {
		vms = append(vms, S3VolumeMount())
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

func (r *DeploymentReconciler) createContainer() corev1.Container {

	image := hivev1alpha1.TransformImage(r.cr.Spec.Image)

	obj := corev1.Container{
		Name:            r.Name(),
		Image:           image.GetImageWithTag(),
		ImagePullPolicy: *image.GetPullPolicy(),
		Env: []corev1.EnvVar{
			{
				Name:  "SERVICE_NAME",
				Value: "metastore",
			},
		},
		Command: r.command(),
		Args:    r.Args(),
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

	if IsKerberosEnabled(r.cr.Spec.ClusterConfig) {
		obj.Env = KrbEnv(obj.Env)
	}

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
		Type:               hivev1alpha1.ConditionTypeClusterAvailable,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: r.cr.GetGeneration(),
	})

	return r.client.Status().Update(context.Background(), r.cr)
}
