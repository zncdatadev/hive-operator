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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	WarehouseDir    = "/opt/hive/data"
	ImageRepository = "docker.io/apache/hive"
	ImageTag        = "4.0.0-beta-1"
	ImagePullPolicy = corev1.PullAlways
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

type ImageSpec struct {
	// +kubebuilder:validation=Optional
	// +kubebuilder:default=docker.io/apache/hive
	Repository string `json:"repository,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="4.0.0-beta-1"
	Tag string `json:"tag,omitempty"`

	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +kubebuilder:default=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

// HiveMetastoreSpec defines the desired state of HiveMetastore
type HiveMetastoreSpec struct {
	//+kubebuilder:validation:Optional
	Image *ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Optional
	ClusterConfig *ClusterConfigSpec `json:"clusterConfig,omitempty"`

	// +kubebuilder:validation:Optional
	ClusterOperation *ClusterOperationSpec `json:"clusterOperation,omitempty"`

	// +kubebuilder:validation:Required
	Metastore *RoleSpec `json:"metastore"`
}

type ClusterConfigSpec struct {
	// +kubebuilder:validation:Optional
	Database *DatabaseSpec `json:"database,omitempty"`

	// +kubebuilder:validation:Optional
	S3Bucket *S3BucketSpec `json:"s3Bucket,omitempty"`

	// +kubebuilder:validation:Optional
	Listener *ListenerSpec `json:"listener,omitempty"`
}

type ClusterOperationSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	ReconciliationPaused bool `json:"reconciliationPaused,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	Stopped bool `json:"stopped,omitempty"`
}

type ListenerSpec struct {
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

type RoleSpec struct {

	// +kubebuilder:validation:Optional
	Config *ConfigSpec `json:"config,omitempty"`

	RoleGroups map[string]*RoleGroupSpec `json:"roleGroups,omitempty"`

	// +kubebuilder:validation:Optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`

	// +kubebuilder:validation:Optional
	CommandArgsOverrides []string `json:"commandArgsOverrides,omitempty"`
	// +kubebuilder:validation:Optional
	ConfigOverrides *ConfigOverridesSpec `json:"configOverrides,omitempty"`
	// +kubebuilder:validation:Optional
	EnvOverrides map[string]string `json:"envOverrides,omitempty"`
	//// +kubebuilder:validation:Optional
	//PodOverride corev1.PodSpec `json:"podOverride,omitempty"`
}

type ConfigOverridesSpec struct {
	HiveSite map[string]string `json:"hive-site.xml,omitempty"`
}

type ConfigSpec struct {
	// +kubebuilder:validation:Optional
	Resources *ResourcesSpec `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext"`

	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity"`

	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations"`

	// +kubebuilder:validation:Optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`

	// Use time.ParseDuration to parse the string
	// +kubebuilder:validation:Optional
	GracefulShutdownTimeout *string `json:"gracefulShutdownTimeout,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="/opt/hive/data"
	WarehouseDir string `json:"warehouseDir,omitempty"`

	// +kubebuilder:validation:Optional
	StorageClass string `json:"storageClass,omitempty"`

	// +kubebuilder:validation:Optional
	Logging *ContainerLoggingSpec `json:"logging,omitempty"`
}

type PodDisruptionBudgetSpec struct {
	// +kubebuilder:validation:Optional
	MinAvailable int32 `json:"minAvailable,omitempty"`

	// +kubebuilder:validation:Optional
	MaxUnavailable int32 `json:"maxUnavailable,omitempty"`
}

type RoleGroupSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=1
	Replicas int32 `json:"replicas,omitempty"`

	Config *ConfigSpec `json:"config,omitempty"`

	// +kubebuilder:validation:Optional
	CommandArgsOverrides []string `json:"commandArgsOverrides,omitempty"`
	// +kubebuilder:validation:Optional
	ConfigOverrides *ConfigOverridesSpec `json:"configOverrides,omitempty"`
	// +kubebuilder:validation:Optional
	EnvOverrides map[string]string `json:"envOverrides,omitempty"`
	//// +kubebuilder:validation:Optional
	//PodOverride corev1.PodSpec `json:"podOverride,omitempty"`
}

type DatabaseSpec struct {
	// +kubebuilder:validation=Optional
	Reference string `json:"reference"`

	// +kubebuilder:validation=Optional
	Inline *DatabaseInlineSpec `json:"inline,omitempty"`
}

// DatabaseInlineSpec defines the inline database spec.
type DatabaseInlineSpec struct {
	// +kubebuilder:validation:Enum=mysql;postgres
	// +kubebuilder:default="postgres"
	Driver string `json:"driver,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="hive"
	DatabaseName string `json:"databaseName,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="hive"
	Username string `json:"username,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="hive"
	Password string `json:"password,omitempty"`

	// +kubebuilder:validation=Required
	Host string `json:"host,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=5432
	Port int32 `json:"port,omitempty"`
}

type S3BucketSpec struct {
	// S3 bucket name with S3Bucket
	// +kubebuilder:validation=Optional
	Reference *string `json:"reference"`

	// +kubebuilder:validation=Optional
	Inline *S3BucketInlineSpec `json:"inline,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=20
	MaxConnect int `json:"maxConnect"`

	// +kubebuilder:validation=Optional
	PathStyleAccess bool `json:"pathStyle_access"`
}

type S3BucketInlineSpec struct {

	// +kubeBuilder:validation=Required
	Bucket string `json:"bucket"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="us-east-1"
	Region string `json:"region,omitempty"`

	// +kubebuilder:validation=Required
	Endpoints string `json:"endpoints"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=false
	SSL bool `json:"ssl,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	PathStyle bool `json:"pathStyle,omitempty"`

	// +kubebuilder:validation=Optional
	AccessKey string `json:"accessKey,omitempty"`

	// +kubebuilder:validation=Optional
	SecretKey string `json:"secretKey,omitempty"`
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

// GetNameWithSuffix returns the name of the HiveMetastore with the provided suffix appended.
func (instance *HiveMetastore) GetNameWithSuffix(name string) string {
	return instance.GetName() + "-" + name
}

// GetPvcName returns the name of the PVC for the HiveMetastore.
func (instance *HiveMetastore) GetPvcName() string {
	return instance.GetNameWithSuffix("pvc")
}
