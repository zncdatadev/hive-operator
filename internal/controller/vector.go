package controller

import (
	"context"

	"emperror.dev/errors"
	"github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	appsv1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var vectorLogger = ctrl.Log.WithName("vector")

const ContainerVector = "vector"

func IsVectorEnable(roleLoggingConfig *v1alpha1.ContainerLoggingSpec) bool {
	if roleLoggingConfig != nil {
		return roleLoggingConfig.EnableVectorAgent
	}
	return false

}

type VectorConfigParams struct {
	Client        client.Client
	ClusterConfig *v1alpha1.ClusterConfigSpec
	Namespace     string
	InstanceName  string
	Role          string
	GroupName     string
}

func generateVectorYAML(ctx context.Context, params VectorConfigParams) (string, error) {
	aggregatorConfigMapName := params.ClusterConfig.VectorAggregatorConfigMapName
	if aggregatorConfigMapName == "" {
		return "", errors.New("vectorAggregatorConfigMapName is not set")
	}
	return builder.MakeVectorYaml(ctx, params.Client, params.Namespace, params.InstanceName, params.Role,
		params.GroupName, aggregatorConfigMapName)
}

func ExtendConfigMapByVector(ctx context.Context, params VectorConfigParams, data map[string]string) {
	vectorYaml, err := generateVectorYAML(ctx, params)
	if err != nil {
		vectorLogger.Error(errors.Wrap(err, "error creating vector YAML"), "failed to create vector YAML")
	} else {
		data[builder.VectorConfigFile] = vectorYaml
	}
}

func ExtendWorkloadByVector(
	logProvider []string,
	dep *appsv1.Deployment,
	vectorConfigMapName string) {
	decorator := builder.VectorDecorator{
		WorkloadObject:           dep,
		LogVolumeName:            v1alpha1.KubeDataLogDirName,
		VectorConfigVolumeName:   v1alpha1.KubeDataConfigMountDirName,
		VectorConfigMapName:      vectorConfigMapName,
		LogProviderContainerName: logProvider,
	}
	err := decorator.Decorate()
	if err != nil {
		return
	}
}
