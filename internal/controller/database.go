package controller

import (
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

const databaseTypeDerby = "derby"

// jdbcDriverNames maps the CRD databaseType to the JDBC driver class.
var jdbcDriverNames = map[string]string{
	"mysql":           "com.mysql.cj.jdbc.Driver",
	"postgres":        "org.postgresql.Driver",
	"oracle":          "oracle.jdbc.OracleDriver",
	databaseTypeDerby: "org.apache.derby.jdbc.EmbeddedDriver",
}

// databaseJVMOpts renders the JDO connection system properties for the metastore JVM.
// Credentials are referenced as $(username)/$(password), expanded by the kubelet from the
// database credentials secret injected via envFrom.
func databaseJVMOpts(database *hivev1alpha1.DatabaseSpec) []string {
	connString := database.ConnString
	driver, known := jdbcDriverNames[database.DatabaseType]
	if !known {
		// Unknown type: fall back to an embedded derby store, matching the legacy behavior.
		connString = "jdbc:derby:/tmp/metastore_db;create=true"
		driver = jdbcDriverNames[databaseTypeDerby]
	}

	jvmOpts := []string{
		"-Djavax.jdo.option.ConnectionURL=" + connString,
		"-Djavax.jdo.option.ConnectionDriverName=" + driver,
	}

	if database.DatabaseType != databaseTypeDerby {
		jvmOpts = append(jvmOpts,
			"-Djavax.jdo.option.ConnectionUserName=$(username)",
			"-Djavax.jdo.option.ConnectionPassword=$(password)",
		)
	}
	return jvmOpts
}
