package graphqlws

import (
	"github.com/graphql-go/graphql/gqlerrors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {
	Describe("HandlerError", func() {
		It("should prevent a default behavior of a event", func() {
			err := HandlerError{}
			Expect(err.defaultPrevented).To(BeFalse())
			err.PreventDefault()
			Expect(err.defaultPrevented).To(BeTrue())
		})

		It("should stop propagation", func() {
			err := HandlerError{}
			Expect(err.propagationStopped).To(BeFalse())
			err.StopPropagation()
			Expect(err.propagationStopped).To(BeTrue())
		})

		It("should return an empty error message", func() {
			err := HandlerError{}
			Expect(err.Error()).To(BeEmpty())
		})
	})

	Describe("ErrorsFromGraphQLErrors", func() {
		It("should return no error when an empty list is provided", func() {
			Expect(ErrorsFromGraphQLErrors([]gqlerrors.FormattedError{})).To(BeNil())
		})

		It("should return a list with the errors", func() {
			errs := ErrorsFromGraphQLErrors([]gqlerrors.FormattedError{
				{
					Message: "message1",
				},
				{
					Message: "message2",
				},
			})
			Expect(errs).To(HaveLen(2))
			Expect(errs[0].Error()).To(Equal("message1"))
			Expect(errs[1].Error()).To(Equal("message2"))

		})
	})
})
