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

package luacheck

import (
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestFormatLuaCheckLine(t *testing.T) {
	tc := []struct {
		input    string
		expected *linters.LinterOutput
	}{
		{" video/mp4/libs/mp4lib.lua:184:11: value assigned to variable mem_data is overwritten on line 202 before use", &linters.LinterOutput{
			File:    "video/mp4/libs/mp4lib.lua",
			Line:    184,
			Column:  11,
			Message: "value assigned to variable mem_data is overwritten on line 202 before use",
		}},
		{" utils/jsonschema.lua:723:121: line is too long (142 > 120)", &linters.LinterOutput{
			File:    "utils/jsonschema.lua",
			Line:    723,
			Column:  121,
			Message: "line is too long (142 > 120)",
		}},
		{" utils/httpc/http_simple.lua:24:1: setting read-only global variable _VERSION", &linters.LinterOutput{
			File:    "utils/httpc/http_simple.lua",
			Line:    24,
			Column:  1,
			Message: "setting read-only global variable _VERSION",
		}},
		{" test/qtest_access.lua:1220:1: inconsistent indentation (SPACE followed by TAB)", &linters.LinterOutput{
			File:    "test/qtest_access.lua",
			Line:    1220,
			Column:  1,
			Message: "inconsistent indentation (SPACE followed by TAB)",
		}},
		{"Checking test/qtest_mgrconf.lua", nil},
	}

	for _, c := range tc {
		output, err := luacheckLineParser(c.input)
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
