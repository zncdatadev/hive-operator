package controller

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	stackv1alpha1 "github.com/zncdata-labs/hive-metastore-operator/api/v1alpha1"
)

var _ = Describe("HiveMetastore controller", func() {

	Context("Reconcile", func() {
		It("Should reconcile a HiveMetastore resource", func() {
			ctx := context.Background()
			Expect(true).To(BeTrue())
			objs := &stackv1alpha1.HiveMetastoreList{}
			Expect(k8sClient.List(ctx, objs)).To(Succeed())
			Expect(len(objs.Items)).To(Equal(0))
		})
	})
})
