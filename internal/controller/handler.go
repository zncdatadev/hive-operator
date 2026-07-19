package controller

import (
	"context"
	"fmt"
	"maps"
	"path"
	"strings"

	"github.com/zncdatadev/operator-go/pkg/builder"
	opgoconfig "github.com/zncdatadev/operator-go/pkg/config"
	"github.com/zncdatadev/operator-go/pkg/constant"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	hiveconstant "github.com/zncdatadev/hive-operator/internal/constant"
	"github.com/zncdatadev/hive-operator/internal/util/version"
)

const (
	HiveSiteFileName  = "hive-site.xml"
	CoreSiteFileName  = "core-site.xml"
	LogConfigFileName = "metastore-log4j2.properties"

	warehouseDirProperty = "hive.metastore.warehouse.dir"

	// ConsoleConversionPattern matches the log4j2 console layout of the legacy implementation.
	ConsoleConversionPattern = "%d{ISO8601} %5p [%t] %c{2}: %m%n"
)

// Compile-time proof that HiveMetastore wires framework-owned vector.yaml generation.
var _ reconciler.VectorAggregatorProvider = (*hivev1alpha1.HiveMetastore)(nil)

// HiveRoleGroupHandler builds the metastore role group resources. It embeds the SDK's
// BaseRoleGroupHandler so the framework owns resource orchestration — ConfigMap (merged config
// plus the log4j2/vector files), Services, the StatefulSet (with sidecars, security context and
// overrides applied by the framework) and the role PDB. The override below adds the
// hive-specific pieces: hive-site/core-site content, the metastore container start script,
// database/S3/Kerberos wiring and the metrics Service.
type HiveRoleGroupHandler struct {
	*reconciler.BaseRoleGroupHandler[*hivev1alpha1.HiveMetastore]
}

var _ reconciler.RoleGroupHandler[*hivev1alpha1.HiveMetastore] = &HiveRoleGroupHandler{}

// NewHiveRoleGroupHandler creates the handler and configures the framework defaults.
func NewHiveRoleGroupHandler(scheme *runtime.Scheme) *HiveRoleGroupHandler {
	base := reconciler.NewBaseRoleGroupHandler[*hivev1alpha1.HiveMetastore]("", scheme)

	// The container name must match the per-container logging key (logging.containers.metastore)
	// and is asserted by the e2e suite.
	base.MainContainerName = hivev1alpha1.RoleMetastore
	base.ExtraLabels["app.kubernetes.io/name"] = "hivemetastore"

	// Declarative logging: the framework renders the log4j2 config into the ConfigMap under the
	// hive-conventional key (metastore-log4j2.properties) so the metastore start scripts pick it
	// up from the config directory.
	base.LoggingContainers = []productlogging.ContainerLogging{
		{
			Container: hivev1alpha1.RoleMetastore,
			Framework: productlogging.LoggingFrameworkLog4j2,
			FileName:  LogConfigFileName,
			Pattern:   ConsoleConversionPattern,
		},
	}

	// configOverrides for *.xml files (hive-site.xml, core-site.xml) render as Hadoop XML.
	base.ConfigGenerator = opgoconfig.NewMultiFormatConfigGenerator()
	base.ConfigGenerator.RegisterDefaultFormats()

	base.SetRoleContainerPorts(hivev1alpha1.RoleMetastore, []corev1.ContainerPort{
		{
			Name:          hiveconstant.MetastorePortName,
			ContainerPort: hiveconstant.MetastorePort,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          hiveconstant.MetricsPortName,
			ContainerPort: hiveconstant.MetricsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	})
	base.SetRoleServicePorts(hivev1alpha1.RoleMetastore, []corev1.ServicePort{
		{
			Name:       hiveconstant.MetastorePortName,
			Port:       hiveconstant.MetastorePort,
			TargetPort: intstr.FromString(hiveconstant.MetastorePortName),
			Protocol:   corev1.ProtocolTCP,
		},
		{
			Name:       hiveconstant.MetricsPortName,
			Port:       hiveconstant.MetricsPort,
			TargetPort: intstr.FromString(hiveconstant.MetricsPortName),
			Protocol:   corev1.ProtocolTCP,
		},
	})

	return &HiveRoleGroupHandler{BaseRoleGroupHandler: base}
}

// BuildResources delegates the bulk to the framework, then applies the hive-specific pieces.
func (h *HiveRoleGroupHandler) BuildResources(
	ctx context.Context,
	k8sClient ctrlclient.Client,
	cr *hivev1alpha1.HiveMetastore,
	buildCtx *reconciler.RoleGroupBuildContext,
) (*reconciler.RoleGroupResources, error) {
	clusterConfig := cr.Spec.ClusterConfig

	// Resolve the CR-driven image before the framework builds the StatefulSet.
	h.Image = resolveImage(cr.Spec.Image)
	if cr.Spec.Image != nil && cr.Spec.Image.PullPolicy != "" {
		h.ImagePullPolicy = cr.Spec.Image.PullPolicy
	}

	var s3Config *S3Config
	if clusterConfig != nil && clusterConfig.S3 != nil {
		s3Connection, err := GetS3Connection(ctx, k8sClient, cr.GetNamespace(), clusterConfig.S3)
		if err != nil {
			return nil, err
		}
		s3Config = NewS3Config(s3Connection)
		buildCtx.VolumeProviders = append(buildCtx.VolumeProviders, s3Config)
	}

	var krb5Config *KerberosConfig
	if clusterConfig != nil && clusterConfig.Authentication != nil && clusterConfig.Authentication.Kerberos != nil {
		krb5Config = NewKerberosConfig(
			buildCtx.ClusterNamespace,
			buildCtx.ClusterName,
			buildCtx.RoleName,
			clusterConfig.Authentication.Kerberos.SecretClass,
		)
		buildCtx.VolumeProviders = append(buildCtx.VolumeProviders, krb5Config.Provisioner())
	}

	// Contribute the product-computed configuration as the lowest-precedence layer: keys the
	// user already set via configOverrides are left untouched, so CRD overrides always win
	// (the ProductConfig merge semantics, applied here because resolving a referenced
	// S3Connection needs the Kubernetes client).
	hiveSite := map[string]string{
		warehouseDirProperty: resolveWarehouseDir(cr.Spec.Metastore, buildCtx.RoleGroupName),
	}
	if s3Config != nil {
		maps.Copy(hiveSite, s3Config.GetHiveSite())
	}
	if krb5Config != nil {
		maps.Copy(hiveSite, krb5Config.GetHiveSite())
	}
	ensureConfigProperties(buildCtx, HiveSiteFileName, hiveSite)
	if krb5Config != nil {
		// Kerberos without HDFS still needs hadoop-level auth switched on (e.g. S3 warehouse).
		ensureConfigProperties(buildCtx, CoreSiteFileName, krb5Config.GetCoreSite())
	}

	resources, err := h.BaseRoleGroupHandler.BuildResources(ctx, k8sClient, cr, buildCtx)
	if err != nil {
		return nil, err
	}

	h.customizeStatefulSet(resources, cr, s3Config, krb5Config)

	// Prometheus-scrapable metrics Service ("<resource>-metrics"), asserted by the e2e suite.
	resources.MetricsService = builder.NewMetricsServiceBuilder(
		buildCtx.ResourceName,
		buildCtx.ClusterNamespace,
		hiveconstant.MetricsPort,
		resources.StatefulSet.Labels,
	).
		WithSelector(h.SelectorLabels(buildCtx)).
		WithTargetPortName(hiveconstant.MetricsPortName).
		Build()

	return resources, nil
}

// customizeStatefulSet applies the metastore container start script, environment and probes on
// top of the framework-built StatefulSet.
func (h *HiveRoleGroupHandler) customizeStatefulSet(
	resources *reconciler.RoleGroupResources,
	cr *hivev1alpha1.HiveMetastore,
	s3Config *S3Config,
	krb5Config *KerberosConfig,
) {
	sts := resources.StatefulSet
	if sts == nil {
		return
	}

	containers := sts.Spec.Template.Spec.Containers
	for i := range containers {
		if containers[i].Name != hivev1alpha1.RoleMetastore {
			continue
		}
		main := &containers[i]

		main.Command = []string{"sh", "-x", "-euo", "pipefail", "-c"}
		main.Args = []string{h.mainContainerScript(s3Config, krb5Config)}

		// Product defaults first, framework-applied envOverrides last so overrides win.
		main.Env = append(h.mainContainerEnv(cr, krb5Config), main.Env...)

		// Credentials for the metastore database, expanded via $(username)/$(password).
		if db := databaseSpec(cr); db != nil && db.CredentialsSecret != "" {
			main.EnvFrom = append(main.EnvFrom, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: db.CredentialsSecret},
				},
			})
		}

		main.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(hiveconstant.MetastorePortName)},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			FailureThreshold:    5,
		}
		main.LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(hiveconstant.MetastorePortName)},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			FailureThreshold:    5,
		}
		break
	}
}

// mainContainerScript renders the metastore start script. The config ConfigMap is mounted
// read-only, so it is copied to a writable directory first (the Kerberos realm substitution
// rewrites hive-site.xml in place). The metastore is exec'd so it receives SIGTERM directly;
// Vector log shipping is a framework-owned native sidecar and needs no shutdown handshake.
func (h *HiveRoleGroupHandler) mainContainerScript(s3Config *S3Config, krb5Config *KerberosConfig) string {
	steps := []string{
		"mkdir -p " + constant.KubedoopConfigDir,
		"cp -RL " + path.Join(constant.KubedoopConfigDirMount, "*") + " " + constant.KubedoopConfigDir,
	}

	if krb5Config != nil {
		steps = append(steps, krb5Config.GetContainerCommandArgs())
	}
	if s3Config != nil {
		steps = append(steps, s3Config.GetContainerCommandArgs())
	}

	steps = append(steps,
		`DB_TYPE="${DB_DRIVER:-derby}"`,
		"exec bin/start-metastore --config "+constant.KubedoopConfigDir+" --db-type $DB_TYPE --hive-bin-dir bin",
	)

	return strings.Join(steps, "\n")
}

// mainContainerEnv renders the metastore container environment: database JVM options, the
// JMX prometheus agent and, when enabled, the Kerberos client settings.
func (h *HiveRoleGroupHandler) mainContainerEnv(cr *hivev1alpha1.HiveMetastore, krb5Config *KerberosConfig) []corev1.EnvVar {
	database := databaseSpec(cr)

	env := []corev1.EnvVar{
		{
			Name:  "SERVICE_NAME",
			Value: hivev1alpha1.RoleMetastore,
		},
	}
	if database != nil {
		env = append(env,
			corev1.EnvVar{
				Name:  "HADOOP_CLIENT_OPTS",
				Value: strings.Join(databaseJVMOpts(database), " "),
			},
			corev1.EnvVar{
				Name:  "DB_DRIVER",
				Value: database.DatabaseType,
			},
		)
	}

	// HADOOP_OPTS carries the JMX prometheus javaagent plus any Kerberos JVM flags.
	hadoopOpts := []string{
		fmt.Sprintf("-javaagent:%s=%d:%s",
			path.Join(constant.KubedoopJmxDir, "jmx_prometheus_javaagent.jar"),
			hiveconstant.MetricsPort,
			path.Join(constant.KubedoopJmxDir, "config.yaml")),
	}

	if krb5Config != nil {
		for _, e := range krb5Config.GetEnv() {
			if e.Name == hadoopOptsEnvName {
				hadoopOpts = append(hadoopOpts, e.Value)
			} else {
				env = append(env, e)
			}
		}
	}

	env = append(env, corev1.EnvVar{
		Name:  hadoopOptsEnvName,
		Value: strings.Join(hadoopOpts, " "),
	})

	return env
}

// databaseSpec returns the metastore database spec, nil-safe.
func databaseSpec(cr *hivev1alpha1.HiveMetastore) *hivev1alpha1.DatabaseSpec {
	if cr.Spec.ClusterConfig == nil {
		return nil
	}
	return cr.Spec.ClusterConfig.Database
}

// resolveWarehouseDir resolves the warehouse dir with role-group > role > default precedence.
func resolveWarehouseDir(role *hivev1alpha1.RoleSpec, roleGroupName string) string {
	if role != nil {
		if rg, ok := role.RoleGroups[roleGroupName]; ok && rg != nil && rg.Config != nil && rg.Config.WarehouseDir != "" {
			return rg.Config.WarehouseDir
		}
		if role.Config != nil && role.Config.WarehouseDir != "" {
			return role.Config.WarehouseDir
		}
	}
	return hivev1alpha1.DefaultWarehouseDir
}

// ensureConfigProperties merges product-computed properties into the merged config file as the
// lowest-precedence layer: only keys absent from the user's configOverrides are set.
func ensureConfigProperties(buildCtx *reconciler.RoleGroupBuildContext, fileName string, properties map[string]string) {
	if buildCtx.MergedConfig == nil {
		return
	}
	if buildCtx.MergedConfig.ConfigFiles == nil {
		buildCtx.MergedConfig.ConfigFiles = map[string]map[string]string{}
	}
	file := buildCtx.MergedConfig.ConfigFiles[fileName]
	if file == nil {
		file = map[string]string{}
		buildCtx.MergedConfig.ConfigFiles[fileName] = file
	}
	for k, v := range properties {
		if _, exists := file[k]; !exists {
			file[k] = v
		}
	}
}

// resolveImage resolves the metastore container image from the CR spec with the platform
// defaults (repo, product version) and the operator build version as the kubedoop version.
func resolveImage(image *hivev1alpha1.ImageSpec) string {
	if image == nil {
		image = &hivev1alpha1.ImageSpec{}
	}
	if image.Custom != "" {
		return image.Custom
	}

	repo := image.Repo
	if repo == "" {
		repo = hivev1alpha1.DefaultRepository
	}
	productVersion := image.ProductVersion
	if productVersion == "" {
		productVersion = hivev1alpha1.DefaultProductVersion
	}
	kubedoopVersion := image.KubedoopVersion
	if kubedoopVersion == "" {
		kubedoopVersion = version.BuildVersion
	}

	return fmt.Sprintf("%s/%s:%s-kubedoop%s", repo, hivev1alpha1.DefaultProductName, productVersion, kubedoopVersion)
}
