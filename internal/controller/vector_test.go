package controller_test

// Returns true when roleLoggingConfig is not nil and EnableVectorAgent is true
import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/hive-operator/internal/controller"
)

var _ = Describe("IsVectorEnable", func() {
	Context("when roleLoggingConfig is not nil and EnableVectorAgent is true", func() {
		It("should return true", func() {
			// given
			roleLoggingConfig := &v1alpha1.ContainerLoggingSpec{
				EnableVectorAgent: true,
			}

			// when
			result := controller.IsVectorEnable(roleLoggingConfig)

			// then
			Expect(result).To(BeTrue())
		})
	})

	Context("when roleLoggingConfig is not nil and EnableVectorAgent is false", func() {
		It("should return false", func() {
			// given
			roleLoggingConfig := &v1alpha1.ContainerLoggingSpec{
				EnableVectorAgent: false,
			}

			// when
			result := controller.IsVectorEnable(roleLoggingConfig)

			// then
			Expect(result).To(BeFalse())
		})
	})

	Context("when roleLoggingConfig is nil", func() {
		It("should return false", func() {
			// given
			var roleLoggingConfig *v1alpha1.ContainerLoggingSpec = nil

			// when
			result := controller.IsVectorEnable(roleLoggingConfig)

			// then
			Expect(result).To(BeFalse())
		})
	})
})
