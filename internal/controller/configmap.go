package controller

import (
	"context"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/config/xml"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

var _ builder.ConfigBuilder = &ConfigMapBuilder{}

type ConfigMapBuilder struct {
	builder.ConfigMapBuilder

	ClusterConfig *hivev1alpha1.ClusterConfigSpec

	RoleGroupConfig *hivev1alpha1.ConfigSpec
}

func NewConfigMapBuilder(
	client *client.Client,
	name string,
	clusterConfig *hivev1alpha1.ClusterConfigSpec,
	roleGroupConfig *hivev1alpha1.ConfigSpec,
	options ...builder.Option,
) *ConfigMapBuilder {
	opts := builder.Options{}
	for _, o := range options {
		o(&opts)
	}

	return &ConfigMapBuilder{
		ConfigMapBuilder: *builder.NewConfigMapBuilder(
			client,
			name,
			opts.Labels,
			opts.Annotations,
		),
		ClusterConfig:   clusterConfig,
		RoleGroupConfig: roleGroupConfig,
	}
}

func (b *ConfigMapBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	var s3Connection *S3Connection
	if b.ClusterConfig.S3 != nil {
		if s, err := GetS3Connect(ctx, b.Client, b.ClusterConfig.S3); err != nil {
			return nil, err
		} else {
			s3Connection = s
		}
	}
	if err := b.addHiveSite(s3Connection); err != nil {
		return nil, err
	}

	if err := b.addVectorConfig(ctx); err != nil {
		return nil, err
	}

	if err := b.addCoreSite(); err != nil {
		return nil, err
	}

	if err := b.addLog4j2(); err != nil {
		return nil, err
	}

	return b.GetObject(), nil
}

func (b *ConfigMapBuilder) addVectorConfig(ctx context.Context) error {
	if b.RoleGroupConfig != nil && b.RoleGroupConfig.Logging != nil && *b.RoleGroupConfig.Logging.EnableVectorAgent {
		vectorConfig, err := productlogging.MakeVectorYaml(
			ctx,
			b.Client.Client,
			b.Client.GetOwnerNamespace(),
			b.ClusterName,
			b.RoleName,
			b.RoleGroupName,
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
	warehouseDir := hivev1alpha1.DefaultWarehouseDir
	if b.RoleGroupConfig != nil {
		warehouseDir = b.RoleGroupConfig.WarehouseDir
	}
	config := xml.NewXMLConfiguration()
	config.AddPropertyWithString("hive.metastore.warehouse.dir", warehouseDir, "Default is"+hivev1alpha1.DefaultWarehouseDir)

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
	if b.RoleGroupConfig != nil && b.RoleGroupConfig.Logging != nil {
		if log, ok := b.RoleGroupConfig.Logging.Containers[b.RoleName]; ok {
			return &log
		}
	}
	return &commonsv1alpha1.LoggingConfigSpec{
		Console: &commonsv1alpha1.LogLevelSpec{Level: "INFO"},
		File:    &commonsv1alpha1.LogLevelSpec{Level: "INFO"},
	}
}

func (b *ConfigMapBuilder) addLog4j2() error {
	logConfig := b.getLogConfig()
	logGenerator, err := productlogging.NewConfigGenerator(
		logConfig,
		b.RoleName,
		"hive.log4j2.xml",
		productlogging.LogTypeLog4j2,
		func(cgo *productlogging.ConfigGeneratorOption) {
			cgo.ConsoleHandlerFormatter = ptr.To("%d{ISO8601} %5p [%t] %c{2}: %m%n")
		},
	)

	if err != nil {
		return err
	}

	s, err := logGenerator.Content()
	if err != nil {
		return err
	}

	b.AddItem("metastore-log4j2.properties", s)
	return nil
}

func NewConfigMapReconciler(
	client *client.Client,
	clusterConfig *hivev1alpha1.ClusterConfigSpec,
	info reconciler.RoleGroupInfo,
	config *hivev1alpha1.ConfigSpec,
	options ...builder.Option,
) *reconciler.GenericResourceReconciler[*ConfigMapBuilder] {
	cmBuilder := NewConfigMapBuilder(
		client,
		info.GetFullName(),
		clusterConfig,
		config,
		options...,
	)
	return reconciler.NewGenericResourceReconciler[*ConfigMapBuilder](
		client,
		cmBuilder,
	)
}
