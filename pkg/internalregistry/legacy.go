package internalregistry

import (
	"context"
	"fmt"

	"github.com/openshift/openshift-controller-manager/pkg/serviceaccounts/controllers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/util/csaupgrade"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

func (c *imagePullSecretsController) legacyImagePullSecretName(ctx context.Context, serviceAccount *corev1.ServiceAccount) string {
	// find the legacy image pull secret in the same namespace
	secrets, err := c.secretLister.Secrets(serviceAccount.Namespace).List(labels.Everything())
	if err != nil {
		runtime.HandleError(err)
		return ""
	}
	for _, secret := range secrets {
		if isLegacyImagePullSecretForServiceAccount(secret, serviceAccount) {
			return secret.Name
		}
	}
	return ""
}

var legacyAnnotations = []string{
	corev1.ServiceAccountNameKey,
	corev1.ServiceAccountUIDKey,
	controllers.ServiceAccountTokenSecretNameKey,
	controllers.ServiceAccountTokenValueAnnotation,
}

func removeLegacyAnnotations(secret *corev1.Secret) {
	for _, a := range legacyAnnotations {
		delete(secret.Annotations, a)
	}
}

var expectedLegacyAnnotations = map[string]func(*corev1.ServiceAccount, string) bool{
	corev1.ServiceAccountNameKey:                   func(sa *corev1.ServiceAccount, v string) bool { return sa.Name == v },
	corev1.ServiceAccountUIDKey:                    func(sa *corev1.ServiceAccount, v string) bool { return sa.UID == types.UID(v) },
	controllers.ServiceAccountTokenSecretNameKey:   func(sa *corev1.ServiceAccount, v string) bool { return true },
	controllers.ServiceAccountTokenValueAnnotation: func(sa *corev1.ServiceAccount, v string) bool { return true },
}

func isLegacyImagePullSecretForServiceAccount(secret *corev1.Secret, serviceAccount *corev1.ServiceAccount) bool {
	for key, valueOK := range expectedLegacyAnnotations {
		value, ok := secret.Annotations[key]
		if !ok {
			return false
		}
		if !valueOK(serviceAccount, value) {
			return false
		}
	}
	return true
}

func (c *imagePullSecretsController) cleanupLegacyResources(ctx context.Context, serviceAccount *corev1.ServiceAccount) error {

	// get the image pull secret
	imagePullSecret, err := c.secretLister.Secrets(serviceAccount.Namespace).Get(serviceAccount.Annotations[InternalRegistryImagePullSecretRefKey])
	if err != nil {
		return err
	}

	//  delete the legacy token secret
	err = c.cleanupLegacyTokenSecret(ctx, imagePullSecret)
	if err != nil {
		return err
	}

	// take ownership of image pull secret legacy annotations
	imagePullSecret, err = c.cleanupLegacyFieldOwnership(ctx, imagePullSecret)
	if err != nil {
		return err
	}

	// remove image pull secret legacy annotations
	imagePullSecret, err = c.cleanupLegacyImagePullSecretAnnotations(ctx, imagePullSecret)
	if err != nil {
		return err
	}

	// remove the image pull secret from the service account's mountable secrets
	c.cleanupLegacySecretRefs(ctx, imagePullSecret.Name, serviceAccount)

	return nil
}

func (c *imagePullSecretsController) cleanupLegacySecretRefs(ctx context.Context, imagePullSecretName string, serviceAccount *corev1.ServiceAccount) error {
	var secretRefs []corev1.ObjectReference
	for _, secretRef := range serviceAccount.Secrets {

		if secretRef.Name != imagePullSecretName {
			secretRefs = append(secretRefs, secretRef)
		}
	}
	serviceAccount.Secrets = secretRefs
	_, err := c.kubeClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("unable to clean up references to the image pull secret %q (ns=%q) from the service accout %q: %w", imagePullSecretName, serviceAccount.Namespace, serviceAccount.Name, err)
	}
	return nil
}

func (c *imagePullSecretsController) cleanupLegacyTokenSecret(ctx context.Context, imagePullSecret *corev1.Secret) error {
	secretName := imagePullSecret.Annotations[controllers.ServiceAccountTokenSecretNameKey]
	if len(secretName) == 0 {
		return nil
	}
	_, err := c.secretLister.Secrets(imagePullSecret.Namespace).Get(secretName)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = c.kubeClient.CoreV1().Secrets(imagePullSecret.Namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	return err
}

func (c *imagePullSecretsController) cleanupLegacyImagePullSecretAnnotations(ctx context.Context, imagePullSecret *corev1.Secret) (*corev1.Secret, error) {
	patch, err := applycorev1.ExtractSecret(imagePullSecret, controllerName)
	if err != nil {
		return nil, err
	}
	patch.Annotations = imagePullSecret.DeepCopy().Annotations
	for _, a := range legacyAnnotations {
		delete(patch.Annotations, a)
	}
	return c.kubeClient.CoreV1().Secrets(imagePullSecret.Namespace).Apply(ctx, patch, metav1.ApplyOptions{Force: true, FieldManager: controllerName})
}

var legacyAnnotationsFieldPaths = fieldpath.NewSet(
	fieldpath.MakePathOrDie("metadata", "annotations", corev1.ServiceAccountNameKey),
	fieldpath.MakePathOrDie("metadata", "annotations", corev1.ServiceAccountUIDKey),
	fieldpath.MakePathOrDie("metadata", "annotations", controllers.ServiceAccountTokenSecretNameKey),
	fieldpath.MakePathOrDie("metadata", "annotations", controllers.ServiceAccountTokenValueAnnotation),
)

func (c *imagePullSecretsController) cleanupLegacyFieldOwnership(ctx context.Context, imagePullSecret *corev1.Secret) (*corev1.Secret, error) {
	fields := csaupgrade.FindFieldsOwners(imagePullSecret.ManagedFields, metav1.ManagedFieldsOperationUpdate,
		legacyAnnotationsFieldPaths,
	)
	var upgradeNeeded bool
	for _, field := range fields {
		if field.Manager != controllerName {
			upgradeNeeded = true
			break
		}
	}

	if !upgradeNeeded {
		return imagePullSecret, nil
	}

	err := csaupgrade.UpgradeManagedFields(imagePullSecret, sets.New("openshift-controller-manager"), controllerName)
	if err != nil {
		return imagePullSecret, err
	}

	return c.kubeClient.CoreV1().Secrets(imagePullSecret.Namespace).Update(ctx, imagePullSecret, metav1.UpdateOptions{FieldManager: controllerName})
}
