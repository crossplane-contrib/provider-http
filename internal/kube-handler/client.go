package kubehandler

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errCreateSecret      = "create secret failed"
	errGetSecret         = "failed to get secret %s:%s"
	errUpdateFailed      = "update secret failed"
	errSetOwnerReference = "could not set owner reference to secret"
)

// GetSecret retrieves a Kubernetes Secret from the cluster.
func GetSecret(ctx context.Context, kubeClient client.Client, name string, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := kubeClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, secret)

	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errGetSecret, name, namespace))
	}

	return secret, nil
}

// GetOrCreateSecret retrieves a Kubernetes Secret from the cluster. If the secret does not exist, it creates a new one.
// If the secret exists but has no owner reference, it sets the owner reference and updates the secret.
func GetOrCreateSecret(ctx context.Context, kubeClient client.Client, name, namespace string, owner metav1.Object) (*corev1.Secret, error) {
	secret, err := GetSecret(ctx, kubeClient, name, namespace)
	if err != nil {
		if errs.IsNotFound(err) {
			return createSecret(ctx, kubeClient, name, namespace, owner)
		}
		return nil, err
	}

	// Check if the owner reference is missing and set it if needed
	if owner != nil && !hasOwnerReference(secret, owner) {
		if err := controllerutil.SetOwnerReference(owner, secret, kubeClient.Scheme()); err != nil {
			return nil, errors.Wrap(err, errSetOwnerReference)
		}
		if err := UpdateSecret(ctx, kubeClient, secret); err != nil {
			return nil, err
		}
	}

	return secret, nil
}

// UpdateSecret updates a Kubernetes Secret in the cluster.
func UpdateSecret(ctx context.Context, kubeClient client.Client, secret *corev1.Secret) error {
	err := kubeClient.Update(ctx, secret)
	if err != nil {
		return errors.Wrap(err, errUpdateFailed)
	}

	return nil
}

// createSecret creates a new Kubernetes Secret in the cluster.
func createSecret(ctx context.Context, kubeClient client.Client, name, namespace string, owner metav1.Object) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	if owner != nil {
		if err := controllerutil.SetOwnerReference(owner, secret, kubeClient.Scheme()); err != nil {
			return nil, errors.Wrap(err, errSetOwnerReference)
		}
	}

	err := kubeClient.Create(ctx, secret)
	if err != nil {
		return nil, errors.Wrap(err, errCreateSecret)
	}

	return secret, nil
}

// hasOwnerReference checks if the given secret has the specified owner reference.
func hasOwnerReference(secret *corev1.Secret, owner metav1.Object) bool {
	for _, ref := range secret.OwnerReferences {
		if ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}
