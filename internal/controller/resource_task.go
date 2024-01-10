package controller

import (
	"context"
	"fmt"
	"github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
	"github.com/zncdata-labs/operator-go/pkg/status"
	"github.com/zncdata-labs/operator-go/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Deployment = "Deployment"
	Service    = "Service"
	Secret     = "Secret"
	ConfigMap  = "ConfigMap"
	Pvc        = "Pvc"
)

var (
	ResourceMapper = map[string]string{
		Deployment: status.ConditionTypeReconcileDeployment,
		Service:    status.ConditionTypeReconcileService,
		Secret:     status.ConditionTypeReconcileSecret,
		ConfigMap:  status.ConditionTypeReconcileConfigMap,
		Pvc:        status.ConditionTypeReconcilePVC,
	}
)

type ReconcileTask struct {
	resourceName  string
	reconcileFunc func(ctx context.Context, instance *v1alpha1.HiveMetastore) error
}

func ReconcileTasks(tasks *[]ReconcileTask, ctx context.Context, instance *v1alpha1.HiveMetastore,
	r *HiveMetastoreReconciler, serverName string) error {
	for _, task := range *tasks {
		jobFunc := task.reconcileFunc
		if err := jobFunc(ctx, instance); err != nil {
			r.Log.Error(err, fmt.Sprintf("unable to reconcile %s", task.resourceName))
			return err
		}
		if updated := instance.Status.SetStatusCondition(v1.Condition{
			Type:               ResourceMapper[task.resourceName],
			Status:             v1.ConditionTrue,
			Reason:             status.ConditionReasonRunning,
			Message:            createSuccessMessage(serverName, task.resourceName),
			ObservedGeneration: instance.GetGeneration(),
		}); updated {
			err := util.UpdateStatus(ctx, r.Client, instance)
			if err != nil {
				r.Log.Error(err, createUpdateErrorMessage(task.resourceName))
				return err
			}
		}
	}
	return nil
}

func createSuccessMessage(serverName string, resourceName string) string {
	return fmt.Sprintf("%sServer's %s is running", serverName, resourceName)
}

func createUpdateErrorMessage(resourceName string) string {
	return fmt.Sprintf("unable to update status for %s", resourceName)
}
