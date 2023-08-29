package controller

import (
	"context"
	"fmt"
	"strings"

	operatorv1 "github.com/openshift/api/operator/v1"
	imageregistryinformer "github.com/openshift/client-go/imageregistry/informers/externalversions"
	"github.com/openshift/client-go/imageregistry/listers/imageregistry/v1"
	"github.com/openshift/library-go/pkg/build/naming"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	serviceaccountcontrollers "github.com/openshift/openshift-controller-manager/pkg/serviceaccounts/controllers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type imagePullSecretControllerEnablerController struct {
	factory.Controller
	kubeClient                kubernetes.Interface
	imageRegistryConfigLister v1.ConfigLister
	kubeInformers             informers.SharedInformerFactory
	additionalRegistryURLs    []string

	controllersCtxCancel context.CancelFunc
	controllersStarted   bool
	serviceAccountLister corelisters.ServiceAccountLister
	secretLister         corelisters.SecretLister

	recorder events.Recorder

	dockercfgDeletedController      *serviceaccountcontrollers.DockercfgDeletedController
	dockercfgTokenDeletedController *serviceaccountcontrollers.DockercfgTokenDeletedController
	dockercfgController             *serviceaccountcontrollers.DockercfgController
	dockerRegistryServiceController *serviceaccountcontrollers.DockerRegistryServiceController
}

func newImagePullSecretControllerEnablerController(kubeClient kubernetes.Interface,
	kubeInformers informers.SharedInformerFactory,
	registryInformers imageregistryinformer.SharedInformerFactory,
	additionalRegistryURLs []string,
	recorder events.Recorder,
) *imagePullSecretControllerEnablerController {
	dockerURLsInitialized := make(chan struct{})
	c := &imagePullSecretControllerEnablerController{
		kubeClient:                kubeClient,
		imageRegistryConfigLister: registryInformers.Imageregistry().V1().Configs().Lister(),
		kubeInformers:             kubeInformers,
		additionalRegistryURLs:    additionalRegistryURLs,
		serviceAccountLister:      kubeInformers.Core().V1().ServiceAccounts().Lister(),
		secretLister:              kubeInformers.Core().V1().Secrets().Lister(),
		recorder:                  recorder,
		dockercfgDeletedController: serviceaccountcontrollers.NewDockercfgDeletedController(
			kubeInformers.Core().V1().Secrets(),
			kubeClient,
			serviceaccountcontrollers.DockercfgDeletedControllerOptions{},
		),
		dockercfgTokenDeletedController: serviceaccountcontrollers.NewDockercfgTokenDeletedController(
			kubeInformers.Core().V1().Secrets(),
			kubeClient,
			serviceaccountcontrollers.DockercfgTokenDeletedControllerOptions{},
		),
		dockercfgController: serviceaccountcontrollers.NewDockercfgController(
			kubeInformers.Core().V1().ServiceAccounts(),
			kubeInformers.Core().V1().Secrets(),
			kubeClient,
			serviceaccountcontrollers.DockercfgControllerOptions{DockerURLsInitialized: dockerURLsInitialized},
		),
	}
	c.dockerRegistryServiceController = serviceaccountcontrollers.NewDockerRegistryServiceController(
		c.kubeInformers.Core().V1().Secrets(),
		c.kubeInformers.Core().V1().Services(),
		c.kubeClient,
		serviceaccountcontrollers.DockerRegistryServiceControllerOptions{
			DockercfgController:    c.dockercfgController,
			DockerURLsInitialized:  dockerURLsInitialized,
			ClusterDNSSuffix:       "cluster.local",
			AdditionalRegistryURLs: c.additionalRegistryURLs,
		},
	)
	c.Controller = factory.New().
		WithInformers(registryInformers.Imageregistry().V1().Configs().Informer()).
		WithSync(c.sync).
		ToController("ImagePullSecretControllerEnablerController", recorder)
	return c
}

func (c *imagePullSecretControllerEnablerController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	config, err := c.imageRegistryConfigLister.Get("cluster")
	if errors.IsNotFound(err) {
		klog.V(2).Infof("config.imageregistry.operator.openshift.io/cluster not found.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to retrieve config.imageregistry.operator.openshift.io/cluster: %w", err)
	}
	if config.Spec.ManagementState == operatorv1.Removed {
		klog.V(2).InfoS("Internal registry is disabled.", "managementState", config.Spec.ManagementState)
		c.stopControllers(ctx)
		err = c.cleanup(ctx)
		if err != nil {
			return fmt.Errorf("there was a problem cleaning up generated resources: %w", err)
		}
		return nil
	}
	klog.V(2).InfoS("Internal registry is enabled.", "managementState", config.Spec.ManagementState)
	c.startControllers(ctx)
	return nil
}

func (c *imagePullSecretControllerEnablerController) startControllers(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
		if !c.controllersStarted {
			var controllersCtx context.Context
			controllersCtx, c.controllersCtxCancel = context.WithCancel(ctx)
			go c.dockercfgDeletedController.Run(controllersCtx.Done())
			go c.dockercfgTokenDeletedController.Run(controllersCtx.Done())
			go c.dockercfgController.Run(5, controllersCtx.Done())
			go c.dockerRegistryServiceController.Run(10, controllersCtx.Done())
			c.controllersStarted = true
		}
	}
}

func (c *imagePullSecretControllerEnablerController) stopControllers(ctx context.Context) {
	if c.controllersStarted {
		c.controllersCtxCancel()
		c.controllersStarted = false
	}
}

func (c *imagePullSecretControllerEnablerController) cleanup(ctx context.Context) error {
	// cleanup service accounts
	serviceAccounts, err := c.serviceAccountLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("unable to list ServiceAccounts: %w", err)
	}
	for _, serviceAccount := range serviceAccounts {
		imagePullSecretName, imagePullSecret, err := c.imagePullSecretForServiceAccount(serviceAccount)
		if err != nil {
			return fmt.Errorf("unable to retrieve the image pull secret for the service account %q (ns=%q): %w", serviceAccount.Name, serviceAccount.Namespace, err)
		}
		var tokenSecret *corev1.Secret
		if imagePullSecret != nil {
			tokenSecret, err = c.tokenSecretForImagePullSecret(imagePullSecret)
			if err != nil {
				return fmt.Errorf("unable to retrive the service account token secret for the image pull secret %q (ns=%q): %w", imagePullSecret.Name, imagePullSecret.Namespace, err)
			}
		}
		if tokenSecret != nil {
			err := c.kubeClient.CoreV1().Secrets(tokenSecret.Namespace).Delete(ctx, tokenSecret.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("unable to delete the service account token secret %q (ns=%q): %w", tokenSecret.Name, tokenSecret.Namespace, err)
			}
		}
		if imagePullSecret != nil {
			err := c.kubeClient.CoreV1().Secrets(imagePullSecret.Namespace).Delete(ctx, imagePullSecret.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("unable to delete image pull secret %q (ns=%q): %w", imagePullSecret.Name, imagePullSecret.Namespace, err)
			}
		}
		if len(imagePullSecretName) != 0 {
			var secretRefs []corev1.ObjectReference
			for _, secretRef := range serviceAccount.Secrets {

				if secretRef.Name != imagePullSecretName {
					secretRefs = append(secretRefs, secretRef)
				}
			}
			serviceAccount.Secrets = secretRefs

			var imagePullSecretRefs []corev1.LocalObjectReference = []corev1.LocalObjectReference{}
			for _, imagePullSecretRef := range serviceAccount.ImagePullSecrets {
				if imagePullSecretRef.Name != imagePullSecretName {
					imagePullSecretRefs = append(imagePullSecretRefs, imagePullSecretRef)
				}
			}
			serviceAccount.ImagePullSecrets = imagePullSecretRefs
			_, err := c.kubeClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("unable to clean up references to the image pull secret %q (ns=%q) from the service accout %q: %w", imagePullSecret.Name, imagePullSecret.Namespace, serviceAccount.Name, err)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return nil
}

func (c *imagePullSecretControllerEnablerController) imagePullSecretForServiceAccount(serviceAccount *corev1.ServiceAccount) (string, *corev1.Secret, error) {
	var imagePullSecretName string
	imagePullSecretNamePrefix := naming.GetName(serviceAccount.Name, "dockercfg-", 58)
	for _, imagePullSecretRef := range serviceAccount.ImagePullSecrets {
		if strings.HasPrefix(imagePullSecretRef.Name, imagePullSecretNamePrefix) {
			imagePullSecretName = imagePullSecretRef.Name
			break
		}
	}
	if len(imagePullSecretName) == 0 {
		for _, secretRef := range serviceAccount.Secrets {
			if strings.HasPrefix(secretRef.Name, imagePullSecretNamePrefix) {
				imagePullSecretName = secretRef.Name
				break
			}
		}
	}
	if len(imagePullSecretName) == 0 {
		return "", nil, nil
	}
	imagePullSecret, err := c.secretLister.Secrets(serviceAccount.Namespace).Get(imagePullSecretName)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("Referenced imagePullSecret does not exist.", "ns", serviceAccount.Namespace, "sa", serviceAccount.Name, "imagePullSecret", imagePullSecretName)
		return imagePullSecretName, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return imagePullSecretName, imagePullSecret, nil
}

func (c *imagePullSecretControllerEnablerController) tokenSecretForImagePullSecret(secret *corev1.Secret) (*corev1.Secret, error) {
	tokenSecretName := secret.Annotations[serviceaccountcontrollers.ServiceAccountTokenSecretNameKey]
	if len(tokenSecretName) == 0 {
		return nil, nil
	}
	tokenSecret, err := c.secretLister.Secrets(secret.Namespace).Get(tokenSecretName)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	return tokenSecret, err
}
