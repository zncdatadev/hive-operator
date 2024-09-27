package controller

import (
	"context"

	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"github.com/zncdatadev/operator-go/pkg/util"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

var _ reconciler.Reconciler = &RoleReconciler{}

type RoleReconciler struct {
	reconciler.BaseRoleReconciler[*hivev1alpha1.RoleSpec]
	ClusterConfig *hivev1alpha1.ClusterConfigSpec
	Image         *util.Image
}

func NewNodeRoleReconciler(
	client *client.Client,
	clusterStopped bool,
	clusterConfig *hivev1alpha1.ClusterConfigSpec,
	roleInfo reconciler.RoleInfo,
	image *util.Image,
	spec *hivev1alpha1.RoleSpec,
) *RoleReconciler {
	return &RoleReconciler{
		BaseRoleReconciler: *reconciler.NewBaseRoleReconciler(
			client,
			clusterStopped,
			roleInfo,
			spec,
		),
		ClusterConfig: clusterConfig,
		Image:         image,
	}
}

func (r *RoleReconciler) RegisterResources(ctx context.Context) error {
	for name, roleGroup := range r.Spec.RoleGroups {
		mergedRolgGroup := roleGroup.DeepCopy()
		r.MergeRoleGroupSpec(mergedRolgGroup)

		info := reconciler.RoleGroupInfo{
			RoleInfo:      r.RoleInfo,
			RoleGroupName: name,
		}

		reconcilers, err := r.GetImageResourceWithRoleGroup(ctx, info, mergedRolgGroup)

		if err != nil {
			return err
		}

		for _, reconciler := range reconcilers {
			r.AddResource(reconciler)
		}
	}
	return nil
}

func (r *RoleReconciler) GetImageResourceWithRoleGroup(ctx context.Context, info reconciler.RoleGroupInfo, spec *hivev1alpha1.RoleGroupSpec) ([]reconciler.Reconciler, error) {

	cm := NewConfigMapReconciler(
		r.Client,
		r.ClusterConfig,
		info,
		*spec,
	)

	deployment, err := NewDeploymentReconciler(
		r.Client,
		info,
		r.ClusterConfig,
		ContainerPort,
		r.Image,
		r.ClusterStopped,
		spec,
	)
	if err != nil {
		return nil, err
	}

	svc := reconciler.NewServiceReconciler(
		r.Client,
		info.GetFullName(),
		ContainerPort,
		func(sbo *builder.ServiceBuilderOption) {
			sbo.Labels = info.GetLabels()
			sbo.Annotations = info.GetAnnotations()
			sbo.ClusterName = r.ClusterInfo.ClusterName
			sbo.RoleName = r.RoleInfo.RoleName
			sbo.RoleGroupName = info.RoleGroupName
			sbo.ListenerClass = constants.ListenerClass(r.ClusterConfig.ListenerClass)
		},
	)
	return []reconciler.Reconciler{cm, deployment, svc}, nil
}
