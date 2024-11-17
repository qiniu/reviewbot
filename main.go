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
	"errors"
	"expvar"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"

	"github.com/google/go-github/v57/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qiniu/reviewbot/config"
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

type options struct {
	port              int
	dryRun            bool
	debug             bool
	logLevel          int
	gitHubAccessToken string
	gitLabAccessToken string
	gitLabHost        string
	webhookSecret     string
	codeCacheDir      string
	config            string

	// support github app
	appID          int64
	installationID int64
	appPrivateKey  string

	// log storage dir for local storage
	logDir string
	// s3 credential config
	S3CredentialsFile string
	// server addr which is used to generate the log view url
	// e.g. https://domain
	serverAddr string
	// kube config file
	kubeConfig string
}

var (
	errGitlabAccessTokenNotSet = errors.New("either access-token or github app information should be provided")
	errAppNotSet               = errors.New("app-private-key is required when using github app")
	errWebHookNotSet           = errors.New("webhook-secret is required")
)

func (o options) Validate() error {
	if o.gitHubAccessToken == "" && o.appID == 0 {
		return errGitlabAccessTokenNotSet
	}
	if o.appID != 0 && o.appPrivateKey == "" {
		return errAppNotSet
	}
	if o.webhookSecret == "" {
		return errWebHookNotSet
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.IntVar(&o.port, "port", 8888, "port to listen on")
	fs.BoolVar(&o.dryRun, "dry-run", false, "dry run")
	fs.BoolVar(&o.debug, "debug", false, "debug mode")
	fs.IntVar(&o.logLevel, "log-level", 0, "log level")
	fs.StringVar(&o.gitHubAccessToken, "github-access-token", "", "personal github access token")
	fs.StringVar(&o.gitLabAccessToken, "gitlab-access-token", "", "personal gitlab access token")
	fs.StringVar(&o.gitLabHost, "gitlab-host", "", "gitlab server")
	fs.StringVar(&o.webhookSecret, "webhook-secret", "", "webhook secret file")
	fs.StringVar(&o.codeCacheDir, "code-cache-dir", "/tmp", "code cache dir")
	fs.StringVar(&o.config, "config", "", "config file")
	fs.Int64Var(&o.appID, "app-id", 0, "github app id")
	fs.Int64Var(&o.installationID, "app-installation-id", 0, "github app installation id")
	fs.StringVar(&o.appPrivateKey, "app-private-key", "", "github app private key")
	fs.StringVar(&o.logDir, "log-dir", "/tmp", "log storage dir for local storage")
	fs.StringVar(&o.serverAddr, "server-addr", "", "server addr which is used to generate the log view url")
	fs.StringVar(&o.S3CredentialsFile, "s3-credentials-file", "", "File where s3 credentials are stored. For the exact format see http://xxxx/doc")
	fs.StringVar(&o.kubeConfig, "kube-config", "", "kube config file")

	err := fs.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	return o
}

var viewTemplate = template.Must(template.New("view").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body {
            font-family: monospace;
            line-height: 1.4;
            margin: 0;
            padding: 0;
        }
        pre {
            margin: 0;
            padding: 0;
            white-space: pre-wrap;
            word-wrap: break-word;
        }
        .container {
            max-width: 100%;
            margin: 0;
            padding: 0 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <pre>{{.Content}}</pre>
    </div>
</body>
</html>
`))

func (s *Server) HandleView(w http.ResponseWriter, r *http.Request) {
	path, err := url.PathUnescape(r.URL.Path[len("/view/"):])
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	contents, err := s.storage.Read(r.Context(), path)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			http.Error(w, "empty log", http.StatusNotFound)
		} else {
			log.Errorf("Error reading file: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := struct {
		Content string
	}{
		Content: string(contents),
	}

	if err := viewTemplate.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println(version.Version())
		return
	}
	o := gatherOptions()
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

	s := &Server{
		webhookSecret:     []byte(o.webhookSecret),
		gitClientFactory:  v2,
		config:            cfg,
		gitHubAccessToken: o.gitHubAccessToken,
		gitLabAccessToken: o.gitLabAccessToken,
		appID:             o.appID,
		appPrivateKey:     o.appPrivateKey,
		debug:             o.debug,
		serverAddr:        o.serverAddr,
		repoCacheDir:      o.codeCacheDir,
		kubeConfig:        o.kubeConfig,
		gitLabHost:        o.gitLabHost,
	}

	go s.initDockerRunner()
	go s.initKubernetesRunner()
	s.initCustomLinters()

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
