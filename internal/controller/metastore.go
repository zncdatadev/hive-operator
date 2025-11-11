package controller

import (
	"context"

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
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
		info := reconciler.RoleGroupInfo{
			RoleInfo:      r.RoleInfo,
			RoleGroupName: name,
		}

		mergedConfig, err := util.MergeObject(r.Spec.Config, roleGroup.Config)
		if err != nil {
			return err
		}

		mergedOverrides, err := util.MergeObject(r.Spec.OverridesSpec, roleGroup.OverridesSpec)
		if err != nil {
			return err
		}

		reconcilers, err := r.GetImageResourceWithRoleGroup(
			ctx,
			info,
			mergedConfig,
			mergedOverrides,
			&roleGroup.Replicas,
		)
		if err != nil {
			return err
		}

		for _, reconciler := range reconcilers {
			r.AddResource(reconciler)
		}
	}
	return nil
}

func (r *RoleReconciler) GetImageResourceWithRoleGroup(
	ctx context.Context,
	info reconciler.RoleGroupInfo,
	config *hivev1alpha1.ConfigSpec,
	overrides *commonsv1alpha1.OverridesSpec,
	replicas *int32,
) ([]reconciler.Reconciler, error) {

	options := func(o *builder.Options) {
		o.ClusterName = info.ClusterName
		o.RoleName = info.RoleName
		o.RoleGroupName = info.RoleGroupName
		o.Labels = info.GetLabels()
		o.Annotations = info.GetAnnotations()
	}

	cm := NewConfigMapReconciler(
		r.Client,
		r.ClusterConfig,
		info,
		config,
		options,
	)

	var commonsRoleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec
	if config != nil {
		commonsRoleGroupConfig = config.RoleGroupConfigSpec
	}

	sts, err := NewStatefulSetReconciler(
		r.Client,
		info,
		r.ClusterConfig,
		ContainerPort,
		r.Image,
		replicas,
		r.ClusterStopped(),
		overrides,
		commonsRoleGroupConfig,
		options,
	)
	if err != nil {
		return nil, err
	}

	svc := reconciler.NewServiceReconciler(
		r.Client,
		info.GetFullName(),
		ContainerPort,
		func(o *builder.ServiceBuilderOptions) {
			o.ClusterName = info.ClusterName
			o.RoleName = info.RoleName
			o.RoleGroupName = info.RoleGroupName
			o.Annotations = info.GetAnnotations()
			o.Labels = info.GetLabels()

		},
	)

	metricsSvc := NewRoleGroupMetricsService(
		r.Client,
		&info,
	)
	return []reconciler.Reconciler{cm, sts, svc, metricsSvc}, nil
}
