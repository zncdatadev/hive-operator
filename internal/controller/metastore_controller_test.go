package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

var _ = Describe("HiveMetastore controller", func() {

	Context("When reconciling a resource", func() {
		const resourceName = "test-hivemetastore"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		hiveMetastore := &hivev1alpha1.HiveMetastore{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind HiveMetastore")
			err := k8sClient.Get(ctx, typeNamespacedName, hiveMetastore)
			if err != nil && errors.IsNotFound(err) {
				resource := &hivev1alpha1.HiveMetastore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
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
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &hivev1alpha1.HiveMetastore{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance HiveMetastore")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &HiveMetastoreReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
