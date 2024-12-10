package controller

import (
	"fmt"
	"path"
	"strings"

	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Krb5ConfigFile = path.Join(constants.KubedoopKerberosDir, "krb5.conf")
)

type KerberosConfig struct {
	Namespace   string
	ClusterName string
	RoleName    string

	KerberosSecretClass string
	HdfsEnabled         bool
}

func NewKerberosConfig(
	namespace string,
	clustername string,
	rolename string,
	krb5SecretClass string,
) *KerberosConfig {
	return &KerberosConfig{
		Namespace:           namespace,
		ClusterName:         clustername,
		RoleName:            rolename,
		KerberosSecretClass: krb5SecretClass,
	}
}

func (c *KerberosConfig) GetHiveSite() map[string]string {
	return map[string]string{
		"hive.metastore.sasl.enabled":              "true",
		"hive.metastore.kerberos.principal":        c.getPrincipal(c.RoleName),
		"hive.metastore.client.kerberos.principal": c.getPrincipal(c.RoleName),
		"hive.metastore.kerberos.keytab.file":      path.Join(constants.KubedoopKerberosDir, "keytab"),
	}
}

func (c *KerberosConfig) getPrincipal(service string) string {
	host := fmt.Sprintf("%s.%s.svc.cluster.local", c.ClusterName, c.Namespace)
	return fmt.Sprintf("%s/%s@${env.KERBEROS_REALM}", service, host)
}

func (c *KerberosConfig) GetCoreSite() map[string]string {
	return map[string]string{
		"hadoop.security.authentication": "kerberos",
	}
}

func (c *KerberosConfig) GetEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "KRB5_CONFIG",
			Value: Krb5ConfigFile,
		},
		{
			Name:  "HADOOP_OPTS",
			Value: fmt.Sprintf("-Djava.security.krb5.conf=%s/krb5.conf -Dhive.root.logger=console", constants.KubedoopKerberosDir),
		},
	}
}

func (c *KerberosConfig) GetVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "kerberos",
			VolumeSource: corev1.VolumeSource{
				Ephemeral: &corev1.EphemeralVolumeSource{
					VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								constants.AnnotationSecretsClass:                c.KerberosSecretClass,
								constants.AnnotationSecretsScope:                fmt.Sprintf("service=%s", c.ClusterName),
								constants.AnnotationSecretsKerberosServiceNames: strings.Join([]string{c.RoleName, "HTTP"}, constants.CommonDelimiter),
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							StorageClassName: constants.SecretStorageClassPtr(),
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (c *KerberosConfig) GetVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "kerberos",
			MountPath: constants.KubedoopKerberosDir,
		},
	}
}

func (c *KerberosConfig) GetContainerCommandArgs() string {
	cmds := `
export KERBEROS_REALM=$(grep -oP 'default_realm = \K.*' ` + Krb5ConfigFile + `)
sed -i -e 's/${env.KERBEROS_REALM}/'"$KERBEROS_REALM/g"  ` + path.Join(constants.KubedoopConfigDir, "hive-site.xml") + `
`

	if c.HdfsEnabled {
		cmds += `
sed -i -e 's/${env.KERBEROS_REALM}/'"$KERBEROS_REALM/g"  ` + path.Join(constants.KubedoopConfigDir, "core-site.xml") + `
sed -i -e 's/${env.KERBEROS_REALM}/'"$KERBEROS_REALM/g"  ` + path.Join(constants.KubedoopConfigDir, "hdfs-site.xml") + `
`
	}

	return util.IndentTab4Spaces(cmds)
}
