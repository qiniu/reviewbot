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

package gofmt

import (
	"os"
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestGofmtOutput(t *testing.T) {
	content, err := os.ReadFile("./testdata/gofmt_test.txt")
	if err != nil {
		t.Errorf("open file failed ,the err is : %v", err)
		return
	}
	tc := []struct {
		input    []byte
		expected []linters.LinterOutput
	}{
		{
			content,
			[]linters.LinterOutput{
				{
					File:    "testfile/staticcheck.go",
					Line:    7,
					Column:  1,
					Message: "",
				},
				{
					File:      "testfile/test.go",
					Line:      9,
					Column:    4,
					Message:   "",
					StartLine: 6,
				},
			},
		},
	}
	for _, c := range tc {
		outputMap, err := formatGofmtOutput([]byte(c.input))
		for _, outputs := range outputMap {
			for i, output := range outputs {

				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if output.StartLine != 0 {
					if output.File != c.expected[1].File || output.StartLine != c.expected[1].StartLine || output.Line != c.expected[1].Line || output.Column != c.expected[1].Column {
						t.Errorf("expected: %v, got: %v", c.expected[i], output)
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
