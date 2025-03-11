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
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/golang-jwt/jwt"
	"github.com/google/go-github/v57/github"
	"github.com/gregjones/httpcache"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/reviewbot/internal/llm"
	"github.com/qiniu/reviewbot/internal/runner"
	"github.com/qiniu/reviewbot/internal/storage"
	"github.com/qiniu/reviewbot/internal/util"
	"github.com/qiniu/x/log"
	"github.com/tmc/langchaingo/llms"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"
)

var (
	mu    sync.Mutex
	prMap = make(map[string]context.CancelFunc)
)

var (
	ErrPrepareDir = errors.New("failed to prepare repo dir")
)

type Server struct {
	gitClientFactory gitv2.ClientFactory
	config           config.Config
	storage          storage.Storage
	// server addr which is used to generate the log view url
	// e.g. https://domain
	serverAddr          string
	getDockerRunner     func() runner.Runner
	getKubernetesRunner func() runner.Runner
	kubeConfig          string
	webhookSecret       []byte
	debug               bool
	repoCacheDir        string

	// support gitlab
	gitLabHost                string
	gitLabPersonalAccessToken string

	// support github app model
	gitHubAppAuth         *GitHubAppAuth
	gitHubAccessTokenAuth *GitHubAccessTokenAuth

	// llm model related
	modelConfig llm.Config
	modelClient llms.Model
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Gitlab-Event") != "" {
		s.serveGitLab(w, r)
		return
	}
	s.serveGitHub(w, r)
}

func (s *Server) serveGitHub(w http.ResponseWriter, r *http.Request) {
	eventGUID := github.DeliveryID(r)
	if len(eventGUID) > 12 {
		// limit the length of eventGUID to 12
		eventGUID = eventGUID[len(eventGUID)-12:]
	}
	ctx := context.WithValue(context.Background(), util.EventGUIDKey, eventGUID)
	log := util.FromContext(ctx)

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
	case *github.IssueCommentEvent:
		go func() {
			if err := s.processIssueCommentEvent(ctx, event); err != nil {
				log.Errorf("process issue comment event: %v", err)
			}
		}()
	case *github.PullRequestReviewCommentEvent:
		go func() {
			if err := s.processPullRequestReviewCommentEvent(ctx, event); err != nil {
				log.Errorf("process pull request review comment event: %v", err)
			}
		}()
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

func (s *Server) serveGitLab(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	eventGUID := strconv.FormatInt(now.Unix(), 12)
	if len(eventGUID) > 12 {
		// limit the length of eventGUID to 12
		eventGUID = eventGUID[len(eventGUID)-12:]
	}
	ctx := context.WithValue(context.Background(), util.EventGUIDKey, eventGUID)
	log := util.FromContext(ctx)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v := gitlab.HookEventType(r)

	event, err := gitlab.ParseHook(v, payload)
	if err != nil {
		log.Errorf("parse gitlab webhook failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprint(w, "Event received. Have a nice day.")

	switch event := event.(type) {
	case *gitlab.MergeEvent:
		go func() {
			if err := s.processMergeRequestEvent(ctx, event); err != nil {
				log.Errorf("process merge request event: %v", err)
			}
		}()
	default:
		log.Debugf("skipping gitlab event %v\n", event)
	}
}

func (s *Server) handleGitHubEvent(ctx context.Context, event *github.PullRequestEvent) error {
	info := &codeRequestInfo{
		platform: config.GitHub,
		num:      event.GetPullRequest().GetNumber(),
		org:      event.GetRepo().GetOwner().GetLogin(),
		repo:     event.GetRepo().GetName(),
		orgRepo:  event.GetRepo().GetOwner().GetLogin() + "/" + event.GetRepo().GetName(),
	}

	return s.withCancel(ctx, info, func(ctx context.Context) error {
		installationID := event.GetInstallation().GetID()

		platformInfo := lint.ProviderInfo{
			Host:     "github.com",
			Platform: config.GitHub,
		}

		appName, err := s.getAccountName(ctx)
		if err != nil {
			return err
		}
		platformInfo.GitHubAppName = appName

		provider, err := lint.NewGithubProvider(ctx, s.GithubClient(installationID), *event, lint.WithGitHubProviderInfo(platformInfo))
		if err != nil {
			return err
		}
		info.provider = provider

		workspace, workDir, err := s.prepareGitRepos(ctx, info.org, info.repo, info.num, config.GitHub, installationID, provider)
		if err != nil {
			return err
		}
		defer func() {
			if s.debug { // debug mode, not delete workspace
				return
			}
			_ = os.RemoveAll(workspace)
		}()

		info.workDir = workDir
		info.repoDir = workspace

		return s.handleCodeRequestEvent(ctx, info)
	})
}

func (s *Server) getAccountName(ctx context.Context) (string, error) {
	log := util.FromContext(ctx)
	if s.gitHubAccessTokenAuth != nil {
		if s.gitHubAccessTokenAuth.User != "" {
			return s.gitHubAccessTokenAuth.User, nil
		}

		client := s.GithubAccessTokenClient()
		user, resp, err := client.Users.Get(ctx, "")
		if err != nil {
			log.Errorf("failed to get authenticated user: %v", err)
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			log.Errorf("failed to get authenticated user: %v", resp)
			return "", err
		}
		log.Infof("authenticated user name: %s", user.GetLogin())
		s.gitHubAccessTokenAuth.User = user.GetLogin()
		return user.GetLogin(), nil
	}

	if s.gitHubAppAuth.AppName != "" {
		return s.gitHubAppAuth.AppName, nil
	}

	privateKey, err := os.ReadFile(s.gitHubAppAuth.PrivateKeyPath)
	if err != nil {
		log.Errorf("failed to read private key: %v", err)
		return "", err
	}

	parsedKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		log.Errorf("failed to parse private key: %v", err)
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iss": s.gitHubAppAuth.AppID,
	})

	signedToken, err := token.SignedString(parsedKey)
	if err != nil {
		log.Errorf("failed to sign token: %v", err)
		return "", err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: signedToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	appName, err := lint.GetAppUsername(ctx, client)
	if err != nil {
		return "", err
	}
	log.Infof("app name: %s", appName)
	s.gitHubAppAuth.AppName = appName
	return s.gitHubAppAuth.AppName, nil
}

func (s *Server) handleGitLabEvent(ctx context.Context, event *gitlab.MergeEvent) error {
	info := &codeRequestInfo{
		platform: config.GitLab,
		num:      event.ObjectAttributes.IID,
		org:      event.Project.Namespace,
		repo:     event.Project.Name,
		orgRepo:  event.Project.Namespace + "/" + event.Project.Name,
	}

	return s.withCancel(ctx, info, func(ctx context.Context) error {
		log := util.FromContext(ctx)
		platformInfo := lint.ProviderInfo{
			Host:     s.gitLabHost,
			Platform: config.GitLab,
		}
		if platformInfo.Host == "" {
			platformInfo.Host = "gitlab.com"
		}

		gitlabProvider, err := lint.NewGitlabProvider(ctx, s.GitLabClient(), *event, lint.WithGitlabProviderInfo(platformInfo))
		if err != nil {
			log.Errorf("failed to create provider: %v", err)
			return err
		}
		info.provider = gitlabProvider

		workspace, workDir, err := s.prepareGitRepos(ctx, info.org, info.repo, info.num, config.GitLab, 0, gitlabProvider)
		if err != nil {
			log.Errorf("prepare repo dir failed: %v", err)
			return ErrPrepareDir
		}
		defer func() {
			if s.debug { // debug mode, not delete workspace
				return
			}
			_ = os.RemoveAll(workspace)
		}()
		info.workDir = workDir
		info.repoDir = workspace

		return s.handleCodeRequestEvent(ctx, info)
	})
}

type codeRequestInfo struct {
	platform config.Platform
	num      int
	org      string
	repo     string
	orgRepo  string
	workDir  string
	repoDir  string
	// affectedFiles []string
	provider lint.Provider
}

func (s *Server) handleCodeRequestEvent(ctx context.Context, info *codeRequestInfo) error {
	log := util.FromContext(ctx)

	for name, fn := range lint.TotalPullRequestHandlers() {
		linterConfig := s.config.GetLinterConfig(info.org, info.repo, name, info.platform)

		// skip if linter is not enabled
		if linterConfig.Enable != nil && !*linterConfig.Enable {
			continue
		}

		// set work dir
		if linterConfig.WorkDir != "" {
			linterConfig.WorkDir = info.workDir + "/" + linterConfig.WorkDir
		} else {
			linterConfig.WorkDir = info.workDir
		}

		// set workspace
		linterConfig.Workspace = info.repoDir

		log.Infof("[%s] config on repo %v: %+v", name, info.orgRepo, linterConfig)

		agent := lint.Agent{
			LinterConfig: linterConfig,
			// workspace is the root dir of the repo
			RepoDir:  linterConfig.Workspace + "/" + info.repo,
			ID:       util.GetEventGUID(ctx),
			Provider: info.provider,
		}

		// skip if linter is not language related
		if !lint.LinterRelated(linterConfig.Name, agent) {
			log.Debugf("linter %s is not related, skipping", linterConfig.Name)
			continue
		}

		// set runner
		r := runner.NewLocalRunner()
		if linterConfig.DockerAsRunner.Image != "" {
			r = s.getDockerRunner()
		} else if linterConfig.KubernetesAsRunner.Image != "" {
			r = s.getKubernetesRunner()
		}
		agent.Runner = r

		// set storage
		agent.Storage = s.storage

		// generate log key
		agent.GenLogKey = func() string {
			return fmt.Sprintf("%s/%s/%s", agent.LinterConfig.Name, agent.Provider.GetCodeReviewInfo().Org+"/"+agent.Provider.GetCodeReviewInfo().Repo, agent.ID)
		}
		// generate log view url
		agent.GenLogViewURL = func() string {
			// if serverAddr is not provided, return empty string
			if s.serverAddr == "" {
				return ""
			}
			return s.serverAddr + "/view/" + agent.GenLogKey()
		}

		// set issue references
		agent.IssueReferences = s.config.GetCompiledIssueReferences(name)

		// set model client
		agent.ModelClient = s.modelClient

		// run linter finally
		if err := fn(ctx, agent); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			log.Errorf("failed to run linter: %v", err)
			continue
		}
	}

	return nil
}

func processForCLI(ctx context.Context, path string, o cliOptions) error {
	log := util.FromContext(ctx)
	for name, fn := range lint.TotalPullRequestHandlers() {
		// check if linter is installed
		_, err := exec.LookPath(name)
		if err != nil {
			log.Infof("%s not found in environment, skipping", name)
			continue
		}
		log.Infof("%s is started", name)
		linterConfig := config.GetCLILinterConfig(name)

		linterConfig.WorkDir = path
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			linterConfig.WorkDir = filepath.Dir(path)
		}
		if o.logDir == "" {
			o.logDir = "/tmp"
		}
		storage, err := storage.NewLocalStorage(o.logDir)
		if err != nil {
			return err
		}

		agent := lint.Agent{
			LinterConfig: linterConfig,
			// actually, the repoDir is the inputPath in cli mode, maybe it is a dir or a file path
			RepoDir: path,
			CLIMode: true,
			Runner:  runner.NewLocalRunner(),
			Storage: storage,
			GenLogKey: func() string {
				return "log/" + name
			},
		}

		if err := fn(ctx, agent); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			log.Errorf("failed to run linter: %v", err)
			continue
		}
		log.Infof("%s is finished", name)
	}

	return nil
}

func (s *Server) withCancel(ctx context.Context, info *codeRequestInfo, fn func(context.Context) error) error {
	log := util.FromContext(ctx)
	prID := fmt.Sprintf("%s-%s-%s-%d", info.platform, info.org, info.repo, info.num)
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

	return fn(ctx)
}

func (s *Server) initLLMModel() {
	modelClient, err := llm.New(context.Background(), s.modelConfig)
	if err != nil {
		log.Fatalf("failed to init rag server: %v", err)
	}
	s.modelClient = modelClient
}

func (s *Server) initCustomLinters() {
	for linterName, customLinter := range s.config.CustomLinters {
		lint.RegisterPullRequestHandler(linterName, lint.GeneralLinterHandler)
		lint.RegisterLinterLanguages(linterName, customLinter.Languages)
	}
}

func (s *Server) initKubernetesRunner() {
	var toChecks []config.KubernetesAsRunner
	for _, customLinter := range s.config.CustomLinters {
		if customLinter.KubernetesAsRunner.Image != "" {
			toChecks = append(toChecks, customLinter.KubernetesAsRunner)
		}
	}
	for _, customConfig := range s.config.CustomRepos {
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

func (s *Server) initDockerRunner() {
	var images []string
	for _, customLinter := range s.config.CustomLinters {
		if customLinter.DockerAsRunner.Image != "" {
			images = append(images, customLinter.DockerAsRunner.Image)
		}
	}
	for _, customConfig := range s.config.CustomRepos {
		for _, linter := range customConfig.Linters {
			if linter.DockerAsRunner.Image != "" {
				images = append(images, linter.DockerAsRunner.Image)
			}
		}
	}

	var dockerRunner runner.Runner
	if len(images) > 0 {
		dr, err := runner.NewDockerRunner(nil)
		if err != nil {
			log.Fatalf("failed to init docker runner: %v", err)
		}

		s.getDockerRunner = func() runner.Runner {
			return dr.Clone()
		}
		dockerRunner = dr
	}

	if dockerRunner == nil {
		return
	}

	log.Debugf("total images to pull: %d", len(images))
	go func() {
		ctx := context.Background()
		for _, image := range images {
			log.Debugf("pulling image: %s", image)
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

func (s *Server) processIssueCommentEvent(ctx context.Context, event *github.IssueCommentEvent) error {
	log := util.FromContext(ctx)
	if event.GetAction() != "created" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}
	if !strings.Contains(*event.Comment.Body, "@reviewbot") {
		return nil
	}

	query := event.GetComment().GetBody()
	resp, err := llm.Query(ctx, s.modelClient, query, "")
	if err != nil {
		return err
	}
	log.Infof("query success,got resp: %s", resp)

	replyComment := &github.IssueComment{
		Body: github.String(resp),
	}
	installationID := event.GetInstallation().GetID()
	_, _, err = s.GithubClient(installationID).Issues.CreateComment(ctx, event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), event.GetIssue().GetNumber(), replyComment)
	if err != nil {
		return err
	}
	log.Infof("create comment success: %v", replyComment)
	return nil
}

func (s *Server) processPullRequestReviewCommentEvent(ctx context.Context, event *github.PullRequestReviewCommentEvent) error {
	log := util.FromContext(ctx)
	if event.GetAction() != "created" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}

	query := event.GetComment().GetBody()
	path := event.GetComment().GetPath()
	position := event.GetComment().GetPosition()
	diffHunk := event.GetComment().GetDiffHunk()
	inReplyTo := event.GetComment().GetInReplyTo()

	if !strings.Contains(query, "@reviewbot") {
		log.Debugf("skipping reviewbot comment\n")
		return nil
	}

	installationID := event.GetInstallation().GetID()
	githubClient := s.GithubClient(installationID)
	historyComments, err := prepareCommentContext(ctx, event, githubClient, path, position)
	if err != nil {
		return err
	}
	queryContext := "diffHunk: " + diffHunk + "\n" + "history comments: " + historyComments

	// send query to llm model
	resp, err := llm.Query(ctx, s.modelClient, query, queryContext)
	if err != nil {
		return err
	}
	log.Infof("query success,got resp: %s", resp)

	// reply comment
	comment, _, err := githubClient.PullRequests.CreateCommentInReplyTo(ctx, event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), event.GetPullRequest().GetNumber(), resp, inReplyTo)
	if err != nil {
		log.Errorf("failed to create comment: %v", err)
		return err
	}

	log.Infof("create comment success: %v", comment)
	return nil
}

func (s *Server) processPullRequestEvent(ctx context.Context, event *github.PullRequestEvent) error {
	log := util.FromContext(ctx)
	if event.GetAction() != "opened" && event.GetAction() != "reopened" && event.GetAction() != "synchronize" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}

	return s.handleGitHubEvent(ctx, event)
}

func (s *Server) processCheckRunRequestEvent(ctx context.Context, event *github.CheckRunEvent) error {
	log := util.FromContext(ctx)
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
	prs, err := lint.FilterPullRequestsWithCommit(ctx, client, org, repoName, headSHA)
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

		if err := s.handleGitHubEvent(ctx, event); err != nil {
			log.Errorf("failed to handle pull request event: %v", err)
			// continue to handle other pull requests
		}
	}

	return nil
}

func (s *Server) processCheckSuiteEvent(ctx context.Context, event *github.CheckSuiteEvent) error {
	log := util.FromContext(ctx)
	if event.GetAction() != "rerequested" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}

	headSha := event.GetCheckSuite().GetHeadSHA()
	org := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	installationID := event.GetInstallation().GetID()
	plist, err := lint.FilterPullRequestsWithCommit(ctx, s.GithubClient(installationID), org, repo, headSha)
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
		if err := s.handleGitHubEvent(ctx, &event); err != nil {
			log.Errorf("failed to handle pull request event: %v", err)
			// continue to handle other pull requests
		}
	}
	return nil
}

func (s *Server) processMergeRequestEvent(ctx context.Context, event *gitlab.MergeEvent) error {
	log := util.FromContext(ctx)
	if event.ObjectAttributes.State != "opened" && event.ObjectAttributes.State != "reopened" {
		log.Debugf("skipping action %s\n", event.ObjectAttributes.State)
		return nil
	}

	return s.handleGitLabEvent(ctx, event)
}

func (s *Server) GitLabClient() *gitlab.Client {
	host := s.gitLabHost
	if !strings.HasPrefix(host, "http") {
		// default to https if not specified
		host = "https://" + host
	}
	git, err := gitlab.NewClient(s.gitLabPersonalAccessToken, gitlab.WithBaseURL(host))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	return git
}

func (s *Server) githubAppClient(installationID int64) *github.Client {
	tr, err := ghinstallation.NewKeyFromFile(httpcache.NewMemoryCacheTransport(), s.gitHubAppAuth.AppID, installationID, s.gitHubAppAuth.PrivateKeyPath)
	if err != nil {
		log.Fatalf("failed to create github app transport: %v", err)
	}
	return github.NewClient(&http.Client{Transport: tr})
}

func (s *Server) GithubAccessTokenClient() *github.Client {
	gc := github.NewClient(httpcache.NewMemoryCacheTransport().Client())
	gc.WithAuthToken(s.gitHubAccessTokenAuth.AccessToken)
	return gc
}

// GithubClient returns a github client.
func (s *Server) GithubClient(installationID int64) *github.Client {
	if s.gitHubAccessTokenAuth != nil {
		return s.GithubAccessTokenClient()
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

// check kubectl installed.
func checkKubectlInstalled() error {
	cmd := exec.Command("kubectl", "version", "--client")
	return cmd.Run()
}

func prepareCommentContext(ctx context.Context, event *github.PullRequestReviewCommentEvent, githubClient *github.Client, path string, position int) (string, error) {
	// get all comments
	prComments, _, err := githubClient.PullRequests.ListComments(ctx, event.GetRepo().GetOwner().GetLogin(), event.GetRepo().GetName(), event.GetPullRequest().GetNumber(), nil)
	if err != nil {
		return "", err
	}

	// filter comments by path and position
	var comments []string
	if path != "" && position != 0 {
		for _, comment := range prComments {
			if comment.GetPath() == path && comment.GetPosition() == position {
				comments = append(comments, comment.GetBody())
			}
		}
	}
	log.Infof("filter comments success: %v", comments)

	return strings.Join(comments, "\n"), nil
}
