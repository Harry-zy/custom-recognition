package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	//baseURL = "https://api.themoviedb.org/3"
	baseURL = "https://api.tmdb.org/3"
)

// 支持的媒体类型
const (
	MediaTypeMovie = "movie"
	MediaTypeTV    = "tv"
)

type MovieResponse struct {
	Title        string `json:"title"`
	Name         string `json:"name"`           // 电视剧标题
	ReleaseDate  string `json:"release_date"`   // 电影日期
	FirstAirDate string `json:"first_air_date"` // 电视剧日期
	ID           int    `json:"id"`
}

type FileInfo struct {
	FullMatch   string // 完整的匹配部分
	Season      string
	Episode     string
	VideoFormat string // 存储时保持原始大小写
}

// 从用户输入获取字符串
func getInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// 从用户输入获取整数
func getIntInput(prompt string) (int, error) {
	input := getInput(prompt)
	return strconv.Atoi(input)
}

// 从日期字符串中提取年份
func getYear(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d", t.Year())
}

// 确保数字是两位数格式
func ensureTwoDigits(num string) string {
	if len(num) == 1 {
		return "0" + num
	}
	return num
}

// 从文件名中解析信息
func parseFileName(fileName string) FileInfo {
	info := FileInfo{}

	// 匹配视频格式（包括HDR和HEVC/H265）
	formatRegex := regexp.MustCompile(`\b(1080[pP]|720[pP]|2160[pP]|4[kK]|8[kK]|480[pP]|HDR|HEVC|H265)\b`)
	if matches := formatRegex.FindAllString(fileName, -1); len(matches) > 0 {
		formats := make([]string, 0)
		for _, match := range matches {
			format := strings.ToUpper(match)
			if format == "HEVC" || format == "H265" {
				continue // 跳过编码格式
			}
			formats = append(formats, format)
		}
		info.VideoFormat = strings.Join(formats, ".")
	}

	// 匹配季和集信息 (支持多种格式)
	seasonEpisodePatterns := []string{
		`[Ss](\d{1,2})[Ee](\d{1,2})`,               // S01E01 格式
		`第(\d{1,2})季.?第(\d{1,2})集`,                 // 第1季第1集 格式
		`Season\s*(\d{1,2}).*?Episode\s*(\d{1,2})`, // Season 1 Episode 1 格式
	}

	episodeOnlyPatterns := []string{
		`[Ee](\d{1,2})[^0-9]`,    // E01 格式（集数后面不能跟数字）
		`第(\d{1,2})集`,            // 第01集 格式
		`[Ee]p\.?(\d{1,2})`,      // Ep01 或 Ep.01 格式
		`[Ee]pisode\.?(\d{1,2})`, // Episode01 或 Episode.01 格式
		`EP(\d{1,2})`,            // EP01 格式（大写）
		`Ep(\d{1,2})`,            // Ep01 格式（首字母大写）
	}

	// 先尝试匹配带季数的格式
	foundMatch := false
	for _, pattern := range seasonEpisodePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(fileName); len(matches) == 3 {
			info.Season = ensureTwoDigits(matches[1])
			info.Episode = ensureTwoDigits(matches[2])
			info.FullMatch = matches[0]
			foundMatch = true
			break
		}
	}

	// 如果没找到带季数的格式，尝试只匹配集数的格式
	if !foundMatch {
		for _, pattern := range episodeOnlyPatterns {
			re := regexp.MustCompile(pattern)
			if matches := re.FindStringSubmatch(fileName); len(matches) == 2 {
				info.Season = "01" // 默认为第一季
				info.Episode = ensureTwoDigits(matches[1])
				info.FullMatch = matches[0]
				break
			}
		}
	}

	return info
}

// 显示正则替换规则
func showRegexRules(originalName string, title, year string, info FileInfo, mediaType string, tmdbID int) {
	fmt.Println("\n=== 正则替换规则 ===")
	fmt.Println("原始文件名:", originalName)
	fmt.Println("\n要替换成:")

	// 将视频格式转换为小写
	videoFormat := strings.ToLower(info.VideoFormat)

	if mediaType == MediaTypeMovie {
		finalName := fmt.Sprintf("%s.%s.%s.{tmdbid=%d}",
			title, year, videoFormat, tmdbID)
		fmt.Println(finalName)

		// 直接使用原文件名作为匹配模式
		pattern := regexp.QuoteMeta(originalName)

		fmt.Println("\n替换规则:")
		fmt.Printf("被替换词: %s\n", pattern)
		fmt.Printf("替换词: %s.%s.%s.{tmdbid=%d}\n",
			title, year, videoFormat, tmdbID)
	} else {
		finalName := fmt.Sprintf("%s.%s.S%sE%s.%s.{tmdbid=%d}",
			title, year, info.Season, info.Episode, videoFormat, tmdbID)
		fmt.Println(finalName)

		// 将原始季集信息替换为正则表达式
		pattern := regexp.QuoteMeta(originalName)
		if info.FullMatch != "" {
			seasonEpisodePattern := regexp.QuoteMeta(info.FullMatch)
			if info.Season != "01" { // 如果不是默认季数，则替换季数
				seasonEpisodePattern = strings.ReplaceAll(seasonEpisodePattern, info.Season, `(\d{1,2})`)
			}
			seasonEpisodePattern = strings.ReplaceAll(seasonEpisodePattern, info.Episode, `(\d{1,2})`)
			pattern = strings.ReplaceAll(pattern, regexp.QuoteMeta(info.FullMatch), seasonEpisodePattern)
		}

		fmt.Println("\n替换规则:")
		fmt.Printf("被替换词: %s\n", pattern)
		if info.Season == "01" { // 如果是默认季数，不需要捕获季数
			fmt.Printf("替换词: %s.%s.S01E\\1.%s.{tmdbid=%d}\n",
				title, year, videoFormat, tmdbID)
		} else {
			fmt.Printf("替换词: %s.%s.S\\1E\\2.%s.{tmdbid=%d}\n",
				title, year, videoFormat, tmdbID)
		}
	}
}

func main() {
	// 获取文件名
	fileName := getInput("请输入文件名: ")
	if fileName == "" {
		fmt.Println("文件名不能为空，程序退出")
		os.Exit(1)
	}

	// 解析文件名
	fileInfo := parseFileName(fileName)

	// 获取媒体类型
	fmt.Println("请选择要查询的媒体类型：")
	fmt.Println("1. 电影")
	fmt.Println("2. 电视节目")
	choice := getInput("请输入选项（1或2）: ")

	var mediaType string
	switch choice {
	case "1":
		mediaType = MediaTypeMovie
	case "2":
		mediaType = MediaTypeTV
	default:
		fmt.Println("无效的选项，程序退出")
		os.Exit(1)
	}

	// 获取TMDB ID
	tmdbID, err := getIntInput("请输入TMDB ID: ")
	if err != nil || tmdbID <= 0 {
		fmt.Println("无效的TMDB ID，程序退出")
		os.Exit(1)
	}

	// 获取API密钥
	apiKey := getInput("请输入TMDB API密钥: ")
	if apiKey == "" {
		fmt.Println("API密钥不能为空，程序退出")
		os.Exit(1)
	}

	// 如果是电视剧且未从文件名解析出季集信息，则请求用户输入
	if mediaType == MediaTypeTV {
		if fileInfo.Season == "" {
			fileInfo.Season = "01" // 默认为第一季
		}
		if fileInfo.Episode == "" {
			fileInfo.Episode = getInput("未从文件名解析出集数，请手动输入: ")
			fileInfo.Episode = ensureTwoDigits(fileInfo.Episode)
		}
	}

	// 如果未解析出视频格式，则请求用户输入
	if fileInfo.VideoFormat == "" {
		fileInfo.VideoFormat = getInput("未从文件名解析出视频格式，请手动输入(如: 1080P): ")
	}

	// 构建请求URL，添加language参数获取中文数据
	url := fmt.Sprintf("%s/%s/%d?api_key=%s&language=zh-CN", baseURL, mediaType, tmdbID, apiKey)

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		os.Exit(1)
	}

	// 设置请求头
	req.Header.Add("accept", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("发送请求失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		os.Exit(1)
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API请求失败，状态码: %d，响应: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	// 解析JSON响应
	var movie MovieResponse
	if err := json.Unmarshal(body, &movie); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		os.Exit(1)
	}

	// 获取标题和年份
	var title, year string
	if mediaType == MediaTypeMovie {
		title = movie.Title
		year = getYear(movie.ReleaseDate)
	} else {
		title = movie.Name
		year = getYear(movie.FirstAirDate)
	}

	// 显示正则替换规则
	showRegexRules(fileName, title, year, fileInfo, mediaType, movie.ID)

	// 等待用户按回车键退出
	fmt.Print("\n按回车键退出...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}
