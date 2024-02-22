package controller

import (
	"context"

	hivev1alpha1 "github.com/zncdata-labs/hive-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterReconciler struct {
	client client.Client
	scheme *runtime.Scheme

	cr *hivev1alpha1.HiveMetastore
}

func NewClusterReconciler(client client.Client, scheme *runtime.Scheme, cr *hivev1alpha1.HiveMetastore) *ClusterReconciler {
	return &ClusterReconciler{
		client: client,
		scheme: scheme,
		cr:     cr,
	}
}

func (r *ClusterReconciler) Reconcile(ctx context.Context) (ctrl.Result, error) {

	res, err := r.ReconcileMetastore(ctx)

	if err != nil {
		return ctrl.Result{}, err
	}

	if res.RequeueAfter > 0 {
		return res, nil
	}

	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) ReconcileMetastore(ctx context.Context) (ctrl.Result, error) {
	metastoreRole := NewMetastoreRole(r.client, r.scheme, r.cr, r.cr.Spec.Metastore)

	res, err := metastoreRole.Reconcile(ctx)

	if err != nil {
		return ctrl.Result{}, err

	}
	return res, nil
}
