package controller

import (
	"context"
	stackv1alpha1 "github.com/zncdata-labs/hive-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"time"
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

	cr *stackv1alpha1.HiveMetastore
	s3 *S3Configuration
	db *DatabaseConfiguration
}

func NewEnvSecret(ctx context.Context, client client.Client, scheme *runtime.Scheme, cr *stackv1alpha1.HiveMetastore) *EnvSecret {

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

func HiveEnvSecretName(cr *stackv1alpha1.HiveMetastore) string {
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
	var data = make(map[string][]byte)
	if r.s3.Enabled() {
		s3Data, err := r.s3SecretData()
		if err != nil {
			return ctrl.Result{}, err
		}

		for k, v := range s3Data {
			data[k] = v
		}
	}

	if r.db.Enabled() {
		dbData, err := r.databaseSecretData()
		if err != nil {
			return ctrl.Result{}, err
		}

		for k, v := range dbData {
			data[k] = v
		}
	}

	if len(data) == 0 {
		log.Info("Database config not found, use derby by default.")
		return ctrl.Result{Requeue: false}, nil
	}

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

func (r *EnvSecret) s3SecretData() (map[string][]byte, error) {
	if r.s3.ExistingS3Bucket() {
		params, err := r.s3.GetS3ParamsFromResource()
		if err != nil {
			return nil, err
		}
		return map[string][]byte{
			"AWS_ACCESS_KEY":     []byte(params.AccessKey),
			"AWS_SECRET_KEY":     []byte(params.SecretKey),
			"AWS_DEFAULT_REGION": []byte(params.Region),
		}, nil
	}

	params, err := r.s3.GetS3ParamsFromInline()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"AWS_ACCESS_KEY":     []byte(params.AccessKey),
		"AWS_SECRET_KEY":     []byte(params.SecretKey),
		"AWS_DEFAULT_REGION": []byte(params.Region),
	}, nil

}

// databaseValuesFromCR Get database values from the CR.
func (r *EnvSecret) databaseSecretData() (map[string][]byte, error) {

	dataBuilder := func(params *DatabaseParams) map[string][]byte {
		serviceOpts := serviceOptsBuilder(params.Driver, params.Username, params.Password, params.Host, params.Port, params.DbName)

		data := map[string][]byte{
			"SERVICE_OPTS": []byte(serviceOpts),
			"DB_DRIVER":    []byte(params.Driver),
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
func (r *EnvSecret) make(data map[string][]byte) (corev1.Secret, error) {
	obj := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HiveEnvSecretName(r.cr),
			Namespace: r.NameSpace(),
			Labels:    r.Labels(),
		},
		Data: data,
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

const HiveSiteName = "hive-site.xml"

type HiveSiteSecret struct {
	client client.Client
	scheme *runtime.Scheme

	roleGroupName string
	roleGroup     *stackv1alpha1.RoleGroupSpec

	cr *stackv1alpha1.HiveMetastore
	s3 *S3Configuration
	db *DatabaseConfiguration
}

func NewHiveSiteSecret(
	ctx context.Context,
	client client.Client,
	scheme *runtime.Scheme,
	cr *stackv1alpha1.HiveMetastore,
	roleGroupName string,
	roleGroup *stackv1alpha1.RoleGroupSpec,
) *HiveSiteSecret {

	resourceClient := ResourceClient{
		Ctx:       ctx,
		Client:    client,
		Namespace: cr.Namespace,
		Log:       log,
	}

	s3 := NewS3Configuration(cr, resourceClient)

	db := NewDatabaseConfiguration(cr, resourceClient)

	return &HiveSiteSecret{
		client:        client,
		scheme:        scheme,
		cr:            cr,
		roleGroupName: roleGroupName,
		roleGroup:     roleGroup,
		s3:            s3,
		db:            db,
	}
}

func HiveSiteSecretName(cr *stackv1alpha1.HiveMetastore, roleGroupName string) string {
	return cr.GetNameWithSuffix(roleGroupName + "-hive-site")
}

func (r *HiveSiteSecret) Labels() map[string]string {
	return map[string]string{
		"app": r.cr.GetName(),
	}
}

func (r *HiveSiteSecret) NameSpace() string {
	return r.cr.Namespace
}

func (r *HiveSiteSecret) Reconcile(ctx context.Context) (ctrl.Result, error) {
	return r.apply(ctx)
}

func (r *HiveSiteSecret) apply(ctx context.Context) (ctrl.Result, error) {

	value := hiveSiteXML(r.hiveSiteProperties())

	data := map[string][]byte{
		HiveSiteName: []byte(value),
	}

	obj, err := r.make(data)
	if err != nil {
		return ctrl.Result{}, err
	}

	if effected, err := CreateOrUpdate(ctx, r.client, &obj); err != nil {
		return ctrl.Result{}, err
	} else if effected {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *HiveSiteSecret) make(data map[string][]byte) (corev1.Secret, error) {
	obj := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HiveSiteSecretName(r.cr, r.roleGroupName),
			Namespace: r.NameSpace(),
			Labels:    r.Labels(),
		},
		Data: data,
	}

	if err := ctrl.SetControllerReference(r.cr, &obj, r.scheme); err != nil {
		return obj, err
	}

	return obj, nil
}

func (r *HiveSiteSecret) warehouseDir() string {
	if r.roleGroup.Config != nil && r.roleGroup.Config.WarehouseDir != "" {
		return r.roleGroup.Config.WarehouseDir
	}
	return stackv1alpha1.WarehouseDir
}

func (r *HiveSiteSecret) hiveSiteProperties() map[string]string {
	if r.s3.Enabled() {
		var params *S3Params

		if r.s3.ExistingS3Bucket() {
			var err error
			params, err = r.s3.GetS3ParamsFromResource()
			if err != nil {
				return nil
			}
		} else {
			var err error
			params, err = r.s3.GetS3ParamsFromInline()
			if err != nil {
				return nil
			}
		}
		return map[string]string{
			"hive.metastore.warehouse.dir":   r.warehouseDir(),
			"hive.metastore.s3.path":         params.Bucket,
			"fs.s3a.connection.ssl.enabled":  strconv.FormatBool(params.SSL),
			"fs.s3a.path.style.access":       strconv.FormatBool(params.PathStyle),
			"fs.s3a.impl":                    "org.apache.hadoop.fs.s3a.S3AFileSystem",
			"fs.AbstractFileSystem.s3a.impl": "org.apache.hadoop.fs.s3a.S3A",
		}
	}

	return map[string]string{
		"hive.metastore.warehouse.dir": r.warehouseDir(),
	}
}

func hiveSiteXML(properties map[string]string) string {
	xml := "" +
		"<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n" +
		"<?xml-stylesheet type=\"text/xsl\" href=\"configuration.xsl\"?>\n" +
		"<configuration>\n"
	for k, v := range properties {
		xml += "  <property>\n"
		xml += "    <name>" + k + "</name>\n"
		xml += "    <value>" + v + "</value>\n"
		xml += "  </property>\n"
	}
	xml += "</configuration>\n"
	return xml
}
