package controller

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	buildv1 "github.com/openshift/api/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"

	"github.com/openshift/client-go/build/clientset/versioned/fake"
)

func TestHandleBuildConfig(t *testing.T) {
	tests := []struct {
		name              string
		bc                *buildv1.BuildConfig
		expectBuild       bool
		instantiatorError bool
		expectErr         bool
	}{
		{
			name:        "build config with no config change trigger",
			bc:          baseBuildConfig(),
			expectBuild: false,
		},
		{
			name:        "build config with non-zero last version",
			bc:          buildConfigWithNonZeroLastVersion(),
			expectBuild: false,
		},
		{
			name:        "build config with config change trigger",
			bc:          buildConfigWithConfigChangeTrigger(),
			expectBuild: true,
		},
		{
			name:              "instantiator error",
			bc:                buildConfigWithConfigChangeTrigger(),
			instantiatorError: true,
			expectErr:         true,
		},
	}

	for _, tc := range tests {
		buildClient := fake.NewSimpleClientset(tc.bc)
		instantiateRequestName := ""

		if tc.instantiatorError {
			buildClient.PrependReactor("create", "buildconfigs", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				a, ok := action.(ktesting.CreateAction)
				if !ok {
					panic("unexpected action")
				}
				request := a.GetObject().(*buildv1.BuildRequest)
				instantiateRequestName = request.Name
				if tc.expectErr {
					return true, nil, fmt.Errorf("error")
				}
				return true, &buildv1.Build{}, nil
			})
		} else {
			buildClient.PrependReactor("create", "buildconfigs", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				a, ok := action.(ktesting.CreateAction)
				if !ok {
					panic("unexpected action")
				}
				if a.GetSubresource() != "instantiate" {
					return false, nil, nil
				}
				request := a.GetObject().(*buildv1.BuildRequest)
				instantiateRequestName = request.Name
				return true, &buildv1.Build{}, nil
			})
		}
		controller := &BuildConfigController{
			buildLister:       &okBuildLister{},
			buildConfigGetter: buildClient.BuildV1(),
			buildGetter:       buildClient.BuildV1(),
			buildConfigLister: &okBuildConfigGetter{BuildConfig: tc.bc},
			recorder:          &record.FakeRecorder{},
		}
		err := controller.handleBuildConfig(tc.bc)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
			}
			continue
		}
		if tc.expectErr {
			t.Errorf("%s: expected error, but got none", tc.name)
			continue
		}
		if tc.expectBuild && len(instantiateRequestName) == 0 {
			t.Errorf("%s: expected a build to be started.", tc.name)
		}
		if !tc.expectBuild && len(instantiateRequestName) > 0 {
			t.Errorf("%s: did not expect a build to be started.", tc.name)
		}
	}

}

func TestCheckImageChangeTriggerCleared(t *testing.T) {
	cases := []struct {
		name            string
		oldTriggers     []tagTriggerID
		currentTriggers []tagTriggerID
		setOldNil       bool
		setCurrentNil   bool
		expectedResult  bool
	}{
		{
			name:      "old nil",
			setOldNil: true,
		},
		{
			name:          "current nil",
			setCurrentNil: true,
		},
		{
			name:          "both nil",
			setOldNil:     true,
			setCurrentNil: true,
		},
		{
			name: "no trigger changes",
			oldTriggers: []tagTriggerID{
				{
					LastTriggeredId: "abcdef0",
				},
			},
			currentTriggers: []tagTriggerID{
				{
					LastTriggeredId: "abcdef0",
				},
			},
		},
		{
			name: "empty to populated",
			oldTriggers: []tagTriggerID{
				{
					LastTriggeredId: "",
				},
			},
			currentTriggers: []tagTriggerID{
				{
					LastTriggeredId: "abcdef0",
				},
			},
		},
		{
			name: "populated to empty",
			oldTriggers: []tagTriggerID{
				{
					LastTriggeredId: "abcdef0",
				},
			},
			currentTriggers: []tagTriggerID{
				{
					LastTriggeredId: "",
				},
			},
			expectedResult: true,
		},
		{
			name: "multi empty to populated",
			oldTriggers: []tagTriggerID{
				{
					LastTriggeredId: "abcdef0",
				},
				{
					ImageStreamTag:  "test-build:latest",
					LastTriggeredId: "",
				},
			},
			currentTriggers: []tagTriggerID{
				{
					LastTriggeredId: "abcdef0",
				},
				{
					ImageStreamTag:  "test-build:latest",
					LastTriggeredId: "abcdef0",
				},
			},
		},
		{
			name: "multi populated to empty",
			oldTriggers: []tagTriggerID{
				{
					LastTriggeredId: "abcdef0",
				},
				{
					ImageStreamTag:  "test-build:latest",
					LastTriggeredId: "abcdef0",
				},
			},
			currentTriggers: []tagTriggerID{
				{
					LastTriggeredId: "abcdef0",
				},
				{
					ImageStreamTag:  "test-build:latest",
					LastTriggeredId: "",
				},
			},
			expectedResult: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			objects := []runtime.Object{}

			var current *buildv1.BuildConfig
			if !tc.setCurrentNil {
				current = buildConfigWithImageChangeTriggers(tc.currentTriggers)
			}
			if current != nil {
				objects = append(objects, current)
			}
			buildClient := fake.NewSimpleClientset(objects...)
			controller := &BuildConfigController{
				buildLister:       &okBuildLister{},
				buildConfigGetter: buildClient.BuildV1(),
				buildGetter:       buildClient.BuildV1(),
				buildConfigLister: &okBuildConfigGetter{BuildConfig: current},
				recorder:          &record.FakeRecorder{},
			}
			var old *buildv1.BuildConfig
			if !tc.setOldNil {
				old = buildConfigWithImageChangeTriggers(tc.oldTriggers)
			}
			changed := controller.imageChangeTriggerCleared(old, current)
			if changed != tc.expectedResult {
				t.Errorf("expected ImageChangeTriggerCleared to be %v, got %v", tc.expectedResult, changed)
			}
		})
	}
}

func baseBuildConfig() *buildv1.BuildConfig {
	bc := &buildv1.BuildConfig{}
	bc.Name = "testBuildConfig"
	bc.Spec.Strategy.SourceStrategy = &buildv1.SourceBuildStrategy{}
	bc.Spec.Strategy.SourceStrategy.From.Name = "builderimage:latest"
	bc.Spec.Strategy.SourceStrategy.From.Kind = "ImageStreamTag"
	return bc
}

func buildConfigWithConfigChangeTrigger() *buildv1.BuildConfig {
	bc := baseBuildConfig()
	configChangeTrigger := buildv1.BuildTriggerPolicy{}
	configChangeTrigger.Type = buildv1.ConfigChangeBuildTriggerType
	bc.Spec.Triggers = append(bc.Spec.Triggers, configChangeTrigger)
	return bc
}

func buildConfigWithNonZeroLastVersion() *buildv1.BuildConfig {
	bc := buildConfigWithConfigChangeTrigger()
	bc.Status.LastVersion = 1
	return bc
}

func buildConfigWithImageChangeTriggers(triggers []tagTriggerID) *buildv1.BuildConfig {
	bc := baseBuildConfig()
	for _, trigger := range triggers {
		imageChangeTrigger := &buildv1.ImageChangeTrigger{
			LastTriggeredImageID: trigger.LastTriggeredId,
		}
		if len(trigger.ImageStreamTag) > 0 {
			imageChangeTrigger.From = &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: trigger.ImageStreamTag,
			}
		}
		bc.Spec.Triggers = append(bc.Spec.Triggers, buildv1.BuildTriggerPolicy{
			Type:        buildv1.ImageChangeBuildTriggerType,
			ImageChange: imageChangeTrigger,
		})
		imageChangeTriggerStatus := buildv1.ImageChangeTriggerStatus{
			LastTriggeredImageID: trigger.LastTriggeredId,
		}
		if len(trigger.ImageStreamTag) != 0 {
			imageChangeTriggerStatus.From = buildv1.ImageStreamTagReference{
				Name: trigger.ImageStreamTag,
			}
		}
		bc.Status.ImageChangeTriggers = append(bc.Status.ImageChangeTriggers, imageChangeTriggerStatus)
	}
	return bc
}

type tagTriggerID struct {
	ImageStreamTag  string
	LastTriggeredId string
}

type okBuildLister struct{}

func (okc *okBuildLister) List(label labels.Selector) ([]*buildv1.Build, error) {
	return nil, nil
}

func (okc *okBuildLister) Builds(ns string) buildlister.BuildNamespaceLister {
	return okc
}

func (okc *okBuildLister) Get(name string) (*buildv1.Build, error) {
	return nil, nil
}

type okBuildConfigGetter struct {
	BuildConfig *buildv1.BuildConfig
}

func (okc *okBuildConfigGetter) Get(name string) (*buildv1.BuildConfig, error) {
	if okc.BuildConfig != nil {
		return okc.BuildConfig, nil
	}
	return &buildv1.BuildConfig{}, nil
}

func (okc *okBuildConfigGetter) BuildConfigs(ns string) buildlister.BuildConfigNamespaceLister {
	return okc
}

func (okc *okBuildConfigGetter) List(label labels.Selector) ([]*buildv1.BuildConfig, error) {
	return nil, fmt.Errorf("not implemented")
}
