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

package cppcheck

import (
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/x/xlog"
)

func TestParser(t *testing.T) {
	tc := []struct {
		input      string
		expected   map[string][]lint.LinterOutput
		unexpected []string
	}{
		{
			input: "'cppcheck_test.c:6:7: Array 'a[10]' accessed at index 10, which is out of bounds.'",
			expected: map[string][]lint.LinterOutput{
				"cppcheck_test.c": {
					{
						File:    "cppcheck_test.c",
						Line:    6,
						Column:  7,
						Message: "Array 'a[10]' accessed at index 10, which is out of bounds.",
					},
				},
			},
			unexpected: nil,
		},
		{
			input:      "''",
			expected:   map[string][]lint.LinterOutput{},
			unexpected: nil,
		},
	}

	for _, c := range tc {
		output, unexpected := parser(xlog.New("cppcheck"), []byte(c.input))
		if !reflect.DeepEqual(output, c.expected) {
			t.Errorf("expected: %v, got: %v", c.expected, output)
		}
		if !reflect.DeepEqual(unexpected, c.unexpected) {
			t.Errorf("expected: %v, got: %v", c.unexpected, unexpected)
		}
	}
}
