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

package stylecheck

import (
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/xlog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestForConfig(t *testing.T) {
	fileDir, err := os.Getwd()
	rulefiledirpath := filepath.Join(fileDir, "config/linters-config")
	rulefilepath := filepath.Join(rulefiledirpath, ".sun_checks.xml")
	path, err := styleRuleCheck("https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/sun_checks.xml")
	if err != nil {
		t.Errorf("styleRuleCheck(): %v, expected: %v", err, nil)
	}
	if path != rulefilepath {
		t.Errorf("styleRuleCheck(): %v, expected: %v", path, rulefilepath)
	}

}
func TestCheckJar(t *testing.T) {
	var stykejarfilename = "checkstyle-10.17.0-all.jar"
	filePath, err := os.Getwd()
	filename2 := filepath.Join(filePath, stykejarfilename)
	path, err := stylecheckJar()
	if err != nil {
		t.Errorf("styleJarCheck(): %v, expected: %v", err, nil)
	}
	if path != filename2 {
		t.Errorf("styleJarCheck(): %v, expected: %v", path, filename2)
	}

}
func TestFormatStyleCheckLine(t *testing.T) {
	tc := []struct {
		input    []byte
		expected map[string][]linters.LinterOutput
		err      error
	}{
		{
			input: []byte(`[ERROR]/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test2.java:21:18: '{' 前应有空格。 [WhitespaceAround]`),
			expected: map[string][]linters.LinterOutput{
				"[ERROR]/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test2.java": {
					{
						File:    "[ERROR]/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test2.java",
						Line:    21,
						Column:  18,
						Message: "'{' 前应有空格。 [WhitespaceAround]",
					},
				},
			},
			err: nil,
		},
		{
			input: []byte(`[ERROR]/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:1: 文件未以空行结尾。 [NewlineAtEndOfFile]`),
			expected: map[string][]linters.LinterOutput{
				"[ERROR]/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java": {
					{
						File:    "[ERROR]/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java",
						Line:    1,
						Column:  0,
						Message: "文件未以空行结尾。 [NewlineAtEndOfFile]",
					},
				},
			},
			err: nil,
		},
		{
			input:    []byte(`6月 14, 2024 7:19:02 下午 com.puppycrawl.tools.checkstyle.Main runCli`),
			expected: map[string][]linters.LinterOutput{},
			err:      nil,
		},
		{
			input:    []byte(`详细: Checkstyle debug logging enabled`),
			expected: map[string][]linters.LinterOutput{},
			err:      nil,
		},
		{
			input:    []byte(`开始检查……`),
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
		got, err := stylecheckParser(xlog.New("UnitJavaStyleCheckTest"), c.input)
		if !errors.Is(err, c.err) {
			t.Errorf("stylecheckParser() error: %v, expected: %v", err, c.err)
			return
		}
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("stylecheckParser(): %v, expected: %v", got, c.expected)
		}
	}
}
