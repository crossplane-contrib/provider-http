package requestprocessing

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

var testHeaders = map[string][]string{
	"fruits":                {"apple", "banana", "orange"},
	"colors":                {"red", "green", "blue"},
	"countries":             {"USA", "UK", "India", "Germany"},
	"programming_languages": {"Go", "Python", "JavaScript"},
}

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
		"body":    map[string]any{"email": "john.doe@example.com", "username": "john_doe"},
	},
	"response": map[string]any{
		"body":       map[string]any{"id": "123"},
		"method":     "POST",
		"statusCode": float64(200),
	},
}

func Test_ConvertStringToJQQuery(t *testing.T) {
	type args struct {
		input string
	}
	type want struct {
		result string
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				input: `{
					todo_name: .payload.body.name, 
					reminder: .payload.body.reminder, 
					responsible: .payload.body.responsible,
				  }`,
			},
			want: want{
				result: `{ todo_name: .payload.body.name, reminder: .payload.body.reminder, responsible: .payload.body.responsible, }`,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ConvertStringToJQQuery(tc.args.input)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ConvertStringToJQQuery(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_ApplyJQOnStr(t *testing.T) {
	type args struct {
		jqQuery  string
		jqObject map[string]any
	}
	type want struct {
		result string
		err    error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessMapObject": {
			args: args{
				jqQuery:  `{ name: .payload.body.username, email: .payload.body.email }`,
				jqObject: testJQObject,
			},
			want: want{
				result: `{"email":"john.doe@example.com","name":"john_doe"}`,
				err:    nil,
			},
		},
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
			got, gotErr := ApplyJQOnStr(tc.args.jqQuery, tc.args.jqObject)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("ApplyJQOnStr(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ApplyJQOnStr(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_ApplyJQOnMapStrings(t *testing.T) {
	type args struct {
		keyToJQQueries map[string][]string
		jqObject       map[string]any
	}
	type want struct {
		result map[string][]string
		err    error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessNoJQ": {
			args: args{
				keyToJQQueries: testHeaders,
				jqObject:       testJQObject,
			},
			want: want{
				result: testHeaders,
				err:    nil,
			},
		},
		"SuccessWithJQ": {
			args: args{
				keyToJQQueries: map[string][]string{
					"fruits": {"apple", "banana", "orange"},
					"name":   {".payload.body.username"},
				},
				jqObject: testJQObject,
			},
			want: want{
				result: map[string][]string{
					"fruits": {"apple", "banana", "orange"},
					"name":   {"john_doe"},
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := ApplyJQOnMapStrings(tc.args.keyToJQQueries, tc.args.jqObject)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("ApplyJQOnMapStrings(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ApplyJQOnMapStrings(...): -want result, +got result: %s", diff)
			}
		})
	}
}
