package controller

import (
	"context"

	client "github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"github.com/zncdatadev/operator-go/pkg/util"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/hive-operator/internal/util/version"
)

var _ reconciler.Reconciler = &ClusterReconciler{}

type ClusterReconciler struct {
	reconciler.BaseCluster[*hivev1alpha1.HiveMetastoreSpec]
	ClusterConfig *hivev1alpha1.ClusterConfigSpec
}

func NewClusterReconciler(
	client *client.Client,
	clusterInfo reconciler.ClusterInfo,
	spec *hivev1alpha1.HiveMetastoreSpec,
) *ClusterReconciler {
	return &ClusterReconciler{
		BaseCluster: *reconciler.NewBaseCluster(
			client,
			clusterInfo,
			spec.ClusterOperation,
			spec,
		),
		ClusterConfig: spec.ClusterConfig,
	}
}

func (r *ClusterReconciler) GetImage() *util.Image {
	image := util.NewImage(
		hivev1alpha1.DefaultProductName,
		version.BuildVersion,
		hivev1alpha1.DefaultProductVersion,
		func(options *util.ImageOptions) {
			options.Custom = r.Spec.Image.Custom
			options.Repo = r.Spec.Image.Repo
			options.PullPolicy = r.Spec.Image.PullPolicy
		},
	)

	if r.Spec.Image.KubedoopVersion != "" {
		image.KubedoopVersion = r.Spec.Image.KubedoopVersion
	}

	return image
}

func (r *ClusterReconciler) RegisterResource(ctx context.Context) error {
	roleInfo := reconciler.RoleInfo{
		ClusterInfo: r.ClusterInfo,
		RoleName:    "metastore",
	}

	node := NewNodeRoleReconciler(
		r.Client,
		r.IsStopped(),
		r.ClusterConfig,
		roleInfo,
		r.GetImage(),
		r.Spec.Metastore,
	)
	if err := node.RegisterResources(ctx); err != nil {
		return err
	}

	r.AddResource(node)

	return nil
}
