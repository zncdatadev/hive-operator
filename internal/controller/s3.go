package controller

import (
	hivev1alpha1 "github.com/zncdata-labs/hive-operator/api/v1alpha1"
	commonsv1alpha1 "github.com/zncdata-labs/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdata-labs/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	S3AccessKeyName = "ACCESS_KEY"
	S3SecretKeyName = "SECRET_KEY"
)

type S3Params struct {
	AccessKey string
	SecretKey string
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

func (s *S3Configuration) GetRefBucket() (*commonsv1alpha1.S3Bucket, error) {
	s3BucketCR := &commonsv1alpha1.S3Bucket{
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

func (s *S3Configuration) GetRefConnection(name string) (*commonsv1alpha1.S3Connection, error) {
	S3ConnectionCR := &commonsv1alpha1.S3Connection{
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
	ak, err := util.Base64[[]byte]{Data: secret.Data[S3AccessKeyName]}.Decode()
	if err != nil {
		return nil, err
	}
	sk, err := util.Base64[[]byte]{Data: secret.Data[S3SecretKeyName]}.Decode()
	if err != nil {
		return nil, err
	}
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
	credential := &S3Credential{}

	if s3BucketCR.Spec.Credential.ExistSecret != "" {
		existCredential, err := s.GetCredential(s3BucketCR.Spec.Credential.ExistSecret)
		if err != nil {
			return nil, err
		}
		credential = existCredential
	} else {
		credential.AccessKey = s3BucketCR.Spec.Credential.AccessKey
		credential.SecretKey = s3BucketCR.Spec.Credential.SecretKey
	}

	s3ConnectionCR, err := s.GetRefConnection(s3BucketCR.Spec.Reference)
	if err != nil {
		return nil, err
	}
	return &S3Params{
		AccessKey: credential.AccessKey,
		SecretKey: credential.SecretKey,
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
		AccessKey: s3BucketCR.Inline.AccessKey,
		SecretKey: s3BucketCR.Inline.SecretKey,
		Endpoint:  s3BucketCR.Inline.Endpoints,
		Region:    s3BucketCR.Inline.Region,
		SSL:       s3BucketCR.Inline.SSL,
		PathStyle: s3BucketCR.Inline.PathStyle,
		Bucket:    s3BucketCR.Inline.Bucket,
	}, nil
}
