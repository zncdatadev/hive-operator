package controller

import (
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	opgoconfig "github.com/zncdatadev/operator-go/pkg/config"
	"github.com/zncdatadev/operator-go/pkg/reconciler"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
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
			ConnString:   "jdbc:derby:;databaseName=/tmp/hive;create=true",
			DatabaseType: "derby",
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
			RoleGroups: map[string]*hivev1alpha1.RoleGroupSpec{"default": {}},
		}
		if roleDir != "" {
			r.Config = &hivev1alpha1.ConfigSpec{WarehouseDir: roleDir}
		}
		if groupDir != "" {
			r.RoleGroups["default"].Config = &hivev1alpha1.ConfigSpec{WarehouseDir: groupDir}
		}
		return r
	}

	It("prefers the role group value over the role value", func() {
		Expect(resolveWarehouseDir(role("/role", "/group"), "default")).To(Equal("/group"))
	})

	It("falls back to the role value", func() {
		Expect(resolveWarehouseDir(role("/role", ""), "default")).To(Equal("/role"))
	})

	It("defaults when nothing is set", func() {
		Expect(resolveWarehouseDir(role("", ""), "default")).To(Equal(hivev1alpha1.DefaultWarehouseDir))
		Expect(resolveWarehouseDir(nil, "default")).To(Equal(hivev1alpha1.DefaultWarehouseDir))
	})
})

var _ = Describe("resolveImage", func() {
	It("returns the custom image verbatim", func() {
		Expect(resolveImage(&hivev1alpha1.ImageSpec{Custom: "my.repo/hive:x"})).To(Equal("my.repo/hive:x"))
	})

	It("assembles repo, product version and kubedoop version", func() {
		image := resolveImage(&hivev1alpha1.ImageSpec{
			Repo:            "quay.io/zncdatadev",
			ProductVersion:  "3.1.3",
			KubedoopVersion: "0.0.0-dev",
		})
		Expect(image).To(Equal("quay.io/zncdatadev/hive:3.1.3-kubedoop0.0.0-dev"))
	})

	It("defaults the product version when unset", func() {
		image := resolveImage(nil)
		Expect(image).To(ContainSubstring("/hive:" + hivev1alpha1.DefaultProductVersion + "-kubedoop"))
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
	krb5 := NewKerberosConfig("default", "test-hive", "metastore", "kerberos")

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
		s3 := NewS3Config(&S3Connection{Endpoint: mustURL("http", "minio:9000")})
		hiveSite := s3.GetHiveSite()
		Expect(hiveSite).To(HaveKeyWithValue("fs.s3a.endpoint", "http://minio:9000"))
		Expect(hiveSite).To(HaveKeyWithValue("fs.s3a.connection.ssl.enabled", "false"))
		Expect(hiveSite).To(HaveKeyWithValue("fs.s3a.path.style.access", "true"))
	})

	It("enables ssl for https endpoints", func() {
		s3 := NewS3Config(&S3Connection{Endpoint: mustURL("https", "s3.example.com")})
		Expect(s3.GetHiveSite()).To(HaveKeyWithValue("fs.s3a.connection.ssl.enabled", "true"))
	})
})
