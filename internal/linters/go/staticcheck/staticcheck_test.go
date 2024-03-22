package staticcheck

import (
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestGozeroOutputFilter_Filter(t *testing.T) {

	tests := []struct {
		s      GozeroOutputFilter
		input  *linters.LinterOutput
		expect *linters.LinterOutput
	}{
		{
			defaultGozeroFilter,
			&linters.LinterOutput{Message: `unknown JSON option "default=30" (SA5008)`},
			nil,
		},
		{
			defaultGozeroFilter,
			&linters.LinterOutput{Message: `unknown JSON option "optional" (SA5008)`},
			nil,
		},
		{
			defaultGozeroFilter,
			&linters.LinterOutput{Message: `unknown JSON option "range=[0:120]" (SA5008)`},
			nil,
		},
		{
			defaultGozeroFilter,
			&linters.LinterOutput{Message: `unknown JSON option "options=offline|inService|submitted" (SA5008)`},
			nil,
		},
	}
	for _, tt := range tests {
		output := tt.s.Filter(tt.input)
		if output != tt.expect {
			t.Errorf("should filter this output")
		}
	}

	i := &linters.LinterOutput{Message: `unknown JSON option "test" (SA5008)`}
	output := defaultGozeroFilter.Filter(i)
	if output != i {
		t.Errorf("should not filter this output")
	}
}
