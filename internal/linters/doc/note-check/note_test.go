package notecheck

import (
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/internal/linters"
)

func TestNoteCheckFile(t *testing.T) {
	tcs := []struct {
		name     string
		workdir  string
		filename string
		expected map[string][]linters.LinterOutput
		error    error
	}{
		{
			name:     "kinds_of_notes",
			workdir:  "testdata",
			filename: "note.go",
			expected: map[string][]linters.LinterOutput{
				"note.go": []linters.LinterOutput{
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
