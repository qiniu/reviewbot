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
	"testing"
)

func TestTrimReport(t *testing.T) {
	tc := []struct {
		input    string
		expected string
	}{
		{"6月 14, 2024 7:19:02 下午 com.puppycrawl.tools.checkstyle.Main runCli\n详细: Checkstyle debug logging enabled\n6月 14, 2024 7:19:02 下午 com.puppycrawl.tools.checkstyle.Main runCli\n详细: Running Checkstyle with version: 10.17.0\n开始检查……\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:1: 文件未以空行结尾。 [NewlineAtEndOfFile]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:1: 缺少 package-info.java 文件。 [JavadocPackage]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:2: Line has trailing spaces. [RegexpSingleline]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:4:5: 缺少 Javadoc 。 [JavadocVariable]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:4:12: 变量 'name' 应定义为 private 的，并为其创建访问方法。 [VisibilityModifier]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:5:5: 缺少 Javadoc 。 [JavadocVariable]\n检查完成。\nCheckstyle以 27 个错误结束。", "[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:1: 文件未以空行结尾。 [NewlineAtEndOfFile]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:1: 缺少 package-info.java 文件。 [JavadocPackage]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:2: Line has trailing spaces. [RegexpSingleline]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:4:5: 缺少 Javadoc 。 [JavadocVariable]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:4:12: 变量 'name' 应定义为 private 的，并为其创建访问方法。 [VisibilityModifier]\n[ERROR] /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:5:5: 缺少 Javadoc 。 [JavadocVariable]"},
		{"开始检查……\n\t检查完成。", ""},
	}

	for _, c := range tc {
		output := trimReport(c.input)
		//if output != "" {
		if output != c.expected {
			t.Errorf("expected: %v, got: %v", c.expected, output)
		}
		continue
	}

}
func TestFormatStyleCheckLine(t *testing.T) {
	tc := []struct {
		input    string
		expected *linters.LinterOutput
	}{
		{"/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test2.java:21:18: '{' 前应有空格。 [WhitespaceAround]", &linters.LinterOutput{
			File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test2.java",
			Line:    21,
			Column:  18,
			Message: "'{' 前应有空格。 [WhitespaceAround]",
		}},
		{"/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:1: 文件未以空行结尾。 [NewlineAtEndOfFile]", &linters.LinterOutput{
			File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java",
			Line:    1,
			Column:  0,
			Message: "文件未以空行结尾。 [NewlineAtEndOfFile]",
		}},
	}

	for _, c := range tc {
		output, err := stylecheckParser(c.input)
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
