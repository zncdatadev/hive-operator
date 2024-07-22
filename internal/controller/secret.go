package controller

import (
	"context"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretReconciler interface {
	Reconcile(ctx context.Context) (ctrl.Result, error)
	make(map[string][]byte) (corev1.Secret, error)
	apply(ctx context.Context) (ctrl.Result, error)
	describe(ctx context.Context) (corev1.Secret, error)
	Name() string
	Labels() map[string]string
	NameSpace() string
}

type EnvSecret struct {
	client client.Client
	scheme *runtime.Scheme

	cr *hivev1alpha1.HiveMetastore
	s3 *S3Configuration
	db *DatabaseConfiguration
}

func NewEnvSecret(ctx context.Context, client client.Client, scheme *runtime.Scheme, cr *hivev1alpha1.HiveMetastore) *EnvSecret {

	resourceClient := ResourceClient{
		Ctx:       ctx,
		Client:    client,
		Namespace: cr.Namespace,
		Log:       log,
	}

	s3 := NewS3Configuration(cr, resourceClient)

	db := NewDatabaseConfiguration(cr, resourceClient)

	return &EnvSecret{
		client: client,
		scheme: scheme,
		cr:     cr,
		s3:     s3,
		db:     db,
	}
}

func HiveEnvSecretName(cr *hivev1alpha1.HiveMetastore) string {
	return cr.GetName()
}

func (r *EnvSecret) Labels() map[string]string {
	return map[string]string{
		"app": r.cr.GetName(),
	}
}

func (r *EnvSecret) NameSpace() string {
	return r.cr.Namespace
}

func (r *EnvSecret) Reconcile(ctx context.Context) (ctrl.Result, error) {
	return r.apply(ctx)
}

func (r *EnvSecret) apply(ctx context.Context) (ctrl.Result, error) {
	var data = make(map[string]string)
	if r.db.Enabled() {
		dbData, err := r.databaseSecretData()
		if err != nil {
			return ctrl.Result{}, err
		}

		for k, v := range dbData {
			data[k] = v
		}
	}

	//if len(data) == 0 {
	//	log.Info("Database config not found, use derby by default.")
	//	return ctrl.Result{Requeue: false}, nil
	//}

	obj, err := r.make(data)
	if err != nil {
		return ctrl.Result{}, err
	}
	effected, err := CreateOrUpdate(ctx, r.client, &obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	if effected {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{Requeue: false}, nil
}

// databaseValuesFromCR Get database values from the CR.
func (r *EnvSecret) databaseSecretData() (map[string]string, error) {

	dataBuilder := func(params *DatabaseParams) map[string]string {
		serviceOpts := serviceOptsBuilder(params.Driver, params.Username, params.Password, params.Host, params.Port, params.DbName)

		data := map[string]string{
			"SERVICE_OPTS": serviceOpts,
			"DB_DRIVER":    params.Driver,
		}
		if params.Driver == "derby" {
			log.Info("Hive metastore is using derby, no need to set database connection info.")
		}
		return data
	}

	if r.db.ExistingDatabase() {
		params, err := r.db.getDatabaseParamsFromResource()
		if err != nil {
			return nil, err
		}

		return dataBuilder(params), nil
	}

	params, err := r.db.getDatabaseParamsFromInline()

	if err != nil {
		return nil, err
	}

	return dataBuilder(params), nil
}

// makeSecret Make secret object from data, and set the owner reference.
func (r *EnvSecret) make(data map[string]string) (corev1.Secret, error) {
	encodedData := make(map[string][]byte)
	for k, v := range data {
		encodedData[k] = []byte(v)
	}
	obj := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HiveEnvSecretName(r.cr),
			Namespace: r.NameSpace(),
			Labels:    r.Labels(),
		},
		Data: encodedData,
	}

	if err := ctrl.SetControllerReference(r.cr, &obj, r.scheme); err != nil {
		return obj, err
	}

	return obj, nil
}

// serviceOpts Build hive metastore container SERVICE_OPTS environment variable.
// If not mysql or postgres, use derby.
func serviceOptsBuilder(driver string, username string, password string, host string, port string, dbname string) string {

	var connectionDriver string
	var jdbcURL string

	switch driver {
	case "mysql":
		connectionDriver = "com.mysql.cj.jdbc.Driver"
		jdbcURL = "jdbc:mysql://" + host + ":" + port + "/" + dbname

	case "postgres":
		connectionDriver = "org.postgresql.Driver"
		jdbcURL = "jdbc:postgresql://" + host + ":" + port + "/" + dbname

	default:
		if dbname == "" {
			dbname = "metastore_db"
		}
		connectionDriver = "org.apache.derby.jdbc.ClientDriver"
		jdbcURL = "jdbc:derby:" + dbname + ";create=true"
	}

	return "-Xmx1G -Djavax.jdo.option.ConnectionDriverName=" + connectionDriver +
		" -Djavax.jdo.option.ConnectionURL=" + jdbcURL +
		" -Djavax.jdo.option.ConnectionUserName=" + username +
		" -Djavax.jdo.option.ConnectionPassword=" + password
}
