/*
Copyright 2023 zncdatadev.

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
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/common"
	"github.com/zncdatadev/operator-go/pkg/listener"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	DefaultWarehouseDir = "/kubedoop/warehouse"

	RoleMetastore = "metastore"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// HiveMetastore is the Schema for the hivemetastores API
type HiveMetastore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HiveMetastoreSpec   `json:"spec,omitempty"`
	Status HiveMetastoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HiveMetastoreList contains a list of HiveMetastore
type HiveMetastoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HiveMetastore `json:"items"`
}

// HiveMetastoreSpec defines the desired state of HiveMetastore
type HiveMetastoreSpec struct {
	// +kubebuilder:validation:Optional
	// +default:value={"repo": "quay.io/zncdatadev", "pullPolicy": "IfNotPresent"}
	Image *ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Required
	ClusterConfig *ClusterConfigSpec `json:"clusterConfig"`

	// +kubebuilder:validation:Optional
	ClusterOperation *commonsv1alpha1.ClusterOperationSpec `json:"clusterOperation,omitempty"`

	// +kubebuilder:validation:Required
	Metastore *RoleSpec `json:"metastore"`
}

type ClusterConfigSpec struct {

	// +kubebuilder:validation:Optional
	VectorAggregatorConfigMapName string `json:"vectorAggregatorConfigMapName,omitempty"`

	// +kubebuilder:validation:Required
	Database *DatabaseSpec `json:"database"`

	// +kubebuilder:validation:Optional
	S3 *S3Spec `json:"s3,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=cluster-internal
	// +kubebuilder:validation:Enum=cluster-internal;external-unstable;external-stable
	ListenerClass listener.ListenerClass `json:"listenerClass,omitempty"`

	// +kubebuilder:validation:Optional
	HDFS *HDFSSpec `json:"hdfs,omitempty"`

	// +kubebuilder:validation:Optional
	Authentication *AuthenticationSpec `json:"authentication,omitempty"`
}

type HDFSSpec struct {
	// +kubebuilder:validation:Required
	ConfigMap string `json:"configMap"`
}

type S3Spec struct {
	// +kubebuilder:validation:Optional
	Inline *s3v1alpha1.S3ConnectionSpec `json:"inline,omitempty"`

	// S3 connection reference
	// +kubebuilder:validation:Optional
	Reference string `json:"reference,omitempty"`
}

type DatabaseSpec struct {
	// +kubebuilder:validation:Required
	ConnString string `json:"connString"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default="derby"
	// +kubebuilder:validation:enum=derby;mysql;postgres;oracle
	DatabaseType string `json:"databaseType"`

	// +kubebuilder:validation:Required
	// A reference to a secret to use for the database connection credentials.
	// It must contain the following keys:
	//  - username
	//  - password
	CredentialsSecret string `json:"credentialsSecret"`
}

type AuthenticationSpec struct {
	// +kubebuilder:validation:Optional
	Tls *TlsSpec `json:"tls,omitempty"`

	// +kubebuilder:validation:Required
	Kerberos *KerberosSpec `json:"kerberos"`
}

type TlsSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="tls"
	// +kubebuilder:minLength=1
	SecretClass string `json:"secretClass,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="changeit"
	JksPassword string `json:"jksPassword,omitempty"`
}

type KerberosSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:minLength=1
	SecretClass string `json:"secretClass"`
}

type RoleSpec struct {

	// +kubebuilder:validation:Optional
	Config *ConfigSpec `json:"config,omitempty"`

	// +kubebuilder:validation:Required
	RoleGroups map[string]*RoleGroupSpec `json:"roleGroups"`

	// +kubebuilder:validation:Optional
	RoleConfig *commonsv1alpha1.RoleConfigSpec `json:"roleConfig,omitempty"`

	*commonsv1alpha1.OverridesSpec `json:",inline"`
}

type ConfigSpec struct {
	*commonsv1alpha1.RoleGroupConfigSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="/kubedoop/warehouse"
	WarehouseDir string `json:"warehouseDir,omitempty"`
}

type RoleGroupSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=1
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Optional
	Config *ConfigSpec `json:"config,omitempty"`

	*commonsv1alpha1.OverridesSpec `json:",inline"`
}

// HiveMetastoreStatus defines the observed state of HiveMetastore
type HiveMetastoreStatus struct {
	commonsv1alpha1.GenericClusterStatus `json:",inline"`
}

// ClusterInterface implementation

// GetSpec adapts the product spec to the framework's generic cluster spec.
func (h *HiveMetastore) GetSpec() *commonsv1alpha1.GenericClusterSpec {
	return h.Spec.ToGenericSpec()
}

// GetStatus returns the cluster status.
func (h *HiveMetastore) GetStatus() *commonsv1alpha1.GenericClusterStatus {
	return &h.Status.GenericClusterStatus
}

// SetStatus updates the cluster status.
func (h *HiveMetastore) SetStatus(status *commonsv1alpha1.GenericClusterStatus) {
	h.Status.GenericClusterStatus = *status
}

// GetObjectMeta returns the object metadata.
func (h *HiveMetastore) GetObjectMeta() *metav1.ObjectMeta {
	return &h.ObjectMeta
}

// GetScheme returns the cached runtime scheme.
func (h *HiveMetastore) GetScheme() *runtime.Scheme {
	return cachedScheme
}

// DeepCopyCluster creates a deep copy of the cluster.
func (h *HiveMetastore) DeepCopyCluster() common.ClusterInterface {
	return h.DeepCopy()
}

// GetRuntimeObject returns the underlying runtime.Object.
func (h *HiveMetastore) GetRuntimeObject() runtime.Object {
	return h
}

// VectorAggregatorConfigMapName implements reconciler.VectorAggregatorProvider so the framework
// owns vector.yaml generation: when a role group enables the Vector agent, the GenericReconciler
// resolves the aggregator address from this ConfigMap and renders vector.yaml into the role group
// ConfigMap. Returns "" when unset (the framework then omits vector.yaml).
func (h *HiveMetastore) VectorAggregatorConfigMapName() string {
	if h.Spec.ClusterConfig == nil {
		return ""
	}
	return h.Spec.ClusterConfig.VectorAggregatorConfigMapName
}

// ToGenericSpec adapts HiveMetastoreSpec to GenericClusterSpec.
func (s *HiveMetastoreSpec) ToGenericSpec() *commonsv1alpha1.GenericClusterSpec {
	result := &commonsv1alpha1.GenericClusterSpec{
		ClusterOperation: s.ClusterOperation,
	}

	if s.Image != nil {
		result.Image = &commonsv1alpha1.ImageSpec{
			Custom:          s.Image.Custom,
			Repo:            s.Image.Repo,
			ProductVersion:  s.Image.ProductVersion,
			KubedoopVersion: s.Image.KubedoopVersion,
			PullPolicy:      s.Image.PullPolicy,
		}
	}

	if s.Metastore != nil {
		roleSpec := commonsv1alpha1.RoleSpec{
			RoleConfig: s.Metastore.RoleConfig,
		}

		if s.Metastore.Config != nil {
			roleSpec.Config = s.Metastore.Config.RoleGroupConfigSpec
		}

		if s.Metastore.OverridesSpec != nil {
			roleSpec.ConfigOverrides = s.Metastore.ConfigOverrides
			roleSpec.EnvOverrides = s.Metastore.EnvOverrides
			roleSpec.CliOverrides = s.Metastore.CliOverrides
			roleSpec.PodOverrides = s.Metastore.PodOverrides
		}

		roleGroups := make(map[string]commonsv1alpha1.RoleGroupSpec)
		for name, rg := range s.Metastore.RoleGroups {
			if rg == nil {
				continue
			}
			adapted := commonsv1alpha1.RoleGroupSpec{}
			if rg.Replicas > 0 {
				replicas := rg.Replicas
				adapted.Replicas = &replicas
			}
			if rg.Config != nil {
				adapted.Config = rg.Config.RoleGroupConfigSpec
			}
			if rg.OverridesSpec != nil {
				adapted.ConfigOverrides = rg.ConfigOverrides
				adapted.EnvOverrides = rg.EnvOverrides
				adapted.CliOverrides = rg.CliOverrides
				adapted.PodOverrides = rg.PodOverrides
			}
			roleGroups[name] = adapted
		}
		roleSpec.RoleGroups = roleGroups

		result.Roles = map[string]commonsv1alpha1.RoleSpec{
			RoleMetastore: roleSpec,
		}
	}

	return result
}

// cachedScheme is initialized once and reused across all reconcile calls.
var cachedScheme *runtime.Scheme

func init() {
	SchemeBuilder.Register(&HiveMetastore{}, &HiveMetastoreList{})
	cachedScheme = runtime.NewScheme()
	_ = SchemeBuilder.AddToScheme(cachedScheme)
}
