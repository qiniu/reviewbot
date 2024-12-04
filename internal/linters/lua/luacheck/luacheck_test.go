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
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/x/xlog"
)

func TestParser(t *testing.T) {
	tc := []struct {
		input      []byte
		expected   map[string][]lint.LinterOutput
		unexpected []string
	}{
		{
			input: []byte(`
video/mp4/libs/mp4lib.lua:184:11: value assigned to variable mem_data is overwritten on line 202 before use
`),
			expected: map[string][]lint.LinterOutput{
				"video/mp4/libs/mp4lib.lua": {
					{
						File:    "video/mp4/libs/mp4lib.lua",
						Line:    184,
						Column:  11,
						Message: "value assigned to variable mem_data is overwritten on line 202 before use",
					},
				},
			},
			unexpected: nil,
		},
		{
			input: []byte(`
utils/jsonschema.lua:723:121: line is too long (142 > 120)
`),
			expected: map[string][]lint.LinterOutput{
				"utils/jsonschema.lua": {
					{
						File:    "utils/jsonschema.lua",
						Line:    723,
						Column:  121,
						Message: "line is too long (142 > 120)",
					},
				},
			},
			unexpected: nil,
		},
		{
			input: []byte(`
Total: 0 warnings / 0 errors in 0 files
`),
			expected:   map[string][]lint.LinterOutput{},
			unexpected: nil,
		},
		{
			input: []byte(`
Checking test/qtest_mgrconf.lua
`),
			expected:   map[string][]lint.LinterOutput{},
			unexpected: nil,
		},
		{
			input:      []byte(``),
			expected:   map[string][]lint.LinterOutput{},
			unexpected: nil,
		},
	}

	for _, c := range tc {
		got, unexpected := parser(xlog.New("UnitLuaCheckTest"), c.input)
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("parser(): %v, expected: %v", got, c.expected)
		}
		if !reflect.DeepEqual(unexpected, c.unexpected) {
			t.Errorf("parser(): %v, expected: %v", unexpected, c.unexpected)
		}
	}
}
