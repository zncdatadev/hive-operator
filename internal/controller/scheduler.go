package controller

import (
	stackv1alpha1 "github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// CreateScheduler todo: refactor
func CreateScheduler(instance *stackv1alpha1.HiveMetastore, dep *appsv1.Deployment, roleGroup *stackv1alpha1.RoleConfigSpec) {

	var (
		nodeSelector = instance.Spec.RoleConfig.NodeSelector
		tolerations  = instance.Spec.RoleConfig.Tolerations
		affinity     = instance.Spec.RoleConfig.Affinity
	)
	if roleGroup != nil {
		if rgNodeSelector := roleGroup.NodeSelector; rgNodeSelector != nil {
			nodeSelector = rgNodeSelector
		}
		if rgTolerations := roleGroup.Tolerations; rgTolerations != nil {
			tolerations = rgTolerations
		}
		if rgAffinity := roleGroup.Affinity; rgAffinity != nil {
			affinity = rgAffinity
		}
	}

	if nodeSelector != nil {
		dep.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	if tolerations != nil {
		dep.Spec.Template.Spec.Tolerations = []corev1.Toleration{
			*tolerations.DeepCopy(),
		}
	}

	if affinity != nil {
		dep.Spec.Template.Spec.Affinity = affinity.DeepCopy()
	}
}
