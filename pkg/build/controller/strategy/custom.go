package strategy

import (
	"errors"
	"fmt"
	"path"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildv1 "github.com/openshift/api/build/v1"
	buildutil "github.com/openshift/openshift-controller-manager/pkg/build/buildutil"
)

var (
	customBuildEncodingScheme       = runtime.NewScheme()
	customBuildEncodingCodecFactory = serializer.NewCodecFactory(customBuildEncodingScheme)
)

func init() {
	utilruntime.Must(buildv1.Install(customBuildEncodingScheme))
	utilruntime.Must(buildv1.DeprecatedInstallWithoutGroup(customBuildEncodingScheme))
	customBuildEncodingCodecFactory = serializer.NewCodecFactory(customBuildEncodingScheme)
}

// CustomBuildStrategy creates a build using a custom builder image.
type CustomBuildStrategy struct {
	Image string // git clone init-container image
}

// CreateBuildPod creates the pod to be used for the Custom build
func (bs *CustomBuildStrategy) CreateBuildPod(build *buildv1.Build, additionalCAs map[string]string, internalRegistryHost string) (*corev1.Pod, error) {
	strategy := build.Spec.Strategy.CustomStrategy
	if strategy == nil {
		return nil, errors.New("CustomBuildStrategy cannot be executed without CustomStrategy parameters")
	}

	codec := customBuildEncodingCodecFactory.LegacyCodec(buildv1.GroupVersion)
	if len(strategy.BuildAPIVersion) != 0 {
		gv, err := schema.ParseGroupVersion(strategy.BuildAPIVersion)
		if err != nil {
			return nil, &FatalError{fmt.Sprintf("failed to parse buildAPIVersion specified in custom build strategy (%q): %v", strategy.BuildAPIVersion, err)}
		}
		codec = customBuildEncodingCodecFactory.LegacyCodec(gv)
	}

	data, err := runtime.Encode(codec, build)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the build: %v", err)
	}

	containerEnv := []corev1.EnvVar{
		{Name: "BUILD", Value: string(data)},
		{Name: "LANG", Value: "C.utf8"},
	}

	if build.Spec.Source.Git != nil {
		addSourceEnvVars(build.Spec.Source, &containerEnv)
	}
	addTrustedCAMountEnvVar(build.Spec.MountTrustedCA, &containerEnv)

	if len(strategy.Env) > 0 {
		buildutil.MergeTrustedEnvWithoutDuplicates(strategy.Env, &containerEnv, true)
	}

	if build.Spec.Output.To != nil {
		addOutputEnvVars(build.Spec.Output.To, &containerEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the output docker tag %q: %v", build.Spec.Output.To.Name, err)
		}
	}

	if len(strategy.From.Name) == 0 {
		return nil, errors.New("CustomBuildStrategy cannot be executed without image")
	}

	if len(strategy.Env) > 0 {
		containerEnv = append(containerEnv, strategy.Env...)
	}

	if strategy.ExposeDockerSocket {
		klog.V(2).Infof("ExposeDockerSocket is enabled for %s build", build.Name)
		containerEnv = append(containerEnv, corev1.EnvVar{Name: "DOCKER_SOCKET", Value: dockerSocketPath})
	}

	serviceAccount := build.Spec.ServiceAccount
	if len(serviceAccount) == 0 {
		serviceAccount = buildutil.BuilderServiceAccountName
	}

	securityContext := securityContextForBuild(strategy.Env)
	workingDir := path.Join(buildutil.BuildWorkDirMount, "inputs")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildutil.GetBuildPodName(build),
			Namespace: build.Namespace,
			Labels:    getPodLabels(build),
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: serviceAccount,
			Containers: []corev1.Container{
				{
					Name:                     CustomBuild,
					Image:                    strategy.From.Name,
					Env:                      containerEnv,
					SecurityContext:          securityContext,
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					VolumeMounts: []corev1.VolumeMount{{
						Name:      BuildWorkDirVolume,
						MountPath: buildutil.BuildWorkDirMount,
					}, {
						Name:      "buildcachedir",
						MountPath: buildutil.BuildBlobsMetaCache,
					}},
					Resources: build.Spec.Resources,
					// setting the container to run directly in the location the repository is cloned
					// using the openshift helper
					WorkingDir: workingDir,
				},
			},
			Volumes: []corev1.Volume{{
				Name: BuildWorkDirVolume,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}, {
				Name: "buildcachedir",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: buildutil.BuildBlobsMetaCache,
					},
				},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector:  build.Spec.NodeSelector,
		},
	}

	if build.Spec.Source.Git != nil || build.Spec.Source.Binary != nil {
		// when source repository is declared, executing the clone as usual
		setupGitCloneInitContainer(pod, build, bs.Image, containerEnv, securityContext)
		// making sure the build working directory is writtable
		setupChmodInitContainer(pod, build, bs.Image, containerEnv, securityContext, 0o777, workingDir)
	}

	pod = setupActiveDeadline(pod, build)

	if !strategy.ForcePull {
		pod.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent
	} else {
		klog.V(2).Infof("ForcePull is enabled for %s build", build.Name)
		pod.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	}

	if strategy.ExposeDockerSocket {
		setupDockerSocket(pod)
	}
	setupDockerSecrets(pod, &pod.Spec.Containers[0], build.Spec.Output.PushSecret, strategy.PullSecret, build.Spec.Source.Images)
	setOwnerReference(pod, build)
	setupSourceSecrets(pod, &pod.Spec.Containers[0], build.Spec.Source.SourceSecret)
	setupInputSecrets(pod, &pod.Spec.Containers[0], build.Spec.Source.Secrets)
	setupAdditionalSecrets(pod, &pod.Spec.Containers[0], build.Spec.Strategy.CustomStrategy.Secrets)
	setupContainersConfigs(build, pod)
	setupBuildCAs(build, pod, additionalCAs, internalRegistryHost)
	setupContainersStorage(pod, &pod.Spec.Containers[0])
	if securityContext == nil || securityContext.Privileged == nil || !*securityContext.Privileged {
		setupBuilderAutonsUser(build, strategy.Env, pod)
		setupBuilderDeviceFUSE(pod)
	}
	setupBlobCache(pod)
	return pod, nil
}
