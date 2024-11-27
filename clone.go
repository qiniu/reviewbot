package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	"github.com/qiniu/x/log"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"
)

func (s *Server) prepareGitRepos(ctx context.Context, org, repo string, num int, platform config.Platform, installationID int64) (workspace string, workDir string, err error) {
	log := lintersutil.FromContext(ctx)
	workspace, err = prepareRepoDir(org, repo, num)
	if err != nil {
		log.Errorf("failed to prepare workspace: %v", err)
		return "", "", err
	}

	defer func() {
		if s.debug { // debug mode, not delete workspace
			return
		}
		_ = os.RemoveAll(workspace)
	}()

	refs, workDir := s.fixRefs(workspace, org, repo)
	log.Debugf("refs: %+v", refs)
	for _, ref := range refs {
		if err := s.handleSingleRef(ctx, ref, org, repo, platform, installationID, num); err != nil {
			return "", "", err
		}
	}

	return workspace, workDir, nil
}

func (s *Server) handleSingleRef(ctx context.Context, ref config.Refs, org, repo string, platform config.Platform, installationID int64, num int) error {
	opt := gitv2.ClientFactoryOpts{
		CacheDirBase: github.String(s.repoCacheDir),
		Persist:      github.Bool(true),
	}

	gitConfig := s.newGitConfigBuilder(ref.Org, ref.Repo, platform, installationID).Build()
	log.Debugf("git config: %+v", gitConfig)
	if err := s.configureGitAuth(&opt, gitConfig); err != nil {
		return fmt.Errorf("failed to configure git auth: %w", err)
	}

	log.Debugf("git options: %+v", opt)
	gitClient, err := gitv2.NewClientFactory(opt.Apply)
	if err != nil {
		log.Errorf("failed to create git client factory: %v", err)
		return err
	}

	r, err := gitClient.ClientForWithRepoOpts(ref.Org, ref.Repo, gitv2.RepoOpts{
		CopyTo: ref.PathAlias,
	})
	if err != nil {
		log.Errorf("failed to clone for %s/%s: %v", ref.Org, ref.Repo, err)
		return err
	}

	// main repo, need to checkout PR/MR and update submodules if any
	if ref.Org == org && ref.Repo == repo {
		if err := s.checkoutAndUpdateRepo(ctx, r, platform, num, repo); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) checkoutAndUpdateRepo(ctx context.Context, r gitv2.RepoClient, platform config.Platform, num int, repo string) error {
	log := lintersutil.FromContext(ctx)
	if err := s.checkoutCode(ctx, r, platform, num); err != nil {
		return err
	}

	// update submodules if any
	if err := updateSubmodulesIfExisted(ctx, r.Directory(), repo); err != nil {
		log.Errorf("error updating submodules: %v", err)
		// continue to run other linters
	}

	return nil
}

func (s *Server) checkoutCode(ctx context.Context, r gitv2.RepoClient, platform config.Platform, num int) error {
	log := lintersutil.FromContext(ctx)
	switch platform {
	case config.GitHub:
		if err := r.CheckoutPullRequest(num); err != nil {
			log.Errorf("failed to checkout pull request %d: %v", num, err)
			return err
		}
	case config.GitLab:
		// For GitLab MR, fetch and checkout the source branch
		if err := r.FetchRef(fmt.Sprintf("refs/merge-requests/%d/head", num)); err != nil {
			log.Errorf("failed to fetch merge request %d: %v", num, err)
			return err
		}
		if err := r.Checkout("FETCH_HEAD"); err != nil {
			log.Errorf("failed to checkout merge request %d: %v", num, err)
			return err
		}
		if err := r.CheckoutNewBranch(fmt.Sprintf("mr%d", num)); err != nil {
			log.Errorf("failed to checkout merge request %d: %v", num, err)
			return err
		}
	}
	return nil
}

func updateSubmodulesIfExisted(ctx context.Context, repoDir, repo string) error {
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
	if v, ok := s.config.CustomRepos[org]; ok {
		repoCfg = v
	}
	if v, ok := s.config.CustomRepos[org+"/"+repo]; ok {
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

// GitHubAppAuth stores the authentication information for GitHub App.
// See https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps
type GitHubAppAuth struct {
	AppID          int64
	InstallationID int64
	PrivateKeyPath string
}

// GitLabOAuthAppAuth stores the authentication information for GitLab OAuth App.
// See https://docs.gitlab.com/ee/api/oauth2.html#web-application-flow
type GitLabOAuthAppAuth struct {
	ApplicationID string
	Secret        string
	CallbackURL   string
}

// GitAuth stores the authentication information for different platforms.
type GitAuth struct {
	// GitHub authentication
	GitHubAccessToken string
	GitHubAppAuth     *GitHubAppAuth

	// GitLab authentication
	GitLabPersonalAccessToken string
}

// GitConfig stores the Git repository configuration.
type GitConfig struct {
	Platform config.Platform
	Host     string // gitlab/github host
	Auth     GitAuth
}

// githubAppTokenCache implements the cache for GitHub App tokens.
type githubAppTokenCache struct {
	sync.RWMutex
	tokens  map[string]tokenWithExp // key: installationID, value: token
	appAuth *GitHubAppAuth
}

// GitConfigBuilder is used to build the Git configuration for a specific request.
type GitConfigBuilder struct {
	server   *Server
	org      string
	repo     string
	platform config.Platform
	// installationID is the installation ID for the GitHub App
	installationID int64
}

type tokenWithExp struct {
	token string
	exp   time.Time
}

// newGitHubAppTokenCache creates a new token cache.
func newGitHubAppTokenCache(appID int64, privateKeyPath string) *githubAppTokenCache {
	return &githubAppTokenCache{
		tokens: make(map[string]tokenWithExp),
		appAuth: &GitHubAppAuth{
			AppID:          appID,
			PrivateKeyPath: privateKeyPath,
		},
	}
}
func (c *githubAppTokenCache) getToken(org string, installationID int64) (string, error) {
	key := fmt.Sprintf("%s-%d", org, installationID)
	c.RLock()
	t, exists := c.tokens[key]
	c.RUnlock()

	if exists && t.exp.After(time.Now()) {
		return t.token, nil
	}

	c.Lock()
	defer c.Unlock()

	// double check
	if t, exists := c.tokens[key]; exists {
		if t.exp.After(time.Now()) {
			return t.token, nil
		}
	}

	// get new token
	token, err := c.refreshToken(installationID)
	if err != nil {
		return "", err
	}

	log.Debugf("refreshed token for %s, installationID: %d", org, installationID)
	// cache token, 1 hour expiry
	c.tokens[key] = tokenWithExp{
		token: token,
		// add a buffer to avoid token expired
		exp: time.Now().Add(time.Hour - time.Minute),
	}

	return token, nil
}

// refreshToken refresh the GitHub App token.
func (c *githubAppTokenCache) refreshToken(installationID int64) (string, error) {
	tr, err := ghinstallation.NewKeyFromFile(
		http.DefaultTransport,
		c.appAuth.AppID,
		installationID,
		c.appAuth.PrivateKeyPath,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create github app transport: %w", err)
	}

	token, err := tr.Token(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get installation token: %w", err)
	}

	return token, nil
}

func (s *Server) newGitConfigBuilder(org, repo string, platform config.Platform, installationID int64) *GitConfigBuilder {
	return &GitConfigBuilder{
		server:         s,
		org:            org,
		repo:           repo,
		platform:       platform,
		installationID: installationID,
	}
}

func (b *GitConfigBuilder) Build() GitConfig {
	config := GitConfig{
		Platform: b.platform,
		Host:     b.getHostForPlatform(b.platform),
		Auth:     b.buildAuth(),
	}

	return config
}

func (b *GitConfigBuilder) getHostForPlatform(platform config.Platform) string {
	switch platform {
	case config.GitLab:
		if b.server.gitLabHost != "" {
			return b.server.gitLabHost
		}
		return "gitlab.com"
	default:
		return "github.com"
	}
}

func (b *GitConfigBuilder) buildAuth() GitAuth {
	switch b.platform {
	case config.GitHub:
		return b.buildGitHubAuth()
	case config.GitLab:
		return b.buildGitLabAuth()
	default:
		return GitAuth{}
	}
}

func (b *GitConfigBuilder) buildGitHubAuth() GitAuth {
	if b.server.gitHubAppAuth != nil {
		appAuth := *b.server.gitHubAppAuth
		appAuth.InstallationID = b.installationID
		return GitAuth{
			GitHubAppAuth: &appAuth,
		}
	}

	if b.server.gitHubPersonalAccessToken != "" {
		return GitAuth{
			GitHubAccessToken: b.server.gitHubPersonalAccessToken,
		}
	}

	return GitAuth{}
}

func (b *GitConfigBuilder) buildGitLabAuth() GitAuth {
	if b.server.gitLabPersonalAccessToken != "" {
		return GitAuth{
			GitLabPersonalAccessToken: b.server.gitLabPersonalAccessToken,
		}
	}

	return GitAuth{}
}

func (s *Server) configureGitAuth(opt *gitv2.ClientFactoryOpts, gConf GitConfig) error {
	opt.Host = gConf.Host
	switch gConf.Platform {
	case config.GitHub:
		return s.configureGitHubAuth(opt, gConf)
	case config.GitLab:
		return s.configureGitLabAuth(opt, gConf)
	default:
		return fmt.Errorf("unsupported platform: %s", gConf.Platform)
	}
}

func (s *Server) configureGitHubAuth(opt *gitv2.ClientFactoryOpts, config GitConfig) error {
	auth := config.Auth

	switch {
	case auth.GitHubAppAuth != nil:
		opt.UseSSH = github.Bool(false)
		opt.Username = func() (string, error) {
			return "x-access-token", nil
		}
		opt.Token = func(org string) (string, error) {
			log.Debugf("get token for %s, installationID: %d", org, auth.GitHubAppAuth.InstallationID)
			return s.githubAppTokenCache.getToken(org, auth.GitHubAppAuth.InstallationID)
		}
		return nil

	case auth.GitHubAccessToken != "":
		opt.UseSSH = github.Bool(false)
		opt.Username = func() (string, error) {
			return "x-access-token", nil
		}
		opt.Token = func(org string) (string, error) {
			return auth.GitHubAccessToken, nil
		}
		return nil
	}

	// default use ssh key if no auth
	opt.UseSSH = github.Bool(true)
	return nil
}

func (s *Server) configureGitLabAuth(opt *gitv2.ClientFactoryOpts, config GitConfig) error {
	auth := config.Auth

	switch {
	case auth.GitLabPersonalAccessToken != "":
		opt.UseSSH = github.Bool(false)
		opt.Username = func() (string, error) {
			return "oauth2", nil
		}
		opt.Token = func(org string) (string, error) {
			return auth.GitLabPersonalAccessToken, nil
		}
		return nil
	default:
		// default use ssh key if no auth
		opt.UseSSH = github.Bool(true)
	}

	return nil
}
