package datapatcher

import (
	"errors"
	"fmt"
	"testing"

	"github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/google/go-cmp/cmp"
)

func Test_findJqPlaceholders(t *testing.T) {
	type args struct {
		value string
	}
	type want struct {
		result []string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldFindJqPlaceholders": {
			args: args{
				value: "data -> {{jq .body.token }} {{jq .body | fromjson | .token }} {{ some other invalid placeholder }}",
			},
			want: want{
				result: []string{"{{jq .body.token }}", "{{jq .body | fromjson | .token }}"},
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got := findJqPlaceholders(tc.args.value)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("findJqPlaceholders(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_removeJqDuplicates(t *testing.T) {
	type args struct {
		value []string
	}
	type want struct {
		result []string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldRemoveDuplicates": {
			args: args{
				value: []string{"{{jq .body.token }}", "{{jq .body.token }}", "{{jq .body | fromjson | token }}"},
			},
			want: want{
				result: []string{"{{jq .body.token }}", "{{jq .body | fromjson | token }}"},
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got := removeDuplicates(tc.args.value)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("removeJqDuplicates(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_parseJqPlaceholder(t *testing.T) {
	type args struct {
		placeholder string
	}
	type want struct {
		jqFilter string
		err      error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldParsePlaceholderNoTrailingWhitespace": {
			args: args{
				placeholder: "{{jq .body.token}}",
			},
			want: want{
				jqFilter: ".body.token",
				err:      nil,
			},
		},
		"ShouldParsePlaceholderNoTrailingWhitespace2": {
			args: args{
				placeholder: "{{jq .body | fromjson | .token}}",
			},
			want: want{
				jqFilter: ".body | fromjson | .token",
				err:      nil,
			},
		},
		"ShouldParsePlaceholderWithTrailingWhitespace": {
			args: args{
				placeholder: "{{jq .body.token   }}",
			},
			want: want{
				jqFilter: ".body.token",
				err:      nil,
			},
		},
		"ShouldParsePlaceholderWithTrailingWhitespace2": {
			args: args{
				placeholder: "{{jq .body | fromjson | .token   }}",
			},
			want: want{
				jqFilter: ".body | fromjson | .token",
				err:      nil,
			},
		},
		"ShouldParsePlaceholderWithPrecedingWhitespace": {
			args: args{
				placeholder: "{{jq     .body.token   }}",
			},
			want: want{
				jqFilter: ".body.token",
				err:      nil,
			},
		},
		"ShouldFailDueToPrecedingWhitespace": {
			args: args{
				placeholder: "{{ jq .myexpression }}",
			},
			want: want{
				jqFilter: "",
				err:      errors.New("jq regex matching failed in placeholder '{{ jq .myexpression }}'"),
			},
		},
		"ShouldFailDueToEmptyFilter": {
			args: args{
				placeholder: "{{jq}}",
			},
			want: want{
				jqFilter: "",
				err:      errors.New("jq regex matching failed in placeholder '{{jq}}'"),
			},
		},
		"ShouldFailDueToEmptyFilterWithWhitespace": {
			args: args{
				placeholder: "{{jq     }}",
			},
			want: want{
				jqFilter: "",
				err:      errors.New("jq regex matching failed in placeholder '{{jq     }}'"),
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			jqFilter, err := parseJqPlaceholder(tc.args.placeholder)
			if diff := cmp.Diff(tc.want.jqFilter, jqFilter); err == nil && diff != "" {
				t.Errorf("parseJqPlaceholder(...): -want name, +got name: %s", diff)
			}
			var goterr string
			var wanterr string
			if err != nil {
				goterr = err.Error()
			}
			if tc.want.err != nil {
				wanterr = tc.want.err.Error()
			}
			if diff := cmp.Diff(goterr, wanterr); diff != "" {
				t.Errorf("parseJqPlaceholder(...): -want err, +got err: %s", diff)
			}
		})
	}
}

func Test_patchJqPlaceholdersToValue(t *testing.T) {
	type args struct {
		authResponse  http.HttpResponse
		valueToHandle string
		logger        logging.Logger
	}
	type want struct {
		result string
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldFindAndReplaceJqExpressionFromBody": {
			args: args{
				authResponse: http.HttpResponse{
					Body: `{"token":"ey123"}`,
				},
				valueToHandle: "Bearer {{jq .body.token }}",
				logger:        logging.NewNopLogger(),
			},
			want: want{
				result: "Bearer ey123",
				err:    nil,
			},
		},
		"ShouldFailDueToInvalidResponseSyntax": {
			args: args{
				authResponse: http.HttpResponse{
					Body: `{token:"ey123"}`,
				},
				valueToHandle: "Bearer {{jq .body.token }}",
				logger:        logging.NewNopLogger(),
			},
			want: want{
				result: "Bearer ey123",
				err: fmt.Errorf(
					"jq expression '%s' returned no match of type string, bool or float. The HttpResponse could contain invalid json",
					".body.token",
				),
			},
		},
		"ShouldFindAndReplaceComplexJqExpressionFromBody": {
			args: args{
				authResponse: http.HttpResponse{
					Body: `{"result":{"token":"ey123","id_token":"eyidtoken"}}`,
				},
				valueToHandle: "Bearer {{jq .body.result | { tok1: .token, tok2: .id_token } | tostring }}",
				logger:        logging.NewNopLogger(),
			},
			want: want{
				result: `Bearer {"tok1":"ey123","tok2":"eyidtoken"}`,
				err:    nil,
			},
		},
		"ShouldFailComplexJqExpressionDueToInvalidResultType": {
			args: args{
				authResponse: http.HttpResponse{
					Body: `{"result":{"token":"ey123","id_token":"eyidtoken"}}`,
				},
				valueToHandle: "Bearer {{jq .body.result | { tok1: .token, tok2: .id_token } }}",
				logger:        logging.NewNopLogger(),
			},
			want: want{
				result: `Bearer {"tok1":"ey123","tok2":"eyidtoken"}`,
				err: fmt.Errorf(
					"jq expression '%s' returned no match of type string, bool or float. The HttpResponse could contain invalid json",
					".body.result | { tok1: .token, tok2: .id_token }",
				),
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got, err := patchJqPlaceholdersToValue(tc.args.authResponse, tc.args.valueToHandle, tc.args.logger)
			if diff := cmp.Diff(tc.want.result, got); err == nil && diff != "" {
				t.Errorf("patchJqPlaceholdersToValue(...): -want result, +got result: %s", diff)
			}
			var goterr string
			var wanterr string
			if err != nil {
				goterr = err.Error()
			}
			if tc.want.err != nil {
				wanterr = tc.want.err.Error()
			}
			if diff := cmp.Diff(goterr, wanterr); diff != "" {
				t.Errorf("parseJqPlaceholder(...): -want err, +got err: %s", diff)
			}
		})
	}
}
