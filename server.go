package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cr-bot/config"
	"github.com/cr-bot/linters"
	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/xlog"
	gitv2 "k8s.io/test-infra/prow/git/v2"
)

type Server struct {
	gc               *github.Client
	gitClientFactory gitv2.ClientFactory
	config           config.Config

	webhookSecret []byte
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventGUID := github.DeliveryID(r)
	log := xlog.New(eventGUID)

	payload, err := github.ValidatePayload(r, s.webhookSecret)
	if err != nil {
		log.Errorf("validate payload failed: %v", err)
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
			if err := s.processPullRequestEvent(log, event, eventGUID); err != nil {
				log.Errorf("process pull request event: %v", err)
			}
		}()
	default:
		log.Debugf("skipping event type %s\n", github.WebHookType(r))
	}
}

func (s *Server) processPullRequestEvent(log *xlog.Logger, event *github.PullRequestEvent, eventGUID string) error {
	// TODO: synchonization 是什么意思？
	if event.GetAction() != "opened" && event.GetAction() != "reopened" {
		log.Debugf("skipping action %s\n", event.GetAction())
		return nil
	}

	return s.handle(log, context.Background(), event)
}

func (s *Server) handle(log *xlog.Logger, ctx context.Context, event *github.PullRequestEvent) error {
	num := event.GetPullRequest().GetNumber()
	org := event.GetRepo().GetOwner().GetLogin()
	repo := event.GetRepo().GetName()
	log.Infof("processing pull request %d, org %v, repo %v\n", num, org, repo)

	pullRequestAffectedFiles, response, err := s.ListPullRequestsFiles(ctx, org, repo, num)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		log.Errorf("list files failed: %v", response)
		return fmt.Errorf("list files failed: %v", response)
	}

	// clone the repo
	// TODO: cache the repo
	r, err := s.gitClientFactory.ClientFor(org, repo)
	if err != nil {
		log.Errorf("failed to create git client: %v", err)
		return err
	}

	if err := r.CheckoutPullRequest(num); err != nil {
		log.Errorf("failed to checkout pull request %d: %v", num, err)
		return err
	}

	var totalComments []*github.PullRequestComment

	for name, lingerConfig := range s.config.Linters(org, repo) {
		f := linters.LinterHandler(name)
		if f == nil {
			continue
		}

		// 更新完整的工作目录
		lingerConfig.WorkDir = r.Directory() + "/" + lingerConfig.WorkDir

		log.Infof("name: %v, lingerConfig: %+v", name, lingerConfig)
		lintResults, err := f(lingerConfig)
		if err != nil {
			log.Errorf("failed to run linter: %v", err)
			return err
		}

		log.Infof("%s found total %d files with lint errors on repo %v", name, len(lintResults), repo)
		comments, err := buildPullRequestCommentBody(name, lintResults, pullRequestAffectedFiles)
		if err != nil {
			log.Errorf("failed to build pull request comment body: %v", err)
			return err
		}
		log.Infof("%s found valid %d comments related to this PR \n", name, len(comments))
		totalComments = append(totalComments, comments...)
	}

	if err := s.PostCommentsWithRetry(ctx, org, repo, num, totalComments); err != nil {
		log.Errorf("failed to post comments: %v", err)
		return err
	}
	log.Info("posted comments success\n")
	return nil
}
