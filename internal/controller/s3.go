package controller

import (
	"fmt"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

//var s3ConfigLogger = ctrl.Log.WithName("s3-config")

const (
	S3AccessKeyName = "ACCESS_KEY"
	S3SecretKeyName = "SECRET_KEY"

	S3Credentials = "s3-credentials"
)

type S3Params struct {
	Endpoint  string
	Bucket    string
	Region    string
	SSL       bool
	PathStyle bool
}

type S3Configuration struct {
	cr             *hivev1alpha1.HiveMetastore
	ResourceClient ResourceClient
}

func NewS3Configuration(cr *hivev1alpha1.HiveMetastore, resourceClient ResourceClient) *S3Configuration {
	return &S3Configuration{
		cr:             cr,
		ResourceClient: resourceClient,
	}
}

func (s *S3Configuration) GetRefBucketName() string {
	return *s.cr.Spec.ClusterConfig.S3Bucket.Reference
}

func (s *S3Configuration) Enabled() bool {
	return s.cr.Spec.ClusterConfig != nil && s.cr.Spec.ClusterConfig.S3Bucket != nil
}

func (s *S3Configuration) GetRefBucket() (*s3v1alpha1.S3Bucket, error) {
	s3BucketCR := &s3v1alpha1.S3Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.ResourceClient.Namespace,
			Name:      s.GetRefBucketName(),
		},
	}
	// Get Commons S3Bucket CR from the reference
	if err := s.ResourceClient.Get(s3BucketCR); err != nil {
		return nil, err
	}
	return s3BucketCR, nil
}

func (s *S3Configuration) GetRefConnection(name string) (*s3v1alpha1.S3Connection, error) {
	S3ConnectionCR := &s3v1alpha1.S3Connection{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.ResourceClient.Namespace,
			Name:      name,
		},
	}
	if err := s.ResourceClient.Get(S3ConnectionCR); err != nil {
		return nil, err
	}
	return S3ConnectionCR, nil
}

type S3Credential struct {
	AccessKey string `json:"ACCESS_KEY"`
	SecretKey string `json:"SECRET_KEY"`
}

func (s *S3Configuration) GetCredential(name string) (*S3Credential, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.ResourceClient.Namespace,
			Name:      name,
		},
	}
	if err := s.ResourceClient.Get(secret); err != nil {
		return nil, err
	}
	ak := secret.Data[S3AccessKeyName]

	sk := secret.Data[S3SecretKeyName]

	return &S3Credential{
		AccessKey: string(ak),
		SecretKey: string(sk),
	}, nil
}

func (s *S3Configuration) ExistingS3Bucket() bool {
	return s.cr.Spec.ClusterConfig.S3Bucket.Reference != nil
}

func (s *S3Configuration) GetS3ParamsFromResource() (*S3Params, error) {

	s3BucketCR, err := s.GetRefBucket()
	if err != nil {
		return nil, err
	}
	s3ConnectionCR, err := s.GetRefConnection(s3BucketCR.Spec.Reference)
	if err != nil {
		return nil, err
	}
	return &S3Params{
		Endpoint:  s3ConnectionCR.Spec.Endpoint,
		Region:    s3ConnectionCR.Spec.Region,
		SSL:       s3ConnectionCR.Spec.SSL,
		PathStyle: s3ConnectionCR.Spec.PathStyle,
		Bucket:    s3BucketCR.Spec.BucketName,
	}, nil
}

func (s *S3Configuration) GetS3ParamsFromInline() (*S3Params, error) {
	s3BucketCR := s.cr.Spec.ClusterConfig.S3Bucket
	return &S3Params{
		Endpoint:  s3BucketCR.Inline.Endpoints,
		Region:    s3BucketCR.Inline.Region,
		SSL:       s3BucketCR.Inline.SSL,
		PathStyle: s3BucketCR.Inline.PathStyle,
		Bucket:    s3BucketCR.Inline.Bucket,
	}, nil
}

func IsS3Enable(clusterSpec *hivev1alpha1.ClusterConfigSpec) bool {
	return clusterSpec != nil && clusterSpec.S3Bucket != nil && clusterSpec.S3Bucket.Reference != nil
}

func S3HiveSiteXml(xmlProperties map[string]string, params *S3Params) {
	xmlProperties["fs.s3a.endpoint"] = params.Endpoint
	xmlProperties["hive.metastore.s3.path"] = params.Bucket
	xmlProperties["fs.s3a.connection.ssl.enabled"] = strconv.FormatBool(params.SSL)
	xmlProperties["fs.s3a.path.style.access"] = strconv.FormatBool(params.PathStyle)
	xmlProperties["fs.s3a.impl"] = "org.apache.hadoop.fs.s3a.S3AFileSystem"
	xmlProperties["fs.AbstractFileSystem.s3a.impl"] = "org.apache.hadoop.fs.s3a.S3A"
}

func S3Volume(secretClass *string) corev1.Volume {
	if secretClass == nil {
		panic("secretClass cannot be nil")
	}
	return SecretVolume(map[string]string{
		volume.SecretsZncdataClass: *secretClass,
	}, S3Credentials)
}

func S3VolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      S3Credentials,
		MountPath: hivev1alpha1.S3SecretDir,
	}
}

func CreateS3ScriptData(clusterSpec *hivev1alpha1.ClusterConfigSpec) map[string]interface{} {
	if IsS3Enable(clusterSpec) {
		return map[string]interface{}{
			"s3Enabled": true,
			"s3Script": fmt.Sprintf(`
DIR="%s"
for FILE in "$DIR"/*
do
    NAME=$(basename "$FILE")
    VALUE=$(cat "$FILE")
    export "$NAME"="$VALUE"
done
`, hivev1alpha1.S3SecretDir),
		}
	}
	return map[string]interface{}{}
}
