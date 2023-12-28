/*
Copyright 2023 Qiniu Cloud (qiniu.com).

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
	// TODO: synchronization 是什么意思？
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
	log.Infof("found %d files affected by pull request %d\n", len(pullRequestAffectedFiles), num)

	// clone the repo
	r, err := s.gitClientFactory.ClientFor(org, repo)
	if err != nil {
		log.Errorf("failed to create git client: %v", err)
		return err
	}

	if err := r.CheckoutPullRequest(num); err != nil {
		log.Errorf("failed to checkout pull request %d: %v", num, err)
		return err
	}

	defer r.Clean()

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
		log.Infof("%s found valid %d comments related to this PR %d (%s) \n", name, len(comments), num, org+"/"+repo)
		totalComments = append(totalComments, comments...)
	}

	if err := s.PostCommentsWithRetry(ctx, org, repo, num, totalComments); err != nil {
		log.Errorf("failed to post comments: %v", err)
		return err
	}
	log.Info("posted comments success\n")
	return nil
}
