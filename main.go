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
	Platform     string
	Published    string
	GithubStars  string
	PubLikes     string
	Points       string
	Popularity   string
	Issues       string
	PullRequests string
}

// 主 Package 信息
type PackageInfo struct {
	Code         int // 0: error 1：success
	Name         string
	Version      string
	Description  string
	Homepage     string
	Repository   string
	IssueTracker string
	Published    string
	GithubUser   string
	GithubRepo   string
	ScoreInfo    PackageScoreInfo
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
	Packages []PackageName `json:"packages"`
	Next     string        `json:"next"`
}

type PackageName struct {
	Package string `json:"package"`
}

func main() {
	var filename, publisherList, packageList, sortField, sortMode string
	flag.StringVar(&filename, "filename", "README.md", "文件名 如: README.md")
	flag.StringVar(&publisherList, "publisherList", "", "publisher 如: aa,bb,cc")
	flag.StringVar(&packageList, "packageList", "", "package 如: aa,bb,cc")
	flag.StringVar(&sortField, "sortField", "name", "name | published")
	flag.StringVar(&sortMode, "sortMode", "asc", "asc | desc")
	flag.Parse()

	var packageAllList string
	publisherPackageList := getPublisherPackages(publisherList)
	packageAllList = publisherPackageList + "," + packageList
	packageInfoList := getPackageInfo(packageAllList)
	sortPackageInfo(packageInfoList, sortField, sortMode)
	findGithubInfo(packageInfoList)
	markdownTable := assembleMarkdownTable(packageInfoList, sortField, sortMode)

	// 更新表格
	updateMarkdownTable(filename, markdownTable)
	// 更新总数
	updateMarkdownPackageTotal(filename, len(packageInfoList))
}

// 通过 Publisher 获取所有 Package 名称
// [publisherName] publisher 列表(逗号,分割)
// Return 与 packageList 相同的 package 名称列表(逗号,分割)
func getPublisherPackages(publisherName string) string {
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
			fmt.Println("🌏 Publisher: " + publisherName + ", Page: " + strconv.Itoa(pageIndex))
			res, err := http.Get("https://pub.dev/api/search?q=publisher:" + publisherName + "&page=" + strconv.Itoa(pageIndex))
			if err != nil {
				fmt.Println(err)
			}
			defer res.Body.Close()
			jsonData, getErr := io.ReadAll(res.Body)
			if getErr != nil {
				fmt.Println(getErr)
			}
			data := PublisherInfo{}
			if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
				fmt.Println(err)
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
// [packagesName] package 名称列表(逗号,分割)
func getPackageInfo(packagesName string) []PackageInfo {
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
			fmt.Println(err)
		}
		defer res.Body.Close()
		jsonData, getErr := io.ReadAll(res.Body)
		if getErr != nil {
			fmt.Println(getErr)
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
			fmt.Println(err)
		}

		var pubName, pubVersion, pubDescription, pubHomepage, pubRepository, pubIssueTracker, pubPublished string
		if value, ok := data["error"].(map[string]interface{}); !ok {
			if len(value) <= 0 {
				if value, ok := data["name"].(string); ok {
					pubName = value
				}
				if value, ok := data["latest"].(map[string]interface{})["version"].(string); ok {
					pubVersion = value
				}
				if value, ok := data["latest"].(map[string]interface{})["pubspec"].(map[string]interface{})["description"].(string); ok {
					pubDescription = value
				}
				if value, ok := data["latest"].(map[string]interface{})["pubspec"].(map[string]interface{})["homepage"].(string); ok {
					pubHomepage = value
				}
				if value, ok := data["latest"].(map[string]interface{})["pubspec"].(map[string]interface{})["repository"].(string); ok {
					pubRepository = value
				}
				if value, ok := data["latest"].(map[string]interface{})["pubspec"].(map[string]interface{})["issue_tracker"].(string); ok {
					pubIssueTracker = value
				}
				if value, ok := data["latest"].(map[string]interface{})["published"].(string); ok {
					pubPublished = value
				}
			}
		}
		if pubName != "" {
			// 可获取信息
			packageInfoList = append(
				packageInfoList,
				PackageInfo{
					Code:         1,
					Name:         pubName,
					Version:      pubVersion,
					Description:  pubDescription,
					Homepage:     pubHomepage,
					Repository:   pubRepository,
					IssueTracker: pubIssueTracker,
					Published:    pubPublished,
					ScoreInfo:    getPackageScoreInfo(pubName),
				},
			)
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
	res, err := http.Get("https://pub.dev/api/packages/" + packageName + "/score")
	if err != nil {
		fmt.Println(err)
	}
	defer res.Body.Close()
	jsonData, getErr := io.ReadAll(res.Body)
	if getErr != nil {
		fmt.Println(getErr)
	}
	var data PackageScoreInfo
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		fmt.Println(err)
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

// 排序
// [packageInfoList] 	信息列表
// [sortField] 				排序字段 可选：name(default) | published | pubLikes
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
		// 按 pub 最新发布时间排序
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

// 寻找 Github 信息
// [packageInfoList] 	信息列表
func findGithubInfo(packageInfoList []PackageInfo) {
	for key, value := range packageInfoList {
		if value.Code == 0 {
			continue
		}
		var user, repo string
		user, repo = formatGithubInfo(value.Repository)
		if user != "" {
			packageInfoList[key].GithubUser = user
			packageInfoList[key].GithubRepo = repo
			continue
		}
		user, repo = formatGithubInfo(value.IssueTracker)
		if user != "" {
			packageInfoList[key].GithubUser = user
			packageInfoList[key].GithubRepo = repo
			continue
		}
		user, repo = formatGithubInfo(value.Homepage)
		if user != "" {
			packageInfoList[key].GithubUser = user
			packageInfoList[key].GithubRepo = repo
			continue
		}
	}
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

// 组装表格内容
// [packageInfoList] 	信息列表
// [sortField] 				排序字段 可选：name(default) | published
// [sortMode] 				排序方式 可选：asc(default) | desc
func assembleMarkdownTable(packageInfoList []PackageInfo, sortField string, sortMode string) string {
	markdownTableList := []MarkdownTable{}
	for _, value := range packageInfoList {
		var name, version, platform, published, githubStars, pubLikes, points, popularity, issues, pullRequests string
		switch value.Code {
		case 0:
			// 无法获取信息
			name = value.Name + " ⁉️"
		case 1:
			// 已获取信息
			// Base
			name = "[" + value.Name + "](https://pub.dev/packages/" + value.Name + ")"
			version = "v" + value.Version
			if len(value.ScoreInfo.TagsPlatform) > 0 {
				platform = "<strong>Platform:</strong> " + strings.Join(value.ScoreInfo.TagsPlatform, ", ")
			} else {
				platform = "-"
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
				githubStars = "[![GitHub stars](https://img.shields.io/github/stars/" + githubURL + "?style=social&logo=github&logoColor=1F2328&label=)](https://github.com/" + githubURL + ")"
				issues = "[![GitHub issues](https://img.shields.io/github/issues/" + githubURL + "?label=)](https://github.com/" + githubURL + "/issues)"
				pullRequests = "[![GitHub pull requests](https://img.shields.io/github/issues-pr/" + githubURL + "?label=)](https://github.com/" + githubURL + "/pulls)"
			}
		}
		markdownTableList = append(
			markdownTableList,
			MarkdownTable{
				Name:         name,
				Version:      version,
				Description:  value.Description,
				Platform:     platform,
				Published:    published,
				GithubStars:  githubStars,
				PubLikes:     pubLikes,
				Points:       points,
				Popularity:   popularity,
				Issues:       issues,
				PullRequests: pullRequests,
			},
		)
	}

	markdown := ""
	markdown += "<sub>Sort by " + sortField + " | Total " + strconv.Itoa(len(markdownTableList)) + "</sub> \n\n" +
		"| <sub>Package</sub> | <sub>Stars/Likes</sub> | <sub>Points/Popularity</sub> | <sub>Issues</sub> | <sub>Pull requests</sub> | \n" +
		"|--------------------|------------------------|------------------------------|-------------------|--------------------------| \n"
	for _, value := range markdownTableList {
		markdown += "" +
			"| " + value.Name + " <sup><strong>" + value.Version + "</strong></sup> <br/> <sub>" + formatString(value.Description) + "</sub> <br/> " + "<sub>" + value.Platform + "</sub> <br/> " + "<sub>" + value.Published + "</sub>" +
			" | " + value.GithubStars + " <br/> " + value.PubLikes +
			" | " + value.Points + " <br/> " + value.Popularity +
			" | " + value.Issues +
			" | " + value.PullRequests +
			" | \n"
	}
	return markdown
}

// 更新 Markdown 表格
// [filename]	更新的文件
// [markdown]	更新内容
//
// <!-- md:PubDashboard start --><!-- md:PubDashboard end -->
func updateMarkdownTable(filename string, markdown string) error {
	md, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("📄❌ updateMarkdownTable: Error reade a file: %w", err)
	}

	start := "<!-- md:PubDashboard start -->"
	end := "<!-- md:PubDashboard end -->"
	newMdText := bytes.NewBuffer(nil)
	newMdText.WriteString(start)
	newMdText.WriteString(" \n")
	newMdText.WriteString(markdown)
	newMdText.WriteString(" \n")
	newMdText.WriteString("Updated on " + time.Now().Format(time.RFC3339) + " by [Action](https://github.com/AmosHuKe/pub-dashboard). \n")
	newMdText.WriteString(end)

	reg := regexp.MustCompile(start + "(?s)(.*?)" + end)
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
// <!-- md:PubDashboard-total start --><!-- md:PubDashboard-total end -->
func updateMarkdownPackageTotal(filename string, total int) error {
	md, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("📄❌ updateMarkdownPackageTotal: Error reade a file: %w", err)
	}

	start := "<!-- md:PubDashboard-total start -->"
	end := "<!-- md:PubDashboard-total end -->"
	newMdText := bytes.NewBuffer(nil)
	newMdText.WriteString(start)
	newMdText.WriteString(strconv.Itoa(total))
	newMdText.WriteString(end)

	reg := regexp.MustCompile(start + "(?s)(.*?)" + end)
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
