package util

import "github.com/zncdatadev/operator-go/pkg/reconciler"

func GetMetricsServiceName(roleGroupInfo *reconciler.RoleGroupInfo) string {
	return roleGroupInfo.GetFullName() + "-metrics"
}
