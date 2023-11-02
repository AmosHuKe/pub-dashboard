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

type PackageInfo struct {
	Code         int // 0: error 1：success
	Name         string
	Description  string
	Homepage     string
	Repository   string
	IssueTracker string
	Published    string
	GithubUser   string
	GithubRepo   string
}

type MarkdownTable struct {
	Name         string
	Description  string
	Published    string
	GithubStars  string
	PubLikes     string
	Version      string
	Points       string
	Popularity   string
	Issues       string
	PullRequests string
}

func main() {
	var filename, packageList, sortField, sortMode string
	flag.StringVar(&filename, "filename", "README.md", "文件名 如: README.md")
	flag.StringVar(&packageList, "packageList", "", "package 如: aa,bb,cc")
	flag.StringVar(&sortField, "sortField", "name", "name | published")
	flag.StringVar(&sortMode, "sortMode", "asc", "asc | desc")
	flag.Parse()

	packageInfoList := getPackageInfo(packageList)
	sortPackageInfo(packageInfoList, sortField, sortMode)
	findGithubInfo(packageInfoList)
	markdownTable := assembleMarkdownTable(packageInfoList, sortField, sortMode)
	updateMarkdownTable(filename, markdownTable)
}

// 获取 Package 信息
// [packagesName] package 名称列表(逗号,分割)
func getPackageInfo(packagesName string) []PackageInfo {
	packageList := removeDuplicates(strings.Split(packagesName, ","))
	fmt.Println("📄", packageList)
	packageInfoList := []PackageInfo{}
	for _, value := range packageList {
		if value == "" {
			continue
		}
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
		// pubInfo := string(jsonData)
		var pubName, pubDescription, pubHomepage, pubRepository, pubIssueTracker, pubPublished string
		if value, ok := data["name"].(string); ok {
			pubName = value
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
		if value, ok := data["latest"].(map[string]interface{})["pubspec"].(map[string]interface{})["published"].(string); ok {
			pubPublished = value
		}
		if pubName != "" {
			// 可获取信息
			packageInfoList = append(
				packageInfoList,
				PackageInfo{
					Code:         1,
					Name:         pubName,
					Description:  pubDescription,
					Homepage:     pubHomepage,
					Repository:   pubRepository,
					IssueTracker: pubIssueTracker,
					Published:    pubPublished,
				},
			)
		} else {
			// 无法获取信息
			packageInfoList = append(
				packageInfoList,
				PackageInfo{
					Code: 0,
					Name: packageName,
				},
			)
		}
	}
	return packageInfoList
}

// 排序
// [packageInfoList] 	信息列表
// [sortField] 				排序字段 可选：name(default) | published
// [sortMode] 				排序方式 可选：asc(default) | desc
func sortPackageInfo(packageInfoList []PackageInfo, sortField string, sortMode string) {
	switch sortField {
	case "name":
		// 按照名称排序
		sort.SliceStable(packageInfoList, func(i, j int) bool {
			switch sortMode {
			case "asc":
				return packageInfoList[i].Name < packageInfoList[j].Name
			case "desc":
				return packageInfoList[i].Name > packageInfoList[j].Name
			default:
				return packageInfoList[i].Name < packageInfoList[j].Name
			}
		})
	case "published":
		// 按最新发布时间排序
		sort.SliceStable(packageInfoList, func(i, j int) bool {
			switch sortMode {
			case "asc":
				return packageInfoList[i].Published > packageInfoList[j].Published
			case "desc":
				return packageInfoList[i].Published < packageInfoList[j].Published
			default:
				return packageInfoList[i].Published > packageInfoList[j].Published
			}
		})
	default:
		// 按照名称排序
		sort.SliceStable(packageInfoList, func(i, j int) bool {
			switch sortMode {
			case "asc":
				return packageInfoList[i].Name < packageInfoList[j].Name
			case "desc":
				return packageInfoList[i].Name > packageInfoList[j].Name
			default:
				return packageInfoList[i].Name < packageInfoList[j].Name
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
		var name, githubStars, pubLikes, version, points, popularity, issues, pullRequests string
		switch value.Code {
		case 0:
			// 无法获取信息
			name = value.Name + " ⁉️"
		case 1:
			// 已获取信息
			// Base
			name = "[" + value.Name + "](https://pub.dev/packages/" + value.Name + ")"
			githubStars = ""
			pubLikes = "[![Pub likes](https://img.shields.io/pub/likes/" + value.Name + "?style=social&logo=flutter&logoColor=168AFD&label=)](https://pub.dev/packages/" + value.Name + ")"
			version = "[![Pub package](https://img.shields.io/pub/v/" + value.Name + "?label=)](https://pub.dev/packages/" + value.Name + ")"
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
				Description:  value.Description,
				Published:    value.Published,
				GithubStars:  githubStars,
				PubLikes:     pubLikes,
				Version:      version,
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
			"| " + value.Name + " " + value.Version + " <br/> <sub>" + formatString(value.Description) + "</sub> <br/> " + "<sub>Published: " + value.Published + "</sub>" +
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
func updateMarkdownTable(filename string, markdown string) error {
	md, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("❌ updateMarkdownTable: Error reade a file: %w", err)
	}

	start := []byte("<!-- md:PubDashboard start -->")
	before := md[:bytes.Index(md, start)+len(start)]
	end := []byte("<!-- md:PubDashboard end -->")
	after := md[bytes.Index(md, end):]

	newMd := bytes.NewBuffer(nil)
	newMd.Write(before)
	newMd.WriteString(" \n")
	newMd.WriteString(markdown)
	newMd.WriteString(" \n")
	newMd.WriteString("Updated on " + time.Now().Format(time.RFC3339) + " by [Action](https://github.com/AmosHuKe/pub-dashboard). \n")
	newMd.Write(after)

	err = os.WriteFile(filename, newMd.Bytes(), os.ModeAppend)
	if err != nil {
		return fmt.Errorf("❌ updateMarkdownTable: Error writing a file: %w", err)
	}
	fmt.Println("✅ updateMarkdownTable: Success")
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
