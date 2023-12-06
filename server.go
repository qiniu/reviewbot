package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/xlog"
	gitv2 "k8s.io/test-infra/prow/git/v2"
)

type Server struct {
	gc               *github.Client
	gitClientFactory gitv2.ClientFactory

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

	// ListFiles lists the files for the specified pull request.
	affectedFiles, response, err := s.gc.PullRequests.ListFiles(ctx, org, repo, num, &github.ListOptions{})
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("list files failed: %v", response.Status)
	}

	// filter out files that are not go files
	var affectedGoFiles []string
	for _, f := range affectedFiles {
		if !strings.HasSuffix(f.GetFilename(), ".go") {
			continue
		}
		affectedGoFiles = append(affectedGoFiles, f.GetFilename())
	}
	log.Debugf("PR affected golang files: %v\n", affectedGoFiles)

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

	// run staticcheck
	executor, err := NewStaticcheckExecutor(r.Directory())
	if err != nil {
		log.Errorf("failed to create staticcheck executor: %v", err)
		return err
	}

	output, _ := executor.Run(log, "./...")
	results := formatStaticcheckOutput(output)
	// filter out files that are not affected by the PR
	var filteredLintErrs []StaticcheckOutput
	for _, g := range affectedGoFiles {
		if v, ok := results[g]; ok {
			filteredLintErrs = append(filteredLintErrs, v...)
		}
	}

	// TODO: remove old comment if deprecated

	if len(filteredLintErrs) == 0 {
		log.Infof("no lint errors\n")
		return nil
	}

	log.Infof("lint errors: %v\n", filteredLintErrs)
	// comment on the PR
	for _, lintErr := range filteredLintErrs {
		line, err := strconv.ParseInt(lintErr.line, 10, 64)
		if err != nil {
			log.Errorf("failed to parse line number: %v", err)
			return err
		}
		if _, _, err := s.gc.PullRequests.CreateComment(ctx, org, repo, num, &github.PullRequestComment{
			Body:        github.String(fmt.Sprintf("%s: %s", lintErr.code, lintErr.message)),
			CommitID:    github.String(event.GetPullRequest().GetHead().GetSHA()),
			Path:        github.String(lintErr.file),
			Line:        github.Int(int(line)),
			Side:        github.String("RIGHT"),
			SubjectType: github.String("file"),
			InReplyTo:   github.Int64(0),
			Position:    github.Int(int(line)),
		}); err != nil {
			log.Errorf("failed to create comment: %v", err)
			return err
		}
	}

	return nil
}
