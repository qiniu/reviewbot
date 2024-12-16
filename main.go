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

package main

import (
	"context"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/llm"
	"github.com/qiniu/reviewbot/internal/storage"
	"github.com/qiniu/reviewbot/internal/version"
	"github.com/qiniu/x/log"
	"github.com/sirupsen/logrus"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"

	// linters import
	_ "github.com/qiniu/reviewbot/internal/linters/c/cppcheck"
	_ "github.com/qiniu/reviewbot/internal/linters/doc/note-check"
	_ "github.com/qiniu/reviewbot/internal/linters/git-flow/commit"
	_ "github.com/qiniu/reviewbot/internal/linters/go/gofmt"
	_ "github.com/qiniu/reviewbot/internal/linters/go/golangci_lint"
	_ "github.com/qiniu/reviewbot/internal/linters/go/gomodcheck"
	_ "github.com/qiniu/reviewbot/internal/linters/java/pmdcheck"
	_ "github.com/qiniu/reviewbot/internal/linters/java/stylecheck"
	_ "github.com/qiniu/reviewbot/internal/linters/lua/luacheck"
	_ "github.com/qiniu/reviewbot/internal/linters/shell/shellcheck"
)

type serverOptions struct {
	port          int
	dryRun        bool
	debug         bool
	logLevel      int
	webhookSecret string
	codeCacheDir  string
	config        string

	// support gitlab
	gitLabPersonalAccessToken string
	gitLabHost                string

	// support github
	gitHubPersonalAccessToken string
	gitHubAppID               int64
	gitHubAppInstallationID   int64
	gitHubAppPrivateKey       string

	// log storage dir for local storage
	logDir string
	// s3 credential config
	S3CredentialsFile string
	// server addr which is used to generate the log view url
	// e.g. https://domain
	serverAddr string
	// kube config file
	kubeConfig string

	// llm related
	llmProvider  string
	llmModel     string
	llmServerURL string
	llmAPIKey    string
}

var (
	errAppNotSet       = errors.New("app-private-key is required when using github app")
	errWebHookNotSet   = errors.New("webhook-secret is required")
	errLLMKeyNotSet    = errors.New("llm api key is not set")
	errLLMServerNotSet = errors.New("llm model or server url is not set")
)

func (o serverOptions) Validate() error {
	if o.gitHubAppID != 0 && o.gitHubAppPrivateKey == "" {
		return errAppNotSet
	}

	if o.webhookSecret == "" {
		return errWebHookNotSet
	}

	if o.llmProvider != "" {
		switch o.llmProvider {
		case "openai":
			if o.llmAPIKey == "" {
				return errLLMKeyNotSet
			}
		case "ollama":
			if o.llmModel == "" || o.llmServerURL == "" {
				return errLLMServerNotSet
			}
		default:
			return llm.ErrUnsupportedProvider
		}
	}
	return nil
}

func gatherSeverOptions(fs *flag.FlagSet) serverOptions {
	o := serverOptions{}
	fs.IntVar(&o.port, "port", 8888, "port to listen on")
	fs.BoolVar(&o.dryRun, "dry-run", false, "dry run")
	fs.BoolVar(&o.debug, "debug", false, "debug mode")
	fs.IntVar(&o.logLevel, "log-level", 0, "log level")
	fs.StringVar(&o.webhookSecret, "webhook-secret", "", "webhook secret file")
	fs.StringVar(&o.codeCacheDir, "code-cache-dir", "/tmp", "code cache dir")
	fs.StringVar(&o.config, "config", "", "config file")
	fs.StringVar(&o.logDir, "log-dir", "/tmp", "log storage dir for local storage")
	fs.StringVar(&o.serverAddr, "server-addr", "", "server addr which is used to generate the log view url")
	fs.StringVar(&o.S3CredentialsFile, "s3-credentials-file", "", "File where s3 credentials are stored. For the exact format see http://xxxx/doc")
	fs.StringVar(&o.kubeConfig, "kube-config", "", "kube config file")

	// github related
	fs.StringVar(&o.gitHubPersonalAccessToken, "github.personal-access-token", "", "personal github access token")
	fs.Int64Var(&o.gitHubAppID, "github.app-id", 0, "github app id")
	fs.Int64Var(&o.gitHubAppInstallationID, "github.app-installation-id", 0, "github app installation id")
	fs.StringVar(&o.gitHubAppPrivateKey, "github.app-private-key", "", "github app private key")
	// gitlab related
	fs.StringVar(&o.gitLabPersonalAccessToken, "gitlab.personal-access-token", "", "personal gitlab access token")
	fs.StringVar(&o.gitLabHost, "gitlab.host", "", "gitlab server")

	// llm related
	fs.StringVar(&o.llmProvider, "llm.provider", "", "llm provider")
	fs.StringVar(&o.llmModel, "llm.model", "", "llm model")
	fs.StringVar(&o.llmServerURL, "llm.server-url", "", "llm server url")
	fs.StringVar(&o.llmAPIKey, "llm.api-key", "", "llm api key")

	err := fs.Parse(os.Args[3:])
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	return o
}

type cliOptions struct {
	logDir string
}

func gatherCLIOptions(fs *flag.FlagSet) cliOptions {
	o := cliOptions{}
	fs.StringVar(&o.logDir, "logDir", "", "log dir")
	err := fs.Parse(os.Args[3:])
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	return o
}

func (o cliOptions) Validate() error {
	return nil
}

var viewTemplate = template.Must(template.New("view").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LOG VIEW</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            margin: 0;
            padding: 20px;
            background: #f5f7f9;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        .panel {
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .panel-header {
            padding: 12px 20px;
            background: #f8f9fa;
            border-bottom: 1px solid #e9ecef;
            border-radius: 8px 8px 0 0;
            display: flex;
            align-items: center;
            cursor: pointer;
            user-select: none;
        }
        .panel-title {
            font-size: 16px;
            font-weight: 600;
            color: #2d3748;
            margin: 0;
            display: flex;
            align-items: center;
        }
        .toggle-icon {
            display: inline-block;
            margin-right: 8px;
            transition: transform 0.2s;
        }
        .toggle-icon::before {
            content: "▼";
            font-size: 12px;
        }
        .panel-header.collapsed .toggle-icon::before {
            content: "▶";
        }
        .panel-header.collapsed + .panel-content {
            display: none;
        }
        .panel-timestamp {
            margin-left: auto;
            color: #718096;
            font-size: 14px;
        }
        .panel-content {
            padding: 20px;
            overflow-x: auto;
        }
        .script-content {
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            font-size: 14px;
            line-height: 1.5;
            margin: 0;
            white-space: pre-wrap;
            word-wrap: break-word;
            background: #282c34;
            color: #abb2bf;
            padding: 15px;
            border-radius: 4px;
        }
        .log-content {
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            font-size: 14px;
            line-height: 1.5;
            margin: 0;
            white-space: pre-wrap;
            word-wrap: break-word;
        }
    </style>
    <script>
        function togglePanel(header) {
            header.classList.toggle('collapsed');
        }
    </script>
</head>
<body>
    <div class="container">
        <!-- script panel -->
        <div class="panel">
            <div class="panel-header" onclick="togglePanel(this)">
                <h2 class="panel-title">
                    <span class="toggle-icon"></span>
                    script
                </h2>
                <span class="panel-timestamp">{{.ScriptTimestamp}}</span>
            </div>
            <div class="panel-content">
                <pre class="script-content">{{.Script}}</pre>
            </div>
        </div>

        <!-- output panel -->
        <div class="panel">
            <div class="panel-header">
                <h2 class="panel-title">
                    <span class="toggle-icon"></span>
                    output
                </h2>
                <span class="panel-timestamp">{{.OutputTimestamp}}</span>
            </div>
            <div class="panel-content">
                <pre class="log-content">{{.Output}}</pre>
            </div>
        </div>
    </div>
</body>
</html>
`))

func (s *Server) HandleView(w http.ResponseWriter, r *http.Request) {
	logID := strings.TrimPrefix(r.URL.Path, "/view/")
	if logID == "" {
		http.Error(w, "log ID is required", http.StatusBadRequest)
		return
	}

	// get log content from storage
	content, err := s.storage.Read(r.Context(), logID)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			http.Error(w, "empty log", http.StatusNotFound)
		} else {
			log.Errorf("failed to read log content: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// parse log content
	logContent := parseContent(string(content))

	// render page with template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := viewTemplate.Execute(w, logContent); err != nil {
		log.Errorf("failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

type LogContent struct {
	Script          string
	Output          string
	ScriptTimestamp string
	OutputTimestamp string
}

func parseContent(content string) LogContent {
	lines := strings.Split(content, "\n")
	var result LogContent
	var currentSection string

	for _, line := range lines {
		if strings.Contains(line, "run script:") {
			currentSection = "script"
			if timestamp := extractTimestamp(line); timestamp != "" {
				result.ScriptTimestamp = timestamp
			}
			continue
		}
		if strings.Contains(line, "output:") {
			currentSection = "output"
			if timestamp := extractTimestamp(line); timestamp != "" {
				result.OutputTimestamp = timestamp
			}
			continue
		}

		if currentSection == "script" {
			result.Script += line + "\n"
		} else if currentSection == "output" {
			result.Output += line + "\n"
		}
	}

	return result
}

func extractTimestamp(line string) string {
	// format: [2024-12-06T17:04:35+08:00]
	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start >= 0 && end > start {
		timestamp := line[start+1 : end]
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	return ""
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: reviewbot server-run")
		return
	}

	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	switch os.Args[1] {
	case "server-run":
		o := gatherSeverOptions(fs)
		serverMode(o)

	case "cli-run":
		o := gatherCLIOptions(fs)
		cliMode(o)

	case "version", "-v", "--version":
		fmt.Println(version.Version())

	default:
		fmt.Println("Usage: reviewbot [command] [flags]")
	}

}

func serverMode(o serverOptions) {
	if err := o.Validate(); err != nil {
		log.Fatalf("invalid options: %v", err)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Llevel)
	log.SetOutputLevel(o.logLevel)
	if o.codeCacheDir != "" {
		if err := os.MkdirAll(o.codeCacheDir, 0o755); err != nil {
			log.Fatalf("failed to create code cache dir: %v", err)
		}
	}

	opt := gitv2.ClientFactoryOpts{
		CacheDirBase: github.String(o.codeCacheDir),
		Persist:      github.Bool(true),
		UseSSH:       github.Bool(true),
	}
	v2, err := gitv2.NewClientFactory(opt.Apply)
	if err != nil {
		log.Fatalf("failed to create git client factory: %v", err)
	}

	logrus.SetLevel(logrus.DebugLevel)

	var cfg config.Config
	if o.config != "" {
		cfg, err = config.NewConfig(o.config)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
	}

	modelConfig := llm.Config{
		Provider:  o.llmProvider,
		APIKey:    o.llmAPIKey,
		Model:     o.llmModel,
		ServerURL: o.llmServerURL,
	}

	s := &Server{
		webhookSecret:             []byte(o.webhookSecret),
		gitClientFactory:          v2,
		config:                    cfg,
		debug:                     o.debug,
		serverAddr:                o.serverAddr,
		repoCacheDir:              o.codeCacheDir,
		kubeConfig:                o.kubeConfig,
		gitLabHost:                o.gitLabHost,
		gitLabPersonalAccessToken: o.gitLabPersonalAccessToken,
		modelConfig:               modelConfig,
	}

	// github access token
	if o.gitHubPersonalAccessToken != "" {
		s.gitHubAccessTokenAuth = &GitHubAccessTokenAuth{
			AccessToken: o.gitHubPersonalAccessToken,
		}
	}

	// github app
	if o.gitHubAppID != 0 {
		s.gitHubAppAuth = &GitHubAppAuth{
			AppID:          o.gitHubAppID,
			InstallationID: o.gitHubAppInstallationID,
			PrivateKeyPath: o.gitHubAppPrivateKey,
		}
	}

	go s.initDockerRunner()
	go s.initKubernetesRunner()
	s.initCustomLinters()
	if o.llmProvider != "" {
		s.initLLMModel()
	}

	if o.S3CredentialsFile != "" {
		s.storage, err = storage.NewS3Storage(o.S3CredentialsFile)
		if err != nil {
			log.Fatalf("failed to create s3 storage: %v", err)
		}
	} else {
		// Fallback to local storage
		s.storage, err = storage.NewLocalStorage(o.logDir)
		if err != nil {
			log.Fatalf("failed to create local storage: %v", err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/", s)
	mux.Handle("/view/", http.HandlerFunc(s.HandleView))
	mux.Handle("/metrics", promhttp.Handler())
	log.Infof("listening on port %d", o.port)

	debugMux := http.NewServeMux()
	debugMux.HandleFunc("/debug/pprof/", pprof.Index)
	debugMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	debugMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	debugMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	debugMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	debugMux.Handle("/debug/vars", http.HandlerFunc(expvar.Handler().ServeHTTP))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to listen: %v\n", err)
	}
	defer listener.Close()
	log.Infof("debug port running in: %s\n", listener.Addr().String())
	go func() {
		log.Fatal(http.Serve(listener, debugMux))
	}()
	// TODO(CarlJi): graceful shutdown
	//nolint:gocritic
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", o.port), mux))
}

func cliMode(o cliOptions) {
	if err := o.Validate(); err != nil {
		log.Fatalf("invalid options: %v", err)
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Llevel)
	log.SetOutputLevel(2)
	inputPath := os.Args[2]
	processForCLI(context.Background(), inputPath, o)

}
