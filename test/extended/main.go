package extended

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

var _ = g.Describe("[Jira:openshift-controller-manager][sig-openshift-controller-manager] sanity test", func() {
	g.It("should always pass [Suite:openshift/openshift-controller-manager/conformance/parallel]", func() {
		o.Expect(true).To(o.BeTrue())
	})
})
