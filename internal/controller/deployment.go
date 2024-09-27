package controller

import (
	"path"
	"strings"
	"time"

	"github.com/zncdatadev/operator-go/pkg/builder"
	client "github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"github.com/zncdatadev/operator-go/pkg/util"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

var (
	MatestoreConfigmapVolumeName = "mount-config" // configmap > volume > mount

	MatestoreLogVolumeName = "log"
	MetastorePortName      = "metastore"

	ContainerPort = []corev1.ContainerPort{
		{
			ContainerPort: 9083,
			Protocol:      corev1.ProtocolTCP,
			Name:          MetastorePortName,
		},
	}
)

type DeploymentBUilderOption struct {
	ClusterName   string
	RoleName      string
	RoleGroupName string
	Labels        map[string]string
	Annotations   map[string]string
}

var _ builder.DeploymentBuilder = &DeploymentBuilder{}

// TODO: Add Vector when vector bug fix
type DeploymentBuilder struct {
	builder.Deployment
	Ports         []corev1.ContainerPort
	ClusterName   string
	RoleName      string
	ClusterConfig *hivev1alpha1.ClusterConfigSpec
}

func NewDeploymentBuilder(
	client *client.Client,
	name string,
	clusterConfig *hivev1alpha1.ClusterConfigSpec,
	replicas *int32,
	image *util.Image,
	options builder.WorkloadOptions,
) *DeploymentBuilder {

	return &DeploymentBuilder{
		Deployment: *builder.NewDeployment(
			client,
			name,
			replicas,
			image,
			options,
		),
		// TODO: Add the ports
		RoleName:      options.RoleName,
		ClusterName:   options.ClusterName,
		ClusterConfig: clusterConfig,
	}
}

func (b *DeploymentBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
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

	return b.GetObject()
}

func (b *DeploymentBuilder) getMainContainer(krb5Config *KerberosConfig, s3Config *S3Config) *builder.Container {
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
		})

	return container
}

func (b *DeploymentBuilder) getMainContainerCommandArgs(krb5Config *KerberosConfig, S3Config *S3Config) []string {
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

func (b *DeploymentBuilder) getJVMOpts(
	envs []corev1.EnvVar,
) corev1.EnvVar {
	jvmOpt := []string{
		"-javaagent:" + path.Join(constants.KubedoopJmxDir, "jmx_prometheus_javaagent.jar") + "=8080:" + path.Join(constants.KubedoopJmxDir, "config.yaml"),
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

func (b *DeploymentBuilder) getMainContainerEnv(krb5Config *KerberosConfig) []corev1.EnvVar {

	jvmOpts := []string{}
	// database is required in ClusterConfig
	database := b.ClusterConfig.Database

	switch database.DatabaseType {
	case "mysql":
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL="+database.ConnectionString,
			"-Djavax.jdo.option.ConnectionDriverName=com.mysql.cj.jdbc.Driver")
	case "postgres":
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL="+database.ConnectionString,
			"-Djavax.jdo.option.ConnectionDriverName=org.postgresql.Driver")
	case "oracle":
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL="+database.ConnectionString,
			"-Djavax.jdo.option.ConnectionDriverName=oracle.jdbc.OracleDriver")
	case "derby":
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionURL="+database.ConnectionString,
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

// func (b *DeploymentBuilder) getVectorContainer() *builder.Container {
// 	panic("not implemented")
// }

func (b *DeploymentBuilder) getVolumes(s3Config *S3Config, krb5Cofig *KerberosConfig) []corev1.Volume {
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

func (b *DeploymentBuilder) getMainContainerVolumeMounts(s3Config *S3Config, krb5Cofig *KerberosConfig) []corev1.VolumeMount {
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

func NewDeploymentReconciler(
	client *client.Client,
	roleGroupInfo reconciler.RoleGroupInfo,
	clusterConfig *hivev1alpha1.ClusterConfigSpec,
	ports []corev1.ContainerPort,
	image *util.Image,
	stopped bool,
	spec *hivev1alpha1.RoleGroupSpec,
) (*reconciler.Deployment, error) {
	options := builder.WorkloadOptions{
		Options: builder.Options{
			ClusterName:   roleGroupInfo.ClusterName,
			RoleName:      roleGroupInfo.RoleName,
			RoleGroupName: roleGroupInfo.RoleGroupName,
			Labels:        roleGroupInfo.GetLabels(),
			Annotations:   roleGroupInfo.GetAnnotations(),
		},
		// PodOverrides:     spec.PodOverrides,
		EnvOverrides:     spec.EnvOverrides,
		CommandOverrides: spec.CommandOverrides,
	}

	if spec.Config != nil {
		if spec.Config.GracefulShutdownTimeout != nil {
			if gracefulShutdownTimeout, err := time.ParseDuration(*spec.Config.GracefulShutdownTimeout); err != nil {
				return nil, err
			} else {
				options.TerminationGracePeriod = &gracefulShutdownTimeout
			}

		}

		options.Affinity = spec.Config.Affinity
		options.Resource = spec.Config.Resources
	}

	b := NewDeploymentBuilder(
		client,
		roleGroupInfo.GetFullName(),
		clusterConfig,
		&spec.Replicas,
		image,
		options,
	)

	return reconciler.NewDeployment(
		client,
		roleGroupInfo.GetFullName(),
		b,
		stopped,
	), nil
}
