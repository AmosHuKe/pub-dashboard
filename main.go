// pub.dev Package 仪表盘
//
// 识别并更新指定 [filename] Markdown 文件中的特定占位内容，
//
// 特定占位:
//   - `<!-- md:PubDashboard begin --><!-- md:PubDashboard end -->`              仪表盘表格（Markdown 格式）
//   - `<!-- md:PubDashboard-total begin --><!-- md:PubDashboard-total end -->`  Package 数量
//
// 使用:
//   - `go run main.go -githubToken xxx -filename xxx -publisherList xxx -packageList xxx -sortField xxx -sortMode xxx`
//
// 参数:
//   - [githubToken]    拥有 repo 权限的 Github 令牌
//   - [filename]       需要更新的 Markdown 文件，例如："README.md" "test/test.md"
//   - [publisherList]  Publisher 名称列表 (`,`逗号分割) ，例如："aa,bb,cc"
//   - [packageList]    Package 名称列表 (`,`逗号分割)，例如："aa,bb,cc"
//   - [sortField]      排序字段 可选：name(default) | published | pubLikes | pubDownloads | githubStars
//   - [sortMode]       排序方式 可选：asc(default) | desc
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// maxConcurrency 是 package 级别的并发抓取上限。
	// 需要按 pub.dev 和 GitHub 限流取保守值。
	maxConcurrency = 8
	// maxAttempts 是单个 HTTP 请求的最大尝试次数（含首次）。
	maxAttempts = 3
	// httpTimeout 是单个 HTTP 请求的超时时间，防止请求挂起拖跨整个 Action。
	httpTimeout = 30 * time.Second
	// retryBaseDelay 是重试的基础退避时长（指数增长）。
	retryBaseDelay = 500 * time.Millisecond
)

// 主 MarkdownTable 用于存储每个 package 在 Markdown 表格中的展示信息
type MarkdownTable struct {
	Name                   string
	Version                string
	Description            string
	LicenseName            string
	Platform               string
	Published              string
	GithubStars            string
	PubLikes               string
	PubPoints              string
	PubDownloadCount30Days string
	Issues                 string
	PullRequests           string
	Contributors           string
}

// 主 Package 信息，聚合 package 所有相关的数据
type PackageInfo struct {
	Code                   int // 0: error 1：success
	Name                   string
	Version                string
	Description            string
	Homepage               string
	Repository             string
	IssueTracker           string
	Published              string
	GithubUser             string
	GithubRepo             string
	GithubBaseInfo         GithubBaseInfo
	GithubContributorsInfo []GithubContributorsInfo
	ScoreInfo              PackageScoreInfo
}

// 每个 package 对应 Github 仓库的基础信息
type GithubBaseInfo struct {
	StargazersCount float64 `json:"stargazers_count"`
	ForksCount      float64 `json:"forks_count"`
	OpenIssuesCount float64 `json:"open_issues_count"`
	License         struct {
		Name string `json:"name"`
	} `json:"license"`
	ContributorsTotal int
}

// 每个 package 对应 Github 仓库的贡献者基础信息
type GithubContributorsInfo struct {
	Login     string `json:"login"`
	Id        int    `json:"id"`
	AvatarUrl string `json:"avatar_url"`
	HtmlUrl   string `json:"html_url"`
	Type      string `json:"type"`
}

// Pub.dev package 基础信息
type PackageBaseInfo struct {
	Name   string `json:"name"`
	Latest struct {
		Pubspec struct {
			Version      string `json:"version"`
			Description  string `json:"description"`
			Homepage     string `json:"homepage"`
			Repository   string `json:"repository"`
			IssueTracker string `json:"issue_tracker"`
		} `json:"pubspec"`
		Published string `json:"published"`
	} `json:"latest"`
}

// Pub.dev package 评分相关信息
type PackageScoreInfo struct {
	GrantedPoints       float64  `json:"grantedPoints"`
	MaxPoints           float64  `json:"maxPoints"`
	LikeCount           float64  `json:"likeCount"`
	DownloadCount30Days int      `json:"downloadCount30Days"`
	Tags                []string `json:"tags"`
	LastUpdated         string   `json:"lastUpdated"`
	TagsPlatform        []string
}

// Pub.dev publisher 下所有 package 信息
type PublisherInfo struct {
	Packages []struct {
		Package string `json:"package"`
	} `json:"packages"`
	Next string `json:"next"`
}

func main() {
	var githubToken, filename, publisherList, packageList, sortField, sortMode string
	flag.StringVar(&githubToken, "githubToken", "Github Token with repo permissions", "Github Token with repo permissions")
	flag.StringVar(&filename, "filename", "README.md", "文件名 如: README.md")
	flag.StringVar(&publisherList, "publisherList", "", "publisher 如: aa,bb,cc")
	flag.StringVar(&packageList, "packageList", "", "package 如: aa,bb,cc")
	flag.StringVar(&sortField, "sortField", "name", "name | published | pubLikes | pubDownloads | githubStars")
	flag.StringVar(&sortMode, "sortMode", "asc", "asc | desc")
	flag.Parse()

	ctx := context.Background()
	client := newHTTPClient()

	packageNames, err := mergePackageList(ctx, client, publisherList, packageList)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	packageInfoList, err := getPackageInfo(ctx, client, githubToken, packageNames)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	sortPackageInfo(packageInfoList, sortField, sortMode)
	markdownTable := assembleMarkdownTable(packageInfoList, sortField)

	// 更新表格
	if err := updateMarkdownTable(filename, markdownTable); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// 更新总数
	if err := updateMarkdownPackageTotal(filename, len(packageInfoList)); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// 合并 publisher 的 package 和自定义 package 列表，并去重（保持顺序）
//
// 参数:
//   - [ctx]           上下文
//   - [client]        共享 HTTP Client
//   - [publisherList] publisher 名称列表（逗号,分割）
//   - [packageList]   package 名称列表（逗号,分割）
//
// 返回值:
//   - 合并去重后的 package 名称列表
func mergePackageList(ctx context.Context, client *http.Client, publisherList string, packageList string) ([]string, error) {
	publisherPackages, err := getPublisherPackages(ctx, client, publisherList)
	if err != nil {
		return nil, err
	}
	all := append(publisherPackages, strings.Split(packageList, ",")...)
	return removeDuplicates(all), nil
}

// 通过 Publisher 获取所有 Package 名称
//
// 参数:
//   - [ctx]           上下文
//   - [client]        共享 HTTP Client
//   - [publisherName] publisher 列表（逗号,分割）
//
// 返回值:
//   - package 名称列表
func getPublisherPackages(ctx context.Context, client *http.Client, publisherName string) ([]string, error) {
	printErrTitle := "🌏⚠️ PublisherPackages: "
	if strings.TrimSpace(publisherName) == "" {
		return nil, nil
	}
	publisherList := removeDuplicates(strings.Split(publisherName, ","))
	fmt.Println("🌏", publisherList)
	packageNameList := []string{}
	for _, publisher := range publisherList {
		// 逐页查询，直至返回空结果
		for pageIndex := 1; ; pageIndex++ {
			fmt.Printf("🌏🔗 Publisher: %s, Page: %d \n", publisher, pageIndex)
			rawURL := fmt.Sprintf("https://pub.dev/api/search?q=publisher:%s&page=%d&sort=downloads", url.QueryEscape(publisher), pageIndex)
			body, status, err := httpGetWithRetry(ctx, client, rawURL, nil)
			if err != nil {
				return nil, fmt.Errorf("%s%w", printErrTitle, err)
			}
			// http.StatusNotFound 		不存在更多数据
			// http.StatusBadRequest 	可能超出最多 10 页的限制
			if status == http.StatusNotFound || status == http.StatusBadRequest {
				break // 无更多结果
			}
			if status != http.StatusOK {
				return nil, fmt.Errorf("%s%s: unexpected status %d", printErrTitle, publisher, status)
			}
			var data PublisherInfo
			if err := json.Unmarshal(body, &data); err != nil {
				return nil, fmt.Errorf("%s%w", printErrTitle, err)
			}
			if len(data.Packages) == 0 {
				break
			}
			for _, packageName := range data.Packages {
				if packageName.Package != "" {
					packageNameList = append(packageNameList, packageName.Package)
				}
			}
		}
	}
	return removeDuplicates(packageNameList), nil
}

// 获取所有 Package 信息（并发抓取）
//
// 以 [maxConcurrency] 为上限并发处理每个 package，结果按输入顺序返回，保证排序前顺序确定。
// 任一 package 抓取失败将取消其余请求并整体返回错误。
//
// 参数:
//   - [ctx]          上下文
//   - [client]       共享 HTTP Client
//   - [githubToken]  Github Token
//   - [packageNames] package 名称列表（已去重清洗）
//
// 返回值:
//   - [PackageInfo] 列表（与 packageNames 顺序一致）
func getPackageInfo(ctx context.Context, client *http.Client, githubToken string, packageNames []string) ([]PackageInfo, error) {
	fmt.Println("📦", packageNames)
	return concurrentMap(ctx, packageNames, maxConcurrency, func(ctx context.Context, name string) (PackageInfo, error) {
		fmt.Println("📦🔥 " + name)
		info, err := fetchPackage(ctx, client, githubToken, name)
		if err != nil {
			return PackageInfo{}, err
		}
		if info.Code == 1 {
			fmt.Printf("📦✅ %s, Code: 1\n", name)
		} else {
			fmt.Printf("📦❌ %s, Code: 0\n", name)
		}
		return info, nil
	})
}

// 抓取单个 package 的全部信息（pub 基础信息 -> 评分 -> Github 信息）
//
// 参数:
//   - [ctx]         上下文
//   - [client]      共享 HTTP Client
//   - [githubToken] Github Token
//   - [name]        package 名称
//
// 返回值:
//   - [PackageInfo]，包不存在时 Code=0（降级展示为 ⁉️，非错误）
func fetchPackage(ctx context.Context, client *http.Client, githubToken string, name string) (PackageInfo, error) {
	printErrTitle := "📦⚠️ PackageInfo: "
	body, status, err := httpGetWithRetry(ctx, client, fmt.Sprintf("https://pub.dev/api/packages/%s", name), nil)
	if err != nil {
		return PackageInfo{}, fmt.Errorf("%s%w", printErrTitle, err)
	}
	// 404：包不存在 -> 降级
	if status == http.StatusNotFound {
		return PackageInfo{Code: 0, Name: name}, nil
	}
	if status != http.StatusOK {
		return PackageInfo{}, fmt.Errorf("%s%s: unexpected status %d", printErrTitle, name, status)
	}
	var data PackageBaseInfo
	if err := json.Unmarshal(body, &data); err != nil {
		return PackageInfo{}, fmt.Errorf("%s%w", printErrTitle, err)
	}
	if data.Name == "" {
		return PackageInfo{Code: 0, Name: name}, nil
	}

	packageInfo := PackageInfo{
		Code:         1,
		Name:         data.Name,
		Version:      data.Latest.Pubspec.Version,
		Description:  data.Latest.Pubspec.Description,
		Homepage:     data.Latest.Pubspec.Homepage,
		Repository:   data.Latest.Pubspec.Repository,
		IssueTracker: data.Latest.Pubspec.IssueTracker,
		Published:    data.Latest.Published,
	}

	scoreInfo, err := getPackageScoreInfo(ctx, client, data.Name)
	if err != nil {
		return PackageInfo{}, err
	}
	packageInfo.ScoreInfo = scoreInfo

	if err := getGithubInfo(ctx, client, githubToken, &packageInfo); err != nil {
		return PackageInfo{}, err
	}
	return packageInfo, nil
}

// 获取 Package score 信息
//
// 参数:
//   - [ctx]         上下文
//   - [client]      共享 HTTP Client
//   - [packageName] 单个 package 名称
//
// 返回值:
//   - [PackageScoreInfo] 信息（404 时降级为空）
func getPackageScoreInfo(ctx context.Context, client *http.Client, packageName string) (PackageScoreInfo, error) {
	printErrTitle := "📦⚠️ PackageScoreInfo: "
	body, status, err := httpGetWithRetry(ctx, client, fmt.Sprintf("https://pub.dev/api/packages/%s/score", packageName), nil)
	if err != nil {
		return PackageScoreInfo{}, fmt.Errorf("%s%w", printErrTitle, err)
	}
	if status == http.StatusNotFound {
		return PackageScoreInfo{}, nil // 无评分数据 -> 降级
	}
	if status != http.StatusOK {
		return PackageScoreInfo{}, fmt.Errorf("%s%s: unexpected status %d", printErrTitle, packageName, status)
	}
	var data PackageScoreInfo
	if err := json.Unmarshal(body, &data); err != nil {
		return PackageScoreInfo{}, fmt.Errorf("%s%w", printErrTitle, err)
	}

	// 获取 Tags 相关内容
	for _, value := range data.Tags {
		tag := strings.SplitN(value, ":", 2)
		// TagsPlatform
		if len(tag) == 2 && tag[0] == "platform" {
			data.TagsPlatform = append(data.TagsPlatform, tag[1])
		}
	}
	return data, nil
}

// 获取 Github 信息，
// 处理 [PackageInfo] 中 GithubUser, GithubRepo, GithubBaseInfo, GithubContributorsInfo 的值
//
// 参数:
//   - [ctx]         上下文
//   - [client]      共享 HTTP Client
//   - [githubToken] Github Token
//   - [packageInfo] 当前 package 信息
func getGithubInfo(ctx context.Context, client *http.Client, githubToken string, packageInfo *PackageInfo) error {
	if packageInfo.Code == 0 {
		return nil
	}
	// 依次尝试 Repository、IssueTracker、Homepage 解析 Github 地址，取首个命中
	for _, link := range []string{packageInfo.Repository, packageInfo.IssueTracker, packageInfo.Homepage} {
		if user, repo := formatGithubInfo(link); repo != "" {
			packageInfo.GithubUser = user
			packageInfo.GithubRepo = repo
			break
		}
	}
	if packageInfo.GithubUser == "" || packageInfo.GithubRepo == "" {
		return nil
	}

	githubBaseInfo, err := getGithubBaseInfo(ctx, client, githubToken, packageInfo.GithubUser, packageInfo.GithubRepo)
	if err != nil {
		return err
	}
	packageInfo.GithubBaseInfo = githubBaseInfo

	githubContributorsInfo, contributorsTotal, err := getGithubContributorsInfo(ctx, client, githubToken, packageInfo.GithubUser, packageInfo.GithubRepo)
	if err != nil {
		return err
	}
	packageInfo.GithubContributorsInfo = githubContributorsInfo
	packageInfo.GithubBaseInfo.ContributorsTotal = contributorsTotal
	return nil
}

// 构造 GitHub API 通用请求头
func githubHeaders(githubToken string) map[string]string {
	return map[string]string{
		"Authorization":        "bearer " + githubToken,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2026-03-10",
	}
}

// 获取 Github 基础信息
//
// 参数:
//   - [ctx]         上下文
//   - [client]      共享 HTTP Client
//   - [githubToken] Github Token
//   - [user]        用户
//   - [repo]        仓库
//
// 返回值:
//   - [GithubBaseInfo] 信息（404 时降级为空）
func getGithubBaseInfo(ctx context.Context, client *http.Client, githubToken string, user string, repo string) (GithubBaseInfo, error) {
	printErrTitle := "📦⚠️ GithubBaseInfo: "
	rawURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", user, repo)
	body, status, err := httpGetWithRetry(ctx, client, rawURL, githubHeaders(githubToken))
	if err != nil {
		return GithubBaseInfo{}, fmt.Errorf("%s%w", printErrTitle, err)
	}
	if status == http.StatusNotFound {
		return GithubBaseInfo{}, nil // 仓库不存在 -> 降级
	}
	if status != http.StatusOK {
		return GithubBaseInfo{}, fmt.Errorf("%s%s/%s: unexpected status %d", printErrTitle, user, repo, status)
	}
	var data GithubBaseInfo
	if err := json.Unmarshal(body, &data); err != nil {
		return GithubBaseInfo{}, fmt.Errorf("%s%w", printErrTitle, err)
	}
	return data, nil
}

// 获取 Github 贡献者信息
//
// 参数:
//   - [ctx]         上下文
//   - [client]      共享 HTTP Client
//   - [githubToken] Github Token
//   - [user]        用户
//   - [repo]        仓库
//
// 返回值:
//   - [GithubContributorsInfo] 贡献者列表（前 3 位非 Bot）
//   - 贡献者总数（最多 100；404/204 时为 0）
func getGithubContributorsInfo(ctx context.Context, client *http.Client, githubToken string, user string, repo string) ([]GithubContributorsInfo, int, error) {
	printErrTitle := "📦⚠️ GithubContributorsInfo: "
	rawURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors?page=1&per_page=100", user, repo)
	body, status, err := httpGetWithRetry(ctx, client, rawURL, githubHeaders(githubToken))
	if err != nil {
		return nil, 0, fmt.Errorf("%s%w", printErrTitle, err)
	}
	// 404（仓库不存在）/ 204（空仓库，无贡献者）-> 降级
	if status == http.StatusNotFound || status == http.StatusNoContent {
		return nil, 0, nil
	}
	if status != http.StatusOK {
		return nil, 0, fmt.Errorf("%s%s/%s: unexpected status %d", printErrTitle, user, repo, status)
	}
	var data []GithubContributorsInfo
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, 0, fmt.Errorf("%s%w", printErrTitle, err)
	}

	githubContributorsInfo := []GithubContributorsInfo{}
	i := 1
	// 取前 3 位非 Bot 贡献者
	for _, value := range data {
		if i > 3 {
			break
		}
		if value.Type == "User" {
			githubContributorsInfo = append(githubContributorsInfo, value)
			i++
		}
	}
	return githubContributorsInfo, len(data), nil
}

// 匹配 github.com/ 之后的 user/repo 路径。
// `.` 已转义，避免误匹配 githubXcom 等相似域名。
var githubURLRegexp = regexp.MustCompile(`github\.com/(.+)`)

// 格式化 Github 信息
//
// 参数:
//   - [value] Github 链接
//
// 返回值:
//   - githubUser 信息
//   - githubRepo 信息
func formatGithubInfo(value string) (string, string) {
	var githubUser, githubRepo string
	result := githubURLRegexp.FindStringSubmatch(value)
	if len(result) >= 2 {
		info := strings.Split(result[1], "/")
		if len(info) >= 2 && info[0] != "" && info[1] != "" {
			githubUser = info[0]
			repo := info[1]
			// 去除 query/fragment 尾巴（如 ?tab=、#readme）及 .git 后缀
			if i := strings.IndexAny(repo, "#?"); i >= 0 {
				repo = repo[:i]
			}
			githubRepo = strings.TrimSuffix(repo, ".git")
		}
	}
	return githubUser, githubRepo
}

// 对 [packageInfoList] 排序
//
// 参数:
//   - [packageInfoList]  信息列表
//   - [sortField]        排序字段 可选：name(default) | published | pubLikes | pubDownloads | githubStars
//   - [sortMode]         排序方式 可选：asc(default) | desc
func sortPackageInfo(packageInfoList []PackageInfo, sortField string, sortMode string) {
	isDesc := sortMode == "desc"
	sort.SliceStable(packageInfoList, func(i, j int) bool {
		p1 := packageInfoList[i]
		p2 := packageInfoList[j]
		var result bool
		switch sortField {
		case "name":
			// 按照 pub 名称排序
			result = p1.Name < p2.Name
		case "published":
			// 按 pub 最新发布时间排序
			result = p1.Published > p2.Published
		case "pubLikes":
			// 按 pub likes 排序
			result = p1.ScoreInfo.LikeCount < p2.ScoreInfo.LikeCount
		case "pubDownloads":
			// 按 pub downloads 排序
			result = p1.ScoreInfo.DownloadCount30Days < p2.ScoreInfo.DownloadCount30Days
		case "githubStars":
			// 按 github stars 排序
			result = p1.GithubBaseInfo.StargazersCount < p2.GithubBaseInfo.StargazersCount
		default:
			result = p1.Name < p2.Name
		}
		if isDesc {
			return !result
		}
		return result
	})
}

// 组装表格内容
//
// 参数:
//   - [packageInfoList]  信息列表
//   - [sortField]        排序字段 可选：name(default) | published | pubLikes | pubDownloads | githubStars
//
// 返回值:
//   - markdown 表格内容
func assembleMarkdownTable(packageInfoList []PackageInfo, sortField string) string {
	markdownTableList := []MarkdownTable{}
	for _, value := range packageInfoList {
		var name, version, platform, licenseName, published,
			githubStars, pubLikes, pubPoints, pubDownloadCount30Days,
			issues, pullRequests, contributors string
		switch value.Code {
		case 0:
			// 无法获取信息
			name = value.Name + " ⁉️"
		case 1:
			// 已获取信息
			// Base
			const downloadIcon = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0icmdiYSgyNTUsMjU1LDI1NSwxKSI+PHBhdGggZmlsbD0ibm9uZSIgZD0iTTAgMGgyNHYyNEgweiI+PC9wYXRoPjxwYXRoIGQ9Ik0zIDE5SDIxVjIxSDNWMTlaTTEzIDEzLjE3MTZMMTkuMDcxMSA3LjEwMDVMMjAuNDg1MyA4LjUxNDcyTDEyIDE3TDMuNTE0NzIgOC41MTQ3Mkw0LjkyODkzIDcuMTAwNUwxMSAxMy4xNzE2VjJIMTNWMTMuMTcxNloiPjwvcGF0aD48L3N2Zz4="
			const pointIcon = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0icmdiYSgyNTUsMjU1LDI1NSwxKSI+PHBhdGggZmlsbD0ibm9uZSIgZD0iTTAgMGgyNHYyNEgweiI+PC9wYXRoPjxwYXRoIGQ9Ik0yMyAxMkwxNS45Mjg5IDE5LjA3MTFMMTQuNTE0NyAxNy42NTY5TDIwLjE3MTYgMTJMMTQuNTE0NyA2LjM0MzE3TDE1LjkyODkgNC45Mjg5NkwyMyAxMlpNMy44Mjg0MyAxMkw5LjQ4NTI4IDE3LjY1NjlMOC4wNzEwNyAxOS4wNzExTDEgMTJMOC4wNzEwNyA0LjkyODk2TDkuNDg1MjggNi4zNDMxN0wzLjgyODQzIDEyWiI+PC9wYXRoPjwvc3ZnPg=="

			name = "[" + value.Name + "](https://pub.dev/packages/" + value.Name + ")"
			version = "v" + value.Version
			platform = "<strong>Platform:</strong> "
			if len(value.ScoreInfo.TagsPlatform) > 0 {
				platform += strings.Join(value.ScoreInfo.TagsPlatform, ", ")
			} else {
				platform += "-"
			}
			published = "<strong>Published:</strong> " + value.Published
			githubStars = ""
			pubLikes = "[![Pub likes](https://img.shields.io/pub/likes/" + value.Name + "?style=social&logo=flutter&logoColor=168AFD&label=)](https://pub.dev/packages/" + value.Name + ")"
			pubPoints = "[![Pub points](https://img.shields.io/pub/points/" + value.Name + "?style=flat&label=&logo=" + pointIcon + ")](https://pub.dev/packages/" + value.Name + "/score)"
			pubDownloadCount30Days = "[![Pub downloads](https://img.shields.io/badge/" + formatDownloadCount(value.ScoreInfo.DownloadCount30Days) + url.PathEscape("/") + "month-4AC51C?style=flat&logo=" + downloadIcon + ")](https://pub.dev/packages/" + value.Name + ")"
			issues = "-"
			pullRequests = "-"

			// Github
			if value.GithubUser != "" && value.GithubRepo != "" {
				githubURL := value.GithubUser + "/" + value.GithubRepo
				licenseName = "<strong>License:</strong> "
				if value.GithubBaseInfo.License.Name != "" {
					licenseName += value.GithubBaseInfo.License.Name
				} else {
					licenseName += "-"
				}
				githubStars = "[![GitHub stars](https://img.shields.io/github/stars/" + githubURL + "?style=social&logo=github&logoColor=1F2328&label=)](https://github.com/" + githubURL + ")"
				issues = "[![GitHub issues](https://img.shields.io/github/issues/" + githubURL + "?label=)](https://github.com/" + githubURL + "/issues)"
				pullRequests = "[![GitHub pull requests](https://img.shields.io/github/issues-pr/" + githubURL + "?label=)](https://github.com/" + githubURL + "/pulls)"

				// contributors begin
				if len(value.GithubContributorsInfo) > 0 {
					var githubContributorsInfoList = value.GithubContributorsInfo
					contributors += `<table align="center" border="0">`

					// contributors
					switch len(value.GithubContributorsInfo) {
					case 1:
						contributors += `<tr align="center">`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[0].HtmlUrl + `"><img width="36px" src="` + getGithubAvatarUrl(githubContributorsInfoList[0].Id) + `" /></a>`
						contributors += `</td>`
						contributors += `</tr>`
					case 2:
						contributors += `<tr align="center">`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[0].HtmlUrl + `"><img width="30px" src="` + getGithubAvatarUrl(githubContributorsInfoList[0].Id) + `" /></a>`
						contributors += `</td>`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[1].HtmlUrl + `"><img width="30px" src="` + getGithubAvatarUrl(githubContributorsInfoList[1].Id) + `" /></a>`
						contributors += `</td>`
						contributors += `</tr>`
					case 3:
						contributors += `<tr align="center">`
						contributors += `<td colspan="2">`
						contributors += `<a href="` + githubContributorsInfoList[0].HtmlUrl + `"><img width="36px" src="` + getGithubAvatarUrl(githubContributorsInfoList[0].Id) + `" /></a>`
						contributors += `</td>`
						contributors += `</tr>`
						contributors += `<tr align="center">`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[1].HtmlUrl + `"><img width="30px" src="` + getGithubAvatarUrl(githubContributorsInfoList[1].Id) + `" /></a>`
						contributors += `</td>`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[2].HtmlUrl + `"><img width="30px" src="` + getGithubAvatarUrl(githubContributorsInfoList[2].Id) + `" /></a>`
						contributors += `</td>`
						contributors += `</tr>`
					}

					// total
					contributors += `<tr align="center">`
					contributors += `<td colspan="2">`
					if value.GithubBaseInfo.ContributorsTotal >= 100 {
						contributors += `<a href="https://github.com/` + githubURL + `/graphs/contributors">Total: 99+</a>`
					} else {
						contributors += `<a href="https://github.com/` + githubURL + `/graphs/contributors">Total: ` + strconv.Itoa(value.GithubBaseInfo.ContributorsTotal) + `</a>`
					}
					contributors += `</td>`
					contributors += `</tr>`

					contributors += `</table>`
				}
				// contributors end
			}
		}
		markdownTableList = append(
			markdownTableList,
			MarkdownTable{
				Name:                   name,
				Version:                version,
				Description:            value.Description,
				LicenseName:            licenseName,
				Platform:               platform,
				Published:              published,
				GithubStars:            githubStars,
				PubLikes:               pubLikes,
				PubPoints:              pubPoints,
				PubDownloadCount30Days: pubDownloadCount30Days,
				Issues:                 issues,
				PullRequests:           pullRequests,
				Contributors:           contributors,
			},
		)
	}

	markdown := ""
	markdown += "<sub>Sort by " + sortField + " | Total " + strconv.Itoa(len(markdownTableList)) + "</sub> \n\n" +
		"| <sub>Package</sub> | <sub>Stars/Likes</sub> | <sub>Downloads/Points</sub> | <sub>Issues / Pull_requests</sub> | <sub>Contributors</sub> | \n" +
		"|--------------------|------------------------|------------------------------|-----------------------------------|:-----------------------:| \n"
	for _, value := range markdownTableList {
		markdown += "" +
			"| " + value.Name + " <sup><strong>" + value.Version + "</strong></sup> <br/> <sub>" + formatString(value.Description) + "</sub> <br/> <sub>" + value.LicenseName + "</sub> <br/> <sub>" + value.Platform + "</sub> <br/> " + "<sub>" + value.Published + "</sub>" +
			" | " + value.GithubStars + " <br/> " + value.PubLikes +
			" | " + value.PubDownloadCount30Days + " <br/> " + value.PubPoints +
			" | " + value.Issues + " <br/> " + value.PullRequests +
			" | " + value.Contributors +
			" | \n"
	}
	return markdown
}

// 更新 Markdown 表格
//
// 识别：<!-- md:PubDashboard begin --><!-- md:PubDashboard end -->
//
// 参数:
//   - [filename] 更新的文件
//   - [markdown] 更新内容
func updateMarkdownTable(filename string, markdown string) error {
	md, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("📄❌ updateMarkdownTable: Error reade a file: %w", err)
	}

	begin := "<!-- md:PubDashboard begin -->"
	end := "<!-- md:PubDashboard end -->"
	newMdText := bytes.NewBuffer(nil)
	newMdText.WriteString(begin)
	newMdText.WriteString(" \n")
	newMdText.WriteString(markdown)
	newMdText.WriteString(" \n")
	newMdText.WriteString("Updated on " + time.Now().Format(time.RFC3339) + " by [Action](https://github.com/AmosHuKe/pub-dashboard). \n")
	newMdText.WriteString(end)

	reg := regexp.MustCompile(begin + "(?s)(.*?)" + end)
	newMd := reg.ReplaceAll(md, newMdText.Bytes())

	err = os.WriteFile(filename, newMd, 0644)
	if err != nil {
		return fmt.Errorf("📄❌ updateMarkdownTable: Error writing a file: %w", err)
	}
	fmt.Println("📄✅ updateMarkdownTable: Success")
	return nil
}

// 更新 Markdown Package 总数计数
//
// 识别：<!-- md:PubDashboard-total begin --><!-- md:PubDashboard-total end -->
//
// 参数:
//   - [filename] 更新的文件
//   - [total]    总数
func updateMarkdownPackageTotal(filename string, total int) error {
	md, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("📄❌ updateMarkdownPackageTotal: Error reade a file: %w", err)
	}

	begin := "<!-- md:PubDashboard-total begin -->"
	end := "<!-- md:PubDashboard-total end -->"
	newMdText := bytes.NewBuffer(nil)
	newMdText.WriteString(begin)
	newMdText.WriteString(strconv.Itoa(total))
	newMdText.WriteString(end)

	reg := regexp.MustCompile(begin + "(?s)(.*?)" + end)
	newMd := reg.ReplaceAll(md, newMdText.Bytes())

	err = os.WriteFile(filename, newMd, 0644)
	if err != nil {
		return fmt.Errorf("📄❌ updateMarkdownPackageTotal: Error writing a file: %w", err)
	}
	fmt.Println("📄✅ updateMarkdownPackageTotal: Success")
	return nil
}

// 创建带超时的共享 HTTP Client。
//
// 复用同一个 Client 可共享连接池；
// Timeout 防止单个请求挂起拖垮整个流程。
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: httpTimeout}
}

// concurrentMap 以 [concurrency] 为上限并发地将 fn 应用到每个 item，
// 结果按输入顺序写入返回切片（result[i] 对应 items[i]）。
//
// 任一 fn 返回错误即取消其余任务并返回首个错误。
// 由于每个 goroutine 只写入互不重叠的 result[i]，无需对结果加锁。
//
// 参数:
//   - [ctx]         上下文（用于取消与超时传播）
//   - [items]       输入切片
//   - [concurrency] 并发数上限
//   - [fn]          处理函数
//
// 返回值:
//   - [results]     结果切片（与 items 顺序一致）
//   - [error]       任一 fn 返回错误时非 nil
func concurrentMap[T any, R any](ctx context.Context, items []T, concurrency int, fn func(context.Context, T) (R, error)) ([]R, error) {
	results := make([]R, len(items))
	if len(items) == 0 {
		return results, nil
	}
	if concurrency < 1 {
		concurrency = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		waitGroup sync.WaitGroup
		mutex     sync.Mutex
		firstErr  error
	)
	semaphore := make(chan struct{}, concurrency)

	for i, item := range items {
		waitGroup.Add(1)
		go func(i int, item T) {
			defer waitGroup.Done()

			// 并发限流，若已取消则直接放弃
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			r, err := fn(ctx, item)
			if err != nil {
				mutex.Lock()
				if firstErr == nil {
					firstErr = err
					cancel() // fail-fast：取消其余在途/未启动任务
				}
				mutex.Unlock()
				return
			}
			results[i] = r
		}(i, item)
	}
	waitGroup.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// 带重试的 HTTP GET 请求
//
// 对传输层错误、429 (Too Many Requests)、403 (限流/鉴权)、5xx 进行指数退避重试，
// 最多尝试 [maxAttempts] 次；退避期间响应 ctx 取消。
//
// 仅负责传输 + 重试瞬时故障，状态码的业务语义（如 404 的含义）
// 由调用方根据返回的 status 自行解释。
//
// 参数:
//   - [ctx]     上下文（用于取消与超时传播）
//   - [client]  共享 HTTP Client
//   - [rawURL]  请求地址
//   - [headers] 附加请求头（可为 nil）
//
// 返回值:
//   - 响应体
//   - HTTP 状态码
//   - 错误（传输层彻底失败或重试耗尽时非 nil）
func httpGetWithRetry(ctx context.Context, client *http.Client, rawURL string, headers map[string]string) ([]byte, int, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// 退避（首次不等待）：500ms, 1s, 2s ...
		if attempt > 1 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-2))
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, 0, err // 构造请求失败不可恢复
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		res, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, 0, ctx.Err() // 已取消则立即返回
			}
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		status := res.StatusCode

		if readErr != nil {
			if ctx.Err() != nil {
				return nil, status, ctx.Err()
			}
			lastErr = readErr
			continue
		}

		// 可重试的状态码：限流与服务端错误
		if status == http.StatusTooManyRequests || status == http.StatusForbidden || status >= 500 {
			lastErr = fmt.Errorf("unexpected status %d", status)
			continue
		}

		// 成功或不可重试的状态码（2xx、404 等），交由调用方判断
		return body, status, nil
	}
	return nil, 0, fmt.Errorf("After %d attempts: %w", maxAttempts, lastErr)
}

// 由于直接获取 GithubContributorsInfo.AvatarUrl 有可能会是私有头像地址，
// 暂时固定头像地址。
//
// 参数:
//   - [githubId] Github ID
func getGithubAvatarUrl(githubId int) string {
	return "https://avatars.githubusercontent.com/u/" + strconv.Itoa(githubId) + "?v=4"
}

// 格式化字符串（防止 markdown 格式错乱）
//
// 参数:
//   - [v] 需要格式化的字符
//
// 返回值:
//   - 格式化后的字符
func formatString(v string) string {
	value := v
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "丨")
	return value
}

// 格式化下载数量（便于展示）
//
// 参数:
//   - [num] 需要格式化的数量
//
// 返回值:
//   - 格式化后的数量字符
func formatDownloadCount(num int) string {
	var formatted string
	var suffix string

	if num >= 1000000 {
		formatted = fmt.Sprintf("%.2f", float64(num)/1000000)
		suffix = "M"
	} else if num >= 1000 {
		formatted = fmt.Sprintf("%.2f", float64(num)/1000)
		suffix = "k"
	} else {
		return strconv.Itoa(num)
	}

	// 去掉多余的0和小数点
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	return formatted + suffix
}

// 去重并保持首次出现的顺序，同时去除首尾空白与空字符串
func removeDuplicates(arr []string) []string {
	seen := make(map[string]bool, len(arr))
	result := []string{}
	for _, v := range arr {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
	}
	return result
}
