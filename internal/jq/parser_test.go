package jq

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

var testJQObject = map[string]any{
	"mappings": []any{
		map[string]any{
			"body":   "{ username: .payload.body.username, email: .payload.body.email }",
			"method": "POST",
			"url":    ".payload.baseUrl",
		},
		map[string]any{
			"method": "GET",
			"url":    `(.payload.baseUrl + "/" + .response.body.id)`,
		},
		map[string]any{
			"body":   `{ username: "john_doe_new_username" }`,
			"method": "PUT",
			"url":    `(.payload.baseUrl + "/" + .response.body.id)`,
		},
		map[string]any{
			"method": "DELETE",
			"url":    `(.payload.baseUrl + "/" + .response.body.id)`,
		},
	},
	"payload": map[string]any{
		"baseUrl": "https://api.example.com/users",
		"body":    map[string]any{"email": "john.doe@example.com", "username": "john_doe", "age": float64(30)},
	},
	"response": map[string]any{
		"body":       map[string]any{"id": "123"},
		"method":     "POST",
		"statusCode": float64(200),
	},
}

func Test_runJQQuery(t *testing.T) {
	type args struct {
		jqQuery  string
		jqObject interface{}
	}
	type want struct {
		result interface{}
		err    error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessStringObject": {
			args: args{
				jqQuery:  `(.payload.baseUrl + "/" + .response.body.id)`,
				jqObject: testJQObject,
			},
			want: want{
				result: `https://api.example.com/users/123`,
				err:    nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := runJQQuery(tc.args.jqQuery, tc.args.jqObject)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("runJQQuery(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("runJQQuery(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_ParseString(t *testing.T) {
	type args struct {
		jqQuery string
		obj     interface{}
	}
	type want struct {
		result interface{}
		err    error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessStringObject": {
			args: args{
				jqQuery: `(.payload.baseUrl + "/" + .response.body.id)`,
				obj:     testJQObject,
			},
			want: want{
				result: `https://api.example.com/users/123`,
				err:    nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := ParseString(tc.args.jqQuery, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("ParseString(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ParseString(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_ParseFloat(t *testing.T) {
	type args struct {
		jqQuery string
		obj     interface{}
	}
	type want struct {
		result interface{}
		err    error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessFloatObject": {
			args: args{
				jqQuery: `.payload.body.age`,
				obj:     testJQObject,
			},
			want: want{
				result: float64(30),
				err:    nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := ParseFloat(tc.args.jqQuery, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("ParseFloat(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ParseFloat(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_ParseBool(t *testing.T) {
	type args struct {
		jqQuery string
		obj     interface{}
	}
	type want struct {
		result interface{}
		err    error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessBoolObjectTrue": {
			args: args{
				jqQuery: `.payload.body.age == 30`,
				obj:     testJQObject,
			},
			want: want{
				result: true,
				err:    nil,
			},
		},
		"SuccessBoolObjectFalse": {
			args: args{
				jqQuery: `.payload.body.age == 31`,
				obj:     testJQObject,
			},
			want: want{
				result: false,
				err:    nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := ParseBool(tc.args.jqQuery, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("ParseBool(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ParseBool(...): -want result, +got result: %s", diff)
			}
		})
	}
}
func Test_ParseMapInterface(t *testing.T) {
	type args struct {
		jqQuery string
		obj     interface{}
	}
	type want struct {
		result map[string]interface{}
		err    error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				jqQuery: `{ name: .payload.body.username, email: .payload.body.email }`,
				obj:     testJQObject,
			},
			want: want{
				result: map[string]any{"email": "john.doe@example.com", "name": "john_doe"},
				err:    nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := ParseMapInterface(tc.args.jqQuery, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("ParseMapInterface(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ParseMapInterface(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_ParseMapStrings(t *testing.T) {
	// implemented on Test_ApplyJQOnMapStrings
}
