package controller

import (
	"context"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/config/xml"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

var _ builder.ConfigBuilder = &ConfigMapBuilder{}

type ConfigMapBuilder struct {
	builder.ConfigMapBuilder

	ClusterName   string
	RoleName      string
	RolegroupName string
	ClusterConfig *hivev1alpha1.ClusterConfigSpec

	Warehouse      string
	productLogging *hivev1alpha1.LoggingSpec
}

func NewConfigMapBuilder(
	client *client.Client,
	name string,
	options builder.Options,
) *ConfigMapBuilder {

	return &ConfigMapBuilder{
		ConfigMapBuilder: *builder.NewConfigMapBuilder(
			client,
			name,
			options.Labels,
			options.Annotations,
		),
		ClusterName:   options.ClusterName,
		RoleName:      options.RoleName,
		RolegroupName: options.RoleGroupName,
	}
}

func (b *ConfigMapBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {

	if b.ClusterConfig.S3 != nil {
		s3Connection, err := GetS3Connect(ctx, b.Client, b.ClusterConfig.S3)
		if err != nil {
			return nil, err
		}
		if err := b.addHiveSite(s3Connection); err != nil {
			return nil, err
		}
	}

	if err := b.addVectorConfig(ctx); err != nil {
		return nil, err
	}

	if err := b.addCoreSite(); err != nil {
		return nil, err
	}

	b.addLog4j2()

	return b.GetObject(), nil
}

func (b *ConfigMapBuilder) addVectorConfig(ctx context.Context) error {
	if b.productLogging != nil && b.productLogging.EnableVectorAgent {
		vectorConfig, err := productlogging.MakeVectorYaml(
			ctx,
			b.Client.Client,
			b.Client.GetOwnerNamespace(),
			b.ClusterName,
			b.RoleName,
			b.RolegroupName,
			b.ClusterConfig.VectorAggregatorConfigMapName,
		)
		if err != nil {
			return err
		}
		b.AddItem(builder.VectorConfigFile, vectorConfig)
	}
	return nil
}

func (b *ConfigMapBuilder) addHiveSite(s3Connection *S3Connection) error {
	config := xml.NewXMLConfiguration()
	config.AddPropertyWithString("hive.metastore.warehouse.dir", b.Warehouse, "Default is"+hivev1alpha1.DefaultWarehouseDir)

	if s3Connection != nil {
		s3Config := NewS3Config(s3Connection)
		config.AddPropertiesWithMap(s3Config.GetHiveSite())
	}

	if b.ClusterConfig.Authentication != nil {
		krb5Config := NewKerberosConfig(
			b.Client.GetOwnerNamespace(),
			b.ClusterName,
			b.RoleName,
			b.ClusterConfig.Authentication.Kerberos.SecretClass,
		)
		config.AddPropertiesWithMap(krb5Config.GetHiveSite())
	}

	s, err := config.Marshal()
	if err != nil {
		return err
	}
	b.AddItem("hive-site.xml", s)
	return nil
}

// If kerberos enable and no hdfs as storage, then add kerberos config.
// Example: When use S3 as storage, kerberos is enabled.
func (b *ConfigMapBuilder) addCoreSite() error {
	if b.ClusterConfig.Authentication != nil {
		config := xml.NewXMLConfiguration()
		config.AddPropertyWithString("hadoop.security.authentication", "kerberos", "")
		s, err := config.Marshal()
		if err != nil {
			return err
		}
		b.AddItem("core-site.xml", s)
	}
	return nil
}

func (b *ConfigMapBuilder) getLogConfig() *commonsv1alpha1.LoggingConfigSpec {
	if log, ok := b.productLogging.Containers[b.RoleName]; ok {
		return &log
	}
	return &commonsv1alpha1.LoggingConfigSpec{
		Console: &commonsv1alpha1.LogLevelSpec{Level: "INFO"},
		File:    &commonsv1alpha1.LogLevelSpec{Level: "INFO"},
	}
}

func (b *ConfigMapBuilder) addLog4j2() {
	logConfig := b.getLogConfig()
	log4j2 := productlogging.NewLog4j2ConfigGenerator(
		logConfig,
		b.RoleName,
		"%d{ISO8601} %5p [%t] %c{2}: %m%n",
		nil,
		"hive.lo4j2.xml",
		"",
	)

	s := log4j2.Generate()
	b.AddItem("metastore-log4j2.properties", s)
}

var _ reconciler.ResourceReconciler[*ConfigMapBuilder] = &ConfigMapReconciler[reconciler.AnySpec]{}

type ConfigMapReconciler[T reconciler.AnySpec] struct {
	reconciler.ResourceReconciler[*ConfigMapBuilder]
	ClusterConfig *hivev1alpha1.ClusterConfigSpec
}

func NewConfigMapReconciler[T reconciler.AnySpec](
	client *client.Client,
	clusterConfig *hivev1alpha1.ClusterConfigSpec,
	options reconciler.RoleGroupInfo,
	spec T,
) *ConfigMapReconciler[T] {
	cmBuilder := NewConfigMapBuilder(
		client,
		options.GetFullName(),
		builder.Options{
			ClusterName:   options.GetClusterName(),
			RoleName:      options.GetRoleName(),
			RoleGroupName: options.GetGroupName(),
			Labels:        options.GetLabels(),
			Annotations:   options.GetAnnotations(),
		},
	)
	return &ConfigMapReconciler[T]{
		ResourceReconciler: reconciler.NewGenericResourceReconciler[*ConfigMapBuilder](
			client,
			options.GetFullName(),
			cmBuilder,
		),
		ClusterConfig: clusterConfig,
	}
}
