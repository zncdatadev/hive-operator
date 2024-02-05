package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HiveMetastore controller", func() {
	type User struct {
		Name  string
		Age   int
		Addr  *string
		Child *User
	}
	Context("Merge objects", func() {
		It("Should merge two objects", func() {
			var exclude []string
			left := &User{
				Name: "left",
			}
			right := &User{
				Name: "right",
				Age:  10,
			}

			expected := &User{
				Name: "left",
				Age:  10,
			}

			MergeObjects(left, right, exclude)

			Expect(left).To(Equal(expected))
		})
	})

})
