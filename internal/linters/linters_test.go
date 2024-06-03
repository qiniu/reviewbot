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

package linters

import (
	"testing"
)

func TestFormatStaticcheckLine(t *testing.T) {
	tc := []struct {
		input    string
		expected *LinterOutput
	}{
		{"cdn-admin.v2/client/dns/dnsapi.go:59:3: assignment to err", &LinterOutput{
			File:    "cdn-admin.v2/client/dns/dnsapi.go",
			Line:    59,
			Column:  3,
			Message: "assignment to err",
		}},
		{"smart_scheduler/provider_scheduler/provider_manager/provider_manager.go:207:31: should use make([]float64, len(result.CDNLog.Points)) instead (S1019)", &LinterOutput{
			File:    "smart_scheduler/provider_scheduler/provider_manager/provider_manager.go",
			Line:    207,
			Column:  31,
			Message: "should use make([]float64, len(result.CDNLog.Points)) instead (S1019)",
		}},
		{"cdn-admin.v2/api/api_line.go:342:3: should replace loop with ret = append(ret, scope.EdgeNodes...) (S1011)", &LinterOutput{
			File:    "cdn-admin.v2/api/api_line.go",
			Line:    342,
			Column:  3,
			Message: "should replace loop with ret = append(ret, scope.EdgeNodes...) (S1011)",
		}},
		{"/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test2.java:21:18: '{' 前应有空格。 [WhitespaceAround]", &LinterOutput{
			File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test2.java",
			Line:    21,
			Column:  18,
			Message: "'{' 前应有空格。 [WhitespaceAround]",
		}},
		{"/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:1: 文件未以空行结尾。 [NewlineAtEndOfFile]", &LinterOutput{
			File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java",
			Line:    1,
			Column:  0,
			Message: "文件未以空行结尾。 [NewlineAtEndOfFile]",
		}},
		{" /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:9: Avoid unused local variables such as 'test'.", &LinterOutput{
			File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java",
			Line:    9,
			Column:  0,
			Message: "Avoid unused local variables such as 'test'.",
		}},
		{" /Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java:10: Usage of System.out/err", &LinterOutput{
			File:    "/Users/zhouxiaoliang/Documents/qproject/prow/cmd/phony/examples/test.java",
			Line:    10,
			Column:  0,
			Message: "Usage of System.out/err",
		}},
		{"cdn-admin.v2/api/api_line.go:342 should replace loop with ret = append(ret, scope.EdgeNodes...)", nil},
	}

	for _, c := range tc {
		output, err := GeneralLineParser(c.input)
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

func TestIsEmpty(t *testing.T) {
	tcs := []struct {
		input    []string
		expected bool
	}{
		{[]string{}, true},
		{[]string{""}, true},
		{[]string{" "}, false},
		{[]string{"a"}, false},
	}
	for _, tc := range tcs {
		actual := IsEmpty(tc.input...)
		if actual != tc.expected {
			t.Errorf("expected: %v, got: %v", tc.expected, actual)
		}
	}
}
