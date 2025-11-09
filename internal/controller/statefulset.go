package controller

import (
	"context"
	"fmt"
	"path"
	"strings"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	client "github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"github.com/zncdatadev/operator-go/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/hive-operator/internal/constant"
)

var (
	MatestoreConfigmapVolumeName = "mount-config" // configmap > volume > mount

	MatestoreLogVolumeName = "log"
	MetastorePortName      = "metastore"

	ContainerPort = []corev1.ContainerPort{
		{
			ContainerPort: constant.MetastorePort,
			Protocol:      corev1.ProtocolTCP,
			Name:          constant.MetastorePortName,
		},
		{
			ContainerPort: constant.MetricsPort,
			Protocol:      corev1.ProtocolTCP,
			Name:          constant.MetricsPortName,
		},
	}
)

var _ builder.StatefulSetBuilder = &StatefulSetBuilder{}

type StatefulSetBuilder struct {
	builder.StatefulSet
	ClusterConfig *hivev1alpha1.ClusterConfigSpec
}

func NewStatefulSetBuilder(
	client *client.Client,
	name string,
	clusterConfig *hivev1alpha1.ClusterConfigSpec,
	replicas *int32,
	image *util.Image,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
	options ...builder.Option,
) *StatefulSetBuilder {

	opts := builder.Options{}
	for _, o := range options {
		o(&opts)
	}

	return &StatefulSetBuilder{
		StatefulSet: *builder.NewStatefulSetBuilder(
			client,
			name,
			replicas,
			image,
			overrides,
			roleGroupConfig,
			options...,
		),
		ClusterConfig: clusterConfig,
	}
}

func (b *StatefulSetBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	var s3Config *S3Config
	if b.ClusterConfig.S3 != nil {
		s3Connection, err := GetS3Connect(ctx, b.Client, b.ClusterConfig.S3)
		if err != nil {
			return nil, err
		}
		s3Config = NewS3Config(s3Connection)
	}

	var kerberosConfig *KerberosConfig
	if b.ClusterConfig.Authentication != nil {
		kerberosConfig = NewKerberosConfig(
			b.Client.GetOwnerNamespace(),
			b.ClusterName,
			b.RoleName,
			b.ClusterConfig.Authentication.Kerberos.SecretClass,
		)
	}

	b.AddContainer(b.getMainContainer(kerberosConfig, s3Config).Build())
	b.AddVolumes(b.getVolumes(s3Config, kerberosConfig))

	obj, err := b.GetObject()
	if err != nil {
		return nil, err
	}

	b.setupVector(obj)

	return obj, nil
}

func (b *StatefulSetBuilder) setupVector(obj *appsv1.StatefulSet) {
	if b.RoleGroupConfig != nil && b.RoleGroupConfig.Logging != nil && *b.RoleGroupConfig.Logging.EnableVectorAgent {
		vectorFactory := builder.NewVector(
			MatestoreConfigmapVolumeName,
			MatestoreLogVolumeName,
			b.GetImage())
		obj.Spec.Template.Spec.Containers = append(
			obj.Spec.Template.Spec.Containers,
			*vectorFactory.GetContainer(),
		)
		obj.Spec.Template.Spec.Volumes = append(
			obj.Spec.Template.Spec.Volumes,
			vectorFactory.GetVolumes()...,
		)
	}
}

func (b *StatefulSetBuilder) getMainContainer(krb5Config *KerberosConfig, s3Config *S3Config) *builder.Container {
	container := builder.NewContainer(
		b.RoleName,
		b.GetImage(),
	)
	container.SetCommand([]string{"sh", "-x", "-euo", "pipefail", "-c"}).
		SetArgs(b.getMainContainerCommandArgs(krb5Config, s3Config)).
		AddEnvVars(b.getMainContainerEnv(krb5Config)).
		AddEnvFromSecret(b.ClusterConfig.Database.CredentialsSecret).
		AddPorts(ContainerPort).
		AddVolumeMounts(b.getMainContainerVolumeMounts(s3Config, krb5Config)).
		SetReadinessProbe(&corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromString(MetastorePortName),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			FailureThreshold:    5,
		}).
		SetLivenessProbe(&corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromString(MetastorePortName),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			FailureThreshold:    5,
		})

	return container
}

func (b *StatefulSetBuilder) getMainContainerCommandArgs(krb5Config *KerberosConfig, S3Config *S3Config) []string {
	shutdownFile := path.Join(constants.KubedoopLogDir, "_vector", "shutdown")
	args := []string{
		`
mkdir -p ` + constants.KubedoopConfigDir + `
cp -RL ` + path.Join(constants.KubedoopConfigDirMount, "*") + ` ` + path.Join(constants.KubedoopConfigDir) + `
`,
	}

	if krb5Config != nil {
		args = append(args, krb5Config.GetContainerCommandArgs())
	}

	if S3Config != nil {
		args = append(args, S3Config.GetContainerCommandArgs())
	}
	args = append(
		args,
		util.CommonBashTrapFunctions,
		`
rm -f `+shutdownFile+`
`+util.InvokePrepareSignalHandlers+`
DB_TYPE="${DB_DRIVER:-derby}"
bin/start-metastore --config `+constants.KubedoopConfigDir+` --db-type $DB_TYPE --hive-bin-dir bin &
`+util.InvokeWaitForTermination+`

`+util.CreateVectorShutdownFileCommand()+`
`,
	)

	return []string{strings.Join(args, "\n")}
}

func (b *StatefulSetBuilder) getJVMOpts(
	envs []corev1.EnvVar,
) corev1.EnvVar {
	jvmOpt := []string{
		fmt.Sprintf("-javaagent:%s=%d:%s",
			path.Join(constants.KubedoopJmxDir, "jmx_prometheus_javaagent.jar"),
			constant.MetricsPort,
			path.Join(constants.KubedoopJmxDir, "config.yaml")),
	}

	for _, env := range envs {
		if env.Name == "HADOOP_OPTS" {
			jvmOpt = append(jvmOpt, env.Value)
		}
	}

	return corev1.EnvVar{
		Name:  "HADOOP_OPTS",
		Value: strings.Join(jvmOpt, " "),
	}
}

func (b *StatefulSetBuilder) getMainContainerEnv(krb5Config *KerberosConfig) []corev1.EnvVar {

	jvmOpts := []string{}
	// database is required in ClusterConfig
	database := b.ClusterConfig.Database

	switch database.DatabaseType {
	case "mysql":
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL="+database.ConnString,
			"-Djavax.jdo.option.ConnectionDriverName=com.mysql.cj.jdbc.Driver")
	case "postgres":
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL="+database.ConnString,
			"-Djavax.jdo.option.ConnectionDriverName=org.postgresql.Driver")
	case "oracle":
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL="+database.ConnString,
			"-Djavax.jdo.option.ConnectionDriverName=oracle.jdbc.OracleDriver")
	case "derby":
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL="+database.ConnString,
			"-Djavax.jdo.option.ConnectionDriverName=org.apache.derby.jdbc.EmbeddedDriver")
	default:
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL=jdbc:derby:/tmp/metastore_db;create=true",
			"-Djavax.jdo.option.ConnectionDriverName=org.apache.derby.jdbc.EmbeddedDriver")
	}

	if database.DatabaseType != "derby" {
		// pass by env secret from database.credentials
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionUserName=$(username)",
			"-Djavax.jdo.option.ConnectionPassword=$(password)",
		)
	}

	env := []corev1.EnvVar{
		{
			Name:  "SERVICE_NAME",
			Value: "metastore",
		},
		{
			Name:  "HADOOP_CLIENT_OPTS",
			Value: strings.Join(jvmOpts, " "),
		},
		{
			Name:  "DB_DRIVER",
			Value: database.DatabaseType,
		},
	}

	jvmEnvs := make([]corev1.EnvVar, 0)

	if krb5Config != nil {
		krb5Envs := krb5Config.GetEnv()
		for _, e := range krb5Envs {
			if e.Name == "HADOOP_OPTS" {
				jvmEnvs = append(jvmEnvs, e)
			} else {
				env = append(env, e)
			}
		}
	}

	env = append(env, b.getJVMOpts(jvmEnvs))

	return env
}

func (b *StatefulSetBuilder) getVolumes(s3Config *S3Config, krb5Cofig *KerberosConfig) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: MatestoreConfigmapVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: b.Name,
					},
				},
			},
		},
		{
			Name: MatestoreLogVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: ptr.To(resource.MustParse("10Mi")),
				},
			},
		},
	}

	if s3Config != nil {
		volumes = append(volumes, s3Config.GetVolumes()...)
	}

	if krb5Cofig != nil {
		volumes = append(volumes, krb5Cofig.GetVolumes()...)
	}

	return volumes
}

func (b *StatefulSetBuilder) getMainContainerVolumeMounts(s3Config *S3Config, krb5Cofig *KerberosConfig) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      MatestoreConfigmapVolumeName,
			MountPath: constants.KubedoopConfigDirMount,
		},
		{
			Name:      MatestoreLogVolumeName,
			MountPath: constants.KubedoopLogDir,
		},
	}

	if s3Config != nil {
		volumeMounts = append(volumeMounts, s3Config.GetVolumeMounts()...)
	}

	if krb5Cofig != nil {
		volumeMounts = append(volumeMounts, krb5Cofig.GetVolumeMounts()...)
	}

	return volumeMounts
}

func NewStatefulSetReconciler(
	client *client.Client,
	roleGroupInfo reconciler.RoleGroupInfo,
	clusterConfig *hivev1alpha1.ClusterConfigSpec,
	ports []corev1.ContainerPort,
	image *util.Image,
	replicas *int32,
	stopped bool,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
	options ...builder.Option,
) (*reconciler.StatefulSet, error) {

	b := NewStatefulSetBuilder(
		client,
		roleGroupInfo.GetFullName(),
		clusterConfig,
		replicas,
		image,
		overrides,
		roleGroupConfig,
		options...,
	)

	return reconciler.NewStatefulSet(
		client,
		b,
		stopped,
	), nil
}
