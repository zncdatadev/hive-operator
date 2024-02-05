package v1alpha1

type ConditionType string

type ConditionReason string

const (
	ConditionTypeClusterAvailable string = "Available"

	ConditionReasonReconcileDeployment ConditionReason = "ReconcileDeployment"
	ConditionReasonLackRepairs         ConditionReason = "LackRepairs"
	ConditionReasonReconcileService    ConditionReason = "ReconcileService"
	ConditionReasonReconcileIngress    ConditionReason = "ReconcileIngress"
	ConditionReasonReconcilePVC        ConditionReason = "ReconcilePVC"
	ConditionReasonReconcileSecret     ConditionReason = "ReconcileSecret"
)
