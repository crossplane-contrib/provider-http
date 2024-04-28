package kubehandler

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSecret retrieves a Kubernetes Secret from the cluster.
func GetSecret(ctx context.Context, kubeClient client.Client, name string, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := kubeClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, secret)

	if err != nil {
		return &corev1.Secret{}, err
	}

	return secret, nil
}

// GetOrCreateSecret retrieves a Kubernetes Secret from the cluster. If the secret does not exist, it creates a new one.
func GetOrCreateSecret(ctx context.Context, kubeClient client.Client, name string, namespace string) (*corev1.Secret, error) {
	secret, err := GetSecret(ctx, kubeClient, name, namespace)
	if err != nil {
		if errs.IsNotFound(err) {
			secret, err = createSecret(ctx, kubeClient, name, namespace)
			if err != nil {
				return &corev1.Secret{}, err
			}
		}
	}

	return secret, nil
}

// UpdateSecret updates a Kubernetes Secret in the cluster.
func UpdateSecret(ctx context.Context, kubeClient client.Client, secret *corev1.Secret) error {
	err := kubeClient.Update(ctx, secret)
	if err != nil {
		return err
	}

	return nil
}

func createSecret(ctx context.Context, kubeClient client.Client, name string, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	err := kubeClient.Create(ctx, secret)
	if err != nil {
		return &corev1.Secret{}, err
	}

	return secret, nil
}
