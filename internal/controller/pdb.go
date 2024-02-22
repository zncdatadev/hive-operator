package controller

import (
	"context"
	hivev1alpha1 "github.com/zncdata-labs/hive-operator/api/v1alpha1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PDBReconciler struct {
	client client.Client
	schema *runtime.Scheme

	cr *hivev1alpha1.HiveMetastore

	name   string
	labels map[string]string
	pdb    *hivev1alpha1.PodDisruptionBudgetSpec
}

func NewReconcilePDB(
	client client.Client,
	schema *runtime.Scheme,
	cr *hivev1alpha1.HiveMetastore,

	name string,
	labels map[string]string,
	pdb *hivev1alpha1.PodDisruptionBudgetSpec,
) *PDBReconciler {

	return &PDBReconciler{
		client: client,
		schema: schema,
		cr:     cr,
		name:   name,
		labels: labels,
		pdb:    pdb,
	}
}

func (r *PDBReconciler) Reconcile(ctx context.Context) (ctrl.Result, error) {
	return r.applyPDB(ctx)
}

func (r *PDBReconciler) applyPDB(ctx context.Context) (ctrl.Result, error) {

	pdb, err := r.makePDB()
	if err != nil {
		return ctrl.Result{}, err

	}

	if _, err = CreateOrUpdate(ctx, r.client, pdb); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PDBReconciler) makePDB() (*policyv1.PodDisruptionBudget, error) {

	obj := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.name,
			Namespace: r.cr.Namespace,
			Labels:    r.labels,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: r.labels,
			},
		},
	}

	if r.pdb.MinAvailable > 0 {
		obj.Spec.MinAvailable = &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: r.pdb.MinAvailable,
		}
	}

	if r.pdb.MaxUnavailable > 0 {
		obj.Spec.MaxUnavailable = &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: r.pdb.MaxUnavailable,
		}
	}

	if err := ctrl.SetControllerReference(r.cr, obj, r.schema); err != nil {
		return nil, err
	}

	return obj, nil
}

type RoleGroupPDBReconciler struct {
	PDBReconciler
	BaseRoleGroupResourceReconciler
}

func NewReconcileRoleGroupPDB(
	client client.Client,
	schema *runtime.Scheme,
	cr *hivev1alpha1.HiveMetastore,

	roleName string,
	roleGroupName string,
	roleGroup *hivev1alpha1.RoleGroupSpec,
) *RoleGroupPDBReconciler {
	base := BaseRoleGroupResourceReconciler{
		client:        client,
		scheme:        schema,
		cr:            cr,
		roleName:      roleName,
		roleGroupName: roleGroupName,
		roleGroup:     roleGroup,
	}

	pdbReconciler := PDBReconciler{
		client: client,
		schema: schema,
		cr:     cr,
		name:   base.Name(),
		labels: base.GetLabels(),
		pdb:    roleGroup.Config.PodDisruptionBudget,
	}

	return &RoleGroupPDBReconciler{
		PDBReconciler:                   pdbReconciler,
		BaseRoleGroupResourceReconciler: base,
	}
}
