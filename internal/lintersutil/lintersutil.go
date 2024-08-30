package lintersutil

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/x/log"
)

// LimitJoin joins the strings in str with a newline separator until the length of the result is greater than length.
func LimitJoin(str []string, length int) string {
	var result string
	for _, s := range str {
		if strings.TrimSpace(s) == "" {
			continue
		}

		if len(result)+len(s) > length {
			break
		}

		result += s + "\n"
	}

	return result
}

func FileExists(path string) (absPath string, exist bool) {
	fileAbs, err := filepath.Abs(path)
	if err != nil {
		log.Warnf("failed to get absolute path of %s: %v", path, err)
		return "", false
	}

	_, err = os.Stat(fileAbs)
	if err != nil {
		return "", false
	}

	return fileAbs, true
}
