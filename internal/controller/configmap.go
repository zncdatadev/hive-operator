package controller

import (
	"context"
	"fmt"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapReconciler struct {
	client client.Client
	scheme *runtime.Scheme

	s3            *S3Configuration
	roleGroup     *hivev1alpha1.RoleGroupSpec
	cr            *hivev1alpha1.HiveMetastore
	roleGroupName string
}

func NewConfigMapRecociler(
	ctx context.Context,
	client client.Client,
	scheme *runtime.Scheme,
	roleGroup *hivev1alpha1.RoleGroupSpec,
	cr *hivev1alpha1.HiveMetastore,
	roleGroupName string,
) *ConfigMapReconciler {
	resourceClient := ResourceClient{
		Ctx:       ctx,
		Client:    client,
		Namespace: cr.Namespace,
		Log:       log,
	}

	s3Configuration := NewS3Configuration(cr, resourceClient)
	return &ConfigMapReconciler{
		client:        client,
		scheme:        scheme,
		s3:            s3Configuration,
		roleGroup:     roleGroup,
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

	if hiveSiteXml, err := c.makeHiveSiteXml(); err != nil {
		return nil, err
	} else {
		data["hive-site.xml"] = hiveSiteXml
	}

	// KrbCoreSiteXml if kerberos is activated ,but we have no HDFS as backend (i.e. S3) then a core-site.xml is
	// needed to set "hadoop.security.authentication"
	if IsKerberosEnabled(c.cr.Spec.ClusterConfig) /*&& c.s3 != nil*/ {
		if coreSiteXml, err := c.makeCoreSiteXml(); err != nil {
			return nil, err
		} else {
			data["core-site.xml"] = coreSiteXml
		}
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

// make core-site.xml
func (c *ConfigMapReconciler) makeCoreSiteXml() (string, error) {
	coreSiteProperties := make(map[string]string)
	KrbCoreSiteXml(coreSiteProperties)

	c.overridesCoreSiteProperties(coreSiteProperties)

	marshal, err := util.NewXMLConfigurationFromMap(coreSiteProperties).Marshal()
	if err != nil {
		return "", err
	} else {
		return marshal, nil
	}
}

func (c *ConfigMapReconciler) makeHiveSiteXml() (string, error) {
	properties, err := c.hiveSiteProperties()
	if err != nil {
		return "", err
	}

	properties, err = c.overridesHiveSiteProperties(properties)
	if err != nil {
		return "", err
	}

	marshal, err := util.NewXMLConfigurationFromMap(properties).Marshal()
	if err != nil {
		return "", err
	} else {
		return marshal, nil
	}
}

func (c *ConfigMapReconciler) hiveSiteProperties() (map[string]string, error) {
	properties := map[string]string{
		"hive.metastore.warehouse.dir": c.warehouseDir(),
	}
	if IsS3Enable(c.cr.Spec.ClusterConfig) {
		var params *S3Params

		if c.s3.ExistingS3Bucket() {
			var err error
			params, err = c.s3.GetS3ParamsFromResource()
			if err != nil {
				return nil, err
			}
		} else {
			var err error
			params, err = c.s3.GetS3ParamsFromInline()
			if err != nil {
				return nil, err
			}
		}
		S3HiveSiteXml(properties, params)
	}
	if IsKerberosEnabled(c.cr.Spec.ClusterConfig) {
		KrbHiveSiteXml(properties, c.cr.Name, c.cr.Namespace)
	}

	return properties, nil
}

func (c *ConfigMapReconciler) warehouseDir() string {
	if c.roleGroup.Config != nil && c.roleGroup.Config.WarehouseDir != "" {
		return c.roleGroup.Config.WarehouseDir
	}
	return hivev1alpha1.WarehouseDir
}

func (c *ConfigMapReconciler) overridesHiveSiteProperties(properties map[string]string) (map[string]string, error) {
	if c.roleGroup.ConfigOverrides != nil && c.roleGroup.ConfigOverrides.HiveSite != nil {
		for k, v := range c.roleGroup.ConfigOverrides.HiveSite {
			properties[k] = v
		}
	}
	return properties, nil
}

func (c *ConfigMapReconciler) overridesCoreSiteProperties(properties map[string]string) map[string]string {
	if c.roleGroup.ConfigOverrides != nil && c.roleGroup.ConfigOverrides.CoreSite != nil {
		for k, v := range c.roleGroup.ConfigOverrides.CoreSite {
			properties[k] = v
		}
	}
	return properties
}

func HiveMetaStoreConfigMapName(cr *hivev1alpha1.HiveMetastore, roleGroupName string) string {
	return fmt.Sprintf("%s-%s-%s", cr.Name, RoleHiveMetaStore, roleGroupName)
}
