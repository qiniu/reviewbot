package staticcheck

import (
	"testing"

	"github.com/cr-bot/linters"
)

func TestFormatStaticcheckLine(t *testing.T) {
	tc := []struct {
		input    string
		expected *linters.LinterOutput
	}{
		{"cdn-admin.v2/client/dns/dnsapi.go:59:3: assignment to err", &linters.LinterOutput{
			File:    "cdn-admin.v2/client/dns/dnsapi.go",
			Line:    59,
			Column:  3,
			Message: "assignment to err",
		}},
		{"smart_scheduler/provider_scheduler/provider_manager/provider_manager.go:207:31: should use make([]float64, len(result.CDNLog.Points)) instead (S1019)", &linters.LinterOutput{
			File:    "smart_scheduler/provider_scheduler/provider_manager/provider_manager.go",
			Line:    207,
			Column:  31,
			Message: "should use make([]float64, len(result.CDNLog.Points)) instead (S1019)",
		}},
		{"cdn-admin.v2/api/api_line.go:342:3: should replace loop with ret = append(ret, scope.EdgeNodes...) (S1011)", &linters.LinterOutput{
			File:    "cdn-admin.v2/api/api_line.go",
			Line:    342,
			Column:  3,
			Message: "should replace loop with ret = append(ret, scope.EdgeNodes...) (S1011)",
		}},
		{"cdn-admin.v2/api/api_line.go:342 should replace loop with ret = append(ret, scope.EdgeNodes...)", nil},
	}

	for _, c := range tc {
		output, err := formatStaticcheckLine(c.input)
		if c.expected == nil && output != nil {
			t.Errorf("expected error, got: %v", output)
		} else {
			continue
		}

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if output == nil {
			if c.expected != nil {
				t.Errorf("expected: %v, got: %v", c.expected, output)
			}
			continue
		}

		if output.File != c.expected.File || output.Line != c.expected.Line || output.Column != c.expected.Column || output.Message != c.expected.Message {
			t.Errorf("expected: %v, got: %v", c.expected, output)
		}
	}
}
