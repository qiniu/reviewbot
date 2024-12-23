package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Linter struct {
	Name    string
	RepoURL string
	Stars   int
}

type GithubResponse struct {
	StargazersCount int `json:"stargazers_count"`
}

type GitLabResponse struct {
	StarCount int `json:"star_count"`
}

func main() {
	// 获取 golangci-lint 文档页面
	doc, err := fetchDocument("https://golangci-lint.run/usage/linters/")
	if err != nil {
		log.Fatal(err)
	}

	linters := extractLinters(doc)

	// 获取每个仓库的 star 数
	for i := range linters {
		stars, err := getStars(linters[i].RepoURL)
		if err != nil {
			// still note the linter
			stars = 0
		}
		linters[i].Stars = stars
		// GitHub API 限制，添加延时
		time.Sleep(time.Second * 2)
		log.Printf("linter: %v, stars: %d, repo: %s", linters[i].Name, linters[i].Stars, linters[i].RepoURL)
	}
}

func fetchDocument(url string) (*goquery.Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return goquery.NewDocumentFromReader(resp.Body)
}

func extractLinters(doc *goquery.Document) []Linter {
	var linters []Linter

	doc.Find("table tbody tr").Each(func(i int, s *goquery.Selection) {
		// 获取第一个 td 中的 linter 名称
		nameCell := s.Find("td").First()
		name := strings.TrimSpace(nameCell.Find("a").First().Text())
		if name == "" {
			return
		}

		// 在同一个 td 中查找 GitHub 链接
		repoURL := ""
		nameCell.Find("a[href*='http']").Each(func(i int, link *goquery.Selection) {
			href, exists := link.Attr("href")
			if exists {
				repoURL = href
			}
		})

		linters = append(linters, Linter{
			Name:    name,
			RepoURL: repoURL,
		})
	})

	return linters
}

func getStars(repoURL string) (int, error) {
	if strings.Contains(repoURL, "github.com") {
		return getGitHubStars(repoURL)
	} else if strings.Contains(repoURL, "gitlab.com") {
		return getGitLabStars(repoURL)
	}
	return 0, fmt.Errorf("not a github or gitlab URL")
}

func getGitHubStars(repoURL string) (int, error) {
	re := regexp.MustCompile(`github\.com/([^/]+/[^/]+)`)
	matches := re.FindStringSubmatch(repoURL)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid github URL format")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s", matches[1])
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, err
	}

	// 添加 GitHub token 支持
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result GithubResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.StargazersCount, nil
}

func getGitLabStars(repoURL string) (int, error) {
	re := regexp.MustCompile(`gitlab\.com/([^/]+/[^/]+)`)
	matches := re.FindStringSubmatch(repoURL)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid gitlab URL format")
	}

	// GitLab API 需要项目路径进行 URL 编码
	projectPath := url.QueryEscape(matches[1])
	apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", projectPath)

	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result GitLabResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.StarCount, nil
}
