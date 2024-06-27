package cppcheck

import (
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

func TestParser(t *testing.T) {
	tc := []struct {
		input      string
		expected   map[string][]linters.LinterOutput
		unexpected []string
	}{
		{
			input: "'cppcheck_test.c:6:7: Array 'a[10]' accessed at index 10, which is out of bounds.'",
			expected: map[string][]linters.LinterOutput{
				"cppcheck_test.c": {
					{
						File:    "cppcheck_test.c",
						Line:    6,
						Column:  7,
						Message: "Array 'a[10]' accessed at index 10, which is out of bounds.",
					},
				},
			},
			unexpected: nil,
		},
		{
			input:      "''",
			expected:   map[string][]linters.LinterOutput{},
			unexpected: nil,
		},
	}

	for _, c := range tc {
		output, unexpected := parser(xlog.New("cppcheck"), []byte(c.input))
		if !reflect.DeepEqual(output, c.expected) {
			t.Errorf("expected: %v, got: %v", c.expected, output)
		}
		if !reflect.DeepEqual(unexpected, c.unexpected) {
			t.Errorf("expected: %v, got: %v", c.unexpected, unexpected)
		}
	}
}
