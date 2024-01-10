package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"

	goerr "errors"
	"github.com/cisco-open/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = ctrl.Log.WithName("utils")
)

func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	namespace := obj.GetNamespace()

	kinds, _, _ := scheme.Scheme.ObjectKinds(obj)

	name := obj.GetName()
	current := obj.DeepCopyObject().(client.Object)
	// Check if the object exists, if not create a new one
	err := c.Get(ctx, key, current)
	if errors.IsNotFound(err) {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(obj); err != nil {
			return err
		}
		logger.Info("Creating a new object", "Kind", kinds, "Namespace", namespace, "Name", name)
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
		result, err := patch.DefaultPatchMaker.Calculate(current, obj, patch.IgnoreStatusFields())
		if err != nil {
			logger.Error(err, "failed to calculate patch to match objects, moving on to update")
			// if there is an error with matching, we still want to update
			resourceVersion := current.(metav1.ObjectMetaAccessor).GetObjectMeta().GetResourceVersion()
			obj.(metav1.ObjectMetaAccessor).GetObjectMeta().SetResourceVersion(resourceVersion)

			return c.Update(ctx, obj)
		}

		if !result.IsEmpty() {
			logger.Info(fmt.Sprintf("Resource update for object %s:%s", kinds, obj.(metav1.ObjectMetaAccessor).GetObjectMeta().GetName()),
				"patch", string(result.Patch),
				// "original", string(result.Original),
				// "modified", string(result.Modified),
				// "current", string(result.Current),
			)

			err := patch.DefaultAnnotator.SetLastAppliedAnnotation(obj)
			if err != nil {
				logger.Error(err, "failed to annotate modified object", "object", obj)
			}

			resourceVersion := current.(metav1.ObjectMetaAccessor).GetObjectMeta().GetResourceVersion()
			obj.(metav1.ObjectMetaAccessor).GetObjectMeta().SetResourceVersion(resourceVersion)

			return c.Update(ctx, obj)
		}

		logger.V(1).Info(fmt.Sprintf("Skipping update for object %s:%s", kinds, obj.(metav1.ObjectMetaAccessor).GetObjectMeta().GetName()))

	}
	return err
}

type Map map[string]string

func (m *Map) MapMerge(source map[string]string, replace bool) {
	if *m == nil {
		*m = make(Map)
	}
	for sourceKey, sourceValue := range source {
		if _, ok := map[string]string(*m)[sourceKey]; !ok || replace {
			map[string]string(*m)[sourceKey] = sourceValue
		}
	}
}

// Convert converts a struct to a map for easy iteration with for range.
// `struc` can be a pointer or a concrete struct.
// error will be nil if everything worked.
func Convert(struc interface{}) (map[string]interface{}, error) {

	returnMap := make(map[string]interface{})

	sType := getStructType(struc)

	if sType.Kind() != reflect.Struct {
		return returnMap, goerr.New("variable given is not a struct or a pointer to a struct")
	}

	for i := 0; i < sType.NumField(); i++ {
		structFieldName := sType.Field(i).Name
		structVal := reflect.ValueOf(struc)
		returnMap[structFieldName] = structVal.FieldByName(structFieldName).Interface()
	}

	return returnMap, nil
}

func getStructType(struc interface{}) reflect.Type {
	sType := reflect.TypeOf(struc)
	if sType.Kind() == reflect.Ptr {
		sType = sType.Elem()
	}

	return sType
}

func extractDecodeData(data *map[string][]byte, key string) (*string, error) {
	obj := *data
	if usernameByte, ok := obj[key]; ok {
		if decodedUsr, err := base64.StdEncoding.DecodeString(string(usernameByte)); err != nil {
			return nil, err
		} else {
			username := string(decodedUsr)
			return &username, nil
		}
	}
	return nil, fmt.Errorf("byte map data not contain key: %s", key)
}
