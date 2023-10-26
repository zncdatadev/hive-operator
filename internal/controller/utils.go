package controller

import (
	"context"

	"github.com/cisco-open/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object) error {
	logger := log.FromContext(ctx)
	kind := obj.GetObjectKind().GroupVersionKind().GroupKind().String()
	namespace := obj.GetNamespace()
	name := obj.GetName()
	key := client.ObjectKeyFromObject(obj)
	current := obj.DeepCopyObject().(client.Object)
	// Check if the object exists, if not create a new one
	err := c.Get(ctx, key, current)
	if errors.IsNotFound(err) {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(obj); err != nil {
			return err
		}
		logger.Info("Creating a new object", "Kind", kind, "Namespace", namespace, "Name", name)
		return c.Create(ctx, obj)
	} else if err == nil {
		switch obj.(type) {
		case *corev1.Service:
			currentSvc := current.(*corev1.Service)
			svc := obj.(*corev1.Service)
			// Preserve the ClusterIP when updating the service
			svc.Spec.ClusterIP = currentSvc.Spec.ClusterIP
			// Preserve the annotation when updating the service, ensure any updated annotation is preserved
			//for key, value := range currentSvc.Annotations {
			//	if _, present := svc.Annotations[key]; !present {
			//		svc.Annotations[key] = value
			//	}
			//}

			if svc.Spec.Type == corev1.ServiceTypeNodePort || svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				for i := range svc.Spec.Ports {
					svc.Spec.Ports[i].NodePort = currentSvc.Spec.Ports[i].NodePort
				}
			}
		}
		// If the object exists, update it
		patchResult, err := patch.DefaultPatchMaker.Calculate(current, obj, patch.IgnoreStatusFields())
		if err != nil {
			return err
		}
		if !patchResult.IsEmpty() {
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(obj); err != nil {
				return err
			}
			logger.Info("Updating the object", "Kind", kind, "Namespace", namespace, "Name", name)
			return c.Update(ctx, obj)
		}
	}
	return err
}
