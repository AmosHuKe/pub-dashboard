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
	"time"
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

	packageAllList := mergePackageList(publisherList, packageList)
	packageInfoList := getPackageInfo(githubToken, packageAllList)
	sortPackageInfo(packageInfoList, sortField, sortMode)
	markdownTable := assembleMarkdownTable(packageInfoList, sortField)

	// 更新表格
	if err := updateMarkdownTable(filename, markdownTable); err != nil {
		fmt.Println(err)
	}
	// 更新总数
	if err := updateMarkdownPackageTotal(filename, len(packageInfoList)); err != nil {
		fmt.Println(err)
	}
}

// 合并 publisher 的 package 和自定义 package 列表，并去重
//
// 参数:
//   - [publisherList] publisher 名称列表（逗号,分割）
//   - [packageList] package 名称列表（逗号,分割）
//
// 返回值:
//   - package 合并后的名称列表（逗号,分割）
func mergePackageList(publisherList, packageList string) string {
	publisherPackageList := getPublisherPackages(publisherList)
	all := strings.Split(publisherPackageList+","+packageList, ",")
	return strings.Join(removeDuplicates(all), ",")
}

// 通过 Publisher 获取所有 Package 名称
//
// 参数:
//   - [publisherName] publisher 列表（逗号,分割）
//
// 返回值:
//   - 与 packageList 相同的 package 名称列表（逗号,分割）
func getPublisherPackages(publisherName string) string {
	printErrTitle := "🌏⚠️ PublisherPackages: "
	if publisherName == "" {
		return ""
	}
	publisherList := removeDuplicates(strings.Split(publisherName, ","))
	fmt.Println("🌏", publisherList)
	packageNameList := []string{}
	for _, value := range publisherList {
		if value == "" {
			continue
		}
		publisherName := strings.TrimSpace(value)

		// 查找每一页
		pageIndex := 1
		for pageIndex != 0 {
			fmt.Printf("🌏🔗 Publisher: %s, Page: %d \n", publisherName, pageIndex)
			res, err := http.Get(fmt.Sprintf("https://pub.dev/api/search?q=publisher:%s&page=%d", publisherName, pageIndex))
			if err != nil {
				fmt.Println(printErrTitle, err)
				break
			}
			jsonData, err := io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				fmt.Println(printErrTitle, err)
				break
			}
			data := PublisherInfo{}
			if err := json.Unmarshal(jsonData, &data); err != nil {
				fmt.Println(printErrTitle, err)
				break
			}
			if len(data.Packages) == 0 {
				pageIndex = 0
				break
			}
			for _, packageName := range data.Packages {
				if packageName.Package != "" {
					packageNameList = append(packageNameList, packageName.Package)
				}
			}
			pageIndex++
		}
	}
	return strings.Join(removeDuplicates(packageNameList), ",")
}

// 获取所有 Package 信息
//
// 参数:
//   - [githubToken] Github Token
//   - [packagesName] package 名称列表（逗号,分割）
//
// 返回值:
//   - [PackageInfo] 列表
func getPackageInfo(githubToken string, packagesName string) []PackageInfo {
	printErrTitle := "📦⚠️ PackageInfo: "
	packageList := removeDuplicates(strings.Split(packagesName, ","))
	fmt.Println("📦", packageList)
	packageInfoList := []PackageInfo{}
	for _, value := range packageList {
		if value == "" {
			continue
		}
		fmt.Println("📦🔥 " + value)
		packageName := strings.TrimSpace(value)
		res, err := http.Get(fmt.Sprintf("https://pub.dev/api/packages/%s", packageName))
		if err != nil {
			fmt.Println(printErrTitle, err)
		}
		jsonData, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			fmt.Println(printErrTitle, err)
		}
		var data PackageBaseInfo
		if err := json.Unmarshal(jsonData, &data); err != nil {
			fmt.Println(printErrTitle, err)
		}
		if data.Name == "" {
			packageInfoList = append(packageInfoList, PackageInfo{Code: 0, Name: packageName})
			fmt.Printf("📦❌ %s, Code: 0\n", packageName)
			continue
		}

		// 可获取信息
		packageInfo := PackageInfo{
			Code:         1,
			Name:         data.Name,
			Version:      data.Latest.Pubspec.Version,
			Description:  data.Latest.Pubspec.Description,
			Homepage:     data.Latest.Pubspec.Homepage,
			Repository:   data.Latest.Pubspec.Repository,
			IssueTracker: data.Latest.Pubspec.IssueTracker,
			Published:    data.Latest.Published,
			ScoreInfo:    getPackageScoreInfo(data.Name),
		}
		getGithubInfo(githubToken, &packageInfo)
		packageInfoList = append(packageInfoList, packageInfo)
		fmt.Printf("📦✅ %s, Code: 1\n", packageName)
	}
	return packageInfoList
}

// 获取 Package score 信息
//
// 参数:
//   - [packageName] 单个 package 名称
//
// 返回值:
//   - [PackageScoreInfo] 信息
func getPackageScoreInfo(packageName string) PackageScoreInfo {
	printErrTitle := "📦⚠️ PackageScoreInfo: "
	res, err := http.Get(fmt.Sprintf("https://pub.dev/api/packages/%s/score", packageName))
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	jsonData, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	var data PackageScoreInfo
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Println(printErrTitle, err)
	}

	// 获取 Tags 相关内容
	for _, value := range data.Tags {
		tag := strings.SplitN(value, ":", 2)
		// TagsPlatform
		if len(tag) == 2 && tag[0] == "platform" {
			data.TagsPlatform = append(data.TagsPlatform, tag[1])
		}
	}
	return data
}

// 获取 Github 信息，
// 处理 [PackageInfo] 中 GithubUser, GithubRepo, GithubBaseInfo, GithubContributorsInfo 的值
//
// 参数:
//   - [githubToken] Github Token
//   - [packageInfo] 当前 package 信息
func getGithubInfo(githubToken string, packageInfo *PackageInfo) {
	if packageInfo.Code == 0 {
		return
	}
	finish := false
	var user, repo string
	user, repo = formatGithubInfo(packageInfo.Repository)
	if repo != "" && !finish {
		packageInfo.GithubUser = user
		packageInfo.GithubRepo = repo
		finish = true
	}
	user, repo = formatGithubInfo(packageInfo.IssueTracker)
	if repo != "" && !finish {
		packageInfo.GithubUser = user
		packageInfo.GithubRepo = repo
		finish = true
	}
	user, repo = formatGithubInfo(packageInfo.Homepage)
	if repo != "" && !finish {
		packageInfo.GithubUser = user
		packageInfo.GithubRepo = repo
		finish = true
	}
	// 获取 Github 相关信息
	if packageInfo.GithubUser != "" && packageInfo.GithubRepo != "" {
		packageInfo.GithubBaseInfo = getGithubBaseInfo(githubToken, packageInfo.GithubUser, packageInfo.GithubRepo)
		packageInfo.GithubContributorsInfo, packageInfo.GithubBaseInfo.ContributorsTotal = getGithubContributorsInfo(githubToken, packageInfo.GithubUser, packageInfo.GithubRepo)
	}
}

// 获取 Github 基础信息
//
// 参数:
//   - [githubToken] Github Token
//   - [user] 用户
//   - [repo] 仓库
//
// 返回值:
//   - [GithubBaseInfo] 信息
func getGithubBaseInfo(githubToken string, user string, repo string) GithubBaseInfo {
	printErrTitle := "📦⚠️ GithubBaseInfo: "
	client := &http.Client{}
	resp, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s", user, repo), strings.NewReader(""))
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	resp.Header.Set("Authorization", "bearer "+githubToken)
	resp.Header.Set("Accept", "application/vnd.github+json")
	resp.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := client.Do(resp)
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	jsonData, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	var data GithubBaseInfo
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Println(printErrTitle, err)
	}

	return data
}

// 获取 Github 贡献者信息
//
// 参数:
//   - [githubToken] Github Token
//   - [user] 用户
//   - [repo] 仓库
//
// 返回值:
//   - [GithubContributorsInfo] 贡献者列表
//   - 贡献者总数（最多100）
func getGithubContributorsInfo(githubToken string, user string, repo string) ([]GithubContributorsInfo, int) {
	printErrTitle := "📦⚠️ GithubContributorsInfo: "
	client := &http.Client{}
	resp, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors?page=1&per_page=100", user, repo), strings.NewReader(""))
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	resp.Header.Set("Authorization", "bearer "+githubToken)
	resp.Header.Set("Accept", "application/vnd.github+json")
	resp.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := client.Do(resp)
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	jsonData, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	var data []GithubContributorsInfo
	if err := json.Unmarshal(jsonData, &data); err != nil {
		fmt.Println(printErrTitle, err)
	}

	githubContributorsInfo := []GithubContributorsInfo{}
	i := 1
	/// 取前 3 位非 Bot 贡献者
	for _, value := range data {
		if i > 3 {
			break
		}
		if value.Type == "User" {
			githubContributorsInfo = append(githubContributorsInfo, value)
			i++
		}
	}
	return githubContributorsInfo, len(data)
}

// 格式化 Github 信息
//
// 参数:
//   - [value] Github 链接
//
// 返回值:
//   - githubUser 信息,
//   - githubRepo 信息
func formatGithubInfo(value string) (string, string) {
	var githubUser, githubRepo string
	result := regexp.MustCompile(`(?:github.com/).*`).FindAllString(value, -1)
	if result != nil {
		info := strings.Split(result[0], "/")
		if len(info) >= 3 {
			githubUser = info[1]
			githubRepo = strings.ReplaceAll(info[2], ".git", "")
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
//   - [sortMode]         排序方式 可选：asc(default) | desc
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

	err = os.WriteFile(filename, newMd, os.ModeAppend)
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

	err = os.WriteFile(filename, newMd, os.ModeAppend)
	if err != nil {
		return fmt.Errorf("📄❌ updateMarkdownPackageTotal: Error writing a file: %w", err)
	}
	fmt.Println("📄✅ updateMarkdownPackageTotal: Success")
	return nil
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

// 去重
func removeDuplicates(arr []string) []string {
	uniqueMap := make(map[string]bool)
	for _, v := range arr {
		if _, ok := uniqueMap[v]; !ok {
			uniqueMap[v] = true
		}
	}
	var uniqueArr []string
	for k := range uniqueMap {
		uniqueArr = append(uniqueArr, k)
	}
	return uniqueArr
}
