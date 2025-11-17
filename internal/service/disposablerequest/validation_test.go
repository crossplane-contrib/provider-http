package disposablerequest

import (
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/disposablerequest/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

func TestIsResponseAsExpected(t *testing.T) {
	type args struct {
		spec *v1alpha2.DisposableRequestParameters
		res  httpClient.HttpResponse
	}

	type want struct {
		expected bool
		err      error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoExpectedResponseDefinition": {
			reason: "Should return true when no expected response is defined",
			args: args{
				spec: &v1alpha2.DisposableRequestParameters{
					URL:    "https://api.example.com/test",
					Method: "GET",
				},
				res: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{"status": "success"}`,
				},
			},
			want: want{
				expected: true,
				err:      nil,
			},
		},
		"ZeroStatusCode": {
			reason: "Should return false when status code is zero",
			args: args{
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".body.status == \"success\"",
				},
				res: httpClient.HttpResponse{
					StatusCode: 0,
					Body:       `{"status": "success"}`,
				},
			},
			want: want{
				expected: false,
				err:      nil,
			},
		},
		"ValidJQFilterReturnsTrue": {
			reason: "Should return true when JQ filter evaluates to true",
			args: args{
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".body.status == \"success\"",
				},
				res: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{"status": "success"}`,
				},
			},
			want: want{
				expected: true,
				err:      nil,
			},
		},
		"ValidJQFilterReturnsFalse": {
			reason: "Should return false when JQ filter evaluates to false",
			args: args{
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".body.status == \"success\"",
				},
				res: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{"status": "failed"}`,
				},
			},
			want: want{
				expected: false,
				err:      nil,
			},
		},

		"ComplexJQFilter": {
			reason: "Should handle complex JQ filter expressions",
			args: args{
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".body.data.items | length > 0",
				},
				res: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{"data": {"items": [1, 2, 3]}}`,
				},
			},
			want: want{
				expected: true,
				err:      nil,
			},
		},
		"JQFilterWithStatusCodeCheck": {
			reason: "Should evaluate JQ filter with status code check",
			args: args{
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".statusCode == 201 and .body.created == true",
				},
				res: httpClient.HttpResponse{
					StatusCode: 201,
					Body:       `{"created": true, "id": "123"}`,
				},
			},
			want: want{
				expected: true,
				err:      nil,
			},
		},
		"JQFilterWithHeadersCheck": {
			reason: "Should evaluate JQ filter checking headers",
			args: args{
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".headers.\"Content-Type\"[0] == \"application/json\"",
				},
				res: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{}`,
					Headers:    map[string][]string{"Content-Type": {"application/json"}},
				},
			},
			want: want{
				expected: true,
				err:      nil,
			},
		},
		"EmptyResponseBody": {
			reason: "Should handle empty response body with JQ filter",
			args: args{
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".statusCode == 204",
				},
				res: httpClient.HttpResponse{
					StatusCode: 204,
					Body:       "",
				},
			},
			want: want{
				expected: true,
				err:      nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := IsResponseAsExpected(tc.args.spec, tc.args.res)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsResponseAsExpected(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if tc.want.err == nil && got != tc.want.expected {
				t.Errorf("\n%s\nIsResponseAsExpected(...): wanted %v, got %v", tc.reason, tc.want.expected, got)
			}
		})
	}
}
