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

package notecheck

import (
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/lint"
)

func TestNoteCheckFile(t *testing.T) {
	tcs := []struct {
		name     string
		workdir  string
		filename string
		expected map[string][]lint.LinterOutput
		error    error
	}{
		{
			name:     "kinds_of_notes",
			workdir:  "testdata",
			filename: "note.go",
			expected: map[string][]lint.LinterOutput{
				"note.go": {
					{
						File:    "note.go",
						Line:    6,
						Column:  1,
						Message: NoteSuggestion,
					},
					{
						File:    "note.go",
						Line:    10,
						Column:  1,
						Message: NoteSuggestion,
					},
					{
						File:    "note.go",
						Line:    39,
						Column:  1,
						Message: NoteSuggestion,
					},
					{
						File:    "note.go",
						Line:    40,
						Column:  1,
						Message: NoteSuggestion,
					},
				},
			},
			error: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := noteCheckFile(tc.workdir, tc.filename)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("\nexpected: %v,\ngot: %v", tc.expected, actual)
			}
		})
	}
}
