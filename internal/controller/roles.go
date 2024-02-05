package controller

import (
	"context"
	stackv1alpha1 "github.com/zncdata-labs/hive-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MetastoreRole struct {
	client client.Client
	scheme *runtime.Scheme

	cr *stackv1alpha1.HiveMetastore

	Role *stackv1alpha1.RoleSpec
}

func NewMetastoreRole(
	client client.Client,
	scheme *runtime.Scheme,
	cr *stackv1alpha1.HiveMetastore,
	role *stackv1alpha1.RoleSpec,
) *MetastoreRole {
	return &MetastoreRole{
		client: client,
		scheme: scheme,
		cr:     cr,
		Role:   role,
	}
}

func (r *MetastoreRole) EnabledClusterConfig() bool {
	return r.cr.Spec.ClusterConfig != nil
}

func (r *MetastoreRole) MergeFromRole(roleGroup *stackv1alpha1.RoleGroupSpec) *stackv1alpha1.RoleGroupSpec {

	copiedRoleGroup := roleGroup.DeepCopy()

	if copiedRoleGroup.Config == nil {
		return copiedRoleGroup
	}

	MergeObjects(copiedRoleGroup, r.Role, []string{"RoleGroups"})

	if r.Role.Config != nil {
		MergeObjects(copiedRoleGroup.Config, r.Role.Config, []string{})
	}
	return copiedRoleGroup
}

func (r *MetastoreRole) Reconcile(ctx context.Context) (ctrl.Result, error) {

	if r.EnabledClusterConfig() {
		envSecret := NewEnvSecret(ctx, r.client, r.scheme, r.cr)
		res, err := envSecret.Reconcile(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	for name, rg := range r.Role.RoleGroups {
		mergedRg := r.MergeFromRole(rg)
		res, err := r.reconcileRoleGroup(ctx, name, mergedRg)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *MetastoreRole) reconcileRoleGroup(
	ctx context.Context,
	name string,
	roleGroup *stackv1alpha1.RoleGroupSpec,
) (ctrl.Result, error) {

	secret := NewHiveSiteSecret(ctx, r.client, r.scheme, r.cr, name, roleGroup)
	if result, err := secret.Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	dataPVC := NewPVCReconciler(r.client, r.scheme, r.cr, name, roleGroup)

	if result, err := dataPVC.Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	if result, err := NewReconcileService(r.client, r.scheme, r.cr, roleGroup).Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	deployment := NewReconcileDeployment(
		r.client,
		r.scheme,
		r.cr,
		name,
		roleGroup,
	)

	if result, err := deployment.Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	return ctrl.Result{}, nil
}
