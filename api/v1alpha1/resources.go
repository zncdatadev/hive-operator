package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

type ResourcesSpec struct {
	// +kubebuilder:validation:Optional
	CPU *CPUResource `json:"cpu,omitempty"`

	// +kubebuilder:validation:Optional
	Memory *MemoryResource `json:"memory,omitempty"`

	// +kubebuilder:validation:Optional
	Storage *StorageResource `json:"storage,omitempty"`
}

type StorageResourceSpec struct {
	Data *StorageResource `json:"data"`
}

type CPUResource struct {
	// +kubebuilder:validation:Optional
	Max *resource.Quantity `json:"max,omitempty"`

	// +kubebuilder:validation:Optional
	Min *resource.Quantity `json:"min,omitempty"`
}

type MemoryResource struct {
	// +kubebuilder:validation:Optional
	Limit *resource.Quantity `json:"limit,omitempty"`
}

type StorageResource struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="10Gi"
	Capacity resource.Quantity `json:"capacity,omitempty"`

	// +kubebuilder:validation:Optional
	StorageClass string `json:"storageClass,omitempty"`
}
