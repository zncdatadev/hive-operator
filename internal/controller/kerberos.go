package controller

import (
	"fmt"
	"path"

	"github.com/zncdatadev/operator-go/pkg/constant"
	"github.com/zncdatadev/operator-go/pkg/security"
	corev1 "k8s.io/api/core/v1"
)

const (
	kerberosVolumeName = "kerberos"
	kerberosAuthType   = "kerberos"
	hadoopOptsEnvName  = "HADOOP_OPTS"
)

var Krb5ConfigFile = path.Join(constant.KubedoopKerberosDir, "krb5.conf")

type KerberosConfig struct {
	Namespace   string
	ClusterName string
	RoleName    string

	KerberosSecretClass string
}

func NewKerberosConfig(
	namespace string,
	clusterName string,
	roleName string,
	krb5SecretClass string,
) *KerberosConfig {
	return &KerberosConfig{
		Namespace:           namespace,
		ClusterName:         clusterName,
		RoleName:            roleName,
		KerberosSecretClass: krb5SecretClass,
	}
}

func (c *KerberosConfig) GetHiveSite() map[string]string {
	return map[string]string{
		"hive.metastore.sasl.enabled":              "true",
		"hive.metastore.kerberos.principal":        c.getPrincipal(c.RoleName),
		"hive.metastore.client.kerberos.principal": c.getPrincipal(c.RoleName),
		"hive.metastore.kerberos.keytab.file":      path.Join(constant.KubedoopKerberosDir, "keytab"),
	}
}

// getPrincipal renders the service principal with a ${env.KERBEROS_REALM} placeholder that the
// container start script substitutes at runtime (the realm is only known once the keytab volume
// is mounted; see GetContainerCommandArgs).
func (c *KerberosConfig) getPrincipal(service string) string {
	host := fmt.Sprintf("%s.%s.svc.cluster.local", c.ClusterName, c.Namespace)
	return fmt.Sprintf("%s/%s@${env.KERBEROS_REALM}", service, host)
}

func (c *KerberosConfig) GetCoreSite() map[string]string {
	return map[string]string{
		"hadoop.security.authentication": kerberosAuthType,
	}
}

func (c *KerberosConfig) GetEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "KRB5_CONFIG",
			Value: Krb5ConfigFile,
		},
		{
			Name:  hadoopOptsEnvName,
			Value: fmt.Sprintf("-Djava.security.krb5.conf=%s -Dhive.root.logger=console", Krb5ConfigFile),
		},
	}
}

// Provisioner declares the Kerberos keytab CSI volume. The keytab is scoped to the cluster's
// service address (scope "service=<cluster>") and covers the "<role>" and "HTTP" service names,
// matching the principals rendered into hive-site.xml.
func (c *KerberosConfig) Provisioner() *security.SecretProvisioner {
	return security.NewSecretProvisioner().
		// KubedoopKerberosDir is "<root>/kerberos/": mounting volume "kerberos" under the
		// kubedoop root reproduces it exactly.
		WithMountBasePath(constant.KubedoopRoot).
		Register(
			security.KerberosVolume(kerberosVolumeName, c.KerberosSecretClass, c.RoleName, "HTTP").
				WithScope("service=" + c.ClusterName).
				WithStorageSize("1Mi"),
		)
}

// GetContainerCommandArgs resolves the Kerberos realm from the mounted krb5.conf and substitutes
// the ${env.KERBEROS_REALM} placeholder in the copied (writable) hive-site.xml.
func (c *KerberosConfig) GetContainerCommandArgs() string {
	return `export KERBEROS_REALM=$(grep -oP 'default_realm = \K.*' ` + Krb5ConfigFile + `)
sed -i -e 's/${env.KERBEROS_REALM}/'"$KERBEROS_REALM/g" ` + path.Join(constant.KubedoopConfigDir, HiveSiteFileName)
}
