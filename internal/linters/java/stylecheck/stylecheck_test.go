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
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/xlog"
)

func TestArgs(t *testing.T) {
	tp := true
	tcs := []struct {
		id    string
		input linters.Agent
		want  linters.Agent
	}{
		{
			id: "case1 - default command and args",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"stylecheck"},
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"java"},
					Args:    []string{""},
				},
			},
		},
		{
			id: "case2 - custom command",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"/usr/java"},
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"/usr/java"},
					Args:    []string{""},
				},
			},
		},
		{
			id: "case3 - custom args",
			input: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"stylecheck"},
					Args:    []string{"-f", "xml"},
				},
			},
			want: linters.Agent{
				LinterConfig: config.Linter{
					Enable:  &tp,
					Command: []string{"java"},
					Args:    []string{"-f", "xml"},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			got := argsApply(xlog.New("ut"), tc.input)
			if !reflect.DeepEqual(got.LinterConfig, tc.want.LinterConfig) {
				t.Errorf("args() = %v, want %v", got.LinterConfig, tc.want.LinterConfig)
			}
		})
	}
}

func TestForConfig(t *testing.T) {
	fileDir := "/var/tmp/linters-config/"
	rulefilepath := filepath.Join(fileDir, "sun_checks.xml")
	path, err := styleRuleCheck(fileDir)(xlog.New("ut"), "https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/sun_checks.xml")
	if err != nil {
		t.Errorf("styleRuleCheck(): %v, expected: %v", err, nil)
	}
	if path != rulefilepath {
		t.Errorf("styleRuleCheck(): %v, expected: %v", path, rulefilepath)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(fileDir)
	})
}

func TestStyleRuleCheck(t *testing.T) {
	dir := "/var/tmp/linters-config/"

	tc := []struct {
		input    string
		expected string
		err      error
	}{
		{
			input:    "",
			expected: dir + "sun_checks.xml",
			err:      nil,
		},
		{
			input:    "/config/linters-config/.notjava-sun-checks.xml.xml",
			expected: "",
			err:      errors.New("the style rule file not exist"),
		},
		{
			input:    "https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/sun_checks.xml",
			expected: dir + "sun_checks.xml",
			err:      nil,
		},
	}
	for _, c := range tc {
		got, err := styleRuleCheck(dir)(xlog.New("ut"), c.input)
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("stylecheckParser(): %v, expected: %v", got, c.expected)
		}
		if !reflect.DeepEqual(err, c.err) {
			t.Errorf("stylecheckParser() error: %v, unexpected: %v", err, c.err)
			return
		}
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
}

func TestCheckJar(t *testing.T) {
	stykejarfilename := "checkstyle-10.17.0-all.jar"
	filePath := "/var/tmp/linters-config/"
	filename2 := filepath.Join(filePath, stykejarfilename)
	path, err := stylecheckJar(nil)
	if err != nil {
		t.Errorf("styleJarCheck(): %v, expected: %v", err, nil)
	}
	if path != filename2 {
		t.Errorf("styleJarCheck(): %v, expected: %v", path, filename2)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(filePath)
	})
}

func TestFormatStyleCheckLine(t *testing.T) {
	tc := []struct {
		input      []byte
		expected   map[string][]linters.LinterOutput
		unexpected []string
	}{
		{
			input: []byte(`Starting audit...
6月 14, 2024 7:19:02 下午 com.puppycrawl.tools.checkstyle.Main runCli
详细: Checkstyle debug logging enabled
[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/api/v1/admin/test.java:1: Missing package-info.java file. [JavadocPackage]
[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/api/v1/admin/test.java:16:5: Missing a Javadoc comment. [MissingJavadocMethod]
Audit done.
Checkstyle ends with 20 errors.`),
			expected: map[string][]linters.LinterOutput{
				"api/v1/admin/test.java": {
					{
						File:    "api/v1/admin/test.java",
						Line:    1,
						Column:  0,
						Message: "Missing package-info.java file. [JavadocPackage]",
					},
					{
						File:    "api/v1/admin/test.java",
						Line:    16,
						Column:  5,
						Message: "Missing a Javadoc comment. [MissingJavadocMethod]",
					},
				},
			},
			unexpected: nil,
		},
	}

	for _, c := range tc {
		linterWorkDir := "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples"

		got, err := stylecheckParser(linterWorkDir)(xlog.New("UnitJavaStyleCheckTest"), c.input)
		if !reflect.DeepEqual(err, c.unexpected) {
			t.Errorf("stylecheckParser() error: %v, unexpected: %v", err, c.unexpected)
			return
		}
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("stylecheckParser(): %v, expected: %v", got, c.expected)
		}
	}
}
