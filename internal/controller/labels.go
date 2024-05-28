package controller

import (
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"strings"
)

const (
	LabelCrName    = "app.kubernetes.io/Name"
	LabelComponent = "app.kubernetes.io/component"
	LabelManagedBy = "app.kubernetes.io/managed-by"
)

type RoleLabels struct {
	cr   *hivev1alpha1.HiveMetastore
	name string
}

func (r *RoleLabels) GetLabels() map[string]string {
	return map[string]string{
		LabelCrName:    strings.ToLower(r.cr.Name),
		LabelComponent: r.name,
		LabelManagedBy: "hive-operator",
	}
}
