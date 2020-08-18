package overrides

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/admission"

	buildv1 "github.com/openshift/api/build/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	testutil "github.com/openshift/openshift-controller-manager/pkg/build/controller/common/testutil"
)

func TestBuildOverrideForcePull(t *testing.T) {
	truePtr := true
	falsePtr := false
	tests := []struct {
		name      string
		build     *buildv1.Build
		forcePull *bool
	}{
		{
			name:  "build - custom - forcePull: nil",
			build: testutil.Build().WithCustomStrategy().AsBuild(),
		},
		{
			name:  "build - docker - forcePull: nil",
			build: testutil.Build().WithDockerStrategy().AsBuild(),
		},
		{
			name:  "build - source - forcePull: nil",
			build: testutil.Build().WithSourceStrategy().AsBuild(),
		},
		{
			name:      "build - custom - forcePull: true",
			build:     testutil.Build().WithCustomStrategy().AsBuild(),
			forcePull: &truePtr,
		},
		{
			name:      "build - docker - forcePull: true",
			build:     testutil.Build().WithDockerStrategy().AsBuild(),
			forcePull: &truePtr,
		},
		{
			name:      "build - source - forcePull: true",
			build:     testutil.Build().WithSourceStrategy().AsBuild(),
			forcePull: &truePtr,
		},
		{
			name:      "build - custom - forcePull: false",
			build:     testutil.Build().WithCustomStrategy().AsBuild(),
			forcePull: &falsePtr,
		},
		{
			name:      "build - docker - forcePull: false",
			build:     testutil.Build().WithDockerStrategy().AsBuild(),
			forcePull: &falsePtr,
		},
		{
			name:      "build - source - forcePull: false",
			build:     testutil.Build().WithSourceStrategy().AsBuild(),
			forcePull: &falsePtr,
		},
	}

	ops := []admission.Operation{admission.Create, admission.Update}
	for _, test := range tests {
		for _, op := range ops {
			overrides := BuildOverrides{Config: &openshiftcontrolplanev1.BuildOverridesConfig{ForcePull: test.forcePull}}
			pod := testutil.Pod().WithBuild(t, test.build)
			err := overrides.ApplyOverrides((*v1.Pod)(pod))
			if err != nil {
				t.Errorf("%s: unexpected error: %v", test.name, err)
			}
			build := pod.GetBuild(t)
			strategy := build.Spec.Strategy
			switch {
			case strategy.CustomStrategy != nil:
				if test.forcePull == nil {
					if strategy.CustomStrategy.ForcePull {
						t.Errorf("%s (%s): force pull was %t but should have been %t", test.name, op, strategy.CustomStrategy.ForcePull, false)
					}
				} else {
					pullPolicy := v1.PullIfNotPresent
					if *test.forcePull {
						pullPolicy = v1.PullAlways
					}
					if strategy.CustomStrategy.ForcePull != *test.forcePull {
						t.Errorf("%s (%s): force pull was %t but should have been %t", test.name, op, strategy.CustomStrategy.ForcePull, *test.forcePull)
					}
					if pod.Spec.Containers[0].ImagePullPolicy != pullPolicy {
						t.Errorf("%s (%s): Container image pull policy was %s but should have been %s", test.name, op, pod.Spec.Containers[0].ImagePullPolicy, pullPolicy)
					}
					if pod.Spec.InitContainers[0].ImagePullPolicy != pullPolicy {
						t.Errorf("%s (%s): InitContainer image pull policy was %s but should have been %s", test.name, op, pod.Spec.InitContainers[0].ImagePullPolicy, pullPolicy)
					}
				}

			case strategy.DockerStrategy != nil:
				if test.forcePull == nil {
					if strategy.DockerStrategy.ForcePull {
						t.Errorf("%s (%s): force pull was %t but should have been %t", test.name, op, strategy.DockerStrategy.ForcePull, false)
					}
				} else {
					if strategy.DockerStrategy.ForcePull != *test.forcePull {
						t.Errorf("%s (%s): force pull was %t but should have been %t", test.name, op, strategy.DockerStrategy.ForcePull, *test.forcePull)
					}
				}
			case strategy.SourceStrategy != nil:
				if test.forcePull == nil {
					if strategy.SourceStrategy.ForcePull {
						t.Errorf("%s (%s): force pull was %t but should have been %t", test.name, op, strategy.SourceStrategy.ForcePull, false)
					}
				} else {
					if strategy.SourceStrategy.ForcePull != *test.forcePull {
						t.Errorf("%s (%s): force pull was %t but should have been %t", test.name, op, strategy.SourceStrategy.ForcePull, *test.forcePull)
					}
				}

			}
		}
	}
}

func TestLabelOverrides(t *testing.T) {
	tests := []struct {
		buildLabels    []buildv1.ImageLabel
		overrideLabels []buildv1.ImageLabel
		expected       []buildv1.ImageLabel
	}{
		{
			buildLabels:    nil,
			overrideLabels: nil,
			expected:       nil,
		},
		{
			buildLabels: nil,
			overrideLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
		},
		{
			buildLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			overrideLabels: nil,
			expected: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
		},
		{
			buildLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "public",
				},
			},
			overrideLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
		},
		{
			buildLabels: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
			},
			overrideLabels: []buildv1.ImageLabel{
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
			expected: []buildv1.ImageLabel{
				{
					Name:  "distribution-scope",
					Value: "private",
				},
				{
					Name:  "changelog-url",
					Value: "file:///dev/null",
				},
			},
		},
	}

	for i, test := range tests {
		overridesConfig := &openshiftcontrolplanev1.BuildOverridesConfig{
			ImageLabels: test.overrideLabels,
		}

		admitter := BuildOverrides{overridesConfig}
		pod := testutil.Pod().WithBuild(t, testutil.Build().WithImageLabels(test.buildLabels).AsBuild())
		err := admitter.ApplyOverrides((*v1.Pod)(pod))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		build := pod.GetBuild(t)

		result := build.Spec.Output.ImageLabels
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("expected[%d]: %v, got: %v", i, test.expected, result)
		}
	}
}

func TestBuildOverrideNodeSelector(t *testing.T) {
	tests := []struct {
		name      string
		build     *buildv1.Build
		overrides map[string]string
		expected  map[string]string
	}{
		{
			name:      "build - full override",
			build:     testutil.Build().WithNodeSelector(map[string]string{"key1": "value1"}).AsBuild(),
			overrides: map[string]string{"key1": "override1", "key2": "override2"},
			expected:  map[string]string{"key1": "override1", "key2": "override2"},
		},
		{
			name:      "build - partial override",
			build:     testutil.Build().WithNodeSelector(map[string]string{"key1": "value1"}).AsBuild(),
			overrides: map[string]string{"key2": "override2"},
			expected:  map[string]string{"key1": "value1", "key2": "override2"},
		},
		{
			name:      "build - non empty linux node only",
			build:     testutil.Build().WithNodeSelector(map[string]string{v1.LabelOSStable: "linux"}).AsBuild(),
			overrides: map[string]string{"key1": "default1"},
			expected:  map[string]string{"key1": "default1", v1.LabelOSStable: "linux"},
		},
		{
			name:      "build - try to change linux node only",
			build:     testutil.Build().WithNodeSelector(map[string]string{v1.LabelOSStable: "linux"}).AsBuild(),
			overrides: map[string]string{v1.LabelOSStable: "windows"},
			expected:  map[string]string{v1.LabelOSStable: "linux"},
		},
	}

	for _, test := range tests {
		overrides := BuildOverrides{Config: &openshiftcontrolplanev1.BuildOverridesConfig{NodeSelector: test.overrides}}
		pod := testutil.Pod().WithBuild(t, test.build)
		// normally the pod will have the nodeselectors from the build, due to the pod creation logic
		// in the build controller flow. fake it out here.
		pod.Spec.NodeSelector = test.build.Spec.NodeSelector
		err := overrides.ApplyOverrides((*v1.Pod)(pod))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}
		if len(pod.Spec.NodeSelector) != len(test.expected) {
			t.Errorf("%s: incorrect number of selectors, expected %v, got %v", test.name, test.expected, pod.Spec.NodeSelector)
		}
		for k, v := range pod.Spec.NodeSelector {
			if ev, ok := test.expected[k]; !ok || ev != v {
				t.Errorf("%s: incorrect selector value for key %s, expected %s, got %s", test.name, k, ev, v)
			}
		}
	}
}

func TestBuildOverrideAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		build       *buildv1.Build
		annotations map[string]string
		overrides   map[string]string
		expected    map[string]string
	}{
		{
			name:        "build - nil annotations",
			build:       testutil.Build().AsBuild(),
			annotations: nil,
			overrides:   map[string]string{"key1": "override1", "key2": "override2"},
			expected:    map[string]string{"key1": "override1", "key2": "override2"},
		},
		{
			name:        "build - full override",
			build:       testutil.Build().AsBuild(),
			annotations: map[string]string{"key1": "value1"},
			overrides:   map[string]string{"key1": "override1", "key2": "override2"},
			expected:    map[string]string{"key1": "override1", "key2": "override2"},
		},
		{
			name:        "build - partial override",
			build:       testutil.Build().AsBuild(),
			annotations: map[string]string{"key1": "value1"},
			overrides:   map[string]string{"key2": "override2"},
			expected:    map[string]string{"key1": "value1", "key2": "override2"},
		},
	}

	for _, test := range tests {
		overrides := BuildOverrides{Config: &openshiftcontrolplanev1.BuildOverridesConfig{Annotations: test.overrides}}
		pod := testutil.Pod().WithBuild(t, test.build)
		pod.Annotations = test.annotations
		err := overrides.ApplyOverrides((*v1.Pod)(pod))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}
		if len(pod.Annotations) != len(test.expected) {
			t.Errorf("%s: incorrect number of annotations, expected %v, got %v", test.name, test.expected, pod.Annotations)
		}
		for k, v := range pod.Annotations {
			if ev, ok := test.expected[k]; !ok || ev != v {
				t.Errorf("%s: incorrect annotation value for key %s, expected %s, got %s", test.name, k, ev, v)
			}
		}
	}
}

func TestBuildOverrideTolerations(t *testing.T) {
	tests := []struct {
		name                string
		buildTolerations    []corev1.Toleration
		overrideTolerations []corev1.Toleration
		expected            []corev1.Toleration
	}{
		{
			name:                "everything nil",
			buildTolerations:    nil,
			overrideTolerations: nil,
			expected:            nil,
		},
		{
			name:             "no build tolerations, only overrides",
			buildTolerations: nil,
			overrideTolerations: []corev1.Toleration{
				{
					Key:    "toleration1",
					Value:  "value1",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:   "toleration2",
					Value: "value2",
				},
			},
			expected: []corev1.Toleration{
				{
					Key:    "toleration1",
					Value:  "value1",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:   "toleration2",
					Value: "value2",
				},
			},
		},
		{
			name: "should override value",
			buildTolerations: []corev1.Toleration{
				{
					Key:    "toleration1",
					Value:  "value1",
					Effect: corev1.TaintEffectNoExecute,
				},
				{
					Key:   "toleration2",
					Value: "value2",
				},
			},
			overrideTolerations: []corev1.Toleration{
				{
					Key:   "toleration1",
					Value: "value3",
				},
				{
					Key:   "toleration2",
					Value: "value2",
				},
			},
			expected: []corev1.Toleration{
				{
					Key:   "toleration1",
					Value: "value3",
				},
				{
					Key:   "toleration2",
					Value: "value2",
				},
			},
		},
		{
			name: "should override and add additional",
			buildTolerations: []corev1.Toleration{
				{
					Key:   "toleration1",
					Value: "value1",
				},
				{
					Key:   "toleration2",
					Value: "value2",
				},
			},
			overrideTolerations: []corev1.Toleration{
				{
					Key:   "toleration1",
					Value: "value3",
				},
				{
					Key:   "toleration2",
					Value: "value4",
				},
				{
					Key:    "toleration3",
					Value:  "value5",
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
			},
			expected: []corev1.Toleration{
				{
					Key:   "toleration1",
					Value: "value3",
				},
				{
					Key:   "toleration2",
					Value: "value4",
				},
				{
					Key:    "toleration3",
					Value:  "value5",
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
			},
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			overridesConfig := &openshiftcontrolplanev1.BuildOverridesConfig{
				Tolerations: test.overrideTolerations,
			}

			admitter := BuildOverrides{overridesConfig}
			pod := testutil.Pod().WithTolerations(test.buildTolerations).WithBuild(t, testutil.Build().AsBuild())
			err := admitter.ApplyOverrides((*v1.Pod)(pod))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(pod.Spec.Tolerations, test.expected) {
				t.Errorf("expected[%d]: %v, got: %v", i, test.expected, pod.Spec.Tolerations)
			}
		})
	}
}
