package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
			got := NormalizeWhitespace(tc.args.input)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("ConvertStringToJQQuery(...): -want result, +got result: %s", diff)
			}
		})
	}
}
