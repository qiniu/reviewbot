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
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v57/github"
	"github.com/google/uuid"
	"github.com/gregjones/httpcache"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/runner"
	"github.com/qiniu/reviewbot/internal/storage"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"
)

type Server struct {
	gitClientFactory gitv2.ClientFactory
	config           config.Config

	// server addr which is used to generate the log view url
	// e.g. https://domain
	serverAddr string

	dockerRunner runner.Runner
	storage      storage.Storage

	webhookSecret []byte

	// support developer access token model
	accessToken string
	// support github app model
	appID         int64
	appPrivateKey string

	debug bool
}

func (s *Server) initDockerRunner() {
	var images []string
	for _, customConfig := range s.config.CustomConfig {
		for _, linter := range customConfig {
			if linter.DockerAsRunner != "" {
				images = append(images, linter.DockerAsRunner)
			}
		}
	}

	if len(images) > 0 {
		dockerRunner, err := runner.NewDockerRunner(nil)
		if err != nil {
			log.Fatalf("failed to init docker runner: %v", err)
		}

		s.dockerRunner = dockerRunner
	}

	if s.dockerRunner == nil {
		return
	}

	go func() {
		ctx := context.Background()
		for _, image := range images {
			log.Infof("pulling image %s", image)
			linterConfig := &config.Linter{DockerAsRunner: image}
			if err := s.dockerRunner.Prepare(ctx, linterConfig); err != nil {
				log.Errorf("failed to pull image %s: %v", image, err)
			}
		}
	}()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventGUID := github.DeliveryID(r)
	if len(eventGUID) > 12 {
		// limit the length of eventGUID to 12
		eventGUID = eventGUID[len(eventGUID)-12:]
	}
	ctx := context.WithValue(context.Background(), config.EventGUIDKey, eventGUID)
	log := xlog.New(ctx.Value(config.EventGUIDKey).(string))

	payload, err := github.ValidatePayload(r, s.webhookSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Errorf("parse webhook failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprint(w, "Event received. Have a nice day.")

	switch event := event.(type) {
	case *github.PullRequestEvent:
		go func() {
			if err := s.processPullRequestEvent(ctx, event); err != nil {
				log.Errorf("process pull request event: %v", err)
			}
		}()
	case *github.CheckRunEvent:
		go func() {
			if err := s.processCheckRunRequestEvent(ctx, event); err != nil {
				log.Errorf("process check run request event: %v", err)
			}
		}()
	case *github.CheckSuiteEvent:
		go func() {
			if err := s.processCheckSuiteEvent(ctx, event); err != nil {
				log.Errorf("process check run request event: %v", err)
			}
		}()
	default:
		log.Debugf("skipping event type %s\n", github.WebHookType(r))
	}
}

func (s *Server) processPullRequestEvent(ctx context.Context, event *github.PullRequestEvent) error {
	if event.GetAction() != "opened" && event.GetAction() != "reopened" && event.GetAction() != "synchronize" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}

	return s.handle(ctx, event)
}

func (s *Server) processCheckRunRequestEvent(ctx context.Context, event *github.CheckRunEvent) error {
	if event.GetAction() != "rerequested" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}
	headSha := event.GetCheckRun().GetHeadSHA()
	org := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	installationID := event.GetInstallation().GetID()
	plist, err := linters.FilterPullRequestsWithCommit(ctx, s.GithubClient(installationID), org, repo, headSha)
	if err != nil {
		log.Errorf("Filter pullreqeust fail  %v\n", err)
		return nil
	}
	if len(plist) == 0 {
		log.Errorf("Filter pullreqeust emmpty  ")
		return nil
	}
	pevent := github.PullRequestEvent{}
	pevent.Repo = event.GetRepo()
	pevent.PullRequest = plist[0]
	pevent.Number = plist[0].Number
	pevent.Installation = event.GetInstallation()
	return s.handle(ctx, &pevent)
}

func (s *Server) processCheckSuiteEvent(ctx context.Context, event *github.CheckSuiteEvent) error {
	if event.GetAction() != "rerequested" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}
	headSha := event.GetCheckSuite().GetHeadSHA()
	event.GetCheckSuite()
	org := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	installationID := event.GetInstallation().GetID()
	plist, err := linters.FilterPullRequestsWithCommit(ctx, s.GithubClient(installationID), org, repo, headSha)
	if err != nil {
		log.Errorf("Filter pullreqeust fail  %v\n", err)
		return nil
	}
	pevent := github.PullRequestEvent{}
	pevent.Repo = event.GetRepo()
	pevent.Number = plist[0].Number
	pevent.PullRequest = plist[0]
	pevent.Installation = event.GetInstallation()
	return s.handle(ctx, &pevent)
}

func (s *Server) handle(ctx context.Context, event *github.PullRequestEvent) error {
	var (
		num     = event.GetPullRequest().GetNumber()
		org     = event.GetRepo().GetOwner().GetLogin()
		repo    = event.GetRepo().GetName()
		orgRepo = org + "/" + repo
	)
	log := xlog.New(ctx.Value(config.EventGUIDKey).(string))

	installationID := event.GetInstallation().GetID()
	log.Infof("processing pull request %d, (%v/%v), installationID: %d\n", num, org, repo, installationID)

	pullRequestAffectedFiles, response, err := linters.ListPullRequestsFiles(ctx, s.GithubClient(installationID), org, repo, num)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		log.Errorf("list files failed: %v", response)
		return fmt.Errorf("list files failed: %v", response)
	}
	log.Infof("found %d files affected by pull request %d\n", len(pullRequestAffectedFiles), num)

	repoPath, err := prepareRepoDir(org, repo, num)
	if err != nil {
		return fmt.Errorf("failed to prepare repo dir: %w", err)
	}

	r, err := s.gitClientFactory.ClientForWithRepoOpts(org, repo, gitv2.RepoOpts{
		CopyTo: repoPath,
	})
	if err != nil {
		log.Errorf("failed to create git client: %v", err)
		return err
	}

	if err := r.CheckoutPullRequest(num); err != nil {
		log.Errorf("failed to checkout pull request %d: %v", num, err)
		return err
	}

	gitModulesFile := path.Join(r.Directory(), ".gitmodules")
	_, err = os.Stat(gitModulesFile)
	if err == nil {
		log.Info("git pull submodule in progress")
		cmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
		cmd.Dir = r.Directory()
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Errorf("error when git pull submodule, marked and continue, details :%v ", err)
		}
		log.Infof("git pull submodule output: %s ", out)
	} else {
		log.Infof("no .gitmodules file in repo %s", repo)
	}

	defer func() {
		if s.debug {
			return // do not remove the repository in debug mode
		}
		err := r.Clean()
		if err != nil {
			log.Errorf("failed to remove the repository , err : %v", err)
		}
	}()

	for name, fn := range linters.TotalPullRequestHandlers() {
		linterConfig := s.config.Get(org, repo, name)

		// skip linter if it is disabled
		if linterConfig.Enable != nil && !*linterConfig.Enable {
			continue
		}

		// set workdir
		if linterConfig.WorkDir != "" {
			linterConfig.WorkDir = r.Directory() + "/" + linterConfig.WorkDir
		} else {
			linterConfig.WorkDir = r.Directory()
		}

		log.Infof("[%s] config on repo %v: %+v", name, orgRepo, linterConfig)

		agent := linters.Agent{
			GithubClient:            s.GithubClient(installationID),
			LinterConfig:            linterConfig,
			GitClient:               s.gitClientFactory,
			PullRequestEvent:        *event,
			PullRequestChangedFiles: pullRequestAffectedFiles,
			LinterName:              name,
			RepoDir:                 r.Directory(),
			Context:                 ctx,
			ID:                      uuid.New().String(),
		}

		if !linters.LinterRelated(name, agent) {
			log.Infof("[%s] linter is not related to the PR, skipping", name)
			continue
		}

		r := runner.NewLocalRunner()
		if linterConfig.DockerAsRunner != "" {
			r = s.dockerRunner
		}
		agent.Runner = r
		agent.Storage = s.storage
		agent.GenLogKey = func() string {
			return fmt.Sprintf("%s/%s/%s", agent.LinterName, agent.PullRequestEvent.Repo.GetFullName(), agent.ID)
		}
		agent.GenLogViewUrl = func() string {
			// if serverAddr is not provided, return empty string
			if s.serverAddr == "" {
				return ""
			}
			return s.serverAddr + "/view/" + agent.GenLogKey()
		}

		if err := fn(ctx, agent); err != nil {
			log.Errorf("failed to run linter: %v", err)
			// continue to run other linters
			continue
		}
	}

	return nil
}

func (s *Server) githubAppClient(installationID int64) *github.Client {
	tr, err := ghinstallation.NewKeyFromFile(httpcache.NewMemoryCacheTransport(), s.appID, installationID, s.appPrivateKey)
	if err != nil {
		log.Fatalf("failed to create github app transport: %v", err)
	}
	return github.NewClient(&http.Client{Transport: tr})
}

func (s *Server) githubAccessTokenClient() *github.Client {
	gc := github.NewClient(httpcache.NewMemoryCacheTransport().Client())
	gc.WithAuthToken(s.accessToken)
	return gc
}

// GithubClient returns a github client
func (s *Server) GithubClient(installationID int64) *github.Client {
	if s.accessToken != "" {
		return s.githubAccessTokenClient()
	}
	return s.githubAppClient(installationID)
}

func prepareRepoDir(org, repo string, num int) (string, error) {
	var parentDir string
	if runtime.GOOS == "darwin" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home dir: %w", err)
		}
		parentDir = filepath.Join(homeDir, "reviewbot-code")
	} else {
		parentDir = filepath.Join("/tmp", "reviewbot-code")
	}

	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create parent dir: %w", err)
	}

	repoPath, err := os.MkdirTemp(parentDir, fmt.Sprintf("%s-%s-%d-", org, repo, num))
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	return repoPath, nil
}
