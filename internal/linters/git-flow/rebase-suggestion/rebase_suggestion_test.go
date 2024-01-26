package rebase_suggestion

import (
	"bytes"
	"testing"
	"text/template"
)

func TestRebaseSuggestionTmpl(t *testing.T) {
	tmpl, err := template.New("rebase_suggestion").Parse(rebaseSuggestionTmpl)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	var rebaseSuggestion = RebaseSuggestion{
		Author:        "author",
		Flag:          rebaseSuggestionFlag,
		TargetCommits: []string{"commit1", "commit2"},
	}
	err = tmpl.Execute(&buf, rebaseSuggestion)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(buf.String())
}
