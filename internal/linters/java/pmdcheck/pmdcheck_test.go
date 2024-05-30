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
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestFormatPmdCheckLine(t *testing.T) {
	tc := []struct {
		input    string
		expected *linters.LinterOutput
	}{
		{" /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:9: Avoid unused local variables such as 'test'.", &linters.LinterOutput{
			File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java",
			Line:    9,
			Column:  0,
			Message: "Avoid unused local variables such as 'test'.",
		}},
		{" /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:10: Usage of System.out/err", &linters.LinterOutput{
			File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java",
			Line:    10,
			Column:  0,
			Message: "Usage of System.out/err",
		}},
	}

	for _, c := range tc {
		output, err := pmdcheckParser(c.input)
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
