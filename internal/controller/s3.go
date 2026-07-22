package controller

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/constant"
	"github.com/zncdatadev/operator-go/pkg/security"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

const (
	S3AccessKeyName = "ACCESS_KEY"
	S3SecretKeyName = "SECRET_KEY"

	S3VolumeName = "s3-credentials"
)

type S3Connection struct {
	Endpoint   url.URL
	PathStyle  bool
	credential *commonsv1alpha1.Credentials
}

// GetS3Connection resolves the CR's S3 configuration to a connection, either from the inline
// spec or by fetching the referenced S3Connection object from the CR's namespace.
func GetS3Connection(ctx context.Context, client ctrlclient.Client, namespace string, s3 *hivev1alpha1.S3Spec) (*S3Connection, error) {
	s3ConnectionSpec := s3.Inline
	if s3.Reference != "" {
		s3Connection := &s3v1alpha1.S3Connection{}
		if err := client.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: s3.Reference}, s3Connection); err != nil {
			return nil, fmt.Errorf("failed to get referenced S3Connection %q: %w", s3.Reference, err)
		}
		s3ConnectionSpec = &s3Connection.Spec
	}
	if s3ConnectionSpec == nil {
		return nil, fmt.Errorf("s3 is configured but neither inline nor reference connection is set")
	}

	scheme := "http"
	if s3ConnectionSpec.Tls != nil {
		scheme = "https"
	}
	endpoint := url.URL{
		Scheme: scheme,
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

type S3Config struct {
	S3Connection *S3Connection
}

func NewS3Config(s3Connection *S3Connection) *S3Config {
	return &S3Config{S3Connection: s3Connection}
}

func (s *S3Config) GetMountPath() string {
	return path.Join(constant.KubedoopSecretDir, S3VolumeName)
}

func (s *S3Config) GetEndpoint() string {
	return s.S3Connection.Endpoint.String()
}

func (s *S3Config) GetHiveSite() map[string]string {
	sslEnabled := s.S3Connection.Endpoint.Scheme == "https"

	return map[string]string{
		"fs.s3a.endpoint": s.GetEndpoint(),
		// Path-style access is intentionally always on: most self-hosted S3 backends (MinIO)
		// require it, and the CRD's pathStyle default (false) predates any consumer of the
		// virtual-host style. Honoring spec.pathStyle is deferred to the operator-go S3
		// rendering helper.
		"fs.s3a.path.style.access":       "true",
		"fs.s3a.connection.ssl.enabled":  strconv.FormatBool(sslEnabled),
		"fs.s3a.impl":                    "org.apache.hadoop.fs.s3a.S3AFileSystem",
		"fs.AbstractFileSystem.s3a.impl": "org.apache.hadoop.fs.s3a.S3A",
	}
}

// Volumes implements reconciler.VolumeProvider: the S3 credentials are delivered by
// secret-operator through an ephemeral CSI volume annotated with the secret class and scope.
// The volume is hand-built (not via security.SecretProvisioner) because plain credential
// secrets carry no format annotation, which the provisioner cannot express yet.
func (s *S3Config) Volumes() []corev1.Volume {
	credential := s.S3Connection.credential

	annotations := map[string]string{
		security.SecretClassAnnotation: credential.SecretClass,
	}

	if credential.Scope != nil {
		scopes := []string{}
		if credential.Scope.Node {
			scopes = append(scopes, string(security.NodeScope))
		}
		if credential.Scope.Pod {
			scopes = append(scopes, string(security.PodScope))
		}
		scopes = append(scopes, credential.Scope.Services...)

		annotations[security.SecretClassScopeAnnotation] = strings.Join(scopes, security.CommonDelimiter)
	}

	return []corev1.Volume{
		{
			Name: S3VolumeName,
			VolumeSource: corev1.VolumeSource{
				Ephemeral: &corev1.EphemeralVolumeSource{
					VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: annotations,
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							StorageClassName: ptr.To(string(security.SecretStorageClass)),
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// VolumeMounts implements reconciler.VolumeProvider.
func (s *S3Config) VolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      S3VolumeName,
			MountPath: s.GetMountPath(),
		},
	}
}

// GetContainerCommandArgs exports the mounted S3 credentials as AWS SDK environment variables
// before the metastore starts.
func (s *S3Config) GetContainerCommandArgs() string {
	return `export AWS_ACCESS_KEY_ID=$(cat ` + path.Join(s.GetMountPath(), S3AccessKeyName) + `)
export AWS_SECRET_ACCESS_KEY=$(cat ` + path.Join(s.GetMountPath(), S3SecretKeyName) + `)`
}
