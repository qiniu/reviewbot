package metric

import (
	"encoding/json"
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

type MessageBody struct {
	MsgType  string     `json:"msgtype"`
	Text     MsgContent `json:"text,omitempty"`
	Markdown MsgContent `json:"markdown,omitempty"`
}

type MsgContent struct {
	Content string `json:"content"`
}

// NotifyWebhook sends message to wework group
// refer: https://developer.work.weixin.qq.com/document/path/91770
func NotifyWebhook(message MessageBody) error {
	if WEWORK_WEBHOOK == "" {
		return nil
	}

	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Post(WEWORK_WEBHOOK, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send message failed: %v", resp)
	}

	errCode := resp.Header.Get("Error-Code")
	if errCode != "0" {
		return fmt.Errorf("send message failed, errCode: %v, errMsg: %v", errCode, resp.Header.Get("Error-Msg"))
	}

	return nil
}
