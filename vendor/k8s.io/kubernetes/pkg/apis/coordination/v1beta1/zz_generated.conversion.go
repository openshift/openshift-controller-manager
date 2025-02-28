//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by conversion-gen. DO NOT EDIT.

package v1beta1

import (
	unsafe "unsafe"

	coordinationv1 "k8s.io/api/coordination/v1"
	coordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	coordination "k8s.io/kubernetes/pkg/apis/coordination"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*coordinationv1beta1.Lease)(nil), (*coordination.Lease)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_Lease_To_coordination_Lease(a.(*coordinationv1beta1.Lease), b.(*coordination.Lease), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*coordination.Lease)(nil), (*coordinationv1beta1.Lease)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_coordination_Lease_To_v1beta1_Lease(a.(*coordination.Lease), b.(*coordinationv1beta1.Lease), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*coordinationv1beta1.LeaseList)(nil), (*coordination.LeaseList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_LeaseList_To_coordination_LeaseList(a.(*coordinationv1beta1.LeaseList), b.(*coordination.LeaseList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*coordination.LeaseList)(nil), (*coordinationv1beta1.LeaseList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_coordination_LeaseList_To_v1beta1_LeaseList(a.(*coordination.LeaseList), b.(*coordinationv1beta1.LeaseList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*coordinationv1beta1.LeaseSpec)(nil), (*coordination.LeaseSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_LeaseSpec_To_coordination_LeaseSpec(a.(*coordinationv1beta1.LeaseSpec), b.(*coordination.LeaseSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*coordination.LeaseSpec)(nil), (*coordinationv1beta1.LeaseSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_coordination_LeaseSpec_To_v1beta1_LeaseSpec(a.(*coordination.LeaseSpec), b.(*coordinationv1beta1.LeaseSpec), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta1_Lease_To_coordination_Lease(in *coordinationv1beta1.Lease, out *coordination.Lease, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1beta1_LeaseSpec_To_coordination_LeaseSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1beta1_Lease_To_coordination_Lease is an autogenerated conversion function.
func Convert_v1beta1_Lease_To_coordination_Lease(in *coordinationv1beta1.Lease, out *coordination.Lease, s conversion.Scope) error {
	return autoConvert_v1beta1_Lease_To_coordination_Lease(in, out, s)
}

func autoConvert_coordination_Lease_To_v1beta1_Lease(in *coordination.Lease, out *coordinationv1beta1.Lease, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_coordination_LeaseSpec_To_v1beta1_LeaseSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

// Convert_coordination_Lease_To_v1beta1_Lease is an autogenerated conversion function.
func Convert_coordination_Lease_To_v1beta1_Lease(in *coordination.Lease, out *coordinationv1beta1.Lease, s conversion.Scope) error {
	return autoConvert_coordination_Lease_To_v1beta1_Lease(in, out, s)
}

func autoConvert_v1beta1_LeaseList_To_coordination_LeaseList(in *coordinationv1beta1.LeaseList, out *coordination.LeaseList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]coordination.Lease)(unsafe.Pointer(&in.Items))
	return nil
}

// Convert_v1beta1_LeaseList_To_coordination_LeaseList is an autogenerated conversion function.
func Convert_v1beta1_LeaseList_To_coordination_LeaseList(in *coordinationv1beta1.LeaseList, out *coordination.LeaseList, s conversion.Scope) error {
	return autoConvert_v1beta1_LeaseList_To_coordination_LeaseList(in, out, s)
}

func autoConvert_coordination_LeaseList_To_v1beta1_LeaseList(in *coordination.LeaseList, out *coordinationv1beta1.LeaseList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]coordinationv1beta1.Lease)(unsafe.Pointer(&in.Items))
	return nil
}

// Convert_coordination_LeaseList_To_v1beta1_LeaseList is an autogenerated conversion function.
func Convert_coordination_LeaseList_To_v1beta1_LeaseList(in *coordination.LeaseList, out *coordinationv1beta1.LeaseList, s conversion.Scope) error {
	return autoConvert_coordination_LeaseList_To_v1beta1_LeaseList(in, out, s)
}

func autoConvert_v1beta1_LeaseSpec_To_coordination_LeaseSpec(in *coordinationv1beta1.LeaseSpec, out *coordination.LeaseSpec, s conversion.Scope) error {
	out.HolderIdentity = (*string)(unsafe.Pointer(in.HolderIdentity))
	out.LeaseDurationSeconds = (*int32)(unsafe.Pointer(in.LeaseDurationSeconds))
	out.AcquireTime = (*v1.MicroTime)(unsafe.Pointer(in.AcquireTime))
	out.RenewTime = (*v1.MicroTime)(unsafe.Pointer(in.RenewTime))
	out.LeaseTransitions = (*int32)(unsafe.Pointer(in.LeaseTransitions))
	out.Strategy = (*coordination.CoordinatedLeaseStrategy)(unsafe.Pointer(in.Strategy))
	out.PreferredHolder = (*string)(unsafe.Pointer(in.PreferredHolder))
	return nil
}

// Convert_v1beta1_LeaseSpec_To_coordination_LeaseSpec is an autogenerated conversion function.
func Convert_v1beta1_LeaseSpec_To_coordination_LeaseSpec(in *coordinationv1beta1.LeaseSpec, out *coordination.LeaseSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_LeaseSpec_To_coordination_LeaseSpec(in, out, s)
}

func autoConvert_coordination_LeaseSpec_To_v1beta1_LeaseSpec(in *coordination.LeaseSpec, out *coordinationv1beta1.LeaseSpec, s conversion.Scope) error {
	out.HolderIdentity = (*string)(unsafe.Pointer(in.HolderIdentity))
	out.LeaseDurationSeconds = (*int32)(unsafe.Pointer(in.LeaseDurationSeconds))
	out.AcquireTime = (*v1.MicroTime)(unsafe.Pointer(in.AcquireTime))
	out.RenewTime = (*v1.MicroTime)(unsafe.Pointer(in.RenewTime))
	out.LeaseTransitions = (*int32)(unsafe.Pointer(in.LeaseTransitions))
	out.Strategy = (*coordinationv1.CoordinatedLeaseStrategy)(unsafe.Pointer(in.Strategy))
	out.PreferredHolder = (*string)(unsafe.Pointer(in.PreferredHolder))
	return nil
}

// Convert_coordination_LeaseSpec_To_v1beta1_LeaseSpec is an autogenerated conversion function.
func Convert_coordination_LeaseSpec_To_v1beta1_LeaseSpec(in *coordination.LeaseSpec, out *coordinationv1beta1.LeaseSpec, s conversion.Scope) error {
	return autoConvert_coordination_LeaseSpec_To_v1beta1_LeaseSpec(in, out, s)
}
