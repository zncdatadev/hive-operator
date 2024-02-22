package controller

import (
	hivev1alpha1 "github.com/zncdata-labs/hive-operator/api/v1alpha1"
	"strings"
)

type RoleLabels struct {
	cr   *hivev1alpha1.HiveMetastore
	name string
}

func (r *RoleLabels) GetLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       strings.ToLower(r.cr.Name),
		"app.kubernetes.io/component":  r.name,
		"app.kubernetes.io/managed-by": "hive-operator",
	}
}
