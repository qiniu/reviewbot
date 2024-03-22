package cppcheck

import (
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestFormatCppCheckLine(t *testing.T) {
	tc := []struct {
		input    string
		expected *linters.LinterOutput
	}{
		{"'cppcheck_test.c:6:7: Array 'a[10]' accessed at index 10, which is out of bounds.'", &linters.LinterOutput{
			File:    "cppcheck_test.c",
			Line:    6,
			Column:  7,
			Message: "Array 'a[10]' accessed at index 10, which is out of bounds.",
		}},
		{"'fdk-aac-0.1.4-libMpegTPDec-src/tpdec_asc.cpp:1198:23: Variable 'bitsAvailable' is reassigned a value before the old one has been used.'", &linters.LinterOutput{
			File:    "fdk-aac-0.1.4-libMpegTPDec-src/tpdec_asc.cpp",
			Line:    1198,
			Column:  23,
			Message: "Variable 'bitsAvailable' is reassigned a value before the old one has been used.",
		}},
		{"Checking test/tpdec_adts.c", nil},
	}

	for _, c := range tc {

		output, err := cppcheckParser(c.input)

		if output == nil {
			if c.expected != nil {
				t.Errorf("expected: %v, got: %v", c.expected, output)
			}
			continue
		}

		if c.expected == nil && output != nil {
			t.Errorf("expected error, got: %v", output)
		}

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if output.File != c.expected.File || output.Line != c.expected.Line || output.Column != c.expected.Column || output.Message != c.expected.Message {
			t.Errorf("expected: %v, got: %v", c.expected, output)
		}
	}
}
