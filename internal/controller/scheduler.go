package controller

import (
	stackv1alpha1 "github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		toleration := *tolerations

		dep.Spec.Template.Spec.Tolerations = []corev1.Toleration{
			{
				Key:               toleration.Key,
				Operator:          toleration.Operator,
				Value:             toleration.Value,
				Effect:            toleration.Effect,
				TolerationSeconds: toleration.TolerationSeconds,
			},
		}
	}

	if affinity != nil {
		dep.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		if affinity != nil {
			dep.Spec.Template.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
			if affinity != nil && affinity.NodeAffinity != nil &&
				affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
				requiredTerms := make([]corev1.NodeSelectorTerm, len(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms))

				for i, term := range affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
					requiredTerms[i] = corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      term.MatchExpressions[0].Key,
								Operator: term.MatchExpressions[0].Operator,
								Values:   term.MatchExpressions[0].Values,
							},
						},
					}
				}

				if dep.Spec.Template.Spec.Affinity.NodeAffinity == nil {
					dep.Spec.Template.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
				}

				dep.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
					NodeSelectorTerms: requiredTerms,
				}
			}

			if affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				preferredTerms := []corev1.PreferredSchedulingTerm{}

				for _, term := range affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					preferredTerm := corev1.PreferredSchedulingTerm{
						Weight: term.Weight,
						Preference: corev1.NodeSelectorTerm{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      term.Preference.MatchExpressions[0].Key,
									Operator: term.Preference.MatchExpressions[0].Operator,
									Values:   term.Preference.MatchExpressions[0].Values,
								},
							},
						},
					}

					preferredTerms = append(preferredTerms, preferredTerm)
				}

				dep.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = preferredTerms
			}
		}

		if affinity.PodAffinity != nil {
			dep.Spec.Template.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
			if affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
				requiredTerms := []corev1.PodAffinityTerm{}

				for _, term := range affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
					requiredTerm := corev1.PodAffinityTerm{
						Namespaces:        term.Namespaces,
						TopologyKey:       term.TopologyKey,
						NamespaceSelector: term.NamespaceSelector,
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      term.LabelSelector.MatchExpressions[0].Key,
									Operator: term.LabelSelector.MatchExpressions[0].Operator,
									Values:   term.LabelSelector.MatchExpressions[0].Values,
								},
							},
						},
					}

					requiredTerms = append(requiredTerms, requiredTerm)
				}

				dep.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = requiredTerms
			}

			if affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				preferredTerms := []corev1.WeightedPodAffinityTerm{}

				for _, term := range affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					preferredTerm := corev1.WeightedPodAffinityTerm{
						Weight: term.Weight,
						PodAffinityTerm: corev1.PodAffinityTerm{
							Namespaces:        term.PodAffinityTerm.Namespaces,
							TopologyKey:       term.PodAffinityTerm.TopologyKey,
							NamespaceSelector: term.PodAffinityTerm.NamespaceSelector,
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      term.PodAffinityTerm.LabelSelector.MatchExpressions[0].Key,
										Operator: term.PodAffinityTerm.LabelSelector.MatchExpressions[0].Operator,
										Values:   term.PodAffinityTerm.LabelSelector.MatchExpressions[0].Values,
									},
								},
							},
						},
					}

					preferredTerms = append(preferredTerms, preferredTerm)
				}

				dep.Spec.Template.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = preferredTerms
			}
		}

		if affinity.PodAntiAffinity != nil {
			dep.Spec.Template.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
			if affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
				requiredTerms := []corev1.PodAffinityTerm{}

				for _, term := range affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
					requiredTerm := corev1.PodAffinityTerm{
						Namespaces:        term.Namespaces,
						TopologyKey:       term.TopologyKey,
						NamespaceSelector: term.NamespaceSelector,
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      term.LabelSelector.MatchExpressions[0].Key,
									Operator: term.LabelSelector.MatchExpressions[0].Operator,
									Values:   term.LabelSelector.MatchExpressions[0].Values,
								},
							},
						},
					}

					requiredTerms = append(requiredTerms, requiredTerm)
				}

				dep.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = requiredTerms
			}

			if affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				preferredTerms := []corev1.WeightedPodAffinityTerm{}

				for _, term := range affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					preferredTerm := corev1.WeightedPodAffinityTerm{
						Weight: term.Weight,
						PodAffinityTerm: corev1.PodAffinityTerm{
							Namespaces:        term.PodAffinityTerm.Namespaces,
							TopologyKey:       term.PodAffinityTerm.TopologyKey,
							NamespaceSelector: term.PodAffinityTerm.NamespaceSelector,
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      term.PodAffinityTerm.LabelSelector.MatchExpressions[0].Key,
										Operator: term.PodAffinityTerm.LabelSelector.MatchExpressions[0].Operator,
										Values:   term.PodAffinityTerm.LabelSelector.MatchExpressions[0].Values,
									},
								},
							},
						},
					}

					preferredTerms = append(preferredTerms, preferredTerm)
				}

				dep.Spec.Template.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = preferredTerms
			}
		}
	}
}
