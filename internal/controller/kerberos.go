package controller

import (
	"fmt"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/config"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	corev1 "k8s.io/api/core/v1"
)

const KrbVolumeName = "kerberos"
const HiveKerberosServiceName = "hive"

func IsKerberosEnabled(clusterSpec *hivev1alpha1.ClusterConfigSpec) bool {
	return clusterSpec != nil && clusterSpec.Authentication != nil && clusterSpec.Authentication.Kerberos != nil
}

func KrbHiveSiteXml(properties map[string]string, instanceName string, ns string) {
	hostPart := PrincipalHostPart(instanceName, ns)
	properties["hive.metastore.kerberos.principal"] = fmt.Sprintf("hive/%s", hostPart)
	properties["hive.metastore.client.kerberos.principal"] = fmt.Sprintf("hive/%s", hostPart)
	properties["hive.metastore.kerberos.keytab.file"] = fmt.Sprintf("%s/keytab", hivev1alpha1.KerberosMountPath)
	properties["hive.metastore.sasl.enabled"] = "true"
}

func PrincipalHostPart(instanceName string, ns string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local@${env.KERBEROS_REALM}", instanceName, ns)
}

func KrbVolume(secretClass string, instanceName string) corev1.Volume {
	return SecretVolume(map[string]string{
		volume.SecretsZncdataClass:                secretClass,
		volume.SecretsZncdataScope:                fmt.Sprintf("service=%s", instanceName),
		volume.SecretsZncdataKerberosServiceNames: HiveKerberosServiceName + ",HTTP",
	}, KrbVolumeName)
}

func KrbVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      KrbVolumeName,
		MountPath: hivev1alpha1.KerberosMountPath,
	}
}

func KrbEnv(envs []corev1.EnvVar) []corev1.EnvVar {
	envs = append(envs, corev1.EnvVar{
		Name:  "KRB5_CONFIG",
		Value: fmt.Sprintf("%s/krb5.conf", hivev1alpha1.KerberosMountPath),
	})

	jvmKrbConfigArgs := fmt.Sprintf("-Djava.security.krb5.conf=%s/krb5.conf -Dhive.root.logger=console", hivev1alpha1.KerberosMountPath)
	serviceOptsExists := false
	// Check if envs contains the Name=SERVICE_OPTS item
	for i, env := range envs {
		if env.Name == "SERVICE_OPTS" {
			serviceOptsExists = true
			// Append the specified value to the existing SERVICE_OPTS item
			envs[i].Value += " " + jvmKrbConfigArgs
			break
		}
	}
	// If SERVICE_OPTS item doesn't exist, add it with the specified value
	if !serviceOptsExists {
		envs = append(envs, corev1.EnvVar{
			Name:  "SERVICE_OPTS",
			Value: jvmKrbConfigArgs,
		})
	}
	return envs
}

// KrbCoreSiteXml if kerberos is activated but we have no HDFS as backend (i.e. S3) then a core-site.xml is
// needed to set "hadoop.security.authentication"
func KrbCoreSiteXml(coreSiteProperties map[string]string) {
	coreSiteProperties["hadoop.security.authentication"] = "kerberos"
}

func ParseKerberosScript(tmpl string, data map[string]interface{}) []string {
	parser := config.TemplateParser{
		Value:    data,
		Template: tmpl,
	}
	if content, err := parser.Parse(); err != nil {
		panic(err)
	} else {
		fmt.Println(content)
		return []string{content}
	}
}

func CreateKrbScriptData(clusterSpec *hivev1alpha1.ClusterConfigSpec) map[string]interface{} {
	if IsKerberosEnabled(clusterSpec) {
		return map[string]interface{}{
			"kerberosEnabled": true,
			// if hdfs enabled, should exec below:
			//sed -i -e 's/${{env.KERBEROS_REALM}}/'\"$KERBEROS_REALM/g\" /kubedoop/config/core-site.xml
			//sed -i -e 's/${{env.KERBEROS_REALM}}/'\"$KERBEROS_REALM/g\" /kubedoop/config/hdfs-site.xml",
			"kerberosScript": fmt.Sprintf(`export KERBEROS_REALM=$(grep -oP 'default_realm = \K.*' %s/krb5.conf)
sed -i -e 's/${env.KERBEROS_REALM}/'"$KERBEROS_REALM/g" %s/hive-site.xml
`, hivev1alpha1.KerberosMountPath, hivev1alpha1.KubeDataConfigDir),
		}
	}
	return map[string]interface{}{}
}
