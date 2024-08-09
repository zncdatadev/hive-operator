package controller_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zncdatadev/hive-operator/internal/controller"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

// Handles empty or missing ClusterConfig gracefully
var _ = Describe("DeploymentReconciler", func() {
	Context("when generating args", func() {
		It("should correctly generate args", func() {
			// given
			cr := &hivev1alpha1.HiveMetastore{
				Spec: hivev1alpha1.HiveMetastoreSpec{
					ClusterConfig: &hivev1alpha1.ClusterConfigSpec{
						Authentication: &hivev1alpha1.AuthenticationSpec{
							Kerberos: &hivev1alpha1.KerberosSpec{
								SecretClass: "",
							},
						},
					},
				},
			}
			r := controller.NewReconcileDeployment(nil, nil, cr, "", "", nil, false)

			// when
			args := r.Args()

			// then
			res := args[0]
			Expect(args).NotTo(BeNil())
			Expect(res).NotTo(ContainSubstring("{{"))
		})

		It("should handle empty or missing ClusterConfig gracefully", func() {
			// given
			cr := &hivev1alpha1.HiveMetastore{}
			r := controller.NewReconcileDeployment(nil, nil, cr, "", "", nil, false)

			// when
			args := r.Args()

			// then
			res := args[0]
			Expect(args).NotTo(BeNil())
			Expect(res).NotTo(ContainSubstring("kerberos"))
		})
	})
})
