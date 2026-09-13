package controller

import (
	"context"
	"maps"
	"path"
	"strings"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	opgoconfig "github.com/zncdatadev/operator-go/pkg/config"
	"github.com/zncdatadev/operator-go/pkg/constant"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
// overrides applied by the framework) and the role PDB. What hive adds is the hive-site/core-site
// content, the metastore start script, the database/S3/Kerberos wiring and the metrics Service.
type HiveRoleGroupHandler struct {
	*reconciler.BaseRoleGroupHandler[*hivev1alpha1.HiveMetastore]
}

var (
	_ reconciler.RoleGroupHandler[*hivev1alpha1.HiveMetastore]  = &HiveRoleGroupHandler{}
	_ reconciler.RoleProvider[*hivev1alpha1.HiveMetastore]      = &HiveRoleGroupHandler{}
	_ reconciler.RoleGroupResolver[*hivev1alpha1.HiveMetastore] = &HiveRoleGroupHandler{}
)

// ImageDefaults supplies what the CR's spec.image leaves empty. It is re-evaluated every
// reconcile — which is why KubedoopVersion can be the operator's own build version, so an
// operator upgrade moves existing clusters onto the co-released product image. Kubedoop
// publishes Hive images only with the "-kubedoop<version>" suffix, so that field must always
// resolve to something.
func ImageDefaults() commonsv1alpha1.ImageSpec {
	return commonsv1alpha1.ImageSpec{
		Repo:            hivev1alpha1.DefaultRepository,
		ProductVersion:  hivev1alpha1.DefaultProductVersion,
		KubedoopVersion: version.BuildVersion,
	}
}

// NewHiveRoleGroupHandler creates the handler. Only reconcile-invariant options live here;
// everything a role is made of is declared per pass in DeclareRoles.
func NewHiveRoleGroupHandler(scheme *runtime.Scheme) *HiveRoleGroupHandler {
	base := reconciler.NewBaseRoleGroupHandler[*hivev1alpha1.HiveMetastore](scheme)

	// configOverrides for *.xml files (hive-site.xml, core-site.xml) render as Hadoop XML.
	base.ConfigGenerator = opgoconfig.NewMultiFormatConfigGenerator()
	base.ConfigGenerator.RegisterDefaultFormats()

	return &HiveRoleGroupHandler{BaseRoleGroupHandler: base}
}

// DeclareRoles implements reconciler.RoleProvider: everything the metastore role is made of,
// produced once per reconcile pass with the CR in hand.
//
// The start script is part of the declaration because it depends on the cluster's S3 and Kerberos
// configuration, which this hook can resolve — it receives the client. Declaring it here rather
// than editing the built container is what keeps a user's podOverrides on top: the framework
// applies declarations first and strategic-merges the user's overrides afterwards.
func (h *HiveRoleGroupHandler) DeclareRoles(
	ctx context.Context,
	k8sClient client.Client,
	cr *hivev1alpha1.HiveMetastore,
) (reconciler.RoleCatalog, error) {
	s3Config, krb5Config, err := h.resolveStorageAndAuth(ctx, k8sClient, cr, hivev1alpha1.RoleMetastore)
	if err != nil {
		return nil, err
	}

	return reconciler.RoleCatalog{
		hivev1alpha1.RoleMetastore: {
			// The container name must match the per-container logging key
			// (logging.containers.metastore) and is asserted by the e2e suite.
			MainContainerName: hivev1alpha1.RoleMetastore,
			// The metastore port comes first: ContainerPorts[0] backs the framework's generated
			// TCP readiness probe, and it is the port that means "this pod can serve".
			ContainerPorts: []corev1.ContainerPort{
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
			},
			ServicePorts: []corev1.ServicePort{
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
			},
			// The entrypoint carries the script inline: arguments are not a declaration field,
			// because cliOverrides is the user's channel for them.
			//
			// Deliberately no `-x`: the script exports S3 credentials read from the mounted
			// files, and xtrace would echo the expanded secret values into the container log.
			Command: append([]string{"sh", "-euo", "pipefail", "-c"},
				h.mainContainerScript(s3Config, krb5Config)),
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(hiveconstant.MetastorePortName)},
				},
				InitialDelaySeconds: 10,
				PeriodSeconds:       10,
				FailureThreshold:    5,
			},
			// The framework declines to guess a liveness probe for the product's container, so
			// hive states its own: a TCP check on the metastore port, with a start budget that
			// clears schema initialisation on a cold database.
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(hiveconstant.MetastorePortName)},
				},
				InitialDelaySeconds: 30,
				PeriodSeconds:       10,
				FailureThreshold:    5,
			},
			// Declarative logging: the framework renders the log4j2 config into the ConfigMap
			// under the hive-conventional key so the metastore start scripts pick it up from the
			// config directory, and mounts the shared Vector log volume on this container.
			LogProducers: []productlogging.ContainerLogging{
				{
					Container: hivev1alpha1.RoleMetastore,
					Framework: productlogging.LoggingFrameworkLog4j2,
					FileName:  LogConfigFileName,
					Pattern:   ConsoleConversionPattern,
				},
			},
			Env: h.mainContainerEnv(cr, krb5Config),
		},
	}, nil
}

// ResolveRoleGroup implements reconciler.RoleGroupResolver: the values that follow from a role
// group's EFFECTIVE config, once the CR's role and role group levels have been folded into one
// answer. hive's listener class is user-settable, so it can only be applied here — a declaration
// is fixed before the fold and would beat the user.
func (h *HiveRoleGroupHandler) ResolveRoleGroup(
	_ context.Context,
	_ client.Client,
	cr *hivev1alpha1.HiveMetastore,
	_ *reconciler.RoleGroupBuildContext,
) (*reconciler.Contribution, error) {
	contribution := &reconciler.Contribution{}

	if cr.Spec.ClusterConfig != nil && cr.Spec.ClusterConfig.ListenerClass != "" {
		contribution.ListenerClass = cr.Spec.ClusterConfig.ListenerClass
	}

	// The metastore reads its database user and password from the credentials Secret, which the
	// JDBC options reference as $(username)/$(password). envFrom is not expressible through the
	// declaration or the map-shaped override channels, so it travels as a product pod-template
	// layer folded beneath the user's own podOverrides.
	if db := databaseSpec(cr); db != nil && db.CredentialsSecret != "" {
		contribution.PodOverrides = &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: hivev1alpha1.RoleMetastore,
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: db.CredentialsSecret},
						},
					}},
				}},
			},
		}
	}

	return contribution, nil
}

// resolveStorageAndAuth resolves the cluster's S3 connection and Kerberos configuration, both of
// which shape the start script, the hive-site properties and the CSI volumes.
func (h *HiveRoleGroupHandler) resolveStorageAndAuth(
	ctx context.Context,
	k8sClient client.Client,
	cr *hivev1alpha1.HiveMetastore,
	roleName string,
) (*S3Config, *KerberosConfig, error) {
	clusterConfig := cr.Spec.ClusterConfig
	if clusterConfig == nil {
		return nil, nil, nil
	}

	var s3Config *S3Config
	if clusterConfig.S3 != nil {
		resolved, err := ResolveS3Config(ctx, k8sClient, cr.GetNamespace(), clusterConfig.S3)
		if err != nil {
			return nil, nil, err
		}
		s3Config = resolved
	}

	var krb5Config *KerberosConfig
	if clusterConfig.Authentication != nil && clusterConfig.Authentication.Kerberos != nil {
		krb5Config = NewKerberosConfig(
			cr.GetNamespace(),
			cr.GetName(),
			roleName,
			clusterConfig.Authentication.Kerberos.SecretClass,
		)
	}

	return s3Config, krb5Config, nil
}

// BuildResources delegates the bulk to the framework, then applies the hive-specific pieces.
func (h *HiveRoleGroupHandler) BuildResources(
	ctx context.Context,
	k8sClient client.Client,
	cr *hivev1alpha1.HiveMetastore,
	buildCtx *reconciler.RoleGroupBuildContext,
) (*reconciler.RoleGroupResources, error) {
	// The image, its pull policy and the app.kubernetes.io/{name,version} labels are all derived
	// by the framework from spec.image and GenericReconcilerConfig.ImageResolution. Nothing per-CR
	// is written onto the handler here: one handler instance serves every HiveMetastore, so a
	// field written during BuildResources would leak into — or race with — another reconcile.
	s3Config, krb5Config, err := h.resolveStorageAndAuth(ctx, k8sClient, cr, buildCtx.RoleName)
	if err != nil {
		return nil, err
	}

	// Hand the CSI volumes to the framework so it injects them into the pod and the main
	// container. VolumeProviders lives on the build context, rebuilt each reconcile, so
	// registrations never accumulate or leak across CRs.
	if s3Config != nil {
		// An anonymous connection has no credentials volume to mount.
		if provisioner := s3Config.CredentialsProvisioner(); provisioner != nil {
			buildCtx.VolumeProviders = append(buildCtx.VolumeProviders, provisioner)
		}
	}
	if krb5Config != nil {
		buildCtx.VolumeProviders = append(buildCtx.VolumeProviders, krb5Config.Provisioner())
	}

	// Contribute the product-computed configuration as the lowest-precedence layer: keys the
	// user already set via configOverrides are left untouched, so CRD overrides always win.
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

	// HADOOP_OPTS carries the JMX prometheus javaagent plus any Kerberos JVM flags. The agent
	// runs inside the metastore's own JVM and serves the metrics port the Service scrapes.
	hadoopOpts := []string{
		constant.JMXJavaAgentOpt(hiveconstant.MetricsPort, "config.yaml"),
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
