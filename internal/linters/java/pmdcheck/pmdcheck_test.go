/*
 Copyright 2024 Qiniu Cloud (qiniu.com).

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package pmdcheck

import (
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/xlog"
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestFormatPmdCheckLine(t *testing.T) {
	tc := []struct {
		input    []byte
		expected map[string][]linters.LinterOutput
		err      error
	}{
		{
			input: []byte(`/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:10: Usage of System.out`),
			expected: map[string][]linters.LinterOutput{
				"/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java": {
					{
						File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java",
						Line:    10,
						Column:  0,
						Message: "Usage of System.out",
					},
				},
			},
			err: nil,
		},
		{
			input:    []byte(`[WARN] Progressbar rendering conflicts with reporting to STDOUT. No progressbar will be shown. Try running with argument -r <file> to output the report to a file instead.`),
			expected: map[string][]linters.LinterOutput{},
			err:      nil,
		},
		{
			input:    []byte(``),
			expected: map[string][]linters.LinterOutput{},
			err:      nil,
		},
	}

	for _, c := range tc {
		got, err := pmdcheckParser(xlog.New("UnitJavaPmdCheckTest"), c.input)
		if !errors.Is(err, c.err) {
			t.Errorf("pmdcheckParser() error: %v, expected: %v", err, c.err)
			return
		}
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("pmdcheckParser(): %v, expected: %v", got, c.expected)
		}
	}
}
