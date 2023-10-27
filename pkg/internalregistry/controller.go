package internalregistry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/build/naming"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	v1 "k8s.io/client-go/applyconfigurations/core/v1"

	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"gopkg.in/square/go-jose.v2/jwt"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/storage/names"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

const (
	controllerName                        = string(openshiftcontrolplanev1.OpenShiftServiceAccountPullSecretsController)
	InternalRegistryImagePullSecretRefKey = "openshift.io/internal-registry-pull-secret-ref"
	ExpirationCheckPeriod                 = 10 * time.Minute
)

type ImagePullSecretsController interface {
	factory.Controller
}

type imagePullSecretsController struct {
	factory.Controller
	kubeClient           kubernetes.Interface
	secretLister         corev1listers.SecretLister
	serviceAccountLister corev1listers.ServiceAccountLister
	serviceLister        corev1listers.ServiceLister
	recorder             events.Recorder

	additionalRegistryURLs []string
}

func NewImagePullSecretsController(
	kubeClient kubernetes.Interface,
	serviceAccountInformerFactory corev1informers.ServiceAccountInformer,
	secretInformerFactory corev1informers.SecretInformer,
	servicesInformerFactory corev1informers.ServiceInformer,
	additionalRegistryURLs []string,
	recorder events.Recorder,
) *imagePullSecretsController {
	c := &imagePullSecretsController{
		kubeClient:             kubeClient,
		secretLister:           secretInformerFactory.Lister(),
		serviceAccountLister:   serviceAccountInformerFactory.Lister(),
		serviceLister:          servicesInformerFactory.Lister(),
		additionalRegistryURLs: additionalRegistryURLs,
		recorder:               recorder,
	}
	triggers := []factory.Informer{
		secretInformerFactory.Informer(),
		serviceAccountInformerFactory.Informer(),
	}
	c.Controller = factory.New().
		WithInformers(triggers...).
		WithFilteredEventsInformers(servicesFilter, servicesInformerFactory.Informer()).
		ResyncEvery(ExpirationCheckPeriod).
		WithSync(c.sync).
		ToController(controllerName, recorder)
	return c
}

func (c *imagePullSecretsController) sync(ctx context.Context, syncContext factory.SyncContext) error {
	serviceAccounts, err := c.serviceAccountLister.List(labels.Everything())
	if err != nil {
		return err
	}

	var errs []error
	for _, serviceAccount := range serviceAccounts {
		errs = append(errs, c.syncServiceAccount(ctx, serviceAccount))
	}
	return errors.Join(errs...)
}

func (c *imagePullSecretsController) syncServiceAccount(ctx context.Context, serviceAccount *corev1.ServiceAccount) error {
	var err error
	// ensure image pull secret name annotation
	serviceAccount, err = c.syncGeneratedImagePullSecretName(ctx, serviceAccount)
	if err != nil {
		return err
	}

	// get generated image pull secret for the service account
	imagePullSecret, err := c.imagePullSecretForServiceAccount(ctx, serviceAccount)
	if err != nil {
		return err
	}

	// update registry authentication in the generated image pull secret
	err = c.syncImagePullSecret(ctx, imagePullSecret, serviceAccount)
	if err != nil {
		return err
	}

	// ensure image pull secret is referenced in service account
	err = c.syncGeneratedImagePullSecretRef(ctx, serviceAccount, imagePullSecret.Name)
	if err != nil {
		return err
	}

	// cleanup legacy resources
	err = c.cleanupLegacyResources(ctx, serviceAccount)

	return err
}

// imagePullSecretForServiceAccount returns a the generated image pull secret that this controller manages.
// If the image pull secret does not already exist, it will be created. A newly created image pull secret
// will not have the registry authentication file data.
func (c *imagePullSecretsController) imagePullSecretForServiceAccount(ctx context.Context, serviceAccount *corev1.ServiceAccount) (*corev1.Secret, error) {
	// get the generated image pull secret
	imagePullSecret, err := c.secretLister.Secrets(serviceAccount.Namespace).Get(serviceAccount.Annotations[InternalRegistryImagePullSecretRefKey])
	if kerrors.IsNotFound(err) {
		// if image pull secret is not found, create it
		imagePullSecret = newImagePullSecretForServiceAccount(serviceAccount)
		var actual *corev1.Secret
		actual, err = c.kubeClient.CoreV1().Secrets(serviceAccount.Namespace).Create(ctx, imagePullSecret, metav1.CreateOptions{FieldManager: controllerName})
		if err != nil {
			c.recorder.Warningf("SecretCreateFailed", "Failed to create %s: %v", resourcehelper.FormatResourceForCLIWithNamespace(imagePullSecret), err)
			return nil, err
		}
		imagePullSecret = actual
		c.recorder.Eventf("SecretCreated", "Created %s because it was missing", resourcehelper.FormatResourceForCLIWithNamespace(imagePullSecret))
	}
	return imagePullSecret, err
}

func newImagePullSecretForServiceAccount(serviceAccount *corev1.ServiceAccount) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceAccount.Annotations[InternalRegistryImagePullSecretRefKey],
			Namespace:       serviceAccount.Namespace,
			OwnerReferences: []metav1.OwnerReference{asOwnerReference(serviceAccount)},
		},
		Type: corev1.SecretTypeDockercfg,
		Data: map[string][]byte{
			corev1.DockerConfigKey: []byte("{}"),
		},
	}
}

func asOwnerReference(serviceAccount *corev1.ServiceAccount) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Name:       serviceAccount.Name,
		UID:        serviceAccount.UID,
	}
}

func (c *imagePullSecretsController) syncGeneratedImagePullSecretName(ctx context.Context, serviceAccount *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	name := serviceAccount.Annotations[InternalRegistryImagePullSecretRefKey]
	if len(name) != 0 {
		return serviceAccount, nil
	}

	// try to reuse the legacy image pull secret name.
	name = c.legacyImagePullSecretName(ctx, serviceAccount)

	if len(name) == 0 {
		name = names.SimpleNameGenerator.GenerateName(naming.GetName(serviceAccount.Name, "dockercfg-", 58))
	}

	patch, err := v1.ExtractServiceAccount(serviceAccount, controllerName)
	if err != nil {
		return serviceAccount, err
	}
	patch.WithAnnotations(map[string]string{InternalRegistryImagePullSecretRefKey: name})
	//	metav1.SetMetaDataAnnotation(&serviceAccount.ObjectMeta, InternalRegistryImagePullSecretRefKey, name)
	// actual, _, err := resourceapply.ApplyServiceAccount(ctx, c.kubeClient.CoreV1(), c.recorder, serviceAccount)
	actual, err := c.kubeClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Apply(ctx, patch, metav1.ApplyOptions{Force: true, FieldManager: controllerName})
	if err != nil {
		c.recorder.Warningf("ServiceAccountUpdateFailed", "Failed to update %s: %v", resourcehelper.FormatResourceForCLIWithNamespace(serviceAccount), err)
		return serviceAccount, err
	}
	c.recorder.Eventf("ServiceAccountUpdated", "Updated %s because it changed", resourcehelper.FormatResourceForCLIWithNamespace(serviceAccount))
	return actual, nil
}

func (c *imagePullSecretsController) syncGeneratedImagePullSecretRef(ctx context.Context, serviceAccount *corev1.ServiceAccount, imagePullSecretName string) error {
	for _, ref := range serviceAccount.ImagePullSecrets {
		if ref.Name == imagePullSecretName {
			return nil
		}
	}
	serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, corev1.LocalObjectReference{Name: imagePullSecretName})
	_, err := c.kubeClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
	if err != nil {
		c.recorder.Warningf(fmt.Sprintf("%sUpdateFailed", "ServiceAccount"), "Failed to update %s: %v", resourcehelper.FormatResourceForCLIWithNamespace(serviceAccount), err)
	}
	c.recorder.Eventf(fmt.Sprintf("%sUpdated", "ServiceAccount"), "Updated %s because it changed", resourcehelper.FormatResourceForCLIWithNamespace(serviceAccount))
	return err
}

func (c *imagePullSecretsController) syncImagePullSecret(ctx context.Context, imagePullSecret *corev1.Secret, serviceAccount *corev1.ServiceAccount) error {
	// if token is not expired and not expiring soon, nothing to do
	if c.registryAuthenticationFileValid(imagePullSecret) {
		return nil
	}
	klog.V(2).InfoS("Refreshing image pull secret", "ns", serviceAccount.Namespace, "ServiceAccount", serviceAccount.Name, "Secret.Name", imagePullSecret.Name)
	// request new token
	tokenRequest, err := c.kubeClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).CreateToken(ctx, serviceAccount.Name,
		&authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{BoundObjectRef: &authenticationv1.BoundObjectReference{
			APIVersion: "v1", Kind: "Secret", Name: imagePullSecret.Name, UID: imagePullSecret.UID,
		}}},
		metav1.CreateOptions{},
	)
	if err != nil {
		return err
	}

	// compute registry authentication data
	data, err := json.Marshal(dockerConfig(tokenRequest.Status.Token, c.urlsForInternalRegistry()))
	if err != nil {
		return fmt.Errorf("unable to serialize registry auth data: %w", err)
	}
	imagePullSecret.Data[corev1.DockerConfigKey] = data

	// ensure the service account is referenced as an owner
	addOwnerReference := true
	for _, ref := range imagePullSecret.OwnerReferences {
		if ref.UID == serviceAccount.UID {
			addOwnerReference = false
			break
		}
	}
	if addOwnerReference {
		imagePullSecret.OwnerReferences = append(imagePullSecret.OwnerReferences, asOwnerReference(serviceAccount))
	}

	// update
	_, err = c.kubeClient.CoreV1().Secrets(imagePullSecret.Namespace).Update(ctx, imagePullSecret, metav1.UpdateOptions{FieldManager: controllerName})

	return err
}

func dockerConfig(token string, urls []string) any {
	// not using credentialprovider.DockerConfig to keep redundant username/password/email out of secret
	auth := map[string]map[string]string{}
	entry := map[string]string{
		"auth": base64.StdEncoding.EncodeToString([]byte("<token>:" + token)),
	}
	for _, url := range urls {
		auth[url] = entry
	}
	return auth
}

func (c *imagePullSecretsController) registryAuthenticationFileValid(imagePullSecret *corev1.Secret) bool {
	if imagePullSecret.Type != corev1.SecretTypeDockercfg {
		klog.V(2).InfoS("Internal registry pull secret type is incorrect.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "type", imagePullSecret.Type)
		return false
	}
	// registry authentication file must exist
	// TODO ok, seems impossible to create such a secret if the type is set correctly, remove this check after confirming.
	if _, ok := imagePullSecret.Data[corev1.DockerConfigKey]; !ok {
		klog.V(2).InfoS("Internal registry pull secret does not contain the expected key.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "keys", reflect.ValueOf(imagePullSecret.Data).MapKeys())
		return false
	}
	// parse the registry authentication file
	auth := credentialprovider.DockerConfig{}
	if err := json.Unmarshal(imagePullSecret.Data[corev1.DockerConfigKey], &auth); err != nil {
		klog.V(2).InfoS("Internal registry pull secret auth data cannot be parse.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name)
		return false
	}

	// there should be an entries for each internal registry url
	urls := c.urlsForInternalRegistry()
	if len(auth) != len(urls) {
		klog.V(2).InfoS("Internal registry pull secret auth data does not contain the correct number of entries.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "expected", len(urls), "actual", len(auth))
		return false
	}
	matches := 0
CheckUrl:
	for _, url := range urls {
		for key := range auth {
			if key == url {
				matches++
				continue CheckUrl
			}
		}
	}
	if matches != len(urls) {
		klog.V(2).InfoS("Internal registry pull secret auth data does not contain the correct entries.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "expected", urls, "actual", reflect.ValueOf(auth).MapKeys())
		return false
	}

	// check the token embedded in the registry authentication file
	for url, entry := range auth {
		token, err := jwt.ParseSigned(entry.Password)
		if err != nil {
			klog.V(2).InfoS("Internal registry pull secret auth token cannot be parsed.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "error", err)
			return false
		}
		var claims jwt.Claims
		// "unsafe" in the following API just means we are not validating the signature
		err = token.UnsafeClaimsWithoutVerification(&claims)
		if err != nil {
			klog.V(2).InfoS("Internal registry pull secret auth token claim cannot be parsed.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "error", err)
			return false
		}
		// if token is expired or about to expire we want to trigger a new token request
		refreshTime := claims.Expiry.Time().Add(-ExpirationCheckPeriod - 1*time.Minute)
		klog.V(4).InfoS("Token expiration check.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "expirtyTime", claims.Expiry.Time(), "refreshTime", refreshTime)
		if time.Now().After(refreshTime) {
			klog.V(2).InfoS("Internal registry pull secret auth token needs to be refreshed.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "expirtyTime", claims.Expiry.Time(), "refreshTime", refreshTime)
			return false
		}
	}
	klog.V(4).InfoS("Internal registry pull secret does not need to be refreshed.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name)
	return true
}
