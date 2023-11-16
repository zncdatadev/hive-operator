package controller

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stackv1alpha1 "github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
)

// make service
func (r *HiveMetastoreReconciler) makeService(instance *stackv1alpha1.HiveMetastore, schema *runtime.Scheme) *corev1.Service {
	labels := instance.GetLabels()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Labels:      labels,
			Annotations: instance.Spec.Service.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     instance.Spec.Service.Port,
					Name:     "http",
					Protocol: "TCP",
				},
			},
			Selector: labels,
			Type:     instance.Spec.Service.Type,
		},
	}
	err := ctrl.SetControllerReference(instance, svc, schema)
	if err != nil {
		r.Log.Error(err, "Failed to set controller reference for service")
		return nil
	}
	return svc
}

func (r *HiveMetastoreReconciler) reconcileService(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	logger := log.FromContext(ctx)
	obj := r.makeService(instance, r.Scheme)
	if obj == nil {
		return nil
	}

	if err := CreateOrUpdate(ctx, r.Client, obj); err != nil {
		logger.Error(err, "Failed to create or update service")
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) makeDeployment(instance *stackv1alpha1.HiveMetastore, schema *runtime.Scheme) *appsv1.Deployment {
	labels := instance.GetLabels()
	secretVarNames := []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_HOST", "POSTGRES_PORT"}
	var envVars []corev1.EnvVar
	for _, secretVarName := range secretVarNames {
		envVar := corev1.EnvVar{
			Name: secretVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: secretVarName,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.GetNameWithSuffix("secret"),
					},
				},
			},
		}
		envVars = append(envVars, envVar)
	}
	envVars = append(envVars,
		corev1.EnvVar{
			Name:  "SERVICE_NAME",
			Value: "metastore",
		},
		corev1.EnvVar{
			Name:  "DB_DRIVER",
			Value: "postgres",
		},
		corev1.EnvVar{
			Name: "SERVICE_OPTS",
			Value: "-Djavax.jdo.option.ConnectionDriverName=org.postgresql.Driver " +
				"-Djavax.jdo.option.ConnectionURL=jdbc:postgresql://$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB) " +
				"-Djavax.jdo.option.ConnectionUserName=$(POSTGRES_USER) " +
				"-Djavax.jdo.option.ConnectionPassword=$(POSTGRES_PASSWORD)",
		})
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &instance.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					SecurityContext: instance.Spec.SecurityContext,
					Containers: []corev1.Container{
						{
							Name:            instance.Name,
							Image:           instance.Spec.Image.Repository + ":" + instance.Spec.Image.Tag,
							ImagePullPolicy: instance.Spec.Image.PullPolicy,
							Env:             envVars,
							Resources:       *instance.Spec.Resources,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 18080,
									Name:          "http",
									Protocol:      "TCP",
								},
							},
						},
					},
					Tolerations: instance.Spec.Tolerations,
				},
			},
		},
	}
	err := ctrl.SetControllerReference(instance, dep, schema)
	if err != nil {
		r.Log.Error(err, "Failed to set controller reference for deployment")
		return nil
	}
	return dep
}

func (r *HiveMetastoreReconciler) updateStatusConditionWithDeployment(ctx context.Context, instance *stackv1alpha1.HiveMetastore, status metav1.ConditionStatus, message string) error {
	instance.SetStatusCondition(metav1.Condition{
		Type:               stackv1alpha1.ConditionTypeProgressing,
		Status:             status,
		Reason:             stackv1alpha1.ConditionReasonReconcileDeployment,
		Message:            message,
		ObservedGeneration: instance.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	})

	if err := r.UpdateStatus(ctx, instance); err != nil {
		return err
	}
	return nil
}

func (r *HiveMetastoreReconciler) reconcileDeployment(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	obj := r.makeDeployment(instance, r.Scheme)
	if obj == nil {
		return nil
	}
	if err := CreateOrUpdate(ctx, r.Client, obj); err != nil {
		r.Log.Error(err, "Failed to create or update deployment")
		return err
	}
	return nil
}

func makeSecret(ctx context.Context, instance *stackv1alpha1.HiveMetastore, schema *runtime.Scheme) []*corev1.Secret {
	var secrets []*corev1.Secret
	labels := instance.GetLabels()
	data := make(map[string][]byte)
	data["POSTGRES_HOST"] = []byte(instance.Spec.PostgresSecret.Host)
	data["POSTGRES_PORT"] = []byte(instance.Spec.PostgresSecret.Port)
	data["POSTGRES_USER"] = []byte(instance.Spec.PostgresSecret.UserName)
	data["POSTGRES_PASSWORD"] = []byte(instance.Spec.PostgresSecret.Password)
	data["POSTGRES_DB"] = []byte(instance.Spec.PostgresSecret.DataBase)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetNameWithSuffix("secret"),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	secrets = append(secrets, secret)
	return secrets
}

func (r *HiveMetastoreReconciler) reconcileSecret(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	objs := makeSecret(ctx, instance, r.Scheme)

	if objs == nil {
		return nil
	}
	for _, obj := range objs {
		if err := CreateOrUpdate(ctx, r.Client, obj); err != nil {
			r.Log.Error(err, "Failed to create or update secret")
			return err
		}
	}
	return nil
}
