package controller

import (
	stackv1alpha1 "github.com/zncdata-labs/hive-operator/api/v1alpha1"
	commonsv1alph1 "github.com/zncdata-labs/operator-go/pkg/apis/commons/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

const (
	DbUsernameName = "USERNAME"
	DbPasswordName = "PASSWORD"
)

type DatabaseCredential struct {
	Username string `json:"USERNAME"`
	Password string `json:"PASSWORD"`
}

type DatabaseParams struct {
	Driver   string
	Username string
	Password string
	Host     string
	Port     string
	DbName   string
}

type DatabaseConfiguration struct {
	cr             *stackv1alpha1.HiveMetastore
	ResourceClient ResourceClient
}

func NewDatabaseConfiguration(cr *stackv1alpha1.HiveMetastore, resourceClient ResourceClient) *DatabaseConfiguration {
	return &DatabaseConfiguration{
		cr:             cr,
		ResourceClient: resourceClient,
	}
}

func (d *DatabaseConfiguration) GetRefDatabaseName() string {
	return d.cr.Spec.ClusterConfig.Database.Reference
}

func (d *DatabaseConfiguration) Enabled() bool {
	return d.cr.Spec.ClusterConfig != nil && d.cr.Spec.ClusterConfig.Database != nil
}

func (d *DatabaseConfiguration) ExistingDatabase() bool {
	return d.cr.Spec.ClusterConfig.Database.Reference != ""
}

func (d *DatabaseConfiguration) GetRefDatabase() (commonsv1alph1.Database, error) {
	databaseCR := &commonsv1alph1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.ResourceClient.Namespace,
			Name:      d.GetRefDatabaseName(),
		},
	}
	if err := d.ResourceClient.Get(databaseCR); err != nil {
		return commonsv1alph1.Database{}, err
	}
	return *databaseCR, nil
}

func (d *DatabaseConfiguration) GetRefDatabaseConnection(name string) (commonsv1alph1.DatabaseConnection, error) {
	databaseConnectionCR := &commonsv1alph1.DatabaseConnection{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.ResourceClient.Namespace,
			Name:      name,
		},
	}

	if err := d.ResourceClient.Get(databaseConnectionCR); err != nil {
		return commonsv1alph1.DatabaseConnection{}, err
	}
	return *databaseConnectionCR, nil
}

func (d *DatabaseConfiguration) GetCredential(name string) (*DatabaseCredential, error) {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.ResourceClient.Namespace,
			Name:      name,
		},
	}

	if err := d.ResourceClient.Get(secret); err != nil {
		return nil, err
	}

	return &DatabaseCredential{
		Username: string(secret.Data[DbUsernameName]),
		Password: string(secret.Data[DbPasswordName]),
	}, nil
}

func (d *DatabaseConfiguration) getDatabaseParamsFromResource() (*DatabaseParams, error) {
	db, err := d.GetRefDatabase()
	if err != nil {
		return nil, err
	}
	credential := &DatabaseCredential{}

	if db.Spec.Credential.ExistSecret != "" {
		c, err := d.GetCredential(db.Spec.Credential.ExistSecret)
		if err != nil {
			return nil, err
		}
		credential = c
	} else {
		credential.Username = db.Spec.Credential.Username
		credential.Password = db.Spec.Credential.Password
	}

	dbConnection, err := d.GetRefDatabaseConnection(db.Spec.Reference)
	if err != nil {
		return nil, err
	}

	dbParams := &DatabaseParams{
		Username: credential.Username,
		Password: credential.Password,
	}

	provider := dbConnection.Spec.Provider

	if provider.Postgres != nil {
		dbParams.Driver = "postgres"
		dbParams.Host = provider.Postgres.Host
		dbParams.Port = strconv.Itoa(provider.Postgres.Port)
		dbParams.DbName = db.Spec.DatabaseName
		return dbParams, nil
	} else if provider.Mysql != nil {
		dbParams.Driver = "mysql"
		dbParams.Host = provider.Mysql.Host
		dbParams.Port = strconv.Itoa(provider.Mysql.Port)
		dbParams.DbName = db.Spec.DatabaseName
		return dbParams, nil
	} else {
		return &DatabaseParams{
			Driver:   "derby",
			Username: "",
			Password: "",
			Host:     "",
			Port:     "",
			DbName:   "",
		}, nil
	}
}

func (d *DatabaseConfiguration) getDatabaseParamsFromInline() (*DatabaseParams, error) {
	db := d.cr.Spec.ClusterConfig.Database.Inline
	return &DatabaseParams{
		Driver:   db.Driver,
		Username: db.Username,
		Password: db.Password,
		Host:     db.Host,
		Port:     strconv.Itoa(int(db.Port)),
		DbName:   db.DatabaseName,
	}, nil
}
