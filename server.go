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
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v57/github"
	"github.com/gregjones/httpcache"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/linters"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/reviewbot/internal/runner"
	"github.com/qiniu/reviewbot/internal/storage"
	"github.com/qiniu/x/log"
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

	repoCacheDir string
}

func (s *Server) initDockerRunner() {
	var images []string
	for _, customConfig := range s.config.CustomConfig {
		for _, linter := range customConfig.Linters {
			if linter.DockerAsRunner.Image != "" {
				images = append(images, linter.DockerAsRunner.Image)
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
			s.pullImageWithRetry(ctx, image)
		}
	}()
}

func (s *Server) pullImageWithRetry(ctx context.Context, image string) {
	maxRetries := 5
	baseDelay := time.Second * 2
	maxDelay := time.Minute * 2

	for attempt := 0; ; attempt++ {
		log.Infof("Attempting to pull image %s (attempt %d)", image, attempt+1)

		linterConfig := &config.Linter{
			DockerAsRunner: config.DockerAsRunner{
				Image: image,
			},
		}

		err := s.dockerRunner.Prepare(ctx, linterConfig)
		if err == nil {
			log.Infof("Successfully pulled image %s", image)
			return
		}

		if attempt >= maxRetries {
			log.Errorf("Failed to pull image %s after %d attempts: %v", image, maxRetries, err)
			return
		}

		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		delay += time.Duration(rand.Int63n(int64(delay) / 2))
		if delay > maxDelay {
			delay = maxDelay
		}

		log.Warnf("Failed to pull image %s: %v. Retrying in %v...", image, err, delay)
		time.Sleep(delay)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventGUID := github.DeliveryID(r)
	if len(eventGUID) > 12 {
		// limit the length of eventGUID to 12
		eventGUID = eventGUID[len(eventGUID)-12:]
	}
	ctx := context.WithValue(context.Background(), lintersutil.EventGUIDKey, eventGUID)
	log := lintersutil.FromContext(ctx)

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
	log := lintersutil.FromContext(ctx)
	if event.GetAction() != "opened" && event.GetAction() != "reopened" && event.GetAction() != "synchronize" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}

	return s.handle(ctx, event)
}

func (s *Server) processCheckRunRequestEvent(ctx context.Context, event *github.CheckRunEvent) error {
	log := lintersutil.FromContext(ctx)
	if event.GetAction() != "rerequested" {
		log.Debugf("Skipping action %s for check run event", event.GetAction())
		return nil
	}

	headSHA := event.GetCheckRun().GetHeadSHA()
	repo := event.GetRepo()
	org := repo.GetOwner().GetLogin()
	repoName := repo.GetName()
	installationID := event.GetInstallation().GetID()

	client := s.GithubClient(installationID)
	prs, err := linters.FilterPullRequestsWithCommit(ctx, client, org, repoName, headSHA)
	if err != nil {
		log.Errorf("failed to filter pull requests: %v", err)
		return nil
	}

	if len(prs) == 0 {
		log.Errorf("No pull requests found for commit SHA: %s", headSHA)
		return nil
	}

	for _, pr := range prs {
		log.Infof("try to reprocessing pull request %d, (%v/%v), installationID: %d\n", pr.GetNumber(), org, repo, installationID)
		event := &github.PullRequestEvent{
			Repo:         repo,
			PullRequest:  pr,
			Number:       pr.Number,
			Installation: event.GetInstallation(),
		}

		if err := s.handle(ctx, event); err != nil {
			log.Errorf("failed to handle pull request event: %v", err)
			// continue to handle other pull requests
		}
	}

	return nil
}

func (s *Server) processCheckSuiteEvent(ctx context.Context, event *github.CheckSuiteEvent) error {
	log := lintersutil.FromContext(ctx)
	if event.GetAction() != "rerequested" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}

	headSha := event.GetCheckSuite().GetHeadSHA()
	org := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	installationID := event.GetInstallation().GetID()
	plist, err := linters.FilterPullRequestsWithCommit(ctx, s.GithubClient(installationID), org, repo, headSha)
	if err != nil {
		log.Errorf("failed to filter pull requests: %v", err)
		return nil
	}

	if len(plist) == 0 {
		log.Errorf("No pull requests found for commit SHA: %s", headSha)
		return nil
	}
	for _, pr := range plist {
		log.Infof("try to reprocessing pull request %d, (%v/%v), installationID: %d\n", pr.GetNumber(), org, repo, installationID)
		event := github.PullRequestEvent{
			Repo:         event.GetRepo(),
			Number:       pr.Number,
			PullRequest:  pr,
			Installation: event.GetInstallation(),
		}
		if err := s.handle(ctx, &event); err != nil {
			log.Errorf("failed to handle pull request event: %v", err)
			// continue to handle other pull requests
		}
	}
	return nil
}

func (s *Server) handle(ctx context.Context, event *github.PullRequestEvent) error {
	var (
		num     = event.GetPullRequest().GetNumber()
		org     = event.GetRepo().GetOwner().GetLogin()
		repo    = event.GetRepo().GetName()
		orgRepo = org + "/" + repo
	)
	log := lintersutil.FromContext(ctx)

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

	defaultWorkDir, err := prepareRepoDir(org, repo, num)
	if err != nil {
		return fmt.Errorf("failed to prepare repo dir: %w", err)
	}
	defer func() {
		if s.debug {
			return // do not remove the repository in debug mode
		}
		if err := os.RemoveAll(defaultWorkDir); err != nil {
			log.Errorf("failed to remove the repository , err : %v", err)
		}
	}()

	r, err := s.gitClientFactory.ClientForWithRepoOpts(org, repo, gitv2.RepoOpts{
		CopyTo: defaultWorkDir + "/" + repo,
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

	_, err = s.PrepareExtraRef(org, repo, defaultWorkDir)
	if err != nil {
		log.Errorf("failed to prepare and clone ExtraRef : %v", err)
		return err
	}

	for name, fn := range linters.TotalPullRequestHandlers() {
		linterConfig := s.config.GetLinterConfig(org, repo, name)

		// skip linter if it is disabled
		if linterConfig.Enable != nil && !*linterConfig.Enable {
			continue
		}

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
			RepoDir:                 r.Directory(),
			Context:                 ctx,
			ID:                      lintersutil.GetEventGUID(ctx),
		}

		if !linters.LinterRelated(name, agent) {
			log.Infof("[%s] linter is not related to the PR, skipping", name)
			continue
		}

		r := runner.NewLocalRunner()
		if linterConfig.DockerAsRunner.Image != "" {
			r = s.dockerRunner
		}
		agent.Runner = r
		agent.Storage = s.storage
		agent.GenLogKey = func() string {
			return fmt.Sprintf("%s/%s/%s", agent.LinterConfig.Name, agent.PullRequestEvent.Repo.GetFullName(), agent.ID)
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
		parentDir = filepath.Join(homeDir, "reviewbot-code-workspace")
	} else {
		parentDir = filepath.Join("/tmp", "reviewbot-code-workspace")
	}

	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create parent dir: %w", err)
	}

	dir, err := os.MkdirTemp(parentDir, fmt.Sprintf("%s-%s-%d", org, repo, num))
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	return dir, nil
}

func (s *Server) PrepareExtraRef(org, repo, defaultWorkDir string) (repoClients []gitv2.RepoClient, err error) {
	var repoCfg config.RepoConfig
	config := s.config
	if v, ok := config.CustomConfig[org]; ok {
		repoCfg = v
	}
	if v, ok := config.CustomConfig[org+"/"+repo]; ok {
		repoCfg = v
	}

	if len(repoCfg.ExtraRefs) == 0 {
		return []gitv2.RepoClient{}, nil
	}

	for _, refConfig := range repoCfg.ExtraRefs {
		opt := gitv2.ClientFactoryOpts{
			CacheDirBase: github.String(s.repoCacheDir),
			Persist:      github.Bool(true),
			UseSSH:       github.Bool(true),
			Host:         refConfig.Host,
		}
		gitClient, err := gitv2.NewClientFactory(opt.Apply)
		if err != nil {
			log.Fatalf("failed to create git client factory: %v", err)
		}

		repoPath := defaultWorkDir + "/" + refConfig.Repo
		if refConfig.PathAlias != "" {
			repoPath = defaultWorkDir + refConfig.PathAlias
		}
		r, err := gitClient.ClientForWithRepoOpts(refConfig.Org, refConfig.Repo, gitv2.RepoOpts{
			CopyTo: repoPath,
		})
		if err != nil {
			return []gitv2.RepoClient{}, fmt.Errorf("failed to clone for %s/%s: %w", refConfig.Org, refConfig.Repo, err)
		}

		repoClients = append(repoClients, r)
	}

	return repoClients, nil
}
