package v1alpha1

import (
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
)

type LoggingSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	EnableVectorAgent bool `json:"enableVectorAgent,omitempty"`

	// +kubebuilder:validation:Optional
	Containers map[string]commonsv1alpha1.LoggingConfigSpec `json:"metastore,omitempty"`
}
