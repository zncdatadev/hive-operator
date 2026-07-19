// Package controller implements the HiveMetastore reconciliation on top of the operator-go
// GenericReconciler framework. This file only carries the RBAC markers consumed by
// `make manifests`.
package controller

// +kubebuilder:rbac:groups=hive.kubedoop.dev,resources=hivemetastores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hive.kubedoop.dev,resources=hivemetastores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hive.kubedoop.dev,resources=hivemetastores/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=s3.kubedoop.dev,resources=s3connections,verbs=get;list;watch
// +kubebuilder:rbac:groups=s3.kubedoop.dev,resources=s3buckets,verbs=get;list;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
