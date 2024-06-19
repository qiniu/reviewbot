package metric

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// use WEWORK_WEBHOOK to send alert message to wework group
	// refer: https://developer.work.weixin.qq.com/document/path/91770
	WEWORK_WEBHOOK = os.Getenv("WEWORK_WEBHOOK")
)

var (
	issueCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "reviewbot_issue_found_total",
		Help: "issue found by linter",
	}, []string{"repo", "linter", "pull_request", "commit"})
)

func IncIssueCounter(repo, linter, pull_request, commit string, count float64) {
	issueCounter.WithLabelValues(repo, linter, pull_request, commit).Add(count)
}

// SendAlertMessageIfNeeded sends alert message to wework group if needed
// refer: https://developer.work.weixin.qq.com/document/path/91770
// The mkMessage is the markdown format message
func SendAlertMessageIfNeeded(message string) error {
	if WEWORK_WEBHOOK == "" || message == "" {
		return nil
	}

	resp, err := http.DefaultClient.Post(WEWORK_WEBHOOK, "application/json", strings.NewReader(message))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send alert message failed: %v", resp)
	}
	return nil
}
