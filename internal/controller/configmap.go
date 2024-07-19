package controller

import (
	"context"
	"fmt"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapReconciler struct {
	client client.Client
	scheme *runtime.Scheme

	cr            *hivev1alpha1.HiveMetastore
	roleGroupName string
}

func NewConfigMapRecociler(
	client client.Client,
	scheme *runtime.Scheme,
	cr *hivev1alpha1.HiveMetastore,
	roleGroupName string,
) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		client:        client,
		scheme:        scheme,
		cr:            cr,
		roleGroupName: roleGroupName,
	}
}

func (c *ConfigMapReconciler) Reconcile(ctx context.Context) (ctrl.Result, error) {
	return c.apply(ctx)
}

func (c *ConfigMapReconciler) apply(ctx context.Context) (ctrl.Result, error) {
	obj, err := c.makeConfigmap()
	if err != nil {
		return ctrl.Result{}, err
	}
	//if len(obj.Data) == 0 {
	//	return ctrl.Result{Requeue: false}, nil
	//}

	effected, err := CreateOrUpdate(ctx, c.client, obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	if effected {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{Requeue: false}, nil
}

func (c *ConfigMapReconciler) makeConfigmap() (*corev1.ConfigMap, error) {
	var data = make(map[string]string)

	// KrbCoreSiteXml if kerberos is activated ,but we have no HDFS as backend (i.e. S3) then a core-site.xml is
	// needed to set "hadoop.security.authentication"
	if IsKerberosEnabled(c.cr.Spec.ClusterConfig) /*&& c.s3 != nil*/ {
		data = KrbCoreSiteXml()
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.cr.Namespace,
			Name:      HiveMetaStoreConfigMapName(c.cr, c.roleGroupName),
			Labels:    c.cr.GetLabels(),
		},
		Data: data,
	}, nil
}

func HiveMetaStoreConfigMapName(cr *hivev1alpha1.HiveMetastore, roleGroupName string) string {
	return fmt.Sprintf("%s-%s-%s", cr.Name, RoleHiveMetaStore, roleGroupName)
}
