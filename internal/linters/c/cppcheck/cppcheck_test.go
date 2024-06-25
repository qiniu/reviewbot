package cppcheck

import (
	"testing"

	"errors"
	"reflect"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

func TestParser(t *testing.T) {
	cases := []struct {
		output []byte
		want   map[string][]linters.LinterOutput
		err    error
	}{
		{
			output: []byte(`
'cppcheck/cppcheck.c:6:7: Array 'a[10]' accessed at index 10, which is out of bounds.'
'cppcheck.cpp:1198:23: Variable 'bitsAvailable' is reassigned a value before the old one has been used.'
'main.cpp:5:1: error: missing closing parenthesis ')''
`),
			want: map[string][]linters.LinterOutput{
				"cppcheck/cppcheck.c": {
					{
						File:    "cppcheck/cppcheck.c",
						Line:    6,
						Column:  7,
						Message: "Array 'a[10]' accessed at index 10, which is out of bounds.",
					},
				},
				"cppcheck.cpp": {
					{
						File:    "cppcheck.cpp",
						Line:    1198,
						Column:  23,
						Message: "Variable 'bitsAvailable' is reassigned a value before the old one has been used.",
					},
				},
				"main.cpp": {
					{
						File:    "main.cpp",
						Line:    5,
						Column:  1,
						Message: "error: missing closing parenthesis ')'",
					},
				},
			},
			err: nil,
		},
		{
			output: []byte(`
cppcheck: error: could not find or open any of the paths given.
`),
			want: map[string][]linters.LinterOutput{},
			err:  nil,
		},
	}

	for _, tt := range cases {
		got, err := parser(xlog.New("UnitTest"), tt.output)
		if !errors.Is(err, tt.err) {
			t.Errorf("parser() error = %v, wantErr %v", err, tt.err)
			return
		}

		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parser() = %v, want %v", got, tt.want)
		}
	}
}
