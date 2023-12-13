package utils

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var limit int32 = 3
var failures int32 = 2
var testTimeout = &v1.Duration{Duration: 5 * time.Second}

func Test_ShouldRetry(t *testing.T) {
	type args struct {
		rollbackRetriesLimit *int32
		statusFailed         int32
	}
	type want struct {
		result bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"ResultTrue": {
			args: args{
				statusFailed:         failures,
				rollbackRetriesLimit: &limit,
			},
			want: want{
				result: true,
			},
		},
		"ResultFalseBothNil": {
			args: args{
				rollbackRetriesLimit: nil,
			},
			want: want{
				result: false,
			},
		},
		"ResultFalseLimitNil": {
			args: args{
				statusFailed:         failures,
				rollbackRetriesLimit: nil,
			},
			want: want{
				result: false,
			},
		},
		"ResultFalseFailuresNil": {
			args: args{
				statusFailed:         0,
				rollbackRetriesLimit: &limit,
			},
			want: want{
				result: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ShouldRetry(tc.args.rollbackRetriesLimit, tc.args.statusFailed)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ShouldRetry(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_RetriesLimitReached(t *testing.T) {
	type args struct {
		rollbackRetriesLimit *int32
		statusFailed         int32
	}
	type want struct {
		result bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"ResultTrue": {
			args: args{
				statusFailed:         failures,
				rollbackRetriesLimit: &failures,
			},
			want: want{
				result: true,
			},
		},
		"ResultFalse": {
			args: args{
				statusFailed:         failures,
				rollbackRetriesLimit: &limit,
			},
			want: want{
				result: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RetriesLimitReached(tc.args.statusFailed, tc.args.rollbackRetriesLimit)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("RetriesLimitReached(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_RollBackEnabled(t *testing.T) {
	type args struct {
		rollbackRetriesLimit *int32
	}
	type want struct {
		result bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"ResultTrue": {
			args: args{
				rollbackRetriesLimit: &limit,
			},
			want: want{
				result: true,
			},
		},
		"ResultFalse": {
			args: args{
				rollbackRetriesLimit: nil,
			},
			want: want{
				result: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RollBackEnabled(tc.args.rollbackRetriesLimit)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("RollBackEnabled(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_WaitTimeout(t *testing.T) {
	type args struct {
		timeout *v1.Duration
	}
	type want struct {
		result time.Duration
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"ResultTrue": {
			args: args{
				timeout: testTimeout,
			},
			want: want{
				result: testTimeout.Duration,
			},
		},
		"ResultFalse": {
			args: args{
				timeout: nil,
			},
			want: want{
				result: defaultWaitTimeout,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := WaitTimeout(tc.args.timeout)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("WaitTimeout(...): -want result, +got result: %s", diff)
			}
		})
	}
}
