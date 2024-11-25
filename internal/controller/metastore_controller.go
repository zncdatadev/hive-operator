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

package controller

import (
	"context"

	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

var log = logf.Log.WithName("hive-metastore-controller")

// HiveMetastoreReconciler reconciles a HiveMetastore object
type HiveMetastoreReconciler struct {
	ctrlclient.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hive.kubedoop.dev,resources=hivemetastores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hive.kubedoop.dev,resources=hivemetastores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hive.kubedoop.dev,resources=hivemetastores/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=s3.kubedoop.dev,resources=s3connections,verbs=get;list;watch
// +kubebuilder:rbac:groups=s3.kubedoop.dev,resources=s3buckets,verbs=get;list;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

func (r *HiveMetastoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Info("Reconciling instance")
	defer log.V(2).Info("Successfully reconciled hiveMetastore")

	instance := &hivev1alpha1.HiveMetastore{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(3).Info("Cannot find HiveMetastore instance, may have been deleted")
			return ctrl.Result{}, nil
		}
		log.V(1).Error(err, "Got error when trying to fetch HiveMetastore instance. Error: %v", err)
		return ctrl.Result{}, err
	}

	log.V(2).Info("HiveMetastore found", "Name", instance.Name)
	resourceClient := &client.Client{
		Client:         r.Client,
		OwnerReference: instance,
	}

	clusterInfo := reconciler.ClusterInfo{
		GVK: &metav1.GroupVersionKind{
			Group:   hivev1alpha1.GroupVersion.Group,
			Version: hivev1alpha1.GroupVersion.Version,
			Kind:    "HiveMetastore",
		},
		ClusterName: instance.Name,
	}

	reconciler := NewClusterReconciler(resourceClient, clusterInfo, &instance.Spec)

	if err := reconciler.RegisterResource(ctx); err != nil {
		return ctrl.Result{}, err
	}

	return reconciler.Run(ctx)
}

// SetupWithManager sets up the controller with the Manager.
func (r *HiveMetastoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hivev1alpha1.HiveMetastore{}).
		Complete(r)
}
