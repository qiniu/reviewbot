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

	"github.com/google/go-github/v57/github"
	"github.com/qiniu/x/errors"
	"github.com/qiniu/x/log"
	"github.com/xanzy/go-gitlab"
)

type HunkChecker interface {
	InHunk(file string, line int) bool
}

type FileHunkChecker struct {
	// map[file][]hunks
	Hunks map[string][]Hunk
}

var (
	errCommitFile = errors.New("commit file error")
)

func NewFileHunkChecker(commitFiles []*github.CommitFile) (*FileHunkChecker, error) {
	hunks := make(map[string][]Hunk)
	for _, commitFile := range commitFiles {
		if commitFile == nil || commitFile.GetPatch() == "" {
			continue
		}

		if commitFile.GetStatus() == "removed" {
			continue
		}

		fileHunks, err := DiffHunks(commitFile)
		if err != nil {
			return nil, err
		}

		v, ok := hunks[commitFile.GetFilename()]
		if ok {
			log.Warnf("duplicate commitFiles: %v, %v", commitFile, v)
			continue
		}

		hunks[commitFile.GetFilename()] = fileHunks
	}

	return &FileHunkChecker{
		Hunks: hunks,
	}, nil
}
func NewGitLabCommitFileHunkChecker(commitFiles []*gitlab.MergeRequestDiff) (*FileHunkChecker, error) {
	hunks := make(map[string][]Hunk)
	for _, commitFile := range commitFiles {
		if commitFile == nil || commitFile.NewPath == "" {
			continue
		}
		if commitFile.DeletedFile {
			continue
		}
		fileHunks, err := DiffHunksMerge(commitFile)
		if err != nil {
			return nil, err
		}
		v, ok := hunks[commitFile.NewPath]
		if ok {
			log.Warnf("duplicate commitFiles: %v, %v", commitFile, v)
			continue
		}
		hunks[commitFile.NewPath] = fileHunks
	}

	return &FileHunkChecker{
		Hunks: hunks,
	}, nil
}
func DiffHunksMerge(commitFile *gitlab.MergeRequestDiff) ([]Hunk, error) {
	if commitFile == nil || commitFile.NewPath == "" {
		log.Errorf("invalid commitFile: %v", commitFile)
		return nil, errCommitFile
	}
	return ParsePatch(commitFile.Diff)
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

var patchRegex = regexp.MustCompile(`@@ \-(\d+),(\d+) \+(\d+),(\d+) @@`)

func ParsePatch(patch string) ([]Hunk, error) {
	var hunks []Hunk

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

type Hunk struct {
	StartLine int
	EndLine   int
}

func DiffHunks(commitFile *github.CommitFile) ([]Hunk, error) {
	if commitFile == nil || commitFile.GetPatch() == "" {
		return nil, fmt.Errorf("invalid commitFile: %v", commitFile)
	}

	return ParsePatch(commitFile.GetPatch())
}
