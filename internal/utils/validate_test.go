package utils

import (
	"net/http"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

func Test_IsRequestValid(t *testing.T) {
	type args struct {
		method string
		url    string
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"EmptyMethod": {
			args: args{
				method: "",
				url:    "https://www.example.com",
			},
			want: want{
				err: errors.New(errEmptyMethod),
			},
		},
		"InvalidUrl": {
			args: args{
				method: http.MethodGet,
				url:    "invalid-url",
			},
			want: want{
				err: errors.Errorf(ErrInvalidURL, "invalid-url"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErr := IsRequestValid(tc.args.method, tc.args.url)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.IsRequestValid(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_IsHTTPSuccess(t *testing.T) {
	type args struct {
		statusCode int
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
				statusCode: 200,
			},
			want: want{
				result: true,
			},
		},
		"ResultFalse": {
			args: args{
				statusCode: 400,
			},
			want: want{
				result: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsHTTPSuccess(tc.args.statusCode)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("IsHTTPSuccess(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_IsHTTPError(t *testing.T) {
	type args struct {
		statusCode         int
		allowedStatusCodes []int
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
				statusCode:         400,
				allowedStatusCodes: nil,
			},
			want: want{
				result: true,
			},
		},
		"ResultFalse": {
			args: args{
				statusCode:         200,
				allowedStatusCodes: nil,
			},
			want: want{
				result: false,
			},
		},
		"AllowedStatusCode404": {
			args: args{
				statusCode:         404,
				allowedStatusCodes: []int{404},
			},
			want: want{
				result: false,
			},
		},
		"AllowedStatusCode500": {
			args: args{
				statusCode:         500,
				allowedStatusCodes: []int{404, 500, 503},
			},
			want: want{
				result: false,
			},
		},
		"NotAllowedStatusCode400": {
			args: args{
				statusCode:         400,
				allowedStatusCodes: []int{404, 500},
			},
			want: want{
				result: true,
			},
		},
		"EmptyAllowedList": {
			args: args{
				statusCode:         404,
				allowedStatusCodes: []int{},
			},
			want: want{
				result: true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsHTTPError(tc.args.statusCode, tc.args.allowedStatusCodes)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("IsHTTPError(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_IsUrlValid(t *testing.T) {
	type args struct {
		url string
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
				url: "https://www.example.com",
			},
			want: want{
				result: true,
			},
		},
		"ResultFalse": {
			args: args{
				url: "",
			},
			want: want{
				result: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsUrlValid(tc.args.url)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("IsUrlValid(...): -want result, +got result: %s", diff)
			}
		})
	}
}
