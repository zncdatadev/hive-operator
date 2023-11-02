package v1alpha1

const (
	ConditionTypeProgressing string = "Progressing"
	ConditionTypeReconcile   string = "Reconcile"
	ConditionTypeAvailable   string = "Available"

	ConditionReasonPreparing           string = "Preparing"
	ConditionReasonRunning             string = "Running"
	ConditionReasonConfig              string = "Config"
	ConditionReasonReconcilePVC        string = "ReconcilePVC"
	ConditionReasonReconcileService    string = "ReconcileService"
	ConditionReasonReconcileIngress    string = "ReconcileIngress"
	ConditionReasonReconcileDeployment string = "ReconcileDeployment"
)
