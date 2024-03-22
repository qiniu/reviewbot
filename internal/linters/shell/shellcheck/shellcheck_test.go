package shellcheck

import (
	"os"
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestShell(t *testing.T) {
	content, err := os.ReadFile("./testdata/shellcheck_test.txt")
	if err != nil {
		t.Errorf("open file failed ,the err is : %v", err)
		return
	}
	expect := []linters.LinterOutput{
		{
			File:    "lua-ut.sh",
			Line:    22,
			Column:  22,
			Message: "",
		},
		{
			File:    "util.sh",
			Line:    13,
			Column:  13,
			Message: "",
		},
	}
	tc := []struct {
		input    []byte
		expected []linters.LinterOutput
	}{
		{
			content,
			expect,
		},
	}
	for _, c := range tc {
		outputMap, err := formatShellcheckOutput([]byte(c.input))
		for _, outputs := range outputMap {
			for _, output := range outputs {

				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if output.File == "util.sh" {
					if output.File != c.expected[1].File || output.Line != c.expected[1].Line || output.Column != c.expected[1].Column {
						t.Errorf("expected: %v, got: %v", c.expected[1], output)
					}
				} else {
					if output.File != c.expected[0].File || output.Line != c.expected[0].Line || output.Column != c.expected[0].Column {
						t.Errorf("expected: %v, got: %v", c.expected[0], output)
					}
				}

			}

		}
	}
}
