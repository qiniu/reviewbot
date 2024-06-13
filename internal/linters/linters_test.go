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
	"testing"
)

func TestFormatStaticcheckLine(t *testing.T) {
	tc := []struct {
		input    string
		expected *LinterOutput
	}{
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

func TestConstructMKAlertMessage(t *testing.T) {
	tcs := []struct {
		Linter        string
		PR            string
		Link          string
		linterResults map[string][]LinterOutput
		expected      string
	}{
		{
			Linter: "golangci-lint",
			PR:     "",
			Link:   "",
			linterResults: map[string][]LinterOutput{
				"cdn-admin.v2/client/dns/dnsapi.go": {
					{
						File:    "cdn-admin.v2/client/dns/dnsapi.go",
						Line:    59,
						Column:  3,
						Message: "assignment to err",
					},
				},
				"provider_manager.go": {
					{
						File:    "provider_manager/provider_manager.go",
						Line:    207,
						Column:  31,
						Message: "should use make([]float64, len(result.CDNLog.Points)) instead (S1019)",
					},
				},
			},
			expected: `{"msgtype":"text","text":{"content":"Linter: golangci-lint \nPR:    \nLink:  \nDetails:\ncdn-admin.v2/client/dns/dnsapi.go:59:3: assignment to err\nprovider_manager.go:207:31: should use make([]float64, len(result.CDNLog.Points)) instead (S1019)\n\n"}}`,
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
		actual := constructMKAlertMessage(tc.Linter, tc.PR, tc.Link, tc.linterResults)
		if actual != tc.expected {
			t.Errorf("expected: %v, got: %v", tc.expected, actual)
		}
	}
}
