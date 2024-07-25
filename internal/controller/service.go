package controller

import (
	"context"
	"time"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceReconciler struct {
	client client.Client
	scheme *runtime.Scheme

	labels        map[string]string
	cr            *hivev1alpha1.HiveMetastore
	roleGroupName string
}

func NewReconcileService(
	client client.Client,
	schema *runtime.Scheme,
	labels map[string]string,
	cr *hivev1alpha1.HiveMetastore,
	roleGroupName string,
) *ServiceReconciler {
	return &ServiceReconciler{
		client:        client,
		scheme:        schema,
		labels:        labels,
		cr:            cr,
		roleGroupName: roleGroupName,
	}
}

func (r *ServiceReconciler) Name() string {
	return r.cr.GetNameWithSuffix(r.roleGroupName)
}

func (r *ServiceReconciler) Labels() map[string]string {
	return r.labels
}

func (r *ServiceReconciler) NameSpace() string {
	return r.cr.Namespace
}

func (r *ServiceReconciler) Reconcile(ctx context.Context) (ctrl.Result, error) {
	return r.apply(ctx)

}

func (r *ServiceReconciler) make() (*corev1.Service, error) {
	obj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Name(),
			Namespace: r.NameSpace(),
		},
		Spec: corev1.ServiceSpec{
			Selector: r.Labels(),
			Ports: []corev1.ServicePort{
				{
					Name: "thrift",
					Port: 9083,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "tcp",
					},
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(r.cr, obj, r.scheme); err != nil {
		return obj, err
	}
	return obj, nil
}

func (r *ServiceReconciler) apply(ctx context.Context) (ctrl.Result, error) {

	obj, err := r.make()
	if err != nil {
		return ctrl.Result{}, err
	}

	if mutant, err := CreateOrUpdate(ctx, r.client, obj); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
	return ctrl.Result{Requeue: false}, nil
}
