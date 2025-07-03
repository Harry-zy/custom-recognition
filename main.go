package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL = "https://api.tmdb.org/3"
)

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
	FullMatch   string
	Season      string
	Episode     string
	VideoFormat string
}

type Config struct {
	TMDBApiKey string `json:"tmdb_api_key"`
}

func getInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func getIntInput(prompt string) (int, error) {
	input := getInput(prompt)
	return strconv.Atoi(input)
}

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

func ensureTwoDigits(num string) string {
	if len(num) == 1 {
		return "0" + num
	}
	return num
}

func parseFileName(fileName string) FileInfo {
	info := FileInfo{}

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

	seasonEpisodePatterns := []string{
		`[Ss](\d{1,2})[Ee](\d{1,2})`,
		`第(\d{1,2})季.?第(\d{1,2})集`,
		`Season\s*(\d{1,2}).*?Episode\s*(\d{1,2})`,
	}

	episodeOnlyPatterns := []string{
		`[Ee](\d{1,2})[^0-9]`,
		`第(\d{1,2})集`,
		`[Ee]p\.?(\d{1,2})`,
		`[Ee]pisode\.?(\d{1,2})`,
		`EP(\d{1,2})`,
		`Ep(\d{1,2})`,
	}

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

func findMatchingFiles(dir, pattern string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			matched, err := regexp.MatchString(pattern, info.Name())
			if err != nil {
				return err
			}
			if matched {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}

func findCommonPattern(files []string, fixedTitle string) (string, string, string) {
	if len(files) == 0 {
		return "", "", ""
	}

	// 获取第一个文件的信息作为基准
	firstFile := filepath.Base(files[0])
	fileInfo := parseFileName(firstFile)
	videoFormat := fileInfo.VideoFormat

	// 查找固定标题在文件名中的位置
	idx := strings.Index(strings.ToLower(firstFile), strings.ToLower(fixedTitle))
	if idx == -1 {
		return "", "", ""
	}

	// 分割前缀和后缀
	prefix := firstFile[:idx]
	suffix := firstFile[idx+len(fixedTitle):]

	// 从后缀中提取季集信息之前的部分
	seasonEpPattern := regexp.MustCompile(`S\d+E\d+`)
	if loc := seasonEpPattern.FindStringIndex(suffix); loc != nil {
		suffix = suffix[loc[0]:]
	}

	// 从后缀中提取视频格式之后的部分
	if videoFormat != "" {
		if idx := strings.Index(strings.ToUpper(suffix), videoFormat); idx != -1 {
			suffix = suffix[:idx] + videoFormat + ".*"
		}
	}

	// 将前缀和后缀中的特殊字符转换为正则表达式
	prefix = regexp.QuoteMeta(prefix)
	suffix = regexp.QuoteMeta(suffix)

	// 替换掉后缀中视频格式后的具体字符
	suffix = strings.Replace(suffix, `\.*`, `.*`, 1)

	// 替换季集信息为通配符
	suffix = seasonEpPattern.ReplaceAllString(suffix, `S(\d{1,2})E(\d{1,2})`)

	return prefix, suffix, videoFormat
}

func generateRegexPattern(files []string, fixedTitle string) (string, string, string) {
	if len(files) == 0 {
		return "", "", ""
	}

	// 获取第一个文件的基本信息
	firstFile := filepath.Base(files[0])
	fileInfo := parseFileName(firstFile)
	videoFormat := fileInfo.VideoFormat

	// 查找固定标题在文件名中的位置
	idx := strings.Index(strings.ToLower(firstFile), strings.ToLower(fixedTitle))
	if idx == -1 {
		return "", "", ""
	}

	// 分析所有文件名，找出共同模式
	commonPrefix := firstFile[:idx]

	// 提取第一个文件的季集信息位置
	seasonEpPattern := regexp.MustCompile(`S\d+E\d+`)
	firstFileSuffix := firstFile[idx+len(fixedTitle):]
	seasonEpLoc := seasonEpPattern.FindStringIndex(firstFileSuffix)
	if seasonEpLoc == nil {
		return "", "", ""
	}

	// 分析所有文件，找出共同的前缀和后缀模式
	for i, file := range files {
		fileName := filepath.Base(file)
		if i == 0 {
			continue
		}

		// 查找当前文件中的固定标题位置
		currIdx := strings.Index(strings.ToLower(fileName), strings.ToLower(fixedTitle))
		if currIdx == -1 {
			continue
		}

		// 更新共同前缀
		currPrefix := fileName[:currIdx]
		commonPrefix = findCommonPrefixPattern(commonPrefix, currPrefix)
	}

	// 构建最终的模式
	prefix := regexp.QuoteMeta(commonPrefix)
	suffix := `S(\d{1,2})E(\d{1,2}).*` + regexp.QuoteMeta(videoFormat)

	// 替换数字序列为通配符
	prefix = regexp.MustCompile(`\d+`).ReplaceAllString(prefix, `\d+`)

	return prefix, suffix, videoFormat
}

func findCommonPrefixPattern(a, b string) string {
	// 如果前缀完全相同，直接返回
	if a == b {
		return a
	}

	// 将连续数字替换为单个占位符
	numPattern := regexp.MustCompile(`\d+`)
	a = numPattern.ReplaceAllString(a, "#")
	b = numPattern.ReplaceAllString(b, "#")

	// 查找共同的非数字部分
	var result strings.Builder
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] == b[i] {
			result.WriteByte(a[i])
		} else if a[i] == '#' && b[i] == '#' {
			result.WriteByte('#')
		} else {
			break
		}
	}

	// 将占位符替换回数字通配符
	return strings.ReplaceAll(result.String(), "#", `\d+`)
}

func showRegexRules(originalName, fixedTitle, title, year string, info FileInfo, mediaType string, tmdbID int) {
	fmt.Println("\n=== 正则替换规则 ===")
	fmt.Println("原始文件名:\n", originalName)
	fmt.Println("\n要替换成:")

	videoFormat := strings.ToLower(info.VideoFormat)

	if mediaType == MediaTypeMovie {
		finalName := fmt.Sprintf("%s.%s.%s.{[tmdbid=%d;type=movie]}",
			title, year, videoFormat, tmdbID)
		fmt.Println(finalName)

		pattern := regexp.QuoteMeta(originalName)

		fmt.Println()
		fmt.Printf("被替换词: \n%s\n", pattern)
		fmt.Printf("替换词: \n%s.%s.%s.{[tmdbid=%d;type=movie]}\n",
			title, year, videoFormat, tmdbID)
	} else {
		finalName := fmt.Sprintf("%s.%s.S%sE%s.%s.{[tmdbid=%d;type=tv]}",
			title, year, info.Season, info.Episode, videoFormat, tmdbID)
		fmt.Println(finalName)

		// 构建正则表达式模式
		pattern := fmt.Sprintf("%s\\.?.*?[Ss](\\d{1,2})[Ee](\\d{1,2})\\.?.*?[0-9]+[pPkK]\\.?.*",
			regexp.QuoteMeta(fixedTitle))

		fmt.Println()
		fmt.Printf("被替换词: \n%s\n", pattern)
		fmt.Printf("替换词: \n%s.%s.S\\1E\\2.%s.{[tmdbid=%d;type=tv]}\n",
			title, year, videoFormat, tmdbID)
	}
}

func showBatchRegexRules(prefix, suffix, fixedTitle, title, year, videoFormat string, tmdbID int) {
	fmt.Println("\n=== 批量正则替换规则 ===")

	// 构建匹配模式
	matchPattern := fmt.Sprintf("%s\\.?.*?[Ss](\\d{1,2})[Ee](\\d{1,2})\\.?.*?[0-9]+[pPkK]\\.?.*",
		regexp.QuoteMeta(fixedTitle))

	fmt.Printf("匹配模式: \n%s\n\n", matchPattern)

	// 构建替换模式
	replacePattern := fmt.Sprintf("%s.%s.S\\1E\\2.%s.{[tmdbid=%d;type=tv]}",
		title, year, videoFormat, tmdbID)
	fmt.Printf("替换为: \n%s\n", replacePattern)

	fmt.Println("\n使用说明:")
	fmt.Println("1. 使用上述正则表达式可以匹配目录下所有相关剧集文件")
	fmt.Println("2. \\1 表示季数，\\2 表示集数")
	fmt.Println("3. 视频格式会保持文件原有的格式")
}

func readConfig() (*Config, error) {
	configPath := "custom-recognition.config"
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func saveConfig(config *Config) error {
	configPath := "custom-recognition.config"
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	return encoder.Encode(config)
}

func main() {
	// 获取当前目录
	dir := getInput("请输入视频文件所在目录（直接回车表示当前目录）: ")
	if dir == "" {
		dir = "."
	}

	// 获取要匹配的标题部分
	fixedTitle := getInput("请输入要匹配的标题固定部分: ")
	if fixedTitle == "" {
		fmt.Println("标题不能为空，程序退出")
		os.Exit(1)
	}

	// 查找匹配的文件
	pattern := fmt.Sprintf(".*%s.*", regexp.QuoteMeta(fixedTitle))
	files, err := findMatchingFiles(dir, pattern)
	if err != nil {
		fmt.Printf("搜索文件失败: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("未找到匹配的文件，程序退出")
		os.Exit(1)
	}

	fmt.Println("\n请选择要查询的媒体类型：")
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

	tmdbID, err := getIntInput("请输入TMDB ID: ")
	if err != nil || tmdbID <= 0 {
		fmt.Println("无效的TMDB ID，程序退出")
		os.Exit(1)
	}

	var apiKey string
	config, err := readConfig()
	if err == nil && config.TMDBApiKey != "" {
		apiKey = config.TMDBApiKey
	} else {
		apiKey = getInput("请输入TMDB API密钥: ")
		if apiKey == "" {
			fmt.Println("API密钥不能为空，程序退出")
			os.Exit(1)
		}

		config = &Config{TMDBApiKey: apiKey}
		if err := saveConfig(config); err != nil {
			fmt.Printf("警告：无法保存配置文件：%v\n", err)
		}
	}

	firstFile := filepath.Base(files[0])
	fileInfo := parseFileName(firstFile)

	if mediaType == MediaTypeTV {
		if fileInfo.Season == "" {
			fileInfo.Season = "01"
		}
		if fileInfo.Episode == "" {
			fileInfo.Episode = "01"
		}
	}

	if fileInfo.VideoFormat == "" {
		fileInfo.VideoFormat = getInput("未从文件名解析出视频格式，请手动输入(如: 1080P): ")
	}

	url := fmt.Sprintf("%s/%s/%d?api_key=%s&language=zh-CN", baseURL, mediaType, tmdbID, apiKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		os.Exit(1)
	}

	req.Header.Add("accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("发送请求失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API请求失败，状态码: %d，响应: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var movie MovieResponse
	if err := json.Unmarshal(body, &movie); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		os.Exit(1)
	}

	var title, year string
	if mediaType == MediaTypeMovie {
		title = movie.Title
		year = getYear(movie.ReleaseDate)
	} else {
		title = movie.Name
		year = getYear(movie.FirstAirDate)
	}

	// 显示单个文件的替换规则
	showRegexRules(firstFile, fixedTitle, title, year, fileInfo, mediaType, movie.ID)

	// 如果是电视剧，还要显示批量替换规则
	if mediaType == MediaTypeTV {
		prefix, suffix, videoFormat := generateRegexPattern(files, fixedTitle)
		if prefix != "" && suffix != "" {
			showBatchRegexRules(prefix, suffix, fixedTitle, title, year, videoFormat, movie.ID)
		}
	}

	fmt.Print("\n按回车键退出...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}
