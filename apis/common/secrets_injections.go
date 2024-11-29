package common

// SecretRef contains the name and namespace of a Kubernetes secret.
type SecretRef struct {
	// Name is the name of the Kubernetes secret.
	Name string `json:"name"`

	// Namespace is the namespace of the Kubernetes secret.
	Namespace string `json:"namespace"`
}

// SecretInjectionConfig represents the configuration for injecting secret data into a Kubernetes secret.
type SecretInjectionConfig struct {
	// SecretRef contains the name and namespace of the Kubernetes secret where the data will be injected.
	SecretRef SecretRef `json:"secretRef"`

	// SecretKey is the key within the Kubernetes secret where the data will be injected.
	// Deprecated: Use KeyMappings for injecting single or multiple keys.
	SecretKey string `json:"secretKey,omitempty"`

	// ResponsePath is a jq filter expression representing the path in the response where the secret value will be extracted from.
	// Deprecated: Use KeyMappings for injecting single or multiple keys.
	ResponsePath string `json:"responsePath,omitempty"`

	// KeyMappings allows injecting data into single or multiple keys within the same Kubernetes secret.
	KeyMappings []KeyInjection `json:"keyMappings,omitempty"`

	// Metadata contains labels and annotations to apply to the Kubernetes secret.
	Metadata Metadata `json:"metadata,omitempty"`

	// SetOwnerReference determines whether to set the owner reference on the Kubernetes secret.
	SetOwnerReference bool `json:"setOwnerReference,omitempty"`
}

// KeyInjection represents the configuration for injecting data into a specific key in a Kubernetes secret.
type KeyInjection struct {
	// SecretKey is the key within the Kubernetes secret where the data will be injected.
	SecretKey string `json:"secretKey"`

	// ResponseJQ is a jq filter expression representing the path in the response where the secret value will be extracted from.
	ResponseJQ string `json:"responseJQ"`
}

// Metadata contains labels and annotations to apply to a Kubernetes secret.
type Metadata struct {
	// Labels contains key-value pairs to apply as labels to the Kubernetes secret.
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations contains key-value pairs to apply as annotations to the Kubernetes secret.
	Annotations map[string]string `json:"annotations,omitempty"`
}
