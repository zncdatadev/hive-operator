package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("HiveMetastore controller", func() {

	Context("Reconcile", func() {
		It("Should reconcile a HiveMetastore resource", func() {
			ctx := context.Background()

			// Create a HiveMetastore resource
			hiveMetastore := &hivev1alpha1.HiveMetastore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hivemetastore",
					Namespace: "default",
				},
				Spec: hivev1alpha1.HiveMetastoreSpec{
					ClusterConfig: &hivev1alpha1.ClusterConfigSpec{
						Database: &hivev1alpha1.DatabaseSpec{},
					},
					Metastore: &hivev1alpha1.RoleSpec{
						RoleGroups: map[string]*hivev1alpha1.RoleGroupSpec{
							"default": {
								Replicas: 1,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hiveMetastore)).To(Succeed())

			// List HiveMetastore resources and verify the created resource is present
			objs := &hivev1alpha1.HiveMetastoreList{}
			Expect(k8sClient.List(ctx, objs)).To(Succeed())
			Expect(objs.Items).To(HaveLen(1))
			Expect(objs.Items[0].Name).To(Equal("test-hivemetastore"))
		})
	})
})
