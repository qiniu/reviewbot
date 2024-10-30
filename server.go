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
	"fmt"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
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

	// getDockerRunner returns the docker runner
	getDockerRunner func() runner.Runner
	// getKubernetesRunner returns the kubernetes runner
	getKubernetesRunner func() runner.Runner

	storage storage.Storage

	webhookSecret []byte
	// support gitlab
	gitLabHost string

	// support developer access token model
	gitLabAccessToken string
	gitHubAccessToken string
	// support github app model
	appID         int64
	appPrivateKey string

	debug bool

	repoCacheDir string
	kubeConfig   string
}

var (
	mu    sync.Mutex
	prMap = make(map[string]context.CancelFunc)
)

func (s *Server) initKubernetesRunner() {
	var toChecks []config.KubernetesAsRunner
	for _, customConfig := range s.config.CustomConfig {
		for _, linter := range customConfig.Linters {
			if linter.KubernetesAsRunner.Image != "" {
				toChecks = append(toChecks, linter.KubernetesAsRunner)
			}
		}
	}

	if len(toChecks) == 0 {
		return
	}

	kubeRunner, err := runner.NewKubernetesRunner(s.kubeConfig)
	if err != nil {
		log.Fatalf("failed to init kubernetes runner: %v", err)
	}

	s.getKubernetesRunner = func() runner.Runner {
		return kubeRunner.Clone()
	}

	for _, toCheck := range toChecks {
		if err := kubeRunner.Prepare(context.Background(), &config.Linter{
			KubernetesAsRunner: toCheck,
		}); err != nil {
			log.Fatalf("failed to check permission for kubeconfig: %v", err)
		}
	}

	// check kubectl installed
	msg := "kubectl not installed or cannot be executed, please install kubectl first since your reviewbot config(%s) depends on it"
	if err := checkKubectlInstalled(); err != nil {
		log.Fatalf(msg, s.kubeConfig)
	}

	log.Infof("init kubernetes runner success")
}

// check kubectl installed
func checkKubectlInstalled() error {
	cmd := exec.Command("kubectl", "version", "--client")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
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

	var dockerRunner runner.Runner
	if len(images) > 0 {
		dockerRunner, err := runner.NewDockerRunner(nil)
		if err != nil {
			log.Fatalf("failed to init docker runner: %v", err)
		}

		s.getDockerRunner = func() runner.Runner {
			return dockerRunner.Clone()
		}
	}

	if dockerRunner == nil {
		return
	}

	go func() {
		ctx := context.Background()
		for _, image := range images {
			s.pullImageWithRetry(ctx, image, dockerRunner)
		}
	}()

	log.Infof("init docker runner success")
}

func (s *Server) pullImageWithRetry(ctx context.Context, image string, dockerRunner runner.Runner) {
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

		err := dockerRunner.Prepare(ctx, linterConfig)
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
	if r.Header.Get("X-Gitlab-Event") != "" {
		s.gitlabRequestHandle(w, r)
	}

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
func (s *Server) gitlabRequestHandle(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	eventGUID := strconv.FormatInt(now.Unix(), 12)
	if len(eventGUID) > 12 {
		// limit the length of eventGUID to 12
		eventGUID = eventGUID[len(eventGUID)-12:]
	}

	ctx := context.WithValue(context.Background(), lintersutil.EventGUIDKey, eventGUID)
	log := lintersutil.FromContext(ctx)

	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Infof("%v", string(payload))
	v := gitlab.HookEventType(r)
	log.Infof("%v", string(v))

	event, err := gitlab.ParseHook(v, payload)
	if err != nil {
		log.Errorf("parse webhook failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Fprint(w, "Event received. Have a nice day.")

	switch event := event.(type) {
	case *gitlab.MergeEvent:
		go func() {
			log.Info("start process")
			if err := s.processMergeRequestEvent(ctx, event); err != nil {
				log.Errorf("process merge request event: %v", err)
			}
		}()

	default:
		log.Debugf("skipping event type %s\n", github.WebHookType(r))
	}
}
func (s *Server) processMergeRequestEvent(ctx context.Context, event *gitlab.MergeEvent) error {
	log.Infof("stastr evet")
	log := lintersutil.FromContext(ctx)
	if event.ObjectAttributes.State != "opened" && event.ObjectAttributes.State != "reopened" {
		log.Debugf("skipping action %s\n", event.ObjectAttributes.Action)
		return nil
	}

	return s.gitlabHandle(ctx, event)
}

func (s *Server) gitlabHandle(ctx context.Context, event *gitlab.MergeEvent) error {
	var (
		num          = event.ObjectAttributes.IID
		org          = event.Project.Namespace
		repo         = event.Repository.Name
		orgRepo      = event.Project.PathWithNamespace
		pid          = event.ObjectAttributes.TargetProjectID
		latestCommit = event.ObjectAttributes.LastCommit.ID
	)
	log := lintersutil.FromContext(ctx)

	prID := fmt.Sprintf("%s-%s-%d", org, repo, num)
	if cancel, exists := prMap[prID]; exists {
		log.Infof("Cancelling processing for Pull Request : %s\n", prID)
		cancel()
	}

	ctx, cancel := context.WithCancel(ctx)
	mu.Lock()
	prMap[prID] = cancel
	mu.Unlock()

	defer func() {
		select {
		case <-ctx.Done():
			return
		default:
			mu.Lock()
			delete(prMap, prID)
			mu.Unlock()
		}
	}()

	log.Infof("processing pull request %d, (%v/%v)\n", num, org, repo)

	mergeRequestAffectedFiles, response, errx := linters.ListMergeRequestsFiles(ctx, s.GitLabClient(), org, repo, pid, num)
	if errx != nil {
		log.Errorf("List MergeRequestFiles Error: %v\n", errx)
		return errx
	}

	if response.StatusCode != http.StatusOK {
		log.Errorf("list files failed: %v", response)
		return fmt.Errorf("list files failed: %v", response)
	}
	log.Infof("found %d files affected by pull request %d\n", len(mergeRequestAffectedFiles), num)

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
	if err := r.Checkout(latestCommit); err != nil {
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
		opt := gitv2.ClientFactoryOpts{
			CacheDirBase: github.String(s.repoCacheDir),
			Persist:      github.Bool(true),
			UseSSH:       github.Bool(true),
			Host:         s.gitLabHost,
		}
		v2new, err := gitv2.NewClientFactory(opt.Apply)
		if err != nil {
			log.Fatalf("failed to create git client factory: %v", err)
		}
		gitlabProvider, err := linters.NewGitlabProvider(s.GitLabClient(), v2new, mergeRequestAffectedFiles, *event)

		if err != nil {
			log.Errorf("failed to create provider: %v", err)
			return err
		}
		agent := linters.Agent{
			LinterConfig: linterConfig,
			RepoDir:      r.Directory(),
			ID:           lintersutil.GetEventGUID(ctx),
			Provider:     gitlabProvider,
		}

		if !linters.LinterRelated(name, agent) {
			log.Infof("[%s] linter is not related to the PR, skipping", name)
			continue
		}

		r := runner.NewLocalRunner()
		if linterConfig.DockerAsRunner.Image != "" {
			r = s.getDockerRunner()
		}
		agent.Runner = r
		agent.Storage = s.storage
		agent.GenLogKey = func() string {
			return fmt.Sprintf("%s/%s/%s", agent.LinterConfig.Name, agent.Provider.GetCodeReviewInfo().Org+"/"+agent.Provider.GetCodeReviewInfo().Repo, agent.ID)
		}
		agent.GenLogViewURL = func() string {
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

func (s *Server) GitLabClient() *gitlab.Client {
	git, err := gitlab.NewClient(s.gitLabAccessToken, gitlab.WithBaseURL("https://"+s.gitLabHost+"/"))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	return git
}

func (s *Server) handle(ctx context.Context, event *github.PullRequestEvent) error {
	var (
		num     = event.GetPullRequest().GetNumber()
		org     = event.GetRepo().GetOwner().GetLogin()
		repo    = event.GetRepo().GetName()
		orgRepo = org + "/" + repo
	)
	log := lintersutil.FromContext(ctx)

	prID := fmt.Sprintf("%s-%s-%d", org, repo, num)
	if cancel, exists := prMap[prID]; exists {
		log.Infof("Cancelling processing for Pull Request : %s\n", prID)
		cancel()
	}

	ctx, cancel := context.WithCancel(ctx)
	mu.Lock()
	prMap[prID] = cancel
	mu.Unlock()

	defer func() {
		select {
		case <-ctx.Done():
			return
		default:
			mu.Lock()
			delete(prMap, prID)
			mu.Unlock()
		}
	}()

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

	workspace, defaultWorkDir, err := s.prepareGitRepos(ctx, org, repo, num)
	if err != nil {
		return err
	}

	// clean up workspace
	defer func() {
		if s.debug {
			return // do not remove the repository in debug mode
		}
		if err := os.RemoveAll(workspace); err != nil {
			log.Errorf("failed to remove the repository, err: %v", err)
		}
	}()

	for name, fn := range linters.TotalPullRequestHandlers() {
		linterConfig := s.config.GetLinterConfig(org, repo, name)
		linterConfig.Number = num
		linterConfig.Workspace = workspace
		// skip linter if it is disabled
		if linterConfig.Enable != nil && !*linterConfig.Enable {
			continue
		}

		if linterConfig.WorkDir != "" {
			linterConfig.WorkDir = defaultWorkDir + "/" + linterConfig.WorkDir
		} else {
			linterConfig.WorkDir = defaultWorkDir
		}

		log.Infof("[%s] config on repo %v: %+v", name, orgRepo, linterConfig)

		agent := linters.Agent{
			LinterConfig: linterConfig,
			RepoDir:      defaultWorkDir,
			ID:           lintersutil.GetEventGUID(ctx),
		}

		provider, err := linters.NewGithubProvider(s.GithubClient(installationID), s.gitClientFactory, pullRequestAffectedFiles, *event)
		if err != nil {
			log.Errorf("failed to create provider: %v", err)
			return err
		}
		agent.Provider = provider

		if !linters.LinterRelated(name, agent) {
			log.Infof("[%s] linter is not related to the PR, skipping", name)
			continue
		}

		var r runner.Runner
		switch {
		case linterConfig.DockerAsRunner.Image != "":
			r = s.getDockerRunner()
		case linterConfig.KubernetesAsRunner.Image != "":
			r = s.getKubernetesRunner()
		default:
			r = runner.NewLocalRunner()
		}
		agent.Runner = r
		agent.Storage = s.storage
		agent.GenLogKey = func() string {
			return fmt.Sprintf("%s/%s/%s", agent.LinterConfig.Name, agent.Provider.GetCodeReviewInfo().Org+"/"+agent.Provider.GetCodeReviewInfo().Repo, agent.ID)
		}
		agent.GenLogViewURL = func() string {
			// if serverAddr is not provided, return empty string
			if s.serverAddr == "" {
				return ""
			}
			return s.serverAddr + "/view/" + agent.GenLogKey()
		}

		agent.IssueReferences = s.config.GetCompiledIssueReferences(name)

		if err := fn(ctx, agent); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Infof("linter %s is canceled", name)
				// no need to continue
				return nil
			}
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
	gc.WithAuthToken(s.gitHubAccessToken)
	return gc
}

// GithubClient returns a github client
func (s *Server) GithubClient(installationID int64) *github.Client {
	if s.gitHubAccessToken != "" {
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

func (s *Server) prepareGitRepos(ctx context.Context, org, repo string, num int) (workspace string, workDir string, err error) {
	log := lintersutil.FromContext(ctx)
	workspace, err = prepareRepoDir(org, repo, num)
	if err != nil {
		log.Errorf("failed to prepare workspace: %v", err)
		return "", "", err
	}

	refs, workDir := s.fixRefs(workspace, org, repo)
	log.Debugf("refs: %+v", refs)
	for _, ref := range refs {
		opt := gitv2.ClientFactoryOpts{
			CacheDirBase: github.String(s.repoCacheDir),
			Persist:      github.Bool(true),
			UseSSH:       github.Bool(true),
			Host:         ref.Host,
		}

		gitClient, err := gitv2.NewClientFactory(opt.Apply)
		if err != nil {
			log.Errorf("failed to create git client factory: %v", err)
			return "", "", err
		}

		r, err := gitClient.ClientForWithRepoOpts(ref.Org, ref.Repo, gitv2.RepoOpts{
			CopyTo: ref.PathAlias,
		})
		if err != nil {
			log.Errorf("failed to clone for %s/%s: %v", ref.Org, ref.Repo, err)
			return "", "", err
		}

		// main repo, need to checkout PR and update submodules if any
		if ref.Org == org && ref.Repo == repo {
			if err := r.CheckoutPullRequest(num); err != nil {
				log.Errorf("failed to checkout pull request %d: %v", num, err)
				return "", "", err
			}

			// update submodules if any
			if err := updateSubmodules(ctx, r.Directory(), repo); err != nil {
				log.Errorf("error updating submodules: %v", err)
				// continue to run other linters
			}
		}
	}

	return workspace, workDir, nil
}

func updateSubmodules(ctx context.Context, repoDir, repo string) error {
	log := lintersutil.FromContext(ctx)
	gitModulesFile := path.Join(repoDir, ".gitmodules")
	if _, err := os.Stat(gitModulesFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Infof("no .gitmodules file in repo %s", repo)
			return nil
		}
		return err
	}

	log.Info("git pull submodule in progress")
	cmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("error when git pull submodule: %v, output: %s", err, out)
		return err
	}

	log.Infof("git pull submodule output: %s", out)
	return nil
}

func (s *Server) fixRefs(workspace string, org, repo string) ([]config.Refs, string) {
	var repoCfg config.RepoConfig
	if v, ok := s.config.CustomConfig[org]; ok {
		repoCfg = v
	}
	if v, ok := s.config.CustomConfig[org+"/"+repo]; ok {
		repoCfg = v
	}

	var mainRepoFound bool
	var workDir string
	refs := make([]config.Refs, 0, len(repoCfg.Refs))
	for _, ref := range repoCfg.Refs {
		if ref.PathAlias != "" {
			ref.PathAlias = filepath.Join(workspace, ref.PathAlias)
		} else {
			ref.PathAlias = filepath.Join(workspace, ref.Repo)
		}
		refs = append(refs, ref)

		if ref.Repo == repo && ref.Org == org {
			mainRepoFound = true
			workDir = ref.PathAlias
		}
	}

	if !mainRepoFound {
		// always add the main repo to the list
		workDir = filepath.Join(workspace, repo)
		refs = append(refs, config.Refs{
			Org:       org,
			Repo:      repo,
			PathAlias: workDir,
		})
	}

	return refs, workDir
}
