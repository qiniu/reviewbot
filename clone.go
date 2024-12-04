package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lint"
	"github.com/qiniu/reviewbot/internal/util"
	"github.com/qiniu/x/log"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"
)

var errUnsupportedPlatform = errors.New("unsupported platform")

func (s *Server) prepareGitRepos(ctx context.Context, org, repo string, num int, platform config.Platform, installationID int64, provider lint.Provider) (workspace string, workDir string, err error) {
	log := util.FromContext(ctx)
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
		if err := s.handleSingleRef(ctx, ref, org, repo, platform, installationID, num, provider); err != nil {
			return "", "", err
		}
	}

	return workspace, workDir, nil
}

func (s *Server) handleSingleRef(ctx context.Context, ref config.Refs, org, repo string, platform config.Platform, installationID int64, num int, provider lint.Provider) error {
	opt := gitv2.ClientFactoryOpts{
		CacheDirBase: github.String(s.repoCacheDir),
		Persist:      github.Bool(true),
	}

	gb := s.newGitConfigBuilder(ref.Org, ref.Repo, platform, installationID, provider)
	if err := gb.configureGitAuth(&opt); err != nil {
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
	log := util.FromContext(ctx)
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
	log := util.FromContext(ctx)
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
	log := util.FromContext(ctx)
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
	AppName        string
	AppID          int64
	InstallationID int64
	PrivateKeyPath string
}

type GitHubAccessTokenAuth struct {
	AccessToken string
	User        string
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

// GitConfigBuilder is used to build the Git configuration for a specific request.
type GitConfigBuilder struct {
	server   *Server
	org      string
	repo     string
	host     string
	platform config.Platform
	provider lint.Provider
	// installationID is the installation ID for the GitHub App
	installationID int64
}

func (s *Server) newGitConfigBuilder(org, repo string, platform config.Platform, installationID int64, provider lint.Provider) *GitConfigBuilder {
	g := &GitConfigBuilder{
		server:         s,
		org:            org,
		repo:           repo,
		platform:       platform,
		installationID: installationID,
		provider:       provider,
	}
	g.host = g.getHostForPlatform(platform)
	return g
}

func (g *GitConfigBuilder) configureGitAuth(opt *gitv2.ClientFactoryOpts) error {
	auth := g.buildAuth()
	opt.Host = g.host
	switch g.platform {
	case config.GitHub:
		return g.configureGitHubAuth(opt, auth)
	case config.GitLab:
		return g.configureGitLabAuth(opt, auth)
	default:
		log.Errorf("unsupported platform: %s", g.platform)
		return errUnsupportedPlatform
	}
}

func (g *GitConfigBuilder) getHostForPlatform(platform config.Platform) string {
	switch platform {
	case config.GitLab:
		if g.server.gitLabHost != "" {
			return g.server.gitLabHost
		}
		return "gitlab.com"
	case config.GitHub:
		return "github.com"
	default:
		log.Errorf("unsupported platform: %s", g.platform)
		return ""
	}
}

func (g *GitConfigBuilder) buildAuth() GitAuth {
	switch g.platform {
	case config.GitHub:
		return g.buildGitHubAuth()
	case config.GitLab:
		return g.buildGitLabAuth()
	default:
		return GitAuth{}
	}
}

func (g *GitConfigBuilder) buildGitHubAuth() GitAuth {
	if g.server.gitHubAppAuth != nil {
		appAuth := *g.server.gitHubAppAuth
		appAuth.InstallationID = g.installationID
		return GitAuth{
			GitHubAppAuth: &appAuth,
		}
	}

	if g.server.gitHubAccessTokenAuth != nil {
		return GitAuth{
			GitHubAccessToken: g.server.gitHubAccessTokenAuth.AccessToken,
		}
	}

	return GitAuth{}
}

func (g *GitConfigBuilder) buildGitLabAuth() GitAuth {
	if g.server.gitLabPersonalAccessToken != "" {
		return GitAuth{
			GitLabPersonalAccessToken: g.server.gitLabPersonalAccessToken,
		}
	}

	return GitAuth{}
}

func (g *GitConfigBuilder) configureGitHubAuth(opt *gitv2.ClientFactoryOpts, auth GitAuth) error {
	switch {
	case auth.GitHubAppAuth != nil:
		opt.UseSSH = github.Bool(false)
		opt.Username = func() (string, error) {
			return "x-access-token", nil
		}
		opt.Token = func(org string) (string, error) {
			log.Debugf("get token for %s, installationID: %d", org, auth.GitHubAppAuth.InstallationID)
			return g.provider.GetToken()
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

func (g *GitConfigBuilder) configureGitLabAuth(opt *gitv2.ClientFactoryOpts, auth GitAuth) error {
	switch {
	case auth.GitLabPersonalAccessToken != "":
		opt.UseSSH = github.Bool(false)
		opt.Username = func() (string, error) {
			return "oauth2", nil
		}
		opt.Token = func(org string) (string, error) {
			log.Infof("get token for %s, personal access token: %s", org, auth.GitLabPersonalAccessToken)
			return g.provider.GetToken()
		}
		return nil
	default:
		// default use ssh key if no auth
		opt.UseSSH = github.Bool(true)
	}

	return nil
}
