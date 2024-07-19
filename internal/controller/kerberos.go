package controller

import (
	"fmt"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/config"
	"github.com/zncdatadev/operator-go/pkg/util"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return corev1.Volume{
		Name: KrbVolumeName,
		VolumeSource: corev1.VolumeSource{
			Ephemeral: &corev1.EphemeralVolumeSource{
				VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							volume.SecretsZncdataClass:                secretClass,
							volume.SecretsZncdataScope:                fmt.Sprintf("service=%s", instanceName),
							volume.SecretsZncdataKerberosServiceNames: HiveKerberosServiceName + ",HTTP",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: func() *string {
							cs := "secrets.zncdata.dev"
							return &cs
						}(),
						VolumeMode: func() *corev1.PersistentVolumeMode { v := corev1.PersistentVolumeFilesystem; return &v }(),
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	}
}

func KrbVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      KrbVolumeName,
		MountPath: hivev1alpha1.KerberosMountPath,
	}
}

func KrbEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "KRB5_CONFIG",
			Value: fmt.Sprintf("%s/krb5.conf", hivev1alpha1.KerberosMountPath),
		},
		{
			//official docker images jvm args for kerberos
			//https://hub.docker.com/r/apache/hive
			//https://github.com/apache/hive/blob/master/packaging/src/docker/entrypoint.sh
			//https://github.com/apache/hive/blob/master/packaging/src/docker/README.md
			Name:  "SERVICE_OPTS",
			Value: fmt.Sprintf("-Djava.security.krb5.conf=%s/krb5.conf", hivev1alpha1.KerberosMountPath),
		},
	}
}

// KrbCoreSiteXml if kerberos is activated but we have no HDFS as backend (i.e. S3) then a core-site.xml is
// needed to set "hadoop.security.authentication"
func KrbCoreSiteXml() map[string]string {
	xml := util.XmlConfiguration{Properties: []util.XmlNameValuePair{{Name: "hadoop.security.authentication", Value: "kerberos"}}}
	content := xml.String(nil)
	return map[string]string{
		"core-site.xml": content,
	}
}

func ParseKerberosScript(tmpl string, data map[string]interface{}) []string {
	parser := config.TemplateParser{
		Value:    data,
		Template: tmpl,
	}
	if content, err := parser.Parse(); err != nil {
		panic(err)
	} else {
		return []string{content}
	}
}

func CreateKrbScriptData(clusterSpec *hivev1alpha1.ClusterConfigSpec) map[string]interface{} {
	if IsKerberosEnabled(clusterSpec) {
		return map[string]interface{}{
			"kerberosEnabled": true,
			// if hdfs enabled, should exec below:
			//sed -i -e 's/${{env.KERBEROS_REALM}}/'\"$KERBEROS_REALM/g\" {STACKABLE_CONFIG_DIR}/core-site.xml
			//sed -i -e 's/${{env.KERBEROS_REALM}}/'\"$KERBEROS_REALM/g\" {STACKABLE_CONFIG_DIR}/hdfs-site.xml",
			"kerberosScript": `export KERBEROS_REALM=$(grep -oP 'default_realm = \K.*' /zncdata/kerberos/krb5.conf)
sed -i -e 's/${env.KERBEROS_REALM}/'"$KERBEROS_REALM/g" /zncdata/config/hive-site.xml
`,
		}
	}
	return map[string]interface{}{}
}
