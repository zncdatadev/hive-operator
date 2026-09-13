package controller

import (
	"context"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	opgoconfig "github.com/zncdatadev/operator-go/pkg/config"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgos3 "github.com/zncdatadev/operator-go/pkg/s3"
	"github.com/zncdatadev/operator-go/pkg/testutil"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/hive-operator/internal/util/version"
)

// Shared fixture values across the controller test suite.
const (
	testNamespace         = "default"
	testRoleGroup         = "default"
	testCredentialsSecret = "hive-credentials"
	testDerbyConnString   = "jdbc:derby:;databaseName=/tmp/hive;create=true"
)

func mustURL(scheme, host string) url.URL {
	return url.URL{Scheme: scheme, Host: host}
}

var _ = Describe("databaseJVMOpts", func() {
	It("renders postgres connection options with secret-expanded credentials", func() {
		opts := databaseJVMOpts(&hivev1alpha1.DatabaseSpec{
			ConnString:   "jdbc:postgresql://hive-postgres:5432/hive",
			DatabaseType: "postgres",
		})
		Expect(opts).To(Equal([]string{
			"-Djavax.jdo.option.ConnectionURL=jdbc:postgresql://hive-postgres:5432/hive",
			"-Djavax.jdo.option.ConnectionDriverName=org.postgresql.Driver",
			"-Djavax.jdo.option.ConnectionUserName=$(username)",
			"-Djavax.jdo.option.ConnectionPassword=$(password)",
		}))
	})

	It("omits credentials for derby", func() {
		opts := databaseJVMOpts(&hivev1alpha1.DatabaseSpec{
			ConnString:   testDerbyConnString,
			DatabaseType: databaseTypeDerby,
		})
		Expect(opts).To(HaveLen(2))
		Expect(opts[1]).To(Equal("-Djavax.jdo.option.ConnectionDriverName=org.apache.derby.jdbc.EmbeddedDriver"))
	})

	It("falls back to an embedded derby store for unknown types", func() {
		opts := databaseJVMOpts(&hivev1alpha1.DatabaseSpec{DatabaseType: "sqlite"})
		Expect(opts[0]).To(ContainSubstring("jdbc:derby:/tmp/metastore_db;create=true"))
	})
})

var _ = Describe("resolveWarehouseDir", func() {
	role := func(roleDir, groupDir string) *hivev1alpha1.RoleSpec {
		r := &hivev1alpha1.RoleSpec{
			RoleGroups: map[string]*hivev1alpha1.RoleGroupSpec{testRoleGroup: {}},
		}
		if roleDir != "" {
			r.Config = &hivev1alpha1.ConfigSpec{WarehouseDir: roleDir}
		}
		if groupDir != "" {
			r.RoleGroups[testRoleGroup].Config = &hivev1alpha1.ConfigSpec{WarehouseDir: groupDir}
		}
		return r
	}

	It("prefers the role group value over the role value", func() {
		Expect(resolveWarehouseDir(role("/role", "/group"), testRoleGroup)).To(Equal("/group"))
	})

	It("falls back to the role value", func() {
		Expect(resolveWarehouseDir(role("/role", ""), testRoleGroup)).To(Equal("/role"))
	})

	It("defaults when nothing is set", func() {
		Expect(resolveWarehouseDir(role("", ""), testRoleGroup)).To(Equal(hivev1alpha1.DefaultWarehouseDir))
		Expect(resolveWarehouseDir(nil, testRoleGroup)).To(Equal(hivev1alpha1.DefaultWarehouseDir))
	})

	// A group that declares `config` for an unrelated reason must still inherit the role value.
	It("inherits the role value when the group declares config without a warehouse dir", func() {
		r := role("/role", "")
		r.RoleGroups[testRoleGroup].Config = &hivev1alpha1.ConfigSpec{}
		Expect(resolveWarehouseDir(r, testRoleGroup)).To(Equal("/role"))
	})
})

// A +kubebuilder:default on a leaf inside `config` is stamped by structural defaulting into
// every role group that declares that block for any reason, so the role's value can never be
// inherited — warehouseDir had exactly that defect. The framework's guard checks the generated
// schema statically, covering every field rather than the one this operator tripped over.
var _ = Describe("CRD schema", func() {
	It("declares no default inside a role config block", func() {
		Expect("../../config/crd/bases/*.yaml").To(testutil.HaveNoInheritedConfigDefaults())
	})
})

var _ = Describe("warehouseDir role-level inheritance", func() {
	It("inherits the role value through a real API server round trip", func() {
		ctx := context.Background()
		cr := &hivev1alpha1.HiveMetastore{
			ObjectMeta: metav1.ObjectMeta{Name: "warehouse-defaulting", Namespace: testNamespace},
			Spec: hivev1alpha1.HiveMetastoreSpec{
				ClusterConfig: &hivev1alpha1.ClusterConfigSpec{
					Database: &hivev1alpha1.DatabaseSpec{
						ConnString:        "jdbc:derby:;databaseName=/tmp/metastore_db;create=true",
						DatabaseType:      databaseTypeDerby,
						CredentialsSecret: testCredentialsSecret,
					},
				},
				Metastore: &hivev1alpha1.RoleSpec{
					Config: &hivev1alpha1.ConfigSpec{WarehouseDir: "/role-level/warehouse"},
					RoleGroups: map[string]*hivev1alpha1.RoleGroupSpec{
						// Declares `config` only to carry a logging setting.
						testRoleGroup: {Config: &hivev1alpha1.ConfigSpec{
							RoleGroupConfigSpec: &commonsv1alpha1.RoleGroupConfigSpec{
								Logging: &commonsv1alpha1.LoggingSpec{EnableVectorAgent: ptr.To(true)},
							},
						}},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		defer func() { Expect(k8sClient.Delete(ctx, cr)).To(Succeed()) }()

		stored := &hivev1alpha1.HiveMetastore{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), stored)).To(Succeed())

		Expect(stored.Spec.Metastore.RoleGroups[testRoleGroup].Config.WarehouseDir).To(BeEmpty())
		Expect(resolveWarehouseDir(stored.Spec.Metastore, testRoleGroup)).To(Equal("/role-level/warehouse"))
	})
})

// Image resolution is the framework's (ImageSpec.ResolveImage with the handler's ProductName
// and ImageDefaults); what this pins is the wiring hive supplies to it, since getting either
// half wrong yields a tag that does not exist in the registry.
var _ = Describe("image resolution wiring", func() {
	defaults := ImageDefaults

	It("assembles repo, product name and the kubedoop suffix from the defaults", func() {
		image, err := (&commonsv1alpha1.ImageSpec{}).ResolveImage(hivev1alpha1.DefaultProductName, defaults())
		Expect(err).NotTo(HaveOccurred())
		Expect(image).To(Equal(hivev1alpha1.DefaultRepository + "/hive:" +
			hivev1alpha1.DefaultProductVersion + "-kubedoop" + version.BuildVersion))
	})

	It("lets the CR's product version win while keeping the kubedoop suffix", func() {
		image, err := (&commonsv1alpha1.ImageSpec{ProductVersion: "3.1.3"}).
			ResolveImage(hivev1alpha1.DefaultProductName, defaults())
		Expect(err).NotTo(HaveOccurred())
		Expect(image).To(Equal(hivev1alpha1.DefaultRepository + "/hive:3.1.3-kubedoop" + version.BuildVersion))
	})

	It("returns a custom image verbatim", func() {
		image, err := (&commonsv1alpha1.ImageSpec{Custom: "my.repo/hive:x"}).
			ResolveImage(hivev1alpha1.DefaultProductName, defaults())
		Expect(err).NotTo(HaveOccurred())
		Expect(image).To(Equal("my.repo/hive:x"))
	})
})

var _ = Describe("ensureConfigProperties", func() {
	It("adds product properties without overriding user configOverrides", func() {
		buildCtx := &reconciler.RoleGroupBuildContext{
			MergedConfig: &opgoconfig.MergedConfig{
				ConfigFiles: map[string]map[string]string{
					HiveSiteFileName: {warehouseDirProperty: "/user-overridden"},
				},
			},
		}
		ensureConfigProperties(buildCtx, HiveSiteFileName, map[string]string{
			warehouseDirProperty: "/product-default",
			"fs.s3a.endpoint":    "http://minio:9000",
		})

		file := buildCtx.MergedConfig.ConfigFiles[HiveSiteFileName]
		Expect(file).To(HaveKeyWithValue(warehouseDirProperty, "/user-overridden"))
		Expect(file).To(HaveKeyWithValue("fs.s3a.endpoint", "http://minio:9000"))
	})
})

var _ = Describe("KerberosConfig", func() {
	krb5 := NewKerberosConfig(testNamespace, "test-hive", "metastore", "kerberos")

	It("renders SASL properties with the runtime realm placeholder", func() {
		hiveSite := krb5.GetHiveSite()
		Expect(hiveSite).To(HaveKeyWithValue("hive.metastore.sasl.enabled", "true"))
		Expect(hiveSite["hive.metastore.kerberos.principal"]).To(
			Equal("metastore/test-hive.default.svc.cluster.local@${env.KERBEROS_REALM}"))
	})

	It("declares the keytab CSI volume scoped to the cluster service", func() {
		volumes := krb5.Provisioner().Volumes()
		Expect(volumes).To(HaveLen(1))
		annotations := volumes[0].Ephemeral.VolumeClaimTemplate.Annotations
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/class", "kerberos"))
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/scope", "service=test-hive"))
		Expect(annotations).To(HaveKeyWithValue("secrets.kubedoop.dev/kerberosServiceNames", "metastore,HTTP"))

		mounts := krb5.Provisioner().VolumeMounts()
		Expect(mounts).To(HaveLen(1))
		Expect(mounts[0].MountPath).To(Equal("/kubedoop/kerberos"))
	})

	It("substitutes the realm into the copied hive-site.xml at startup", func() {
		script := krb5.GetContainerCommandArgs()
		Expect(script).To(ContainSubstring("export KERBEROS_REALM="))
		Expect(script).To(ContainSubstring("/kubedoop/config/hive-site.xml"))
	})
})

var _ = Describe("S3Config", func() {
	It("renders fs.s3a properties for a plain http endpoint", func() {
		s3 := &S3Config{Connection: &opgos3.ConnectionInfo{Endpoint: mustURL("http", "minio:9000")}}
		hiveSite := s3.GetHiveSite()
		Expect(hiveSite).To(HaveKeyWithValue("fs.s3a.endpoint", "http://minio:9000"))
		Expect(hiveSite).To(HaveKeyWithValue("fs.s3a.connection.ssl.enabled", "false"))
		Expect(hiveSite).To(HaveKeyWithValue("fs.s3a.impl", "org.apache.hadoop.fs.s3a.S3AFileSystem"))
		Expect(hiveSite).To(HaveKeyWithValue("fs.AbstractFileSystem.s3a.impl", "org.apache.hadoop.fs.s3a.S3A"))
	})

	It("enables ssl when the connection carries TLS", func() {
		s3 := &S3Config{Connection: &opgos3.ConnectionInfo{
			Endpoint: mustURL("https", "s3.example.com"),
			TLS:      &s3v1alpha1.Tls{},
		}}
		Expect(s3.GetHiveSite()).To(HaveKeyWithValue("fs.s3a.connection.ssl.enabled", "true"))
	})

	// Path-style addressing now follows the user's spec.pathStyle rather than being pinned on;
	// MinIO-style backends must declare it.
	It("reflects the declared path-style addressing", func() {
		pathStyle := &S3Config{Connection: &opgos3.ConnectionInfo{
			Endpoint:  mustURL("http", "minio:9000"),
			PathStyle: true,
		}}
		Expect(pathStyle.GetHiveSite()).To(HaveKeyWithValue("fs.s3a.path.style.access", "true"))

		virtualHost := &S3Config{Connection: &opgos3.ConnectionInfo{
			Endpoint: mustURL("https", "s3.amazonaws.com"),
		}}
		Expect(virtualHost.GetHiveSite()).To(HaveKeyWithValue("fs.s3a.path.style.access", "false"))
	})

	It("omits the credential exports for an anonymous connection", func() {
		anonymous := &S3Config{Connection: &opgos3.ConnectionInfo{Endpoint: mustURL("http", "minio:9000")}}
		Expect(anonymous.CredentialsProvisioner()).To(BeNil())
		Expect(anonymous.GetContainerCommandArgs()).To(BeEmpty())
	})

	It("exports the mounted credentials when a secret class is configured", func() {
		withCreds := &S3Config{Connection: &opgos3.ConnectionInfo{
			Endpoint:    mustURL("http", "minio:9000"),
			Credentials: &commonsv1alpha1.Credentials{SecretClass: "hive-s3-credentials"},
		}}
		Expect(withCreds.CredentialsProvisioner()).NotTo(BeNil())
		Expect(withCreds.GetContainerCommandArgs()).To(ContainSubstring("/kubedoop/secret/s3-credentials/ACCESS_KEY"))
	})
})
