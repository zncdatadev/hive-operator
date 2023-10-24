package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stackv1alpha1 "github.com/zncdata-labs/hive-metadata-operator/api/v1alpha1"
)

// make service
func makeService(ctx context.Context, instance *stackv1alpha1.HiveMetastore, schema *runtime.Scheme) *corev1.Service {
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
		return nil
	}
	return svc
}

func (r *HiveMetastoreReconciler) reconcileService(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	logger := log.FromContext(ctx)
	obj := makeService(ctx, instance, r.Scheme)
	if obj == nil {
		return nil
	}

	if err := CreateOrUpdate(ctx, r.Client, obj); err != nil {
		logger.Error(err, "Failed to create or update service")
		return err
	}
	return nil
}

func makeDeployment(ctx context.Context, instance *stackv1alpha1.HiveMetastore, schema *runtime.Scheme) *appsv1.Deployment {
	labels := instance.GetLabels()
	secretVarNames := []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_HOST", "POSTGRES_PORT"}
	envVars := []corev1.EnvVar{}

	for _, secretVarName := range secretVarNames {
		envVar := corev1.EnvVar{
			Name: secretVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: secretVarName,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "hive-postgres",
					},
				},
			},
		}
		envVars = append(envVars, envVar)
	}
	envVars = append(envVars,
		corev1.EnvVar{
			Name:  "POSTGRES_HOST",
			Value: instance.Spec.PostgresSecret["host"],
		},
		corev1.EnvVar{
			Name:  "POSTGRES_PORT",
			Value: instance.Spec.PostgresSecret["port"],
		},
		corev1.EnvVar{
			Name:  "SERVICE_NAME",
			Value: "metastore",
		},
		corev1.EnvVar{
			Name: "SERVICE_OPTS",
			Value: `-Xmx1G -Djavax.jdo.option.ConnectionDriverName=org.postgresql.Driver
			-Djavax.jdo.option.ConnectionURL=jdbc:postgresql://${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}
			-Djavax.jdo.option.ConnectionUserName=${POSTGRES_USER}
			-Djavax.jdo.option.ConnectionPassword=${POSTGRES_PASSWORD}`,
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
					Containers: []corev1.Container{
						{
							Name:            instance.Name,
							Image:           instance.Spec.Image.Repository + ":" + instance.Spec.Image.Tag,
							ImagePullPolicy: instance.Spec.Image.PullPolicy,
							Env:             envVars,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 18080,
									Name:          "http",
									Protocol:      "TCP",
								},
							},
							// VolumeMounts: []corev1.VolumeMount{
							// 	{
							// 		Name:      instance.GetNameWithSuffix("-data"),
							// 		MountPath: "/tmp/spark-events",
							// 	},
							// },
						},
					},
					Tolerations: instance.Spec.Tolerations,
					// Volumes: []corev1.Volume{
					// 	{
					// 		Name: instance.GetNameWithSuffix("-data"),
					// 		VolumeSource: corev1.VolumeSource{
					// 			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					// 				ClaimName: instance.GetPvcName(),
					// 			},
					// 		},
					// 	},
					// },
				},
			},
		},
	}
	err := ctrl.SetControllerReference(instance, dep, schema)
	if err != nil {
		return nil
	}
	return dep
}

func (r *HiveMetastoreReconciler) reconcileDeployment(ctx context.Context, instance *stackv1alpha1.HiveMetastore) error {
	logger := log.FromContext(ctx)
	obj := makeDeployment(ctx, instance, r.Scheme)
	if obj == nil {
		return nil
	}
	if err := CreateOrUpdate(ctx, r.Client, obj); err != nil {
		logger.Error(err, "Failed to create or update deployment")
		return err
	}
	return nil
}
