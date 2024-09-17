package strategy

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildv1 "github.com/openshift/api/build/v1"
)

const (
	dummyCA = `
	---- BEGIN CERTIFICATE ----
	VEhJUyBJUyBBIEJBRCBDRVJUSUZJQ0FURQo=
	---- END CERTIFICATE ----
	`
	testInternalRegistryHost = "registry.svc.localhost:5000"
)

func TestSetupDockerSocketHostSocket(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{},
			},
		},
	}

	setupDockerSocket(&pod)

	if len(pod.Spec.Volumes) != 1 {
		t.Fatalf("Expected 1 volume, got: %#v", pod.Spec.Volumes)
	}
	volume := pod.Spec.Volumes[0]
	if e, a := "docker-socket", volume.Name; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if volume.Name == "" {
		t.Fatalf("Unexpected empty volume source name")
	}
	if isVolumeSourceEmpty(volume.VolumeSource) {
		t.Fatalf("Unexpected nil volume source")
	}
	if volume.HostPath == nil {
		t.Fatalf("Unexpected nil host directory")
	}
	if volume.EmptyDir != nil {
		t.Errorf("Unexpected non-nil empty directory: %#v", volume.EmptyDir)
	}
	if e, a := "/var/run/docker.sock", volume.HostPath.Path; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}

	if len(pod.Spec.Containers[0].VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volume mount, got: %#v", pod.Spec.Containers[0].VolumeMounts)
	}
	mount := pod.Spec.Containers[0].VolumeMounts[0]
	if e, a := "docker-socket", mount.Name; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if e, a := "/var/run/docker.sock", mount.MountPath; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if pod.Spec.Containers[0].SecurityContext != nil && pod.Spec.Containers[0].SecurityContext.Privileged != nil && *pod.Spec.Containers[0].SecurityContext.Privileged {
		t.Error("Expected privileged to be false")
	}
}

func isVolumeSourceEmpty(volumeSource corev1.VolumeSource) bool {
	if volumeSource.EmptyDir == nil &&
		volumeSource.HostPath == nil &&
		volumeSource.GCEPersistentDisk == nil &&
		volumeSource.GitRepo == nil {
		return true
	}

	return false
}

func TestSetupDockerSecrets(t *testing.T) {
	pod := emptyPod()

	pushSecret := &corev1.LocalObjectReference{
		Name: "my.pushSecret.with.full.stops.and.longer.than.sixty.three.characters",
	}
	pullSecret := &corev1.LocalObjectReference{
		Name: "pullSecret",
	}
	imageSources := []buildv1.ImageSource{
		{PullSecret: &corev1.LocalObjectReference{Name: "imageSourceSecret1"}},
		// this is a duplicate value on purpose, don't change it.
		{PullSecret: &corev1.LocalObjectReference{Name: "imageSourceSecret1"}},
	}

	setupDockerSecrets(&pod, &pod.Spec.Containers[0], pushSecret, pullSecret, imageSources)

	if len(pod.Spec.Volumes) != 4 {
		t.Fatalf("Expected 4 volumes, got: %#v", pod.Spec.Volumes)
	}

	seenName := map[string]bool{}
	for _, v := range pod.Spec.Volumes {
		if seenName[v.Name] {
			t.Errorf("Duplicate volume name %s", v.Name)
		}
		seenName[v.Name] = true

		if v.VolumeSource.Secret == nil {
			t.Errorf("expected volume %s to have source type Secret", v.Name)
		} else {
			defaultMode := v.VolumeSource.Secret.DefaultMode
			if *defaultMode != int32(0600) {
				t.Errorf("expected volume source to default file permissions to read-write-user (0600), got %o", *defaultMode)
			}
		}
	}

	if !seenName["my-pushSecret-with-full-stops-and-longer-than-six-c6eb4d75-push"] {
		t.Errorf("volume my-pushSecret-with-full-stops-and-longer-than-six-c6eb4d75-push was not seen")
	}
	if !seenName["pullSecret-pull"] {
		t.Errorf("volume pullSecret-pull was not seen")
	}

	seenMount := map[string]bool{}
	seenMountPath := map[string]bool{}
	for _, m := range pod.Spec.Containers[0].VolumeMounts {
		if seenMount[m.Name] {
			t.Errorf("Duplicate volume mount name %s", m.Name)
		}
		seenMount[m.Name] = true

		if seenMountPath[m.MountPath] {
			t.Errorf("Duplicate volume mount path %s", m.MountPath)
		}
		seenMountPath[m.Name] = true
	}

	if !seenMount["my-pushSecret-with-full-stops-and-longer-than-six-c6eb4d75-push"] {
		t.Errorf("volumemount my-pushSecret-with-full-stops-and-longer-than-six-c6eb4d75-push was not seen")
	}
	if !seenMount["pullSecret-pull"] {
		t.Errorf("volumemount pullSecret-pull was not seen")
	}
}

func emptyPod() corev1.Pod {
	return corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{},
			},
		},
	}
}

func TestCopyEnvVarSlice(t *testing.T) {
	s1 := []corev1.EnvVar{{Name: "FOO", Value: "bar"}, {Name: "BAZ", Value: "qux"}}
	s2 := copyEnvVarSlice(s1)

	if !reflect.DeepEqual(s1, s2) {
		t.Error(s2)
	}

	if (*reflect.SliceHeader)(unsafe.Pointer(&s1)).Data == (*reflect.SliceHeader)(unsafe.Pointer(&s2)).Data {
		t.Error("copyEnvVarSlice didn't copy backing store")
	}
}

func checkAliasing(t *testing.T, pod *corev1.Pod) {
	m := map[uintptr]bool{}
	for _, c := range pod.Spec.Containers {
		p := (*reflect.SliceHeader)(unsafe.Pointer(&c.Env)).Data
		if m[p] {
			t.Error("pod Env slices are aliased")
			return
		}
		m[p] = true
	}
	for _, c := range pod.Spec.InitContainers {
		p := (*reflect.SliceHeader)(unsafe.Pointer(&c.Env)).Data
		if m[p] {
			t.Error("pod Env slices are aliased")
			return
		}
		m[p] = true
	}
}

func TestMountConfigsAndSecrets(t *testing.T) {
	pod := emptyPod()
	configs := []buildv1.ConfigMapBuildSource{
		{
			ConfigMap: corev1.LocalObjectReference{
				Name: "my.config.with.full.stops.and.longer.than.sixty.three.characters",
			},
			DestinationDir: "./a/rel/path",
		},
		{
			ConfigMap: corev1.LocalObjectReference{
				Name: "config",
			},
			DestinationDir: "some/path",
		},
	}
	secrets := []buildv1.SecretBuildSource{
		{
			Secret: corev1.LocalObjectReference{
				Name: "my.secret.with.full.stops.and.longer.than.sixty.three.characters",
			},
			DestinationDir: "./a/secret/path",
		},
		{
			Secret: corev1.LocalObjectReference{
				Name: "super-secret",
			},
			DestinationDir: "secret/path",
		},
	}
	setupInputConfigMaps(&pod, &pod.Spec.Containers[0], configs)
	setupInputSecrets(&pod, &pod.Spec.Containers[0], secrets)
	if len(pod.Spec.Volumes) != 4 {
		t.Fatalf("Expected 4 volumes, got: %#v", pod.Spec.Volumes)
	}

	seenName := map[string]bool{}
	for _, v := range pod.Spec.Volumes {
		if seenName[v.Name] {
			t.Errorf("Duplicate volume name %s", v.Name)
		}
		seenName[v.Name] = true
		t.Logf("Saw volume %s", v.Name)
	}
	seenMount := map[string]bool{}
	for _, m := range pod.Spec.Containers[0].VolumeMounts {
		if seenMount[m.Name] {
			t.Errorf("Duplicate volume mount name %s", m.Name)
		}
		seenMount[m.Name] = true
	}
	expectedVols := []string{
		"my-config-with-full-stops-and-longer-than-sixty--1935b127-build",
		"config-build",
		"my-secret-with-full-stops-and-longer-than-sixty--2f06b2d9-build",
		"super-secret-build",
	}
	for _, vol := range expectedVols {
		if !seenName[vol] {
			t.Errorf("volume %s was not seen", vol)
		}
		if !seenMount[vol] {
			t.Errorf("volumemount %s was not seen", vol)
		}
	}
}

func checkContainersMounts(containers []corev1.Container, t *testing.T) {
	for _, c := range containers {
		foundCA := false
		for _, v := range c.VolumeMounts {
			if v.Name == "build-ca-bundles" {
				foundCA = true
				if v.MountPath != ConfigMapCertsMountPath {
					t.Errorf("ca bundle %s was not mounted to %s", v.Name, ConfigMapCertsMountPath)
				}
				if v.ReadOnly {
					t.Errorf("ca bundle volume %s should be writeable, but was mounted read-only.", v.Name)
				}
				break
			}
		}
		if !foundCA {
			t.Errorf("build CA bundle was not mounted into container %s", c.Name)
		}
	}
}

func TestSetupBuildCAs(t *testing.T) {
	tests := []struct {
		name           string
		certs          map[string]string
		expectedMounts map[string]string
	}{
		{
			name: "no certs",
		},
		{
			name: "additional certs",
			certs: map[string]string{
				"first":                        dummyCA,
				"second.domain.com":            dummyCA,
				"internal.svc.localhost..5000": dummyCA,
				"myregistry.foo...2345":        dummyCA,
			},
			expectedMounts: map[string]string{
				"first":                        "first",
				"second.domain.com":            "second.domain.com",
				"internal.svc.localhost..5000": "internal.svc.localhost:5000",
				"myregistry.foo...2345":        "myregistry.foo.:2345",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			build := mockDockerBuild()
			podSpec := &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "initfirst",
							Image: "busybox",
						},
						{
							Name:  "initsecond",
							Image: "busybox",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "first",
							Image: "busybox",
						},
						{
							Name:  "second",
							Image: "busybox",
						},
					},
				},
			}
			setupBuildCAs(build, podSpec, tc.certs, testInternalRegistryHost)
			if len(podSpec.Spec.Volumes) != 2 {
				t.Fatalf("expected pod to have 2 volume, got %d", len(podSpec.Spec.Volumes))
			}
			volume := podSpec.Spec.Volumes[0]
			if volume.Name != "build-ca-bundles" {
				t.Errorf("build volume should have name %s, got %s", "build-ca-bundles", volume.Name)
			}
			if volume.ConfigMap == nil {
				t.Fatal("expected volume to use a ConfigMap volume source")
			}
			// The service-ca.crt is always mounted
			expectedItems := len(tc.certs) + 1
			if len(volume.ConfigMap.Items) != expectedItems {
				t.Errorf("expected volume to have %d items, got %d", expectedItems, len(volume.ConfigMap.Items))

			}

			resultItems := make(map[string]corev1.KeyToPath)
			for _, result := range volume.ConfigMap.Items {
				resultItems[result.Key] = result
			}

			for expected := range tc.certs {
				foundItem, ok := resultItems[expected]
				if !ok {
					t.Errorf("could not find %s as a referenced key in volume source", expected)
					continue
				}

				expectedPath := fmt.Sprintf("certs.d/%s/ca.crt", tc.expectedMounts[expected])
				if foundItem.Path != expectedPath {
					t.Errorf("expected mount path to be %s; got %s", expectedPath, foundItem.Path)
				}
			}

			foundItem, ok := resultItems[buildv1.ServiceCAKey]
			if !ok {
				t.Errorf("could not find %s as a referenced key in volume source", buildv1.ServiceCAKey)
			}
			expectedPath := fmt.Sprintf("certs.d/%s/ca.crt", testInternalRegistryHost)
			if foundItem.Path != expectedPath {
				t.Errorf("expected %s to be mounted at %s, got %s", buildv1.ServiceCAKey, expectedPath, foundItem.Path)
			}

			checkContainersMounts(podSpec.Spec.Containers, t)
			checkContainersMounts(podSpec.Spec.InitContainers, t)
		})
	}
}

func TestDefaultActiveDeadline(t *testing.T) {
	// note, existing custom/docker/sti tests check that build deadline override lands
	// in pod activeDeadlineSeconds
	tests := []struct {
		build *buildv1.Build
		cs    *CustomBuildStrategy
		ds    *DockerBuildStrategy
		ss    *SourceBuildStrategy
	}{
		{
			build: mockCustomBuild(false, false),
			cs:    &CustomBuildStrategy{},
		},
		{
			build: mockDockerBuild(),
			ds: &DockerBuildStrategy{
				Image: "docker-test-image",
			},
		},
		{
			build: mockSTIBuild(),
			ss: &SourceBuildStrategy{
				Image:          "sti-test-image",
				SecurityClient: newFakeSecurityClient(false),
			},
		},
	}

	for _, test := range tests {
		test.build.Spec.CompletionDeadlineSeconds = nil
		var pod *corev1.Pod
		var err error
		switch {
		case test.cs != nil:
			pod, err = test.cs.CreateBuildPod(test.build, nil, testInternalRegistryHost)
		case test.ds != nil:
			pod, err = test.ds.CreateBuildPod(test.build, nil, testInternalRegistryHost)
		case test.ss != nil:
			pod, err = test.ss.CreateBuildPod(test.build, nil, testInternalRegistryHost)

		}
		if err != nil {
			t.Errorf("err creating pod for build %#v: %#v", test.build, err)
			continue
		}
		if pod == nil {
			t.Errorf("pod nil for build %#v", test.build)
			continue
		}
		//pod = setupActiveDeadline(pod, test.build)
		if pod.Spec.ActiveDeadlineSeconds == nil {
			t.Errorf("active deadline not set for build %#v", test.build)
			continue
		}
		if *pod.Spec.ActiveDeadlineSeconds != 604800 {
			t.Errorf("active deadline set to unexpected value %d for build %#v",
				*pod.Spec.ActiveDeadlineSeconds, test.build)
		}
	}
}

func TestSetupBuildSystem(t *testing.T) {
	const registryMount = "build-system-configs"
	build := mockDockerBuild()
	podSpec := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "first",
					Image: "busybox",
				},
				{
					Name:  "second",
					Image: "busybox",
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:  "init",
					Image: "busybox",
				},
			},
		},
	}
	setupContainersConfigs(build, podSpec)
	if len(podSpec.Spec.Volumes) != 1 {
		t.Fatalf("expected pod to have 1 volume, got %d", len(podSpec.Spec.Volumes))
	}
	volume := podSpec.Spec.Volumes[0]
	if volume.Name != registryMount {
		t.Errorf("build volume should have name %s, got %s", registryMount, volume.Name)
	}
	if volume.ConfigMap == nil {
		t.Fatal("expected volume to use a ConfigMap volume source")
	}
	containers := podSpec.Spec.Containers
	containers = append(containers, podSpec.Spec.InitContainers...)
	for _, c := range containers {
		foundMount := false
		for _, v := range c.VolumeMounts {
			if v.Name == registryMount {
				foundMount = true
				if v.MountPath != ConfigMapBuildSystemConfigsMountPath {
					t.Errorf("registry config %s was not mounted to %s in container %s", v.Name, ConfigMapBuildSystemConfigsMountPath, c.Name)
				}
				if !v.ReadOnly {
					t.Errorf("registry config volume %s in container %s should be read-only, but was mounted writeable.", v.Name, c.Name)
				}
				break
			}
		}
		if !foundMount {
			t.Errorf("registry config was not mounted into container %s", c.Name)
		}
		foundRegistriesConf := false
		foundSignaturePolicy := false
		for _, env := range c.Env {
			if env.Name == "BUILD_REGISTRIES_CONF_PATH" {
				foundRegistriesConf = true
				expectedMountPath := filepath.Join(ConfigMapBuildSystemConfigsMountPath, buildv1.RegistryConfKey)
				if env.Value != expectedMountPath {
					t.Errorf("expected BUILD_REGISTRIES_CONF_PATH %s, got %s", expectedMountPath, env.Value)
				}
			}
			if env.Name == "BUILD_SIGNATURE_POLICY_PATH" {
				foundSignaturePolicy = true
				expectedMountMapth := filepath.Join(ConfigMapBuildSystemConfigsMountPath, buildv1.SignaturePolicyKey)
				if env.Value != expectedMountMapth {
					t.Errorf("expected BUILD_SIGNATURE_POLICY_PATH %s, got %s", expectedMountMapth, env.Value)
				}
			}
		}
		if !foundRegistriesConf {
			t.Errorf("env var %s was not present in container %s", "BUILD_REGISTRIES_CONF_PATH", c.Name)
		}
		if !foundSignaturePolicy {
			t.Errorf("env var %s was not present in container %s", "BUILD_SIGNATURE_POLICY_PATH", c.Name)
		}
	}
}

func TestSetupBuildVolumes(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					VolumeMounts: []corev1.VolumeMount{},
				},
			},
			Volumes: []corev1.Volume{},
		},
	}

	var defaultMode int32 = 0600
	var UnSupportedBuildVolumeType buildv1.BuildVolumeSourceType = "UnSupportedBuildVolumeType"

	tests := []struct {
		Name                 string
		CSIVolumeEnabled     bool
		ShouldFail           bool
		ErrorMessage         string
		StartingVolumes      []corev1.Volume
		StartingVolumeMounts []corev1.VolumeMount
		BuildVolumes         []buildv1.BuildVolume
		WantVolumes          []corev1.Volume
		WantVolumeMounts     []corev1.VolumeMount
	}{
		{
			Name:             "Secret BuildVolume should succeed",
			CSIVolumeEnabled: false,
			ShouldFail:       false,
			ErrorMessage:     "",
			BuildVolumes: []buildv1.BuildVolume{
				{
					Name: "one",
					Source: buildv1.BuildVolumeSource{
						Type: buildv1.BuildVolumeSourceTypeSecret,
						Secret: &corev1.SecretVolumeSource{
							SecretName: "secret-one",
							Items: []corev1.KeyToPath{
								{
									Key:  "my-key",
									Path: "my-path",
								},
							},
							DefaultMode: &defaultMode,
						},
					},
					Mounts: []buildv1.BuildVolumeMount{
						{
							DestinationPath: "my-path",
						},
					},
				},
			},

			WantVolumes: []corev1.Volume{
				{
					Name: NameForBuildVolume("secret-one"),
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "secret-one",
							Items: []corev1.KeyToPath{
								{
									Key:  "my-key",
									Path: "my-path",
								},
							},
							DefaultMode: &defaultMode,
						},
					},
				},
			},
			WantVolumeMounts: []corev1.VolumeMount{
				{
					Name:      NameForBuildVolume("secret-one"),
					ReadOnly:  true,
					MountPath: PathForBuildVolume("secret-one"),
				},
			},
		},
		{
			Name:             "ConfigMap BuildVolume should succeed",
			CSIVolumeEnabled: false,
			ShouldFail:       false,
			ErrorMessage:     "",
			BuildVolumes: []buildv1.BuildVolume{
				{
					Name: "one",
					Source: buildv1.BuildVolumeSource{
						Type: buildv1.BuildVolumeSourceTypeConfigMap,
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "configMap-one"},
							Items: []corev1.KeyToPath{
								{
									Key:  "my-key",
									Path: "my-path",
								},
							},
							DefaultMode: &defaultMode,
						},
					},
					Mounts: []buildv1.BuildVolumeMount{
						{
							DestinationPath: "my-path",
						},
					},
				},
			},

			WantVolumes: []corev1.Volume{
				{
					Name: NameForBuildVolume("configMap-one"),
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "configMap-one"},
							Items: []corev1.KeyToPath{
								{
									Key:  "my-key",
									Path: "my-path",
								},
							},
							DefaultMode: &defaultMode,
						},
					},
				},
			},
			WantVolumeMounts: []corev1.VolumeMount{
				{
					Name:      NameForBuildVolume("configMap-one"),
					ReadOnly:  true,
					MountPath: PathForBuildVolume("configMap-one"),
				},
			},
		},
		{
			Name:             "CSI BuildVolume should succeed",
			CSIVolumeEnabled: true,
			ShouldFail:       false,
			ErrorMessage:     "",
			BuildVolumes: []buildv1.BuildVolume{
				{
					Name: "csi-one",
					Source: buildv1.BuildVolumeSource{
						Type: buildv1.BuildVolumeSourceTypeCSI,
						CSI: &corev1.CSIVolumeSource{
							Driver:           "inline.storage.kubernetes.io",
							VolumeAttributes: map[string]string{"foo": "bar"},
						},
					},
					Mounts: []buildv1.BuildVolumeMount{
						{
							DestinationPath: "my-path",
						},
					},
				},
			},

			WantVolumes: []corev1.Volume{
				{
					Name: NameForBuildVolume("csi-one"),
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver:           "inline.storage.kubernetes.io",
							VolumeAttributes: map[string]string{"foo": "bar"},
						},
					},
				},
			},
			WantVolumeMounts: []corev1.VolumeMount{
				{
					Name:      NameForBuildVolume("csi-one"),
					ReadOnly:  true,
					MountPath: PathForBuildVolume("csi-one"),
				},
			},
		},
		{
			Name:             "Duplicate Secret BuildVolumeMount should fail",
			CSIVolumeEnabled: false,
			ShouldFail:       true,
			ErrorMessage:     "user provided BuildVolumeMount path \"my-path\" collides with VolumeMount path created by the build controller",
			StartingVolumeMounts: []corev1.VolumeMount{
				{
					Name:      "some-name",
					MountPath: "my-path",
				},
			},
			BuildVolumes: []buildv1.BuildVolume{
				{
					Name: "one",
					Source: buildv1.BuildVolumeSource{
						Type: buildv1.BuildVolumeSourceTypeSecret,
						Secret: &corev1.SecretVolumeSource{
							SecretName: "secret-one",
							Items: []corev1.KeyToPath{
								{
									Key:  "my-key",
									Path: "my-path",
								},
							},
							DefaultMode: &defaultMode,
						},
					},
					Mounts: []buildv1.BuildVolumeMount{
						{
							DestinationPath: "my-path",
						},
					},
				},
			},

			WantVolumes: []corev1.Volume{},
			WantVolumeMounts: []corev1.VolumeMount{
				{
					Name:      "some-name",
					ReadOnly:  false,
					MountPath: "my-path",
				},
			},
		},
		{
			Name:             "UnSupported BuildVolumeSourceType should fail",
			CSIVolumeEnabled: false,
			ShouldFail:       true,
			ErrorMessage:     "encountered unsupported build volume source type \"UnSupportedBuildVolumeType\"",
			BuildVolumes: []buildv1.BuildVolume{
				{
					Name: "one",
					Source: buildv1.BuildVolumeSource{
						Type:   UnSupportedBuildVolumeType,
						Secret: &corev1.SecretVolumeSource{},
					},
					Mounts: []buildv1.BuildVolumeMount{},
				},
			},
			WantVolumes:      []corev1.Volume{},
			WantVolumeMounts: []corev1.VolumeMount{},
		},
		{
			Name:             "CSI volume request without csivolumeEnabled should fail",
			CSIVolumeEnabled: false,
			ShouldFail:       true,
			ErrorMessage:     "csi volumes require the BuildCSIVolumes feature gate to be enabled",
			BuildVolumes: []buildv1.BuildVolume{
				{
					Name: "csi-one",
					Source: buildv1.BuildVolumeSource{
						Type: buildv1.BuildVolumeSourceTypeCSI,
						CSI: &corev1.CSIVolumeSource{
							Driver:           "inline.storage.kubernetes.io",
							VolumeAttributes: map[string]string{"foo": "bar"},
						},
					},
					Mounts: []buildv1.BuildVolumeMount{
						{
							DestinationPath: "my-path",
						},
					},
				},
			},
			WantVolumes:      []corev1.Volume{},
			WantVolumeMounts: []corev1.VolumeMount{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			p := pod.DeepCopy()

			if tt.StartingVolumes != nil {
				p.Spec.Volumes = append(p.Spec.Volumes, tt.StartingVolumes...)
			}

			if tt.StartingVolumeMounts != nil {
				p.Spec.Containers[0].VolumeMounts = append(p.Spec.Containers[0].VolumeMounts, tt.StartingVolumeMounts...)
			}

			err := setupBuildVolumes(p, tt.BuildVolumes, tt.CSIVolumeEnabled)

			if err == nil && tt.ShouldFail {
				t.Errorf("test %q should have failed with error %q, but didn't", tt.Name, tt.ErrorMessage)
			}

			if err != nil && tt.ShouldFail {
				if err.Error() != tt.ErrorMessage {
					t.Errorf("test %q failed with incorrect error message, wanted: %q, got: %q", tt.Name, tt.ErrorMessage, err.Error())
				}
			}

			if !reflect.DeepEqual(p.Spec.Volumes, tt.WantVolumes) {
				t.Errorf("adding build volumes to pod failed, have: %#v, want: %#v", p.Spec.Volumes, tt.WantVolumes)
			}

			if !reflect.DeepEqual(p.Spec.Containers[0].VolumeMounts, tt.WantVolumeMounts) {
				t.Errorf("adding build volume mounts to container failed, have: %#v, want: %#v", p.Spec.Containers[0].VolumeMounts, tt.WantVolumeMounts)
			}
		})
	}
}

type buildPodCreator interface {
	CreateBuildPod(build *buildv1.Build, additionalCAs map[string]string, internalRegistryHost string) (*corev1.Pod, error)
}

func testCreateBuildPodAutonsUser(t *testing.T, build *buildv1.Build, strategy buildPodCreator, addEnv func(build *buildv1.Build, env corev1.EnvVar)) {
	for _, testCase := range []struct {
		env           string
		expectError   bool
		privileged    bool
		annotations   map[string]string
		noAnnotations []string
	}{
		{
			env:        "",
			privileged: true,
			noAnnotations: []string{
				"io.openshift.builder",
				"io.kubernetes.cri-o.Devices",
				"io.kubernetes.cri-o.userns-mode",
			},
		},
		{
			env:        "BUILD_PRIVILEGED=0",
			privileged: false,
			annotations: map[string]string{
				"io.openshift.builder":            "",
				"io.kubernetes.cri-o.Devices":     "/dev/fuse:rwm",
				"io.kubernetes.cri-o.userns-mode": "auto:size=65536",
			},
		},
		{
			env:        "BUILD_PRIVILEGED=42",
			privileged: true,
			noAnnotations: []string{
				"io.openshift.builder",
				"io.kubernetes.cri-o.Devices",
				"io.kubernetes.cri-o.userns-mode",
			},
		},
		{
			env:        "BUILD_PRIVILEGED=false",
			privileged: false,
			annotations: map[string]string{
				"io.openshift.builder":            "",
				"io.kubernetes.cri-o.Devices":     "/dev/fuse:rwm",
				"io.kubernetes.cri-o.userns-mode": "auto:size=65536",
			},
		},
		{
			env:        "BUILD_PRIVILEGED=true",
			privileged: true,
			noAnnotations: []string{
				"io.openshift.builder",
				"io.kubernetes.cri-o.Devices",
				"io.kubernetes.cri-o.userns-mode",
			},
		},
	} {
		t.Run(testCase.env, func(t *testing.T) {
			build := build.DeepCopy()
			for _, envVar := range strings.Split(testCase.env, ":") {
				if env := strings.SplitN(envVar, "=", 2); len(env) > 1 {
					addEnv(build, corev1.EnvVar{Name: env[0], Value: env[1]})
				}
			}
			actual, err := strategy.CreateBuildPod(build, nil, testInternalRegistryHost)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			for _, ctr := range append(actual.Spec.Containers, actual.Spec.InitContainers...) {
				sc := ctr.SecurityContext
				if sc == nil {
					t.Errorf("Container %s in pod spec has no SecurityContext", ctr.Name)
					continue
				}
				if sc.Privileged == nil {
					t.Errorf("Container %s in pod spec has no privileged field", ctr.Name)
					continue
				}
				if isPrivilegedContainerAllowed(ctr.Name) && *sc.Privileged != testCase.privileged {
					t.Errorf("Expected privileged: %q to produce privileged=%v for container %s, got %v", testCase.env, testCase.privileged, ctr.Name, *sc.Privileged)
				}
			}
			for annotation, value := range testCase.annotations {
				if !metav1.HasAnnotation(actual.ObjectMeta, annotation) {
					t.Errorf("Expected a %q annotation, but don't see one", annotation)
					continue
				}
				annotations := actual.ObjectMeta.GetAnnotations()
				if val := annotations[annotation]; val != value {
					t.Errorf("Annotation %q was expected to be %q, but was actually %q", annotation, value, val)
				}
			}
			for _, annotation := range testCase.noAnnotations {
				if metav1.HasAnnotation(actual.ObjectMeta, annotation) {
					t.Errorf("Expected no %q annotation, but got one", annotation)
				}
			}
		})
	}
}

func TestMinimalSecurityContext(t *testing.T) {
	securityCtx := builderMinSecurityContext()
	checkSecurityContextMeetsBaseline(t, "test", securityCtx)
}

var allowedCapabilities = []string{
	"AUDIT_WRITE",
	"CHOWN",
	"DAC_OVERRIDE",
	"FOWNER",
	"FSETID",
	"KILL",
	"MKNOD",
	"NET_BIND_SERVICE",
	"SETFCAP",
	"SETGID",
	"SETPCAP",
	"SETUID",
	"SYS_CHROOT",
}

func isCapabilityAllowedBaseline(capability corev1.Capability) bool {
	for _, allowed := range allowedCapabilities {
		if capability == corev1.Capability(allowed) {
			return true
		}
	}
	return false
}

// checkSecurityContextMeetsBaseline verifes if the security context meets the "baseline" Pod Security Standard.
func checkSecurityContextMeetsBaseline(t *testing.T, containerName string, securityCtx *corev1.SecurityContext) {
	// Baseline pod security standard allows containers to run as root, but with extra protections
	// to prevent common/known attack surfaces.

	// Privileged containers are not allowed
	if securityCtx.Privileged != nil && *securityCtx.Privileged {
		t.Errorf("container %s should not be privileged", containerName)
	}

	// A subset of Linux capabilities are allowed for "baseline" pod security standard
	if securityCtx.Capabilities != nil {
		for _, cap := range securityCtx.Capabilities.Add {
			if !isCapabilityAllowedBaseline(cap) {
				t.Errorf("container %s adds privileged capability %q", containerName, cap)
			}
		}
	}

	// SELinux cannot be disabled, or use user/role overrides
	if securityCtx.SELinuxOptions != nil {
		seLinuxType := securityCtx.SELinuxOptions.Type
		if seLinuxType != "container_t" && seLinuxType != "container_init_t" &&
			seLinuxType != "container_kvm_t" && seLinuxType != "container_engine_t" {
			t.Errorf("container %s uses privileged SELinux type %q", containerName, seLinuxType)
		}
		if len(securityCtx.SELinuxOptions.User) > 0 {
			t.Errorf("container %s overrides SELinux user", containerName)
		}
		if len(securityCtx.SELinuxOptions.Role) > 0 {
			t.Errorf("container %s overrides SELinux role", containerName)
		}
	}

	// /proc mount should use defaults (masked)
	if securityCtx.ProcMount != nil && *securityCtx.ProcMount != corev1.DefaultProcMount {
		t.Errorf("container %s does not use default /proc mount type", containerName)
	}

	// Seccomp profile cannot be set to "Unconfined"
	if securityCtx.SeccompProfile != nil && securityCtx.SeccompProfile.Type == corev1.SeccompProfileTypeUnconfined {
		t.Errorf("container %s uses Unconfined Seccomp profile", containerName)
	}

}

// checkPodSecurityContexts verifies if the build pod containers have appropriate security
// contexts. Only a subset of build pod containers are allowed to use a non-restricted security
// context.
func checkPodSecurityContexts(t *testing.T, pod *corev1.Pod) {
	for _, c := range pod.Spec.Containers {
		if isPrivilegedContainerAllowed(c.Name) {
			continue
		}
		checkSecurityContextMeetsBaseline(t, c.Name, c.SecurityContext)
	}
	for _, c := range pod.Spec.InitContainers {
		if isPrivilegedContainerAllowed(c.Name) {
			continue
		}
		checkSecurityContextMeetsBaseline(t, c.Name, c.SecurityContext)
	}
}

// isPrivilegedContainerAllowed returns true if the container is allowed to run as privileged,
// based on its name. Only the following containers in the build pod are allowed to run as
// privileged:
//
// - DockerBuild
// - StiBuild
// - ExtractImageContentContainer
//
// TODO: Remove need for privileged containers by having cri-o mount /dev/fuse safely.
func isPrivilegedContainerAllowed(containerName string) bool {
	switch containerName {
	case DockerBuild, StiBuild, ExtractImageContentContainer:
		return true
	default:
		return false
	}
}
