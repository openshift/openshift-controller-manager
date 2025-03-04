// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1 "github.com/openshift/api/operator/v1"
	operatorv1 "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	typedoperatorv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	gentype "k8s.io/client-go/gentype"
)

// fakeOpenShiftControllerManagers implements OpenShiftControllerManagerInterface
type fakeOpenShiftControllerManagers struct {
	*gentype.FakeClientWithListAndApply[*v1.OpenShiftControllerManager, *v1.OpenShiftControllerManagerList, *operatorv1.OpenShiftControllerManagerApplyConfiguration]
	Fake *FakeOperatorV1
}

func newFakeOpenShiftControllerManagers(fake *FakeOperatorV1) typedoperatorv1.OpenShiftControllerManagerInterface {
	return &fakeOpenShiftControllerManagers{
		gentype.NewFakeClientWithListAndApply[*v1.OpenShiftControllerManager, *v1.OpenShiftControllerManagerList, *operatorv1.OpenShiftControllerManagerApplyConfiguration](
			fake.Fake,
			"",
			v1.SchemeGroupVersion.WithResource("openshiftcontrollermanagers"),
			v1.SchemeGroupVersion.WithKind("OpenShiftControllerManager"),
			func() *v1.OpenShiftControllerManager { return &v1.OpenShiftControllerManager{} },
			func() *v1.OpenShiftControllerManagerList { return &v1.OpenShiftControllerManagerList{} },
			func(dst, src *v1.OpenShiftControllerManagerList) { dst.ListMeta = src.ListMeta },
			func(list *v1.OpenShiftControllerManagerList) []*v1.OpenShiftControllerManager {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1.OpenShiftControllerManagerList, items []*v1.OpenShiftControllerManager) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
