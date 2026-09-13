package controller

import (
	"context"

	opgos3 "github.com/zncdatadev/operator-go/pkg/s3"
	"github.com/zncdatadev/operator-go/pkg/security"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

// S3VolumeName is the credentials volume name; the framework mounts it at
// /kubedoop/secret/<name>.
const S3VolumeName = opgos3.DefaultCredentialsVolumeName

// S3Config adapts a resolved S3 connection to what the metastore needs: the hive-site client
// properties and the start-script credential exports. Resolution, endpoint construction,
// the credentials CSI volume and the AWS env exports are all framework-owned (pkg/s3).
type S3Config struct {
	Connection *opgos3.ConnectionInfo
}

// ResolveS3Config follows the inline-or-reference chain of the CR's S3 configuration.
func ResolveS3Config(
	ctx context.Context,
	client ctrlclient.Client,
	namespace string,
	s3 *hivev1alpha1.S3Spec,
) (*S3Config, error) {
	connection, err := opgos3.ResolveConnection(ctx, client, namespace, s3.Inline, s3.Reference)
	if err != nil {
		return nil, err
	}
	return &S3Config{Connection: connection}, nil
}

// GetHiveSite renders the S3A properties for hive-site.xml. The endpoint, addressing style,
// SSL flag and signing region come from the resolved connection — including
// fs.s3a.path.style.access, which now reflects the user's spec.pathStyle instead of being
// pinned on. Backends that require path-style addressing (MinIO and most self-hosted S3
// implementations) must declare `pathStyle: true` on the S3Connection.
func (s *S3Config) GetHiveSite() map[string]string {
	properties := s.Connection.S3AProperties()
	// The filesystem implementation classes are Hadoop-side wiring, outside the scope of the
	// framework's client-property renderer.
	properties["fs.s3a.impl"] = "org.apache.hadoop.fs.s3a.S3AFileSystem"
	properties["fs.AbstractFileSystem.s3a.impl"] = "org.apache.hadoop.fs.s3a.S3A"
	return properties
}

// CredentialsProvisioner returns the CSI volume provider delivering ACCESS_KEY/SECRET_KEY,
// or nil when the connection is anonymous (no credentials to mount).
func (s *S3Config) CredentialsProvisioner() *security.SecretProvisioner {
	return s.Connection.CredentialsProvisioner(S3VolumeName)
}

// GetContainerCommandArgs exports the mounted credentials as AWS SDK environment variables
// before the metastore starts. Empty for an anonymous connection, which mounts no volume.
func (s *S3Config) GetContainerCommandArgs() string {
	if s.Connection.Credentials == nil {
		return ""
	}
	return opgos3.CredentialsExportScript(opgos3.CredentialsMountPath(S3VolumeName))
}
