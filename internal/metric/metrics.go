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

package metric

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/qiniu/x/log"
)

// use WEWORK_WEBHOOK to send alert message to wework group
// refer: https://developer.work.weixin.qq.com/document/path/91770
var WEWORK_WEBHOOK = os.Getenv("WEWORK_WEBHOOK")

var issueCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "reviewbot_issue_found_total",
	Help: "issue found by linter",
}, []string{"repo", "linter", "pull_request", "commit"})

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

// notify sends message to wework group
// refer: https://developer.work.weixin.qq.com/document/path/91770
func notify(message MessageBody) error {
	if WEWORK_WEBHOOK == "" || (message.Text.Content == "" && message.Markdown.Content == "") {
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

// notifyAsync sends message to wework group asynchronously.
func notifyAsync(message MessageBody) {
	go func() {
		if err := notify(message); err != nil {
			log.Infof("send message failed, err: %v, message: %v\n", err, message)
		}
	}()
}

// NotifyWebhookByText sends text message to wework group.
func NotifyWebhookByText(content string) {
	notifyAsync(MessageBody{
		MsgType: "text",
		Text:    MsgContent{Content: content},
	})
}

// NotifyWebhookByMarkdown sends markdown message to wework group.
func NotifyWebhookByMarkdown(content string) {
	notifyAsync(MessageBody{
		MsgType: "markdown",
		Markdown: MsgContent{
			Content: content,
		},
	})
}
