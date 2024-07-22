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
	"os"
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

func TestPmdRuleCheck(t *testing.T) {
	dir, _ := os.Getwd()
	a := linters.Agent{}
	a.LinterConfig.WorkDir = ""
	tc := []struct {
		input    string
		expected string
		err      error
	}{
		{
			input:    "",
			expected: dir + "/tmp/bestpractices.xml",
			err:      nil,
		},
		{
			input:    "/config/linters-config/.notjava-bestpractices.xml",
			expected: "",
			err:      errors.New("the pmd rule file not exist"),
		},
		{
			input:    "https://raw.githubusercontent.com/pmd/pmd/master/pmd-java/src/main/resources/category/java/bestpractices.xml",
			expected: dir + "/tmp/bestpractices.xml",
			err:      nil,
		},
	}
	for _, c := range tc {
		got, err := pmdRuleCheck(xlog.New("ut"), c.input, a)
		log.Info("E:" + c.input)
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("pmdcheckParser(): %v, expected: %v", got, c.expected)
		}
		if !reflect.DeepEqual(err, c.err) {
			t.Errorf("pmdcheckParser() error: %v, unexpected: %v", err, c.err)
			return
		}
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir + "/tmp/")
	})
}

func TestFormatPmdCheckLine(t *testing.T) {
	tc := []struct {
		input      []byte
		expected   map[string][]linters.LinterOutput
		unexpected []string
	}{
		{
			input: []byte(`[ERROR] No such file ./test3.java
[WARN] Progressbar rendering conflicts with reporting to STDOUT. No progressbar will be shown. Try running with argument -r <file> to output the report to a file instead.
[WARN] This analysis could be faster, please consider using Incremental Analysis: https://docs.pmd-code.org/pmd-doc-7.4.0/pmd_userdocs_incremental_analysis.html
./test.java:8: Avoid unused local variables such as 'test'.`),
			expected: map[string][]linters.LinterOutput{
				"./test.java": {
					{
						File:    "./test.java",
						Line:    8,
						Column:  0,
						Message: "Avoid unused local variables such as 'test'.",
					},
				},
			},
			unexpected: nil,
		},
	}
	for _, c := range tc {
		got, err := pmdcheckParser(xlog.New("UnitJavaPmdCheckTest"), c.input)
		if !reflect.DeepEqual(err, c.unexpected) {
			t.Errorf("stylecheckParser() error: %v, unexpected: %v", err, c.unexpected)
			return
		}
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("stylecheckParser(): %v, expected: %v", got, c.expected)
		}
	}
}
