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
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/log"
	"github.com/reviewbot/config"
	"github.com/sirupsen/logrus"
	gitv2 "k8s.io/test-infra/prow/git/v2"

	// linters import
	_ "github.com/reviewbot/internal/linters/git-flow/commit-check"
	_ "github.com/reviewbot/internal/linters/go/staticcheck"
)

type options struct {
	port          int
	dryRun        bool
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
	o := gatherOptions()
	if err := o.Validate(); err != nil {
		log.Fatalf("invalid options: %v", err)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Llevel)
	log.SetOutputLevel(o.logLevel)

	if o.codeCacheDir != "" {
		if err := os.MkdirAll(o.codeCacheDir, 0755); err != nil {
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

	cfg, err := config.NewConfig(o.config)
	if err != nil {
		log.Fatalf("failed to create config: %v", err)
	}

	s := &Server{
		webhookSecret:    []byte(o.webhookSecret),
		gitClientFactory: v2,
		config:           cfg,
		accessToken:      o.accessToken,
		appID:            o.appID,
		appPrivateKey:    o.appPrivateKey,
	}

	mux := http.NewServeMux()
	mux.Handle("/", s)
	log.Infof("listening on port %d", o.port)

	// TODO: graceful shutdown
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", o.port), mux))
}
