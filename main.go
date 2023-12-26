package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/cr-bot/config"
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
	gitv2 "k8s.io/test-infra/prow/git/v2"

	// linters import
	_ "github.com/cr-bot/linters/staticcheck"
)

type options struct {
	port          int
	dryRun        bool
	LogLevel      int
	accessToken   string
	webhookSecret string
	codeCacheDir  string
	config        string
}

func (o options) Validate() error {
	if o.accessToken == "" {
		return errors.New("access-token is required")
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
	fs.IntVar(&o.LogLevel, "log-level", 0, "log level")
	fs.StringVar(&o.accessToken, "access-token", "", "personal access token")
	fs.StringVar(&o.webhookSecret, "webhook-secret", "", "webhook secret file")
	fs.StringVar(&o.codeCacheDir, "code-cache-dir", "/tmp", "code cache dir")
	fs.StringVar(&o.config, "config", "", "config file")
	fs.Parse(os.Args[1:])
	return o
}

func main() {
	o := gatherOptions()
	if err := o.Validate(); err != nil {
		log.Fatalf("invalid options: %v", err)
	}

	log.SetOutputLevel(o.LogLevel)

	// TODO: support github app
	gc := github.NewClient(nil)
	if o.accessToken != "" {
		gc.WithAuthToken(o.accessToken)
	}

	if o.codeCacheDir != "" {
		if err := os.MkdirAll(o.codeCacheDir, 0755); err != nil {
			log.Fatalf("failed to create code cache dir: %v", err)
		}
	}

	opt := gitv2.ClientFactoryOpts{
		CacheDirBase: github.String(o.codeCacheDir),
		Persist:      github.Bool(true),
	}
	v2, err := gitv2.NewClientFactory(opt.Apply)
	if err != nil {
		log.Fatalf("failed to create git client factory: %v", err)
	}

	cfg, err := config.NewConfig(o.config)
	if err != nil {
		log.Fatalf("failed to create config: %v", err)
	}

	s := &Server{
		gc:               gc,
		webhookSecret:    []byte(o.webhookSecret),
		gitClientFactory: v2,
		config:           cfg,
	}

	mux := http.NewServeMux()
	mux.Handle("/", s)
	log.Infof("listening on port %d", o.port)

	// TODO: graceful shutdown
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", o.port), mux))
}
