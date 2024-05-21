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

	"github.com/go-logr/logr"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

//const memcachedFinalizer = "hive.zncdata.dev/finalizer"

var log = logf.Log.WithName("hive-metastore-controller")

// HiveMetastoreReconciler reconciles a HiveMetastore object
type HiveMetastoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=zncdata.dev,resources=hivemetastores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=zncdata.dev,resources=hivemetastores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=zncdata.dev,resources=hivemetastores/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;

func (r *HiveMetastoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Info("Reconciling instance")
	defer log.V(2).Info("Successfully reconciled hiveMetastore")

	existingInstance := &hivev1alpha1.HiveMetastore{}
	if err := r.Get(ctx, req.NamespacedName, existingInstance); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(3).Info("Cannot find HiveMetastore instance, may have been deleted")
			return ctrl.Result{}, nil
		}
		log.V(1).Error(err, "Got error when trying to fetch HiveMetastore instance. Error: %v", err)
		return ctrl.Result{}, err
	}

	log.V(2).Info("HiveMetastore found", "Name", existingInstance.Name)

	if r.ReconciliationPaused(ctx, existingInstance) {
		log.V(0).Info("Reconciliation paused")
		return ctrl.Result{}, nil
	}

	result, err := NewClusterReconciler(r.Client, r.Scheme, existingInstance).Reconcile(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	return result, nil
}

type ResourceClient struct {
	Ctx       context.Context
	Client    client.Client
	Namespace string
	Log       logr.Logger
}

func (r *ResourceClient) Get(obj client.Object) error {
	name := obj.GetName()
	kind := obj.GetObjectKind()
	if err := r.Client.Get(r.Ctx, client.ObjectKey{Namespace: r.Namespace, Name: name}, obj); err != nil {
		opt := []any{"ns", r.Namespace, "name", name, "kind", kind}
		if apierrors.IsNotFound(err) {
			r.Log.Error(err, "Fetch resource NotFound", opt...)
		} else {
			r.Log.Error(err, "Fetch resource occur some unknown err", opt...)
		}
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) ReconciliationPaused(ctx context.Context, instance *hivev1alpha1.HiveMetastore) bool {
	return instance.Spec.ClusterOperation != nil && instance.Spec.ClusterOperation.ReconciliationPaused
}

// SetupWithManager sets up the controller with the Manager.
func (r *HiveMetastoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hivev1alpha1.HiveMetastore{}).
		Complete(r)
}
