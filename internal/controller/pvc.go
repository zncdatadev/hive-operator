package controller

import (
	"context"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"
)

type PVCReconciler struct {
	client client.Client
	scheme *runtime.Scheme

	cr *hivev1alpha1.HiveMetastore

	roleGroupName string
	roleGroup     *hivev1alpha1.RoleGroupSpec
}

func NewPVCReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	cr *hivev1alpha1.HiveMetastore,
	roleGroupName string,
	roleGroup *hivev1alpha1.RoleGroupSpec,
) *PVCReconciler {
	return &PVCReconciler{
		client:        client,
		scheme:        scheme,
		cr:            cr,
		roleGroupName: roleGroupName,
		roleGroup:     roleGroup,
	}
}

func HiveDataPVCName(cr *hivev1alpha1.HiveMetastore, roleGroupName string) string {
	return cr.GetNameWithSuffix(roleGroupName + "-data")
}

func (r *PVCReconciler) NameSpace() string {
	return r.cr.Namespace
}

func (r *PVCReconciler) Labels() map[string]string {
	return r.cr.GetLabels()
}

func (r *PVCReconciler) Enabled() bool {
	return r.roleGroup.Config != nil &&
		r.roleGroup.Config.Resources != nil &&
		r.roleGroup.Config.Resources.Storage != nil

}

func (r *PVCReconciler) Reconcile(ctx context.Context) (ctrl.Result, error) {
	if r.Enabled() {
		return r.apply(ctx)
	}
	log.Info(
		"Storage configuration is not enabled for role group, so skip.", "roleGroup",
		r.roleGroupName,
	)
	return ctrl.Result{}, nil
}

func (r *PVCReconciler) storage() *hivev1alpha1.StorageResource {
	return r.roleGroup.Config.Resources.Storage
}

func (r *PVCReconciler) apply(ctx context.Context) (ctrl.Result, error) {
	obj, err := r.make()
	if err != nil {
		return ctrl.Result{}, err
	}

	if mutant, err := CreateOrUpdate(ctx, r.client, obj); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *PVCReconciler) make() (*corev1.PersistentVolumeClaim, error) {

	obj := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HiveDataPVCName(r.cr, r.roleGroupName),
			Namespace: r.NameSpace(),
			Labels:    r.Labels(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: r.storage().Capacity,
				},
			},
		},
	}

	if r.storage().StorageClass != "" {
		obj.Spec.StorageClassName = &r.storage().StorageClass
	}

	if err := controllerutil.SetControllerReference(r.cr, obj, r.scheme); err != nil {
		return nil, err
	}

	return obj, nil
}
