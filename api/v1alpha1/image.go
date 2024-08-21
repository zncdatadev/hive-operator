package v1alpha1

import (
	"github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultRepository     = "quay.io/zncdatadev"
	DefaultProductVersion = "3.1.3"
	DefaultProductName    = "hive"
	DefaultStackVersion   = "0.0.0-dev"
)

type ImageSpec struct {
	// +kubebuilder:validation:Optional
	Custom string `json:"custom,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=quay.io/zncdatadev
	Repo string `json:"repo,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="0.0.0-dev"
	PlatformVersion string `json:"platformVersion,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="3.1.3"
	ProductVersion string `json:"productVersion,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	PullPolicy *corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	PullSecretName string `json:"pullSecretName,omitempty"`
}

func TransformImage(imageSpec *ImageSpec) *util.Image {
	if imageSpec == nil {
		return util.NewImage(DefaultProductName, DefaultStackVersion, DefaultProductVersion)
	}
	return &util.Image{
		Custom:         imageSpec.Custom,
		Repository:     imageSpec.Repo,
		StackVersion:   imageSpec.PlatformVersion,
		ProductVersion: imageSpec.ProductVersion,
		PullPolicy:     imageSpec.PullPolicy,
		PullSecretName: imageSpec.PullSecretName,
	}
}
