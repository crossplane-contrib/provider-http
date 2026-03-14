package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCanWatchCRDWithCreate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		create   func(context.Context, *authv1.SelfSubjectAccessReview) error
		want     bool
		wantErr  bool
		errMatch string
	}{
		"Allowed": {
			create: func(_ context.Context, sar *authv1.SelfSubjectAccessReview) error {
				sar.Status.Allowed = true
				return nil
			},
			want: true,
		},
		"Denied": {
			create: func(_ context.Context, sar *authv1.SelfSubjectAccessReview) error {
				sar.Status.Allowed = false
				return nil
			},
			want: false,
		},
		"CreateErrorContainsVerb": {
			create: func(_ context.Context, _ *authv1.SelfSubjectAccessReview) error {
				return errors.New("boom")
			},
			wantErr:  true,
			errMatch: "verb get",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			got, err := canWatchCRDWithCreate(context.Background(), scheme, tc.create)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("canWatchCRDWithCreate(...): expected error")
				}
				if tc.errMatch != "" && !strings.Contains(err.Error(), tc.errMatch) {
					t.Fatalf("canWatchCRDWithCreate(...): error %q does not contain %q", err.Error(), tc.errMatch)
				}
				return
			}

			if err != nil {
				t.Fatalf("canWatchCRDWithCreate(...): unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("canWatchCRDWithCreate(...): got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanWatchCRDWithCreateChecksAllVerbs(t *testing.T) {
	t.Parallel()

	seen := make([]string, 0, 3)
	scheme := runtime.NewScheme()
	allowed, err := canWatchCRDWithCreate(context.Background(), scheme, func(_ context.Context, sar *authv1.SelfSubjectAccessReview) error {
		seen = append(seen, sar.Spec.ResourceAttributes.Verb)
		sar.Status.Allowed = true
		return nil
	})
	if err != nil {
		t.Fatalf("canWatchCRDWithCreate(...): unexpected error: %v", err)
	}
	if !allowed {
		t.Fatalf("canWatchCRDWithCreate(...): expected allowed=true")
	}

	want := []string{"get", "list", "watch"}
	if len(seen) != len(want) {
		t.Fatalf("verb count mismatch: got %d, want %d", len(seen), len(want))
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("verb at index %d: got %q, want %q", i, seen[i], want[i])
		}
	}
}
