package controller

import (
	"strconv"

	"github.com/zncdatadev/hive-operator/internal/constant"
	"github.com/zncdatadev/hive-operator/internal/util"
	"github.com/zncdatadev/operator-go/pkg/builder"
	client "github.com/zncdatadev/operator-go/pkg/client"
	opconstants "github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
)

// NewRoleGroupMetricsService creates a metrics service reconciler using a simple function approach
// This creates a headless service for metrics with Prometheus labels and annotations
func NewRoleGroupMetricsService(
	client *client.Client,
	roleGroupInfo *reconciler.RoleGroupInfo,
) reconciler.Reconciler {
	// Get metrics port
	metricsPort := constant.MetricsPort

	// Create service ports
	servicePorts := []corev1.ContainerPort{
		{
			Name:          constant.MetricsPortName,
			ContainerPort: constant.MetricsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// Create service name with -metrics suffix
	serviceName := util.GetMetricsServiceName(roleGroupInfo)

	scheme := "http"
	// Prepare labels (copy from roleGroupInfo and add metrics labels)
	labels := make(map[string]string)
	for k, v := range roleGroupInfo.GetLabels() {
		labels[k] = v
	}
	labels["prometheus.io/scrape"] = constant.TrueValue

	// Prepare annotations (copy from roleGroupInfo and add Prometheus annotations)
	annotations := make(map[string]string)
	for k, v := range roleGroupInfo.GetAnnotations() {
		annotations[k] = v
	}
	annotations["prometheus.io/scrape"] = constant.TrueValue
	// annotations["prometheus.io/path"] = "/metrics"  // Uncomment and modify if a specific path is needed, default is /metrics
	annotations["prometheus.io/port"] = strconv.Itoa(metricsPort)
	annotations["prometheus.io/scheme"] = scheme

	// Create base service builder
	baseBuilder := builder.NewServiceBuilder(
		client,
		serviceName,
		servicePorts,
		func(sbo *builder.ServiceBuilderOptions) {
			sbo.Headless = true
			sbo.ListenerClass = opconstants.ClusterInternal
			sbo.Labels = labels
			sbo.MatchingLabels = roleGroupInfo.GetLabels() // Use original labels for matching
			sbo.Annotations = annotations
		},
	)

	return reconciler.NewGenericResourceReconciler(
		client,
		baseBuilder,
	)
}
