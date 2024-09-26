package controller

import (
	"context"
	"net/url"
	"path"
	"strconv"
	"strings"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

const (
	S3AccessKeyName = "ACCESS_KEY"
	S3SecretKeyName = "SECRET_KEY"

	S3VolumeName = "s3-credentials"
)

// TODO: Add the tls verification
type S3Connection struct {
	Endpoint   url.URL
	PathStyle  bool
	credential *commonsv1alpha1.Credentials
}

func GetS3Connect(ctx context.Context, client *client.Client, s3 *hivev1alpha1.S3Spec) (*S3Connection, error) {
	s3ConnectionSpec := s3.Inline
	if s3.Reference != "" {
		obj, err := GetRefreenceS3Connection(ctx, client, s3.Reference)
		if err != nil {
			return nil, err
		}
		s3ConnectionSpec = &obj.Spec
	}

	endpoint := url.URL{
		Scheme: "http",
		Host:   s3ConnectionSpec.Host,
	}
	if s3ConnectionSpec.Port != 0 {
		endpoint.Host += ":" + strconv.Itoa(s3ConnectionSpec.Port)
	}

	return &S3Connection{
		Endpoint:   endpoint,
		PathStyle:  s3ConnectionSpec.PathStyle,
		credential: s3ConnectionSpec.Credentials,
	}, nil
}

func GetRefreenceS3Connection(ctx context.Context, client *client.Client, name string) (*v1alpha1.S3Connection, error) {
	s3Connection := &v1alpha1.S3Connection{}
	if err := client.GetWithOwnerNamespace(ctx, name, s3Connection); err != nil {
		return nil, err
	}
	return s3Connection, nil
}

type S3Config struct {
	S3Connection *S3Connection
}

func NewS3Config(
	s3Connection *S3Connection,
) *S3Config {
	return &S3Config{S3Connection: s3Connection}
}

func (s *S3Config) GetMountPath() string {
	return path.Join(constants.KubedoopSecretDir, "s3-credentials")
}

func (s *S3Config) GetVolumeName() string {
	return S3VolumeName
}

func (s *S3Config) GetEndpoint() string {
	return s.S3Connection.Endpoint.String()
}

func (s *S3Config) GetHiveSite() map[string]string {

	sslEnabled := s.S3Connection.Endpoint.Scheme == "https"

	properties := map[string]string{
		"fs.s3a.endpoint":                s.GetEndpoint(),
		"fs.s3a.path.style.access":       "true",
		"fs.s3a.connection.ssl.enabled":  strconv.FormatBool(sslEnabled),
		"fs.s3a.impl":                    "org.apache.hadoop.fs.s3a.S3AFileSystem",
		"fs.AbstractFileSystem.s3a.impl": "org.apache.hadoop.fs.s3a.S3A",
	}
	return properties
}

func (s *S3Config) GetVolumes() []corev1.Volume {

	credential := s.S3Connection.credential

	secretClass := credential.SecretClass

	annotations := map[string]string{
		constants.AnnotationSecretsClass: secretClass,
	}

	if credential.Scope != nil {
		scopes := []string{}
		if credential.Scope.Node {
			scopes = append(scopes, string(constants.NodeScope))
		}
		if credential.Scope.Pod {
			scopes = append(scopes, string(constants.PodScope))
		}
		scopes = append(scopes, credential.Scope.Services...)

		annotations[constants.AnnotationSecretsScope] = strings.Join(scopes, constants.CommonDelimiter)
	}
	secretVolume := corev1.Volume{
		Name: s.GetVolumeName(),
		VolumeSource: corev1.VolumeSource{
			Ephemeral: &corev1.EphemeralVolumeSource{
				VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: annotations,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: constants.SecretStorageClassPtr(),
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Mi"),
							},
						},
					},
				},
			},
		},
	}
	return []corev1.Volume{secretVolume}
}

func (s *S3Config) GetVolumeMount() *corev1.VolumeMount {
	secretVolumeMount := &corev1.VolumeMount{
		Name:      s.GetVolumeName(),
		MountPath: s.GetMountPath(),
	}

	return secretVolumeMount
}

func (s *S3Config) GetContainerCommandArgs() string {
	args := `
export AWS_ACCESS_KEY_ID=$(cat ` + path.Join(s.GetMountPath(), S3AccessKeyName) + `)
export AWS_SECRET_ACCESS_KEY=$(cat ` + path.Join(s.GetMountPath(), S3SecretKeyName) + `)
`

	return util.IndentTab4Spaces(args)
}
