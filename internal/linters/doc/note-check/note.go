package notecheck

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

// refer to https://pkg.go.dev/go/doc#Note
const linterName = "note-check"

func init() {
	linters.RegisterPullRequestHandler(linterName, noteCheckHandler)

	// TODO(CarlJi): can we check other languages?
	linters.RegisterLinterLanguages(linterName, []string{".go"})
}

// noteCheckHandler is the handler of the linter
// Check the notes in the code to see if they comply with the standard rules from
// https://pkg.go.dev/go/doc#Note
func noteCheckHandler(log *xlog.Logger, a linters.Agent) error {
	outputs := make(map[string][]linters.LinterOutput)

	for _, file := range a.PullRequestChangedFiles {
		fileName := file.GetFilename()
		// Only check go files
		if filepath.Ext(fileName) != ".go" {
			continue
		}

		output, err := noteCheckFile(a.LinterConfig.WorkDir, fileName)
		if err != nil {
			return err
		}

		if len(output) > 0 {
			for k, v := range output {
				if vv, ok := outputs[k]; ok {
					outputs[k] = append(vv, v...)
				} else {
					outputs[k] = v
				}
			}
		}
	}

	return linters.Report(log, a, outputs)
}

const NoteSuggestion = "A Note is recommended to use \"MARKER(uid): note body\" format."

func noteCheckFile(workdir, filename string) (map[string][]linters.LinterOutput, error) {
	path := filepath.Join(workdir, filename)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var output = make(map[string][]linters.LinterOutput)
	for _, cmts := range file.Comments {
		for _, cmt := range cmts.List {
			// comments with "/*" may have multiple lines
			lines := strings.Split(cmt.Text, "\n")
			for i, line := range lines {
				if !hasNonstandardNote(line) {
					continue
				}

				log.Debugf("non-standard note: %s, pos: %v", line, fset.Position(cmt.Pos()))

				v, ok := output[filename]
				if !ok {
					output[filename] = []linters.LinterOutput{
						{
							File:    filename,
							Line:    fset.Position(cmt.Pos()).Line + i,
							Column:  fset.Position(cmt.Pos()).Column,
							Message: NoteSuggestion,
						},
					}
				} else {
					v = append(v, linters.LinterOutput{
						File:    filename,
						Line:    fset.Position(cmt.Pos()).Line + i,
						Column:  fset.Position(cmt.Pos()).Column,
						Message: NoteSuggestion,
					})
					output[filename] = v
				}
			}
		}
	}
	return output, nil
}

var (
	standardNoteMarker    = `([A-Z][A-Z]+)\(([^)]+)\):.?`                           // MARKER(uid), MARKER at least 2 chars, uid at least 1 char
	standardNoteMarkerRx  = regexp.MustCompile(`^[ \t]*` + standardNoteMarker)      // MARKER(uid) at text start
	standardNoteCommentRx = regexp.MustCompile(`^/[/*][ \t]*` + standardNoteMarker) // MARKER(uid) at comment start

	nonstandardNoteMarker    = `([A-Z][A-Z]+):.?`                                         // General non-standard MARKER, MARKER at least 2 chars, plus colon
	nonstandardNoteMarkerRx  = regexp.MustCompile(`^[ \t]*` + nonstandardNoteMarker)      // MARKER: at text start
	nonstandardNoteCommentRx = regexp.MustCompile(`^/[/*][ \t]*` + nonstandardNoteMarker) // MARKER: at comment start
)

func hasNonstandardNote(comment string) bool {
	if comment == "" {
		return false
	}
	if nonstandardNoteCommentRx.MatchString(comment) && !standardNoteCommentRx.MatchString(comment) {
		return true
	}

	if nonstandardNoteMarkerRx.MatchString(comment) && !standardNoteMarkerRx.MatchString(comment) {
		return true
	}
	return false
}
