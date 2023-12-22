package main

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/google/go-github/v57/github"
)

type HunkChecker interface {
	InHunk(file string, line int) bool
}

type GithubCommitFileHunkChecker struct {
	// map[file][]hunks
	Hunks map[string][]Hunk
}

func NewGithubCommitFileHunkChecker(commitFiles []*github.CommitFile) (*GithubCommitFileHunkChecker, error) {
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

		_, ok := hunks[commitFile.GetFilename()]
		if ok {
			return nil, fmt.Errorf("duplicate commitFile: %v", commitFile)
		}

		hunks[commitFile.GetFilename()] = fileHunks
	}

	return &GithubCommitFileHunkChecker{
		Hunks: hunks,
	}, nil
}

func (c *GithubCommitFileHunkChecker) InHunk(file string, line int) bool {
	if hunks, ok := c.Hunks[file]; ok {
		for _, hunk := range hunks {
			if line >= hunk.StartLine && line <= hunk.EndLine {
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
