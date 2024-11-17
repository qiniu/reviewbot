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
	"fmt"
	"regexp"
	"strconv"
)

type HunkChecker interface {
	InHunk(file string, line int) bool
}

type FileHunkChecker struct {
	Hunks map[string][]Hunk
}

type Hunk struct {
	StartLine int
	EndLine   int
}

// NewFileHunkChecker creates a new FileHunkChecker with given hunks map.
func NewFileHunkChecker(hunks map[string][]Hunk) *FileHunkChecker {
	return &FileHunkChecker{
		Hunks: hunks,
	}
}

func (c *FileHunkChecker) InHunk(file string, line, startLine int) bool {
	if hunks, ok := c.Hunks[file]; ok {
		for _, hunk := range hunks {
			if startLine != 0 {
				if startLine >= hunk.StartLine && line <= hunk.EndLine {
					return true
				}
			} else if line >= hunk.StartLine && line <= hunk.EndLine {
				return true
			}
		}
	}
	return false
}

// ParsePatch parses a unified diff patch string and returns hunks.
func ParsePatch(patch string) ([]Hunk, error) {
	hunks := make([]Hunk, 0)

	groups := patchRegex.FindAllStringSubmatch(patch, -1)
	for _, group := range groups {
		if len(group) != 5 {
			return nil, fmt.Errorf("invalid patch: %s", patch)
		}
		hunkStartLine, err := strconv.Atoi(group[3])
		if err != nil {
			return nil, fmt.Errorf("invalid patch: %s, hunkStartLine: %s", patch, group[3])
		}

		hunkLength, err := strconv.Atoi(group[4])
		if err != nil {
			return nil, fmt.Errorf("invalid patch: %s, hunkLength: %s", patch, group[4])
		}

		hunks = append(hunks, Hunk{
			StartLine: hunkStartLine,
			EndLine:   hunkStartLine + hunkLength - 1,
		})
	}

	return hunks, nil
}

var patchRegex = regexp.MustCompile(`@@ \-(\d+),(\d+) \+(\d+),(\d+) @@`)
