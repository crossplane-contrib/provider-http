package http

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/common"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errBoom = errors.New("boom")
)

func TestLoadTLSConfig(t *testing.T) {
	type args struct {
		kubeClient kube.Client
		tlsConfig  *common.TLSConfig
	}
	type want struct {
		result *TLSConfigData
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"NilConfig": {
			args: args{
				kubeClient: nil,
				tlsConfig:  nil,
			},
			want: want{
				result: &TLSConfigData{},
				err:    nil,
			},
		},
		"EmptyConfig": {
			args: args{
				kubeClient: nil,
				tlsConfig:  &common.TLSConfig{},
			},
			want: want{
				result: &TLSConfigData{
					InsecureSkipVerify: false,
				},
				err: nil,
			},
		},
		"InlineCABundle": {
			args: args{
				kubeClient: nil,
				tlsConfig: &common.TLSConfig{
					CABundle:           []byte("inline-ca-bundle"),
					InsecureSkipVerify: true,
				},
			},
			want: want{
				result: &TLSConfigData{
					CABundle:           []byte("inline-ca-bundle"),
					InsecureSkipVerify: true,
				},
				err: nil,
			},
		},
		"CACertFromSecret": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: func(ctx context.Context, key kube.ObjectKey, obj kube.Object) error {
						if secret, ok := obj.(*corev1.Secret); ok {
							*secret = corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ca-secret",
									Namespace: "default",
								},
								Data: map[string][]byte{
									"ca.crt": []byte("secret-ca-bundle"),
								},
							}
							return nil
						}
						return errors.New("unexpected object type")
					},
				},
				tlsConfig: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "ca-secret",
							Namespace: "default",
						},
						Key: "ca.crt",
					},
				},
			},
			want: want{
				result: &TLSConfigData{
					CABundle:           []byte("secret-ca-bundle"),
					InsecureSkipVerify: false,
				},
				err: nil,
			},
		},
		"ClientCertAndKeyFromSecrets": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: func(ctx context.Context, key kube.ObjectKey, obj kube.Object) error {
						if secret, ok := obj.(*corev1.Secret); ok {
							switch key.Name {
							case "cert-secret":
								*secret = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "cert-secret",
										Namespace: "default",
									},
									Data: map[string][]byte{
										"tls.crt": []byte("client-certificate"),
									},
								}
							case "key-secret":
								*secret = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "key-secret",
										Namespace: "default",
									},
									Data: map[string][]byte{
										"tls.key": []byte("client-key"),
									},
								}
							}
							return nil
						}
						return errors.New("unexpected object type")
					},
				},
				tlsConfig: &common.TLSConfig{
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "cert-secret",
							Namespace: "default",
						},
						Key: "tls.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "key-secret",
							Namespace: "default",
						},
						Key: "tls.key",
					},
				},
			},
			want: want{
				result: &TLSConfigData{
					ClientCert:         []byte("client-certificate"),
					ClientKey:          []byte("client-key"),
					InsecureSkipVerify: false,
				},
				err: nil,
			},
		},
		"AllFieldsPopulated": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: func(ctx context.Context, key kube.ObjectKey, obj kube.Object) error {
						if secret, ok := obj.(*corev1.Secret); ok {
							switch key.Name {
							case "ca-secret":
								*secret = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "ca-secret",
										Namespace: "default",
									},
									Data: map[string][]byte{
										"ca.crt": []byte("ca-from-secret"),
									},
								}
							case "cert-secret":
								*secret = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "cert-secret",
										Namespace: "default",
									},
									Data: map[string][]byte{
										"tls.crt": []byte("client-cert"),
									},
								}
							case "key-secret":
								*secret = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "key-secret",
										Namespace: "default",
									},
									Data: map[string][]byte{
										"tls.key": []byte("client-key"),
									},
								}
							}
							return nil
						}
						return errors.New("unexpected object type")
					},
				},
				tlsConfig: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "ca-secret",
							Namespace: "default",
						},
						Key: "ca.crt",
					},
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "cert-secret",
							Namespace: "default",
						},
						Key: "tls.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "key-secret",
							Namespace: "default",
						},
						Key: "tls.key",
					},
					InsecureSkipVerify: true,
				},
			},
			want: want{
				result: &TLSConfigData{
					CABundle:           []byte("ca-from-secret"),
					ClientCert:         []byte("client-cert"),
					ClientKey:          []byte("client-key"),
					InsecureSkipVerify: true,
				},
				err: nil,
			},
		},
		"InlineCABundleTakesPrecedenceOverSecret": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: func(ctx context.Context, key kube.ObjectKey, obj kube.Object) error {
						return errors.New("should not be called")
					},
				},
				tlsConfig: &common.TLSConfig{
					CABundle: []byte("inline-ca-bundle"),
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "ca-secret",
							Namespace: "default",
						},
						Key: "ca.crt",
					},
				},
			},
			want: want{
				result: &TLSConfigData{
					CABundle:           []byte("inline-ca-bundle"),
					InsecureSkipVerify: false,
				},
				err: nil,
			},
		},
		"ErrorLoadingCASecret": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				tlsConfig: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "ca-secret",
							Namespace: "default",
						},
						Key: "ca.crt",
					},
				},
			},
			want: want{
				result: nil,
				err:    errors.Wrap(errors.Wrap(errBoom, "cannot get secret default/ca-secret"), "failed to load CA certificate from secret"),
			},
		},
		"ErrorLoadingClientCertSecret": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				tlsConfig: &common.TLSConfig{
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "cert-secret",
							Namespace: "default",
						},
						Key: "tls.crt",
					},
				},
			},
			want: want{
				result: nil,
				err:    errors.Wrap(errors.Wrap(errBoom, "cannot get secret default/cert-secret"), "failed to load client certificate from secret"),
			},
		},
		"ErrorLoadingClientKeySecret": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				tlsConfig: &common.TLSConfig{
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "key-secret",
							Namespace: "default",
						},
						Key: "tls.key",
					},
				},
			},
			want: want{
				result: nil,
				err:    errors.Wrap(errors.Wrap(errBoom, "cannot get secret default/key-secret"), "failed to load client key from secret"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := LoadTLSConfig(context.Background(), tc.args.kubeClient, tc.args.tlsConfig)

			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("LoadTLSConfig(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("LoadTLSConfig(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func TestLoadSecretData(t *testing.T) {
	type args struct {
		kubeClient kube.Client
		secretRef  *xpv1.SecretKeySelector
	}
	type want struct {
		result []byte
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"NilSecretRef": {
			args: args{
				kubeClient: nil,
				secretRef:  nil,
			},
			want: want{
				result: nil,
				err:    nil,
			},
		},
		"SuccessfulLoad": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: func(ctx context.Context, key kube.ObjectKey, obj kube.Object) error {
						if secret, ok := obj.(*corev1.Secret); ok {
							*secret = corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-secret",
									Namespace: "test-namespace",
								},
								Data: map[string][]byte{
									"test-key": []byte("test-value"),
								},
							}
							return nil
						}
						return errors.New("unexpected object type")
					},
				},
				secretRef: &xpv1.SecretKeySelector{
					SecretReference: xpv1.SecretReference{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Key: "test-key",
				},
			},
			want: want{
				result: []byte("test-value"),
				err:    nil,
			},
		},
		"SecretNotFound": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				secretRef: &xpv1.SecretKeySelector{
					SecretReference: xpv1.SecretReference{
						Name:      "missing-secret",
						Namespace: "test-namespace",
					},
					Key: "test-key",
				},
			},
			want: want{
				result: nil,
				err:    errors.Wrap(errBoom, "cannot get secret test-namespace/missing-secret"),
			},
		},
		"SecretKeyNotFound": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: func(ctx context.Context, key kube.ObjectKey, obj kube.Object) error {
						if secret, ok := obj.(*corev1.Secret); ok {
							*secret = corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-secret",
									Namespace: "test-namespace",
								},
								Data: map[string][]byte{
									"other-key": []byte("other-value"),
								},
							}
							return nil
						}
						return errors.New("unexpected object type")
					},
				},
				secretRef: &xpv1.SecretKeySelector{
					SecretReference: xpv1.SecretReference{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Key: "missing-key",
				},
			},
			want: want{
				result: nil,
				err:    errors.New("secret test-namespace/test-secret does not contain key missing-key"),
			},
		},
		"EmptySecretData": {
			args: args{
				kubeClient: &test.MockClient{
					MockGet: func(ctx context.Context, key kube.ObjectKey, obj kube.Object) error {
						if secret, ok := obj.(*corev1.Secret); ok {
							*secret = corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-secret",
									Namespace: "test-namespace",
								},
								Data: map[string][]byte{},
							}
							return nil
						}
						return errors.New("unexpected object type")
					},
				},
				secretRef: &xpv1.SecretKeySelector{
					SecretReference: xpv1.SecretReference{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Key: "test-key",
				},
			},
			want: want{
				result: nil,
				err:    errors.New("secret test-namespace/test-secret does not contain key test-key"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := loadSecretData(context.Background(), tc.args.kubeClient, tc.args.secretRef)

			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("loadSecretData(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("loadSecretData(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func TestMergeTLSConfigs(t *testing.T) {
	type args struct {
		resourceTLS *common.TLSConfig
		providerTLS *common.TLSConfig
	}
	type want struct {
		result *common.TLSConfig
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"BothNil": {
			args: args{
				resourceTLS: nil,
				providerTLS: nil,
			},
			want: want{
				result: nil,
			},
		},
		"ResourceNilProviderSet": {
			args: args{
				resourceTLS: nil,
				providerTLS: &common.TLSConfig{
					CABundle:           []byte("provider-ca"),
					InsecureSkipVerify: true,
				},
			},
			want: want{
				result: &common.TLSConfig{
					CABundle:           []byte("provider-ca"),
					InsecureSkipVerify: true,
				},
			},
		},
		"ResourceSetProviderNil": {
			args: args{
				resourceTLS: &common.TLSConfig{
					CABundle:           []byte("resource-ca"),
					InsecureSkipVerify: false,
				},
				providerTLS: nil,
			},
			want: want{
				result: &common.TLSConfig{
					CABundle:           []byte("resource-ca"),
					InsecureSkipVerify: false,
				},
			},
		},
		"ResourceOverridesProvider": {
			args: args{
				resourceTLS: &common.TLSConfig{
					CABundle:           []byte("resource-ca"),
					InsecureSkipVerify: true,
				},
				providerTLS: &common.TLSConfig{
					CABundle:           []byte("provider-ca"),
					InsecureSkipVerify: false,
				},
			},
			want: want{
				result: &common.TLSConfig{
					CABundle:           []byte("resource-ca"),
					InsecureSkipVerify: true,
				},
			},
		},
		"ResourceCABundleOverridesProviderCABundle": {
			args: args{
				resourceTLS: &common.TLSConfig{
					CABundle: []byte("resource-ca"),
				},
				providerTLS: &common.TLSConfig{
					CABundle: []byte("provider-ca"),
				},
			},
			want: want{
				result: &common.TLSConfig{
					CABundle:           []byte("resource-ca"),
					InsecureSkipVerify: false,
				},
			},
		},
		"ProviderCABundleUsedWhenResourceEmpty": {
			args: args{
				resourceTLS: &common.TLSConfig{
					InsecureSkipVerify: true,
				},
				providerTLS: &common.TLSConfig{
					CABundle: []byte("provider-ca"),
				},
			},
			want: want{
				result: &common.TLSConfig{
					CABundle:           []byte("provider-ca"),
					InsecureSkipVerify: true,
				},
			},
		},
		"ResourceSecretRefsOverrideProviderSecretRefs": {
			args: args{
				resourceTLS: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "resource-ca-secret",
							Namespace: "resource-ns",
						},
						Key: "ca.crt",
					},
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "resource-cert-secret",
							Namespace: "resource-ns",
						},
						Key: "tls.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "resource-key-secret",
							Namespace: "resource-ns",
						},
						Key: "tls.key",
					},
				},
				providerTLS: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-ca-secret",
							Namespace: "provider-ns",
						},
						Key: "ca.crt",
					},
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-cert-secret",
							Namespace: "provider-ns",
						},
						Key: "tls.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-key-secret",
							Namespace: "provider-ns",
						},
						Key: "tls.key",
					},
				},
			},
			want: want{
				result: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "resource-ca-secret",
							Namespace: "resource-ns",
						},
						Key: "ca.crt",
					},
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "resource-cert-secret",
							Namespace: "resource-ns",
						},
						Key: "tls.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "resource-key-secret",
							Namespace: "resource-ns",
						},
						Key: "tls.key",
					},
					InsecureSkipVerify: false,
				},
			},
		},
		"ProviderSecretRefsUsedWhenResourceEmpty": {
			args: args{
				resourceTLS: &common.TLSConfig{
					InsecureSkipVerify: true,
				},
				providerTLS: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-ca-secret",
							Namespace: "provider-ns",
						},
						Key: "ca.crt",
					},
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-cert-secret",
							Namespace: "provider-ns",
						},
						Key: "tls.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-key-secret",
							Namespace: "provider-ns",
						},
						Key: "tls.key",
					},
				},
			},
			want: want{
				result: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-ca-secret",
							Namespace: "provider-ns",
						},
						Key: "ca.crt",
					},
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-cert-secret",
							Namespace: "provider-ns",
						},
						Key: "tls.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-key-secret",
							Namespace: "provider-ns",
						},
						Key: "tls.key",
					},
					InsecureSkipVerify: true,
				},
			},
		},
		"MixedFieldsMerge": {
			args: args{
				resourceTLS: &common.TLSConfig{
					CABundle: []byte("resource-ca"),
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "resource-cert-secret",
							Namespace: "resource-ns",
						},
						Key: "tls.crt",
					},
				},
				providerTLS: &common.TLSConfig{
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-ca-secret",
							Namespace: "provider-ns",
						},
						Key: "ca.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-key-secret",
							Namespace: "provider-ns",
						},
						Key: "tls.key",
					},
					InsecureSkipVerify: true,
				},
			},
			want: want{
				result: &common.TLSConfig{
					CABundle: []byte("resource-ca"),
					CACertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-ca-secret",
							Namespace: "provider-ns",
						},
						Key: "ca.crt",
					},
					ClientCertSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "resource-cert-secret",
							Namespace: "resource-ns",
						},
						Key: "tls.crt",
					},
					ClientKeySecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-key-secret",
							Namespace: "provider-ns",
						},
						Key: "tls.key",
					},
					InsecureSkipVerify: false,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := MergeTLSConfigs(tc.args.resourceTLS, tc.args.providerTLS)

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("MergeTLSConfigs(...): -want result, +got result: %s", diff)
			}
		})
	}
}
