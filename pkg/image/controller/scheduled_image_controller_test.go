package controller

import (
	"net/http"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apitesting "k8s.io/apimachinery/pkg/api/apitesting"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restfake "k8s.io/client-go/rest/fake"

	imagev1 "github.com/openshift/api/image/v1"
	fakeimagev1client "github.com/openshift/client-go/image/clientset/versioned/fake"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
)

func TestScheduledImport(t *testing.T) {
	one := int64(1)
	stream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "other", UID: "1", ResourceVersion: "1",
			Annotations: map[string]string{imagev1.DockerImageRepositoryCheckAnnotation: "done"},
			Generation:  1,
		},
		Spec: imagev1.ImageStreamSpec{
			Tags: []imagev1.TagReference{
				{
					Name:         "default",
					From:         &corev1.ObjectReference{Kind: "DockerImage", Name: "mysql:latest"},
					Generation:   &one,
					ImportPolicy: imagev1.TagImportPolicy{Scheduled: true},
				},
			},
		},
		Status: imagev1.ImageStreamStatus{
			Tags: []imagev1.NamedTagEventList{
				{
					Tag:   "default",
					Items: []imagev1.TagEvent{{Generation: 1}},
				},
			},
		},
	}

	imageInformers := imagev1informer.NewSharedInformerFactory(fakeimagev1client.NewSimpleClientset(), 0)
	isInformer := imageInformers.Image().V1().ImageStreams()
	fake := fakeimagev1client.NewSimpleClientset()
	sched := NewScheduledImageStreamController(fake, isInformer, ScheduledImageStreamControllerOptions{
		Enabled:           true,
		Resync:            1 * time.Second,
		DefaultBucketSize: 4,
	})
	actions := 0
	_, codecs := apitesting.SchemeForOrDie(imagev1.Install)
	fakeREST := &restfake.RESTClient{
		NegotiatedSerializer: codecs,
		GroupVersion:         imagev1.SchemeGroupVersion,
		Client: restfake.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
			actions++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header(),
				Body:       objBody(&imagev1.ImageStreamImport{}),
			}, nil
		}),
	}
	sched.client = fakeREST

	// queue, but don't import the stream
	sched.enqueueImageStream(stream)
	if sched.scheduler.Len() != 1 {
		t.Fatalf("should have scheduled: %#v", sched.scheduler)
	}
	if len(fake.Actions()) != 0 {
		t.Fatalf("should have made no calls: %#v", fake)
	}

	// encountering a not found error for image streams should drop the stream
	for i := 0; i < 3; i++ { // loop all the buckets (2 + the additional internal one)
		sched.scheduler.RunOnce()
	}
	if sched.scheduler.Len() != 0 {
		t.Fatalf("should have removed item in scheduler: %#v", sched.scheduler)
	}
	if len(fake.Actions()) != 0 {
		t.Fatalf("invalid actions: %#v", fake.Actions())
	}

	// queue back
	sched.enqueueImageStream(stream)
	// and add to informer
	isInformer.Informer().GetIndexer().Add(stream)

	// run a background import
	for i := 0; i < 3; i++ { // loop all the buckets (2 + the additional internal one)
		sched.scheduler.RunOnce()
	}
	if sched.scheduler.Len() != 1 {
		t.Fatalf("should have left item in scheduler: %#v", sched.scheduler)
	}
	if actions != 1 || (fakeREST.Req.Method != "POST" && !strings.Contains(fakeREST.Req.URL.String(), "imagestreamimports")) {
		t.Fatalf("invalid actions: %v %v", fakeREST.Req.Method, fakeREST.Req.URL)
	}

	// disabling the scheduled import should drop the stream
	sched.enabled = false
	fake.ClearActions()

	for i := 0; i < 3; i++ { // loop all the buckets (2 + the additional internal one)
		sched.scheduler.RunOnce()
	}
	if sched.scheduler.Len() != 0 {
		t.Fatalf("should have removed item from scheduler: %#v", sched.scheduler)
	}
	if len(fake.Actions()) != 0 {
		t.Fatalf("invalid actions: %#v", fake.Actions())
	}

	// queuing when disabled should not add the stream
	sched.enqueueImageStream(stream)
	if sched.scheduler.Len() != 0 {
		t.Fatalf("should have not added item to scheduler: %#v", sched.scheduler)
	}
}

func TestSchedulingChecks(t *testing.T) {
	one := int64(1)
	tests := map[string]struct {
		is             *imagev1.ImageStream
		expectedResult bool
		tagToBump      int
	}{
		"none scheduled": {
			is: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test", Namespace: "other", UID: "1", ResourceVersion: "1",
					Annotations: map[string]string{imagev1.DockerImageRepositoryCheckAnnotation: "done"},
					Generation:  1,
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:         "default",
							From:         &corev1.ObjectReference{Kind: "DockerImage", Name: "mysql:latest"},
							Generation:   &one,
							ImportPolicy: imagev1.TagImportPolicy{},
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag:   "default",
							Items: []imagev1.TagEvent{{Generation: 1}},
						},
					},
				},
			},
			expectedResult: false,
		},
		"sha ref not imported": {
			is: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test", Namespace: "other", UID: "1", ResourceVersion: "1",
					Annotations: map[string]string{imagev1.DockerImageRepositoryCheckAnnotation: "done"},
					Generation:  1,
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:         "default1",
							From:         &corev1.ObjectReference{Kind: "DockerImage", Name: "abc@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25"},
							Generation:   &one,
							ImportPolicy: imagev1.TagImportPolicy{Scheduled: true},
						},
						{
							Name:         "default2",
							From:         &corev1.ObjectReference{Kind: "DockerImage", Name: "abc@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25"},
							Generation:   &one,
							ImportPolicy: imagev1.TagImportPolicy{Scheduled: true},
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag:   "default1",
							Items: []imagev1.TagEvent{{Generation: 1}},
						},
					},
				},
			},
			expectedResult: true,
			tagToBump:      1,
		},
		"sha ref imported": {
			is: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test", Namespace: "other", UID: "1", ResourceVersion: "1",
					Annotations: map[string]string{imagev1.DockerImageRepositoryCheckAnnotation: "done"},
					Generation:  1,
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:         "default",
							From:         &corev1.ObjectReference{Kind: "DockerImage", Name: "abc@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25"},
							Generation:   &one,
							ImportPolicy: imagev1.TagImportPolicy{Scheduled: true},
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag:   "default",
							Items: []imagev1.TagEvent{{Generation: 1}},
						},
					},
				},
			},
			expectedResult: false,
		},
		"standard docker image ref with tag": {
			is: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test", Namespace: "other", UID: "1", ResourceVersion: "1",
					Annotations: map[string]string{imagev1.DockerImageRepositoryCheckAnnotation: "done"},
					Generation:  1,
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:         "default",
							From:         &corev1.ObjectReference{Kind: "DockerImage", Name: "mysql:latest"},
							Generation:   &one,
							ImportPolicy: imagev1.TagImportPolicy{Scheduled: true},
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag:   "default",
							Items: []imagev1.TagEvent{{Generation: 1}},
						},
					},
				},
			},
			expectedResult: true,
		},
	}

	for name, test := range tests {
		result := needsScheduling(test.is)
		if result != test.expectedResult {
			t.Fatalf("needsScheduling test for %s failed", name)
		}
		resetScheduledTags(test.is)
		if test.expectedResult && *test.is.Spec.Tags[test.tagToBump].Generation <= one {
			t.Fatalf("resetScheduledTags did not bump generation when it should for %s", name)
		}
		if !test.expectedResult && *test.is.Spec.Tags[test.tagToBump].Generation > one {
			t.Fatalf("resetScheduledTags bumped generation when it should not have for %s", name)
		}
	}
}
