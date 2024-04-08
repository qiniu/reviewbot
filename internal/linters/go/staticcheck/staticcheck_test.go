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

package staticcheck

import (
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/xlog"
)

func TestStaticcheckParse(t *testing.T) {
	tcs := []struct {
		name        string
		output      []byte
		expected    map[string][]linters.LinterOutput
		expectedErr error
	}{
		{
			name:   "normal case",
			output: []byte(`./internal/linters/go/staticcheck/staticcheck.go:26:2: SA1019: call of log.Printf copies lock value: log.Printf("Linter{Enable: %v, WorkDir: %v, Command: %v, Args: %v, ReportFormat: %v}", *l.Enable, l.WorkDir, l.Command, l.Args, l.ReportFormat) (staticcheck)`),
			expected: map[string][]linters.LinterOutput{
				"./internal/linters/go/staticcheck/staticcheck.go": {
					{
						Line:    26,
						Column:  2,
						Message: "SA1019: call of log.Printf copies lock value: log.Printf(\"Linter{Enable: %v, WorkDir: %v, Command: %v, Args: %v, ReportFormat: %v}\", *l.Enable, l.WorkDir, l.Command, l.Args, l.ReportFormat) (staticcheck)",
					},
				},
			},
			expectedErr: nil,
		},
		{
			name:        "empty output",
			output:      []byte(``),
			expected:    map[string][]linters.LinterOutput{},
			expectedErr: nil,
		},
		{
			name:        "go-zero custom tags should ignore",
			output:      []byte(`pkg/apis/xxx_2.go:4:16: unknown JSON option "default=abc" (SA5008)`),
			expected:    map[string][]linters.LinterOutput{},
			expectedErr: nil,
		},
		{
			name:   "other tags still need to be reported",
			output: []byte(`pkg/apis/xxx_2.go:4:16: unknown JSON option "other" (SA5008)`),
			expected: map[string][]linters.LinterOutput{
				"pkg/apis/xxx_2.go": {
					{
						Line:    4,
						Column:  16,
						Message: `unknown JSON option "other" (SA5008)`,
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			results, err := staticcheckParse(xlog.New(tc.name), tc.output)
			if err != tc.expectedErr {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
			if len(results) != len(tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, results)
			}
			for file, linterResults := range results {
				if len(linterResults) != len(tc.expected[file]) {
					t.Errorf("expected %v, got %v", tc.expected[file], linterResults)
				}
				for i, linter := range linterResults {
					if linter.Line != tc.expected[file][i].Line {
						t.Errorf("expected %v, got %v", tc.expected[file][i].Line, linter.Line)
					}
					if linter.Column != tc.expected[file][i].Column {
						t.Errorf("expected %v, got %v", tc.expected[file][i].Column, linter.Column)
					}
					if linter.Message != tc.expected[file][i].Message {
						t.Errorf("expected %v, got %v", tc.expected[file][i].Message, linter.Message)
					}
				}
			}
		})
	}
}
