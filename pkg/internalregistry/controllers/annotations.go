package controllers

const (
	// Annotation added to managed image pull secrets to indicate the service account used in the token.
	InternalRegistryAuthTokenServiceAccountAnnotation = "openshift.io/internal-registry-auth-token.service-account"

	// Annotation added to managed image pull secrets to indicate the service account token's binding type.
	InternalRegistryAuthTokenTypeAnnotation = "openshift.io/internal-registry-auth-token.binding"

	// Annotation added to service accounts to document the corresponding managed image pull secret.
	InternalRegistryImagePullSecretRefKey = "openshift.io/internal-registry-pull-secret-ref"

	// Indicates a bound service account token is used for authentication.
	AuthTokenTypeBound = "bound"

	// Indicates a legacy, long-lived, service account token is used for authentication.
	AuthTokenTypeLegacy = "legacy"
)
