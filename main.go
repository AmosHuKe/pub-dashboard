package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 主 MarkdownTable
type MarkdownTable struct {
	Name         string
	Version      string
	Description  string
	LicenseName  string
	Platform     string
	Published    string
	GithubStars  string
	PubLikes     string
	Points       string
	Popularity   string
	Issues       string
	PullRequests string
	Contributors string
}

// 主 Package 信息
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

type GithubBaseInfo struct {
	StargazersCount float64 `json:"stargazers_count"`
	ForksCount      float64 `json:"forks_count"`
	OpenIssuesCount float64 `json:"open_issues_count"`
	License         struct {
		Name string `json:"name"`
	} `json:"license"`
	ContributorsTotal int
}

type GithubContributorsInfo struct {
	Login     string `json:"login"`
	AvatarUrl string `json:"avatar_url"`
	HtmlUrl   string `json:"html_url"`
	Type      string `json:"type"`
}

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

type PackageScoreInfo struct {
	GrantedPoints   float64  `json:"grantedPoints"`
	MaxPoints       float64  `json:"maxPoints"`
	LikeCount       float64  `json:"likeCount"`
	PopularityScore float64  `json:"popularityScore"`
	Tags            []string `json:"tags"`
	LastUpdated     string   `json:"lastUpdated"`
	TagsPlatform    []string
}

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
	flag.StringVar(&sortField, "sortField", "name", "name | published | pubLikes | githubStars")
	flag.StringVar(&sortMode, "sortMode", "asc", "asc | desc")
	flag.Parse()

	var packageAllList string
	publisherPackageList := getPublisherPackages(publisherList)
	packageAllList = publisherPackageList + "," + packageList
	packageInfoList := getPackageInfo(githubToken, packageAllList)
	sortPackageInfo(packageInfoList, sortField, sortMode)
	markdownTable := assembleMarkdownTable(packageInfoList, sortField)

	// 更新表格
	updateMarkdownTable(filename, markdownTable)
	// 更新总数
	updateMarkdownPackageTotal(filename, len(packageInfoList))
}

// 通过 Publisher 获取所有 Package 名称
// [publisherName] publisher 列表(逗号,分割)
// Return 与 packageList 相同的 package 名称列表(逗号,分割)
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
			fmt.Println("🌏🔗 Publisher: " + publisherName + ", Page: " + strconv.Itoa(pageIndex))
			res, err := http.Get("https://pub.dev/api/search?q=publisher:" + publisherName + "&page=" + strconv.Itoa(pageIndex))
			if err != nil {
				fmt.Println(printErrTitle, err)
			}
			defer res.Body.Close()
			jsonData, err := io.ReadAll(res.Body)
			if err != nil {
				fmt.Println(printErrTitle, err)
			}
			data := PublisherInfo{}
			if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
				fmt.Println(printErrTitle, err)
			}
			if len(data.Packages) > 0 {
				for _, packageName := range data.Packages {
					if packageName.Package != "" {
						packageNameList = append(packageNameList, packageName.Package)
					}
				}
				pageIndex++
			} else {
				pageIndex = 0
			}
		}
	}
	return strings.Join(packageNameList, ",")
}

// 获取 Package 信息
// [githubToken] Github Token
// [packagesName] package 名称列表(逗号,分割)
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
		res, err := http.Get("https://pub.dev/api/packages/" + packageName)
		if err != nil {
			fmt.Println(printErrTitle, err)
		}
		defer res.Body.Close()
		jsonData, err := io.ReadAll(res.Body)
		if err != nil {
			fmt.Println(printErrTitle, err)
		}
		var data PackageBaseInfo
		if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
			fmt.Println(printErrTitle, err)
		}

		if data.Name != "" {
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
			fmt.Println("📦✅ " + packageName + ", Code: 1")
		} else {
			// 无法获取信息
			packageInfoList = append(
				packageInfoList,
				PackageInfo{
					Code: 0,
					Name: packageName,
				},
			)
			fmt.Println("📦❌ " + packageName + ", Code: 0")
		}
	}
	return packageInfoList
}

// 获取 Package score 信息
// [packageName] 单个 package 名称
func getPackageScoreInfo(packageName string) PackageScoreInfo {
	printErrTitle := "📦⚠️ PackageScoreInfo: "
	res, err := http.Get("https://pub.dev/api/packages/" + packageName + "/score")
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	defer res.Body.Close()
	jsonData, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	var data PackageScoreInfo
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		fmt.Println(printErrTitle, err)
	}

	// 获取 Tags 相关内容
	var tagsPlatform []string
	for _, value := range data.Tags {
		tag := strings.Split(value, ":")
		tagName := tag[0]
		tagValue := tag[1]
		// TagsPlatform
		if tagName == "platform" {
			tagsPlatform = append(tagsPlatform, tagValue)
		}
	}
	data.TagsPlatform = tagsPlatform
	return data
}

// 获取 Github 信息
// [githubToken] Github Token
// [packageInfo] 当前 package 信息
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
// [githubToken] Github Token
// [user] 用户
// [repo] 仓库
func getGithubBaseInfo(githubToken string, user string, repo string) GithubBaseInfo {
	printErrTitle := "📦⚠️ GithubBaseInfo: "
	client := &http.Client{}
	resp, err := http.NewRequest("GET", "https://api.github.com/repos/"+user+"/"+repo, strings.NewReader(""))
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
	defer res.Body.Close()
	jsonData, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	var data GithubBaseInfo
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		fmt.Println(printErrTitle, err)
	}

	return data
}

// 获取 Github 贡献者信息
// [githubToken] Github Token
// [user] 用户
// [repo] 仓库
//
// @return (贡献者列表, 贡献者总数（最多100）)
func getGithubContributorsInfo(githubToken string, user string, repo string) ([]GithubContributorsInfo, int) {
	printErrTitle := "📦⚠️ GithubContributorsInfo: "
	client := &http.Client{}
	resp, err := http.NewRequest("GET", "https://api.github.com/repos/"+user+"/"+repo+"/contributors?page=1&per_page=100", strings.NewReader(""))
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
	defer res.Body.Close()
	jsonData, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(printErrTitle, err)
	}
	var data []GithubContributorsInfo
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
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
// Return (githubUser, githubRepo)
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

// 排序
// [packageInfoList] 	信息列表
// [sortField] 				排序字段 可选：name(default) | published | pubLikes | githubStars
// [sortMode] 				排序方式 可选：asc(default) | desc
func sortPackageInfo(packageInfoList []PackageInfo, sortField string, sortMode string) {
	switch sortField {
	case "name":
		// 按照 pub 名称排序
		sort.SliceStable(packageInfoList, func(i, j int) bool {
			iData := packageInfoList[i].Name
			jData := packageInfoList[j].Name
			switch sortMode {
			case "asc":
				return iData < jData
			case "desc":
				return iData > jData
			default:
				return iData < jData
			}
		})
	case "published":
		// 按 pub 最新发布时间排序
		sort.SliceStable(packageInfoList, func(i, j int) bool {
			iData := packageInfoList[i].Published
			jData := packageInfoList[j].Published
			switch sortMode {
			case "asc":
				return iData > jData
			case "desc":
				return iData < jData
			default:
				return iData > jData
			}
		})
	case "pubLikes":
		// 按 pub likes 排序
		sort.SliceStable(packageInfoList, func(i, j int) bool {
			iData := packageInfoList[i].ScoreInfo.LikeCount
			jData := packageInfoList[j].ScoreInfo.LikeCount
			switch sortMode {
			case "asc":
				return iData < jData
			case "desc":
				return iData > jData
			default:
				return iData < jData
			}
		})
	case "githubStars":
		// 按 github stars 排序
		sort.SliceStable(packageInfoList, func(i, j int) bool {
			iData := packageInfoList[i].GithubBaseInfo.StargazersCount
			jData := packageInfoList[j].GithubBaseInfo.StargazersCount
			switch sortMode {
			case "asc":
				return iData < jData
			case "desc":
				return iData > jData
			default:
				return iData < jData
			}
		})
	default:
		// 按照 pub 名称排序
		sort.SliceStable(packageInfoList, func(i, j int) bool {
			iData := packageInfoList[i].Name
			jData := packageInfoList[j].Name
			switch sortMode {
			case "asc":
				return iData < jData
			case "desc":
				return iData > jData
			default:
				return iData < jData
			}
		})
	}
}

// 组装表格内容
// [packageInfoList] 	信息列表
// [sortField] 				排序字段 可选：name(default) | published | pubLikes | githubStars
// [sortMode] 				排序方式 可选：asc(default) | desc
func assembleMarkdownTable(packageInfoList []PackageInfo, sortField string) string {
	markdownTableList := []MarkdownTable{}
	for _, value := range packageInfoList {
		var name, version, platform, licenseName, published, githubStars, pubLikes, points, popularity, issues, pullRequests, contributors string
		switch value.Code {
		case 0:
			// 无法获取信息
			name = value.Name + " ⁉️"
		case 1:
			// 已获取信息
			// Base
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
			points = "[![Pub points](https://img.shields.io/pub/points/" + value.Name + "?label=)](https://pub.dev/packages/" + value.Name + "/score)"
			popularity = "[![popularity](https://img.shields.io/pub/popularity/" + value.Name + "?label=)](https://pub.dev/packages/" + value.Name + "/score)"
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
						contributors += `<a href="` + githubContributorsInfoList[0].HtmlUrl + `"><img width="36px" src="` + githubContributorsInfoList[0].AvatarUrl + `" /></a>`
						contributors += `</td>`
						contributors += `</tr>`
					case 2:
						contributors += `<tr align="center">`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[0].HtmlUrl + `"><img width="30px" src="` + githubContributorsInfoList[0].AvatarUrl + `" /></a>`
						contributors += `</td>`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[1].HtmlUrl + `"><img width="30px" src="` + githubContributorsInfoList[1].AvatarUrl + `" /></a>`
						contributors += `</td>`
						contributors += `</tr>`
					case 3:
						contributors += `<tr align="center">`
						contributors += `<td colspan="2">`
						contributors += `<a href="` + githubContributorsInfoList[0].HtmlUrl + `"><img width="36px" src="` + githubContributorsInfoList[0].AvatarUrl + `" /></a>`
						contributors += `</td>`
						contributors += `</tr>`
						contributors += `<tr align="center">`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[1].HtmlUrl + `"><img width="30px" src="` + githubContributorsInfoList[1].AvatarUrl + `" /></a>`
						contributors += `</td>`
						contributors += `<td>`
						contributors += `<a href="` + githubContributorsInfoList[2].HtmlUrl + `"><img width="30px" src="` + githubContributorsInfoList[2].AvatarUrl + `" /></a>`
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
				Name:         name,
				Version:      version,
				Description:  value.Description,
				LicenseName:  licenseName,
				Platform:     platform,
				Published:    published,
				GithubStars:  githubStars,
				PubLikes:     pubLikes,
				Points:       points,
				Popularity:   popularity,
				Issues:       issues,
				PullRequests: pullRequests,
				Contributors: contributors,
			},
		)
	}

	markdown := ""
	markdown += "<sub>Sort by " + sortField + " | Total " + strconv.Itoa(len(markdownTableList)) + "</sub> \n\n" +
		"| <sub>Package</sub> | <sub>Stars/Likes</sub> | <sub>Points / Popularity</sub> | <sub>Issues / Pull_requests</sub> | <sub>Contributors</sub> | \n" +
		"|--------------------|------------------------|------------------------------|-----------------------------------|:-----------------------:| \n"
	for _, value := range markdownTableList {
		markdown += "" +
			"| " + value.Name + " <sup><strong>" + value.Version + "</strong></sup> <br/> <sub>" + formatString(value.Description) + "</sub> <br/> <sub>" + value.LicenseName + "</sub> <br/> <sub>" + value.Platform + "</sub> <br/> " + "<sub>" + value.Published + "</sub>" +
			" | " + value.GithubStars + " <br/> " + value.PubLikes +
			" | " + value.Points + " <br/> " + value.Popularity +
			" | " + value.Issues + " <br/> " + value.PullRequests +
			" | " + value.Contributors +
			" | \n"
	}
	return markdown
}

// 更新 Markdown 表格
// [filename]	更新的文件
// [markdown]	更新内容
//
// <!-- md:PubDashboard begin --><!-- md:PubDashboard end -->
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
// [filename]	更新的文件
// [total]		总数
//
// <!-- md:PubDashboard-total begin --><!-- md:PubDashboard-total end -->
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

// 格式化字符串
func formatString(v string) string {
	value := v
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "丨")
	return value
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
