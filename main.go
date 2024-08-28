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
	"net"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/google/go-github/v57/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/logagent"
	"github.com/qiniu/reviewbot/internal/version"
	"github.com/qiniu/x/log"
	"github.com/sirupsen/logrus"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"

	// linters import
	_ "github.com/qiniu/reviewbot/internal/linters/c/cppcheck"
	_ "github.com/qiniu/reviewbot/internal/linters/doc/note-check"
	_ "github.com/qiniu/reviewbot/internal/linters/git-flow/commit-check"
	_ "github.com/qiniu/reviewbot/internal/linters/go/gofmt"
	_ "github.com/qiniu/reviewbot/internal/linters/go/golangci_lint"
	_ "github.com/qiniu/reviewbot/internal/linters/java/pmdcheck"
	_ "github.com/qiniu/reviewbot/internal/linters/java/stylecheck"
	_ "github.com/qiniu/reviewbot/internal/linters/lua/luacheck"
	_ "github.com/qiniu/reviewbot/internal/linters/shell/shellcheck"
)

type options struct {
	port          int
	dryRun        bool
	debug         bool
	logLevel      int
	accessToken   string
	webhookSecret string
	codeCacheDir  string
	config        string

	// support github app
	appID          int64
	installationID int64
	appPrivateKey  string
}

func (o options) Validate() error {
	if o.accessToken == "" && o.appID == 0 {
		return errors.New("either access-token or github app information should be provided")
	}

	if o.appID != 0 && o.appPrivateKey == "" {
		return errors.New("app-private-key is required when using github app")
	}

	if o.webhookSecret == "" {
		return errors.New("webhook-secret is required")
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
	fs.StringVar(&o.accessToken, "access-token", "", "personal access token")
	fs.StringVar(&o.webhookSecret, "webhook-secret", "", "webhook secret file")
	fs.StringVar(&o.codeCacheDir, "code-cache-dir", "/tmp", "code cache dir")
	fs.StringVar(&o.config, "config", "", "config file")
	fs.Int64Var(&o.appID, "app-id", 0, "github app id")
	fs.Int64Var(&o.installationID, "app-installation-id", 0, "github app installation id")
	fs.StringVar(&o.appPrivateKey, "app-private-key", "", "github app private key")
	fs.Parse(os.Args[1:])
	return o
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
		webhookSecret:    []byte(o.webhookSecret),
		gitClientFactory: v2,
		config:           cfg,
		accessToken:      o.accessToken,
		appID:            o.appID,
		appPrivateKey:    o.appPrivateKey,
		debug:            o.debug,
		la:               logagent.NewLogAgent(),
	}

	go s.initDockerRunner()
	mux := http.NewServeMux()
	mux.Handle("/", s)
	mux.Handle("/logs", HandleLog(s.la, logrus.WithField("handler", "/log")))
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
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", o.port), mux))
}

type logClient interface {
	GetLinterLog(linterName, id string) ([]byte, error)
}

func HandleLog(lc logClient, log *logrus.Entry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		linterName := r.URL.Query().Get("linter")

		logger := log.WithFields(logrus.Fields{"linter": linterName, "id": id})
		linterLog, err := lc.GetLinterLog(linterName, id)

		if err != nil {
			http.Error(w, fmt.Sprintf("Log not found: %v", err), http.StatusNotFound)
			return
		}

		if _, err = w.Write(linterLog); err != nil {
			logger.WithError(err).Warning("Error writing log.")
		}
	}
}
