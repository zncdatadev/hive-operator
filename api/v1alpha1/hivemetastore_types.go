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
	"github.com/zncdatadev/operator-go/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultWarehouseDir = "/kubedoop/warehouse"
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
	ListenerClass constants.ListenerClass `json:"listenerClass,omitempty"`

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
	ConnectionString string `json:"connectionString"`

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
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"condition,omitempty"`

	// +kubebuilder:validation:Optional
	Replicas int32 `json:"replicas,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HiveMetastore{}, &HiveMetastoreList{})
}
