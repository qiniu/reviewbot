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
	"github.com/qiniu/x/xlog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestForConfig(t *testing.T) {
	fileDir, err := os.Getwd()
	rulefiledirpath := filepath.Join(fileDir, "config/linters-config")
	rulefilepath := filepath.Join(rulefiledirpath, ".java-bestpractices.xml")
	path, err := pmdRuleCheck("https://raw.githubusercontent.com/pmd/pmd/master/pmd-java/src/main/resources/category/java/bestpractices.xml")
	if err != nil {
		t.Errorf("pmdRuleCheck(): %v, expected: %v", err, nil)
	}
	if path != rulefilepath {
		t.Errorf("pmdRuleCheck(): %v, expected: %v", path, rulefilepath)
	}

}
func TestFormatPmdCheckLine(t *testing.T) {
	tc := []struct {
		input      []byte
		expected   map[string][]linters.LinterOutput
		unexpected []string
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
			unexpected: nil,
		},
		{
			input:      []byte(`[WARN] Progressbar rendering conflicts with reporting to STDOUT. No progressbar will be shown. Try running with argument -r <file> to output the report to a file instead.`),
			expected:   map[string][]linters.LinterOutput{},
			unexpected: nil,
		},
		{
			input:      []byte(``),
			expected:   map[string][]linters.LinterOutput{},
			unexpected: nil,
		},
	}

	for _, c := range tc {
		got, err := pmdcheckParser(xlog.New("UnitJavaPmdCheckTest"), c.input)
		if !reflect.DeepEqual(err, c.unexpected) {
			t.Errorf("pmdcheckParser() error: %v, unexpected: %v", err, c.unexpected)
			return
		}
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("pmdcheckParser(): %v, expected: %v", got, c.expected)
		}
	}
}
