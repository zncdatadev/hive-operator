package controller

import (
	stackv1alpha1 "github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateScheduler(instance *stackv1alpha1.HiveMetastore, dep *appsv1.Deployment) {

	if instance.Spec.NodeSelector != nil {
		dep.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	if instance.Spec.Tolerations != nil {
		toleration := *instance.Spec.Tolerations

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

	if instance.Spec.Affinity != nil {
		dep.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		if instance.Spec.Affinity.NodeAffinity != nil {
			dep.Spec.Template.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
			if instance.Spec.Affinity != nil && instance.Spec.Affinity.NodeAffinity != nil &&
				instance.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {

				requiredTerms := make([]corev1.NodeSelectorTerm, len(instance.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms))

				for i, term := range instance.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
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

			if instance.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				preferredTerms := []corev1.PreferredSchedulingTerm{}

				for _, term := range instance.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
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

		if instance.Spec.Affinity.PodAffinity != nil {
			dep.Spec.Template.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
			if instance.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
				requiredTerms := []corev1.PodAffinityTerm{}

				for _, term := range instance.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
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

			if instance.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				preferredTerms := []corev1.WeightedPodAffinityTerm{}

				for _, term := range instance.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
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

		if instance.Spec.Affinity.PodAntiAffinity != nil {
			dep.Spec.Template.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
			if instance.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
				requiredTerms := []corev1.PodAffinityTerm{}

				for _, term := range instance.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
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

			if instance.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				preferredTerms := []corev1.WeightedPodAffinityTerm{}

				for _, term := range instance.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
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
