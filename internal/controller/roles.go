package controller

import (
	"context"
	"strings"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MetastoreRole struct {
	client client.Client
	scheme *runtime.Scheme

	cr *hivev1alpha1.HiveMetastore

	Role *hivev1alpha1.RoleSpec
}

func NewMetastoreRole(
	client client.Client,
	scheme *runtime.Scheme,
	cr *hivev1alpha1.HiveMetastore,
	role *hivev1alpha1.RoleSpec,
) *MetastoreRole {
	return &MetastoreRole{
		client: client,
		scheme: scheme,
		cr:     cr,
		Role:   role,
	}
}

func (r *MetastoreRole) Name() string {
	return "hivemetastore"
}

func (r *MetastoreRole) EnabledClusterConfig() bool {
	return r.cr.Spec.ClusterConfig != nil
}

func (r *MetastoreRole) GetLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       strings.ToLower(r.cr.Name),
		"app.kubernetes.io/component":  r.Name(),
		"app.kubernetes.io/managed-by": "hive-operator",
	}
}

func (r *MetastoreRole) MergeFromRole(roleGroup *hivev1alpha1.RoleGroupSpec) *hivev1alpha1.RoleGroupSpec {

	copiedRoleGroup := roleGroup.DeepCopy()

	// Merge the role into the role group.
	// if the role group has a config, and role group not has a config, will
	// merge the role's config into the role group's config.
	MergeObjects(copiedRoleGroup, r.Role, []string{"RoleGroups"})

	// merge the role's config into the role group's config
	if r.Role.Config != nil && copiedRoleGroup.Config != nil {
		MergeObjects(copiedRoleGroup.Config, r.Role.Config, []string{"PodDisruptionBudget"})
	}

	return copiedRoleGroup
}

func (r *MetastoreRole) Reconcile(ctx context.Context) (ctrl.Result, error) {

	roleLabels := RoleLabels{cr: r.cr, name: r.Name()}
	labels := roleLabels.GetLabels()

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

	if r.Role.Config != nil && r.Role.Config.PodDisruptionBudget != nil {
		res, err := NewReconcilePDB(
			r.client,
			r.scheme,
			r.cr,
			r.Name(),
			labels,
			r.Role.Config.PodDisruptionBudget,
		).Reconcile(ctx)

		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	stop := r.shouldStop()

	for name, rg := range r.Role.RoleGroups {
		mergedRg := r.MergeFromRole(rg)
		res, err := r.reconcileRoleGroup(ctx, name, mergedRg, stop)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *MetastoreRole) shouldStop() bool {
	return r.cr.Spec.ClusterOperation != nil && r.cr.Spec.ClusterOperation.Stopped
}

func (r *MetastoreRole) reconcileRoleGroup(
	ctx context.Context,
	name string,
	roleGroup *hivev1alpha1.RoleGroupSpec,
	stop bool,
) (ctrl.Result, error) {

	if roleGroup.Config != nil && roleGroup.Config.PodDisruptionBudget != nil {
		if result, err := NewReconcileRoleGroupPDB(
			r.client,
			r.scheme,
			r.cr,
			r.Name(),
			name,
			roleGroup,
		).Reconcile(ctx); err != nil {
			return ctrl.Result{}, err
		} else if result.RequeueAfter > 0 {
			return result, nil
		}
	}

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

	if result, err := NewReconcileService(r.client, r.scheme, r.cr, roleGroup, name).Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	if result, err := NewLog4jConfigMapRecociler(
		r.client,
		r.scheme,
		r.cr,
		r.Name(),
		name,
		roleGroup,
	).Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	deployment := NewReconcileDeployment(
		r.client,
		r.scheme,
		r.cr,
		r.Name(), // roleName
		name,     // roleGroupName
		roleGroup,
		stop,
	)

	if result, err := deployment.Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	return ctrl.Result{}, nil
}

type BaseRoleGroupResourceReconciler struct {
	client client.Client
	scheme *runtime.Scheme

	cr            *hivev1alpha1.HiveMetastore
	roleName      string
	roleGroupName string
	roleGroup     *hivev1alpha1.RoleGroupSpec
	stop          bool
}

func (r *DeploymentReconciler) NameSpace() string {
	return r.cr.Namespace
}

func (r *BaseRoleGroupResourceReconciler) Name() string {
	return r.cr.GetNameWithSuffix(r.roleGroupName)
}

func (r *BaseRoleGroupResourceReconciler) GetNameWithSuffix(name string) string {
	return r.Name() + "-" + name
}

func (r *BaseRoleGroupResourceReconciler) GetLabels() map[string]string {
	roleLabels := RoleLabels{cr: r.cr, name: r.roleName}
	labels := roleLabels.GetLabels()

	labels["app.kubernetes.io/instance"] = strings.ToLower(r.Name())
	return labels
}
