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

package linters

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/x/xlog"
)

func TestFormatStaticcheckLine(t *testing.T) {
	tc := []struct {
		input    string
		expected *LinterOutput
	}{

		{"cdn.v1/ratelimit/limiter/memory.go:134: 134-161 lines are duplicate of", &LinterOutput{
			File:    "cdn.v1/ratelimit/limiter/memory.go",
			Line:    134,
			Column:  0,
			Message: "134-161 lines are duplicate of",
		}},
		{"config/config_test.go:11: Function 'TestConfig' is too long (129 > 60) (funlen)", &LinterOutput{
			File:    "config/config_test.go",
			Line:    11,
			Column:  0,
			Message: "Function 'TestConfig' is too long (129 > 60) (funlen)",
		}},
		{"cdn-admin.v2/client/dns/dnsapi.go:59:3: assignment to err", &LinterOutput{
			File:    "cdn-admin.v2/client/dns/dnsapi.go",
			Line:    59,
			Column:  3,
			Message: "assignment to err",
		}},
		{"smart_scheduler/provider_scheduler/provider_manager/provider_manager.go:207:31: should use make([]float64, len(result.CDNLog.Points)) instead (S1019)", &LinterOutput{
			File:    "smart_scheduler/provider_scheduler/provider_manager/provider_manager.go",
			Line:    207,
			Column:  31,
			Message: "should use make([]float64, len(result.CDNLog.Points)) instead (S1019)",
		}},
		{"cdn-admin.v2/api/api_line.go:342:3: should replace loop with ret = append(ret, scope.EdgeNodes...) (S1011)", &LinterOutput{
			File:    "cdn-admin.v2/api/api_line.go",
			Line:    342,
			Column:  3,
			Message: "should replace loop with ret = append(ret, scope.EdgeNodes...) (S1011)",
		}},
		{"cdn-admin.v2/api/api_line.go:342 should replace loop with ret = append(ret, scope.EdgeNodes...)", nil},
	}

	for _, c := range tc {
		output, err := GeneralLineParser(c.input)
		if output == nil {
			if c.expected != nil {
				t.Errorf("expected: %v, got: %v", c.expected, output)
			}
			continue
		}

		if c.expected == nil && output != nil {
			t.Errorf("expected error, got: %v", output)
		}

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if output.File != c.expected.File || output.Line != c.expected.Line || output.Column != c.expected.Column || output.Message != c.expected.Message {
			t.Errorf("expected: %v, got: %v", c.expected, output)
		}
	}
}

func TestIsEmpty(t *testing.T) {
	tcs := []struct {
		input    []string
		expected bool
	}{
		{[]string{}, true},
		{[]string{""}, true},
		{[]string{" "}, false},
		{[]string{"a"}, false},
	}
	for _, tc := range tcs {
		actual := IsEmpty(tc.input...)
		if actual != tc.expected {
			t.Errorf("expected: %v, got: %v", tc.expected, actual)
		}
	}
}

func TestConstructGotchaMessage(t *testing.T) {
	tcs := []struct {
		Linter        string
		PR            string
		Link          string
		linterResults map[string][]LinterOutput
		expected      string
	}{
		{
			Linter: "golangci-lint",
			PR:     "1",
			Link:   "http://",
			linterResults: map[string][]LinterOutput{
				"cdn-admin.v2/client/dns/dnsapi.go": {
					{
						File:    "cdn-admin.v2/client/dns/dnsapi.go",
						Line:    59,
						Column:  3,
						Message: "assignment to err",
					},
				},
			},
			expected: fmt.Sprintf("✅ Linter: %v \nPR:   %v \nLink: %v \nDetails:\n%v\n", "golangci-lint", "1", "http://", "cdn-admin.v2/client/dns/dnsapi.go:59:3: assignment to err\n"),
		},
		{
			Linter:        "golangci-lint",
			PR:            "",
			Link:          "",
			linterResults: map[string][]LinterOutput{},
			expected:      "",
		},
	}

	for _, tc := range tcs {
		actual := ConstructGotchaMsg(tc.Linter, tc.PR, tc.Link, tc.linterResults)
		if !reflect.DeepEqual(tc.expected, actual) {
			t.Errorf("expected: %v, got: %v", tc.expected, actual)
		}
	}
}

func TestConstructUnknownMsg(t *testing.T) {
	tcs := []struct {
		Linter   string
		Repo     string
		PR       string
		Event    string
		message  string
		expected string
	}{
		{
			Linter:   "golangci-lint",
			Repo:     "cdn-admin.v2",
			PR:       "1",
			Event:    "opened",
			message:  "message",
			expected: fmt.Sprintf("😱🚀 Linter: %v \nRepo: %v \nPR:   %v \nEvent: %v \nUnexpected: %v\n", "golangci-lint", "cdn-admin.v2", "1", "opened", "message"),
		},
	}

	for _, tc := range tcs {
		actual := ConstructUnknownMsg(tc.Linter, tc.Repo, tc.PR, tc.Event, tc.message)
		if !reflect.DeepEqual(tc.expected, actual) {
			t.Errorf("expected: %v, got: %v", tc.expected, actual)
		}
	}
}

func TestExecRun(t *testing.T) {
	tp := true
	tcs := []struct {
		id     string
		input  Agent
		output []byte
		err    error
	}{
		{
			id: "case1 - without ARTIFACT",
			input: Agent{
				LinterName: "ut",
				LinterConfig: config.Linter{
					Enable:       &tp,
					Command:      []string{"/bin/bash", "-c", "--"},
					Args:         []string{"echo file:line:column:message"},
					ReportFormat: config.Quiet,
				},
			},
			output: []byte("file:line:column:message\n"),
			err:    nil,
		},
		{
			id: "case2 - with ARTIFACT",
			input: Agent{
				LinterName: "ut",
				LinterConfig: config.Linter{
					Enable:       &tp,
					Command:      []string{"/bin/bash", "-c", "--"},
					Args:         []string{"echo file2:6:7:message >> $ARTIFACT/golangci-lint.log 2>&1"},
					ReportFormat: config.Quiet,
				},
			},
			output: []byte("file2:6:7:message\n"),
			err:    nil,
		},
		{
			id: "case2 - with multi files under ARTIFACT",
			input: Agent{
				LinterName: "ut",
				LinterConfig: config.Linter{
					Enable:       &tp,
					Command:      []string{"/bin/bash", "-c", "--"},
					Args:         []string{"echo file2:6:7:message >> $ARTIFACT/golangci-lint.log 2>&1 ;echo file3:6:7:message >> $ARTIFACT/golangci-lint.log 2>&1"},
					ReportFormat: config.Quiet,
				},
			},
			output: []byte("file2:6:7:message\nfile3:6:7:message\n"),
			err:    nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			output, err := ExecRun(tc.input)
			if !errors.Is(err, tc.err) {
				t.Errorf("expected: %v, got: %v", tc.err, err)
			}

			if string(output) != string(tc.output) {
				t.Errorf("expected: %v, got: %v", string(tc.output), string(output))
			}
		})
	}
}

func TestParseV2(t *testing.T) {
	tcs := []struct {
		id         string
		input      []byte
		trainer    func(LinterOutput) (*LinterOutput, []string)
		expected   map[string][]LinterOutput
		unexpected []string
	}{
		{
			id:      "case1 - general case",
			input:   []byte("file.txt:6:7: message\nfile2.txt:6:7: message\nfile2.txt:7:9: message\n"),
			trainer: nil,
			expected: map[string][]LinterOutput{
				"file.txt": []LinterOutput{
					{
						File:    "file.txt",
						Line:    6,
						Column:  7,
						Message: "message",
					},
				},
				"file2.txt": []LinterOutput{
					{
						File:    "file2.txt",
						Line:    6,
						Column:  7,
						Message: "message",
					},
					{
						File:    "file2.txt",
						Line:    7,
						Column:  9,
						Message: "message",
					},
				},
			},
			unexpected: []string{},
		},
		{
			id: "case2 - complex case",
			input: []byte(`
level=warning msg="[linters_context] copyloopvar"
level=error msg="[linters_context]"
golangci_lint.go:16:1: warning: (gochecknoglobals)
xxx.go:18:1: warning: xxxx
xxx.gox (gochecknoglobals)
golangci_lint.go:17:1: warning: (gochecknoglobals)
io/upv2/upform/internal/mime/multipart/multipart_test.go:744:7: wrapperFunc: use strings.ReplaceAll method in ` + "`strings.Replace(`--08b84578eabc563dcba96\n7a945cd<...>: application/octet-stream\n\n`, \"\\n\", \"\\r\\n\", -1)` (gocritic)"),
			trainer: nil,
			expected: map[string][]LinterOutput{
				"golangci_lint.go": []LinterOutput{
					{
						File:    "golangci_lint.go",
						Line:    16,
						Column:  1,
						Message: "warning: (gochecknoglobals)",
					},
					{
						File:    "golangci_lint.go",
						Line:    17,
						Column:  1,
						Message: "warning: (gochecknoglobals)",
					},
				},
				"xxx.go": []LinterOutput{
					{
						File:    "xxx.go",
						Line:    18,
						Column:  1,
						Message: "warning: xxxx\nxxx.gox (gochecknoglobals)",
					},
				},
				"io/upv2/upform/internal/mime/multipart/multipart_test.go": []LinterOutput{
					{
						File:    "io/upv2/upform/internal/mime/multipart/multipart_test.go",
						Line:    744,
						Column:  7,
						Message: "wrapperFunc: use strings.ReplaceAll method in `strings.Replace(`--08b84578eabc563dcba96\n7a945cd<...>: application/octet-stream\n\n`, \"\\n\", \"\\r\\n\", -1)` (gocritic)",
					},
				},
			},
			unexpected: []string{"level=warning msg=\"[linters_context] copyloopvar\"", "level=error msg=\"[linters_context]\""},
		},
		{
			id:         "case3 - invalid input",
			input:      []byte("case3 - invalid input"),
			trainer:    nil,
			expected:   nil,
			unexpected: []string{"case3 - invalid input"},
		},
		{
			id:      "case4 - msg reached max length",
			input:   []byte("file.txt:6:7: message\nfile2.txt:6:7: "),
			trainer: nil,
			expected: map[string][]LinterOutput{
				"file.txt": []LinterOutput{
					{
						File:    "file.txt",
						Line:    6,
						Column:  7,
						Message: "message",
					},
				},
				"file2.txt": []LinterOutput{
					{
						File:    "file2.txt",
						Line:    6,
						Column:  7,
						Message: "",
					},
				},
			},
			unexpected: []string{},
		},
		{
			id:      "case5 - duplicate",
			input:   []byte("file.txt:6:7: message\nfile.txt:6:7: message\n"),
			trainer: nil,
			expected: map[string][]LinterOutput{
				"file.txt": []LinterOutput{
					{
						File:    "file.txt",
						Line:    6,
						Column:  7,
						Message: "message",
					},
				},
			},
			unexpected: []string{},
		},
		{
			id:    "case6 - with trainer",
			input: []byte("file.txt:6:7: message\nfile2.txt:7:9: message\n"),
			trainer: func(o LinterOutput) (*LinterOutput, []string) {
				if o.Line == 6 {
					return nil, []string{fmt.Sprintf("%s:%d:%d: %s", o.File, o.Line, o.Column, o.Message)}
				}
				return &o, nil
			},
			expected: map[string][]LinterOutput{
				"file2.txt": []LinterOutput{
					{
						File:    "file2.txt",
						Line:    7,
						Column:  9,
						Message: "message",
					},
				},
			},
			unexpected: []string{"file.txt:6:7: message"},
		},
		{
			id:      "case7 - no column",
			input:   []byte("file.txt:6: message\nfile2.txt:7: message\n"),
			trainer: nil,
			expected: map[string][]LinterOutput{
				"file.txt": []LinterOutput{
					{
						File:    "file.txt",
						Line:    6,
						Column:  0,
						Message: "message",
					},
				},
				"file2.txt": []LinterOutput{
					{
						File:    "file2.txt",
						Line:    7,
						Column:  0,
						Message: "message",
					},
				},
			},
			unexpected: []string{},
		},
		{
			id: "case8 - unexpected file format",
			input: []byte(`
abc x:18:1: warning: xxxx
golangci_lint.go:16:1: warning: (gochecknoglobals)
`),
			trainer: nil,
			expected: map[string][]LinterOutput{
				"golangci_lint.go": []LinterOutput{
					{
						File:    "golangci_lint.go",
						Line:    16,
						Column:  1,
						Message: "warning: (gochecknoglobals)",
					},
				},
			},
			unexpected: []string{},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.id, func(t *testing.T) {
			got, unexpected := ParseV2(xlog.New("ut"), tc.input, tc.trainer)
			if !reflect.DeepEqual(tc.expected, got) {
				t.Errorf("expected: %v, got: %v", tc.expected, got)
			}

			if !reflect.DeepEqual(tc.unexpected, unexpected) {
				t.Errorf("expected: %v, got: %v", tc.unexpected, unexpected)
			}
		})
	}
}
