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

package controller

import (
	"context"
	"github.com/go-logr/logr"
	stackv1alpha1 "github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
	"github.com/zncdata-labs/operator-go/pkg/status"
	"github.com/zncdata-labs/operator-go/pkg/util"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HiveMetastoreReconciler reconciles a HiveMetastore object
type HiveMetastoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=stack.zncdata.net,resources=hivemetastores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=stack.zncdata.net,resources=hivemetastores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=stack.zncdata.net,resources=hivemetastores/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;

// TODO(user): Modify the Reconcile function to compare the state specified by
// For more details, check Reconcile and its Result here: - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *HiveMetastoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconciling instance")

	hiveMetastore := &stackv1alpha1.HiveMetastore{}

	if err := r.Get(ctx, req.NamespacedName, hiveMetastore); err != nil {
		if client.IgnoreNotFound(err) != nil {
			r.Log.Error(err, "unable to fetch instance")
			return ctrl.Result{}, err
		}
		r.Log.Info("HiveMetastore resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	//// Get the status condition, if it exists and its generation is not the
	////same as the HiveMetastore's generation, reset the status conditions
	readCondition := apimeta.FindStatusCondition(hiveMetastore.Status.Conditions, status.ConditionTypeProgressing)
	if readCondition == nil || readCondition.ObservedGeneration != hiveMetastore.GetGeneration() {
		hiveMetastore.InitStatusConditions()

		if err := util.UpdateStatus(ctx, r.Client, hiveMetastore); err != nil {
			r.Log.Error(err, "unable to update HiveMetastoreServer status")
			return ctrl.Result{}, err
		}
	}

	r.Log.Info("HiveMetastore found", "Name", hiveMetastore.Name)

	tasks := []ReconcileTask{
		{
			resourceName:  Pvc,
			reconcileFunc: r.reconcilePvc,
		},
		{
			resourceName:  ConfigMap,
			reconcileFunc: r.reconcileConfigMap,
		},
		{
			resourceName:  Secret,
			reconcileFunc: r.reconcileSecret,
		},
		{
			resourceName:  Deployment,
			reconcileFunc: r.reconcileDeployment,
		}, {
			resourceName:  Service,
			reconcileFunc: r.reconcileService,
		},
	}

	if err := ReconcileTasks(&tasks, ctx, hiveMetastore, r, "hiveMetaStore"); err != nil {
		return ctrl.Result{}, err
	}

	if err := util.UpdateStatus(ctx, r.Client, hiveMetastore); err != nil {
		r.Log.Error(err, "unable to update SparkHistoryServer status")
		return ctrl.Result{}, err
	}

	r.Log.Info("Successfully reconciled hiveMetastore")
	return ctrl.Result{}, nil
}

func (r *HiveMetastoreReconciler) UpdateStatus(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, instance)
		//return r.Status().Patch(ctx, instance, client.MergeFrom(instance))
	})

	if retryErr != nil {
		r.Log.Error(retryErr, "Failed to update vfm status after retries")
		return retryErr
	}

	r.Log.V(1).Info("Successfully patched object status")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HiveMetastoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stackv1alpha1.HiveMetastore{}).
		Complete(r)
}
