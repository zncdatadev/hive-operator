/*
Copyright 2023 zncdata-labs.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HiveMetastoreSpec defines the desired state of HiveMetastore
type HiveMetastoreSpec struct {
	Image ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Required
	Resources *corev1.ResourceRequirements `json:"resources"`

	// +kubebuilder:validation:Required
	SecurityContext *corev1.PodSecurityContext `json:"securityContext"`

	// +kubebuilder:validation:Optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// +kubebuilder:validation:Optional
	Service *ServiceSpec `json:"service,omitempty"`

	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity"`

	// +kubebuilder:validation:Optional
	Tolerations *corev1.Toleration `json:"tolerations,omitempty"`

	// +kubebuilder:validation:Optional
	Persistence *PersistenceSpec `json:"persistence,omitempty"`

	// +kubebuilder:validation:Optional
	PostgresSecret *PostgresSecretSpec `json:"postgres,omitempty"`
}

// GetNameWithSuffix returns the name of the HiveMetastore with the provided suffix appended.
func (instance *HiveMetastore) GetNameWithSuffix(name string) string {
	return instance.GetName() + "-" + name
}

type PostgresSecretSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="postgresql"
	Host string `json:"host,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="5432"
	Port string `json:"port"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="hive"
	UserName string `json:"username"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="12345678"
	Password string `json:"password"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="hive"
	DataBase string `json:"database"`
}

type ImageSpec struct {

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=docker.io/apache/hive-metastore
	Repository string `json:"repository,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=latest
	Tag string `json:"tag,omitempty"`

	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +kubebuilder:default=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

type ServiceSpec struct {
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +kubebuilder:validation:enum=ClusterIP;NodePort;LoadBalancer;ExternalName
	// +kubebuilder:default=ClusterIP
	Type corev1.ServiceType `json:"type,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=9083
	Port int32 `json:"port,omitempty"`
}

type PersistenceSpec struct {
	// +kubebuilder:validation:Optional
	StorageClass *string `json:"storageClass,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={ReadWriteOnce}
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`

	// +kubebuilder:default="10Gi"
	Size string `json:"size,omitempty"`

	// +kubebuilder:validation:Optional
	ExistingClaim *string `json:"existingClaim,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=Filesystem
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode,omitempty"`

	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`

	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GetPvcName returns the name of the PVC for the HiveMetastore.
func (instance *HiveMetastore) GetPvcName() string {
	return instance.GetNameWithSuffix("pvc")
}

// SetStatusCondition updates the status condition using the provided arguments.
// If the condition already exists, it updates the condition; otherwise, it appends the condition.
// If the condition status has changed, it updates the condition's LastTransitionTime.
func (r *HiveMetastore) SetStatusCondition(condition metav1.Condition) {
	// if the condition already exists, update it
	existingCondition := apimeta.FindStatusCondition(r.Status.Conditions, condition.Type)
	if existingCondition == nil {
		condition.ObservedGeneration = r.GetGeneration()
		condition.LastTransitionTime = metav1.Now()
		r.Status.Conditions = append(r.Status.Conditions, condition)
	} else if existingCondition.Status != condition.Status || existingCondition.Reason != condition.Reason || existingCondition.Message != condition.Message {
		existingCondition.Status = condition.Status
		existingCondition.Reason = condition.Reason
		existingCondition.Message = condition.Message
		existingCondition.ObservedGeneration = r.GetGeneration()
		existingCondition.LastTransitionTime = metav1.Now()
	}
}

// InitStatusConditions initializes the status conditions to the provided conditions.
func (r *HiveMetastore) InitStatusConditions() {
	r.Status.Conditions = []metav1.Condition{}
	r.SetStatusCondition(metav1.Condition{
		Type:               ConditionTypeProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             ConditionReasonPreparing,
		Message:            "HiveMetastore is preparing",
		ObservedGeneration: r.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	})
	r.SetStatusCondition(metav1.Condition{
		Type:               ConditionTypeAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             ConditionReasonPreparing,
		Message:            "HiveMetastore is preparing",
		ObservedGeneration: r.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	})
}

// HiveMetastoreStatus defines the observed state of HiveMetastore
type HiveMetastoreStatus struct {
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"condition,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HiveMetastore is the Schema for the hivemetastores API
type HiveMetastore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HiveMetastoreSpec   `json:"spec,omitempty"`
	Status HiveMetastoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HiveMetastoreList contains a list of HiveMetastore
type HiveMetastoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HiveMetastore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HiveMetastore{}, &HiveMetastoreList{})
}
