package stylecheck

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/xlog"
)

const linterName = "stylecheck"

const styleJarURL = "https://github.com/checkstyle/checkstyle/releases/download/checkstyle-10.17.0/checkstyle-10.17.0-all.jar"
const localStyleJar = "/checkstyle.jar"
const styleRuleURL = "https://raw.githubusercontent.com/checkstyle/checkstyle/master/src/main/resources/sun_checks.xml"

func init() {
	linters.RegisterPullRequestHandler(linterName, stylecheckHandler)
	linters.RegisterLinterLanguages(linterName, []string{".java"})
}

func stylecheckHandler(slog *xlog.Logger, a linters.Agent) error {
	var javaFiles []string
	rulePath := a.LinterConfig.ConfigPath
	for _, arg := range a.PullRequestChangedFiles {
		if strings.HasSuffix(arg.GetFilename(), ".java") {
			javaFiles = append(javaFiles, arg.GetFilename())
		}
	}
	if len(javaFiles) == 0 {
		return nil
	}
	checkrulePath, checkerr := styleRuleCheck(a.LinterConfig.WorkDir)(slog, rulePath)
	if checkerr != nil {
		slog.Errorf("style rule check failed: %v", checkerr)
		return checkerr
	}
	if linters.IsEmpty(a.LinterConfig.Args...) {
		args := append([]string{}, "-jar", localStyleJar)
		args = append(args, javaFiles...)
		args = append(args, "-c", checkrulePath)
		a.LinterConfig.Args = args
	}

	a.LinterConfig.Command = []string{"java"}

	return linters.GeneralHandler(slog, a, linters.ExecRun, stylecheckParser(a.LinterConfig.WorkDir))
}

func stylecheckParser(codedir string) func(slog *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
	return func(slog *xlog.Logger, output []byte) (map[string][]linters.LinterOutput, []string) {
		var lineParse = func(line string) (*linters.LinterOutput, error) {
			// stylecheck will output lines starting with ' 开始检查 ' or '检查结束 ' or 'stylecheck info'
			// which are no meaningful for the reviewbot scenario, so we discard them Starting audit done.
			if strings.Contains(strings.ToLower(line), "checkstyle") || strings.HasPrefix(line, "Starting audit") || strings.HasPrefix(line, "Audit done") || strings.HasPrefix(line, "开始") || strings.HasPrefix(line, "检查") || line == "" {
				return nil, nil
			}
			line = strings.ReplaceAll(line, "[ERROR]", "")
			line = strings.ReplaceAll(line, codedir+"/", "")
			return linters.GeneralLineParser(strings.TrimLeft(line, " "))
		}
		return linters.Parse(slog, output, lineParse)
	}
}

func stylecheckJar(slog *xlog.Logger) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	jarfilepath := filepath.Join(dir, localStyleJar)
	_, exist := lintersutil.FileExists(jarfilepath)
	if !exist {
		res, err := getFileFromURL(slog, styleJarURL)
		if err != nil {
			return "", err
		}
		return res, nil
	}
	return jarfilepath, nil
}

func getFileFromURL(slog *xlog.Logger, url string) (string, error) {
	absPath, err := os.Getwd()
	if err != nil {
		return "", err
	}
	downloaddir := filepath.Join(absPath, "tmp")
	newfile := filepath.Join(downloaddir, filepath.Base(url))
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	merr := os.MkdirAll(downloaddir, os.ModePerm)
	if merr != nil {
		return "", merr
	}
	f, err := os.Create(newfile)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, res.Body)
	defer res.Body.Close()
	if err != nil {
		slog.Errorf("the file saving   encountered an error: %v", err)
		return "", err
	}
	return newfile, nil
}

func styleRuleCheck(codedir string) func(slog *xlog.Logger, styleConf string) (string, error) {
	return func(slog *xlog.Logger, styleConf string) (string, error) {
		absPath, err := os.Getwd()
		if err != nil {
			return "", err
		}
		tmpnewfile := filepath.Join(absPath, "tmp", filepath.Base(styleRuleURL))
		if styleConf == "" {
			absfilepath, _ := lintersutil.FileExists(tmpnewfile)
			if absfilepath != "" {
				return absfilepath, nil
			}
			downloadfilepath, err := getFileFromURL(slog, styleRuleURL)
			if err != nil {
				slog.Errorf("the style rule file download faild: %v", err)
				return "", err
			}
			return downloadfilepath, nil
		}
		if strings.HasPrefix(styleConf, "http://") || strings.HasPrefix(styleConf, "https://") {
			downloadfilepath, err := getFileFromURL(slog, styleRuleURL)
			if err != nil {
				slog.Errorf("the style rule file download faild: %v", err)
				return "", err
			}
			return downloadfilepath, nil
		}
		absfilepath, exist := lintersutil.FileExists(styleConf)
		if exist {
			return absfilepath, nil
		}
		rulefilepathcode := filepath.Join(codedir, styleConf)
		absfilepathcode, existcode := lintersutil.FileExists(rulefilepathcode)
		if existcode {
			return absfilepathcode, nil
		}
		return "", errors.New("the style rule file not exist")
	}
}
