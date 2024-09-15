package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

func Test_newClient(t *testing.T) {
	log := logging.NewNopLogger()
	clientTlsCrt, clientTlsKey, err := createCertBundle()
	if err != nil {
		t.Fatal(err)
	}
	serverTlsCrt, serverTlsKey, err := createCertBundle()
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		cert               []byte
		key                []byte
		ca                 []byte
		insecure           bool
		serverRequiresMTLS bool
	}
	type want struct {
		newClientErr      error
		sendRequestHasErr bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"NoMTLSConfig": {
			args: args{
				insecure:           true,
				serverRequiresMTLS: false,
			},
			want: want{},
		},
		"ValidMTLSConfig": {
			args: args{
				cert:               clientTlsCrt,
				key:                clientTlsKey,
				ca:                 serverTlsCrt,
				insecure:           false,
				serverRequiresMTLS: true,
			},
			want: want{},
		},
		"InvalidMTLSConfig": {
			args: args{
				cert:     []byte("invalid cert"),
				key:      []byte("invalid key"),
				insecure: false,
			},
			want: want{
				newClientErr: errors.New("tls: failed to find any PEM data in certificate input"),
			},
		},
		"ServerNotInCA": {
			args: args{
				cert:               clientTlsCrt,
				key:                clientTlsKey,
				insecure:           false,
				serverRequiresMTLS: true,
			},
			want: want{
				sendRequestHasErr: true,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			client, gotErr := NewClient(log, 10*time.Second, []byte(tc.args.cert), []byte(tc.args.key), []byte(tc.args.ca), tc.args.insecure)
			if diff := cmp.Diff(tc.want.newClientErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("NewClient(...): -want error, +got error: %s", diff)
			}
			if gotErr != nil {
				return
			}

			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			cert, err := tls.X509KeyPair(serverTlsCrt, serverTlsKey)
			if err != nil {
				t.Fatalf("invalid server certificates: %s", err)
			}
			cas := x509.NewCertPool()
			cas.AppendCertsFromPEM(clientTlsCrt)
			server.TLS = &tls.Config{
				Certificates: []tls.Certificate{cert},
				ClientCAs:    cas,
			}
			if tc.args.serverRequiresMTLS {
				server.TLS.ClientAuth = tls.RequireAndVerifyClientCert
			}

			server.StartTLS()
			defer server.Close()

			_, err = client.SendRequest(context.Background(), http.MethodGet, server.URL, Data{Decrypted: "", Encrypted: ""}, Data{Decrypted: map[string][]string{}, Encrypted: map[string][]string{}})
			if tc.want.sendRequestHasErr == (err == nil) {
				t.Fatal(err)
			}
		})
	}
}

func createCertBundle() ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	// Create a self-signed certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}
	encodedCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if encodedCert == nil {
		return nil, nil, errors.New("could not PEM encode certificate")
	}
	encodedKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	if encodedKey == nil {
		return nil, nil, errors.New("could not PEM encode private key")
	}

	return encodedCert, encodedKey, nil
}
