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

func showRegexRules(originalName string, title, year string, info FileInfo, mediaType string, tmdbID int, seasonOffset int) {
	fmt.Println("\n=== 正则替换规则 ===")
	fmt.Println("原始文件名:\n", originalName)
	fmt.Println("\n要替换成:")

	videoFormat := strings.ToLower(info.VideoFormat)

	if mediaType == MediaTypeMovie {
		finalName := fmt.Sprintf("%s.%s.%s.{tmdbid=%d;type=movie}",
			title, year, videoFormat, tmdbID)
		fmt.Println(finalName)

		pattern := regexp.QuoteMeta(originalName)

		fmt.Println()
		fmt.Printf("被替换词: \n%s\n", pattern)
		fmt.Printf("替换词: \n%s.%s.%s.{tmdbid=%d;type=movie}\n",
			title, year, videoFormat, tmdbID)
	} else {
		finalName := fmt.Sprintf("%s.%s.S%sE%s.%s.{tmdbid=%d;type=tv}",
			title, year, info.Season, info.Episode, videoFormat, tmdbID)
		fmt.Println(finalName)

		pattern := regexp.QuoteMeta(originalName)
		if info.FullMatch != "" {
			seasonEpisodePattern := regexp.QuoteMeta(info.FullMatch)
			if seasonOffset == 0 && info.Season != "01" {
				seasonEpisodePattern = strings.ReplaceAll(seasonEpisodePattern, info.Season, `(\d{1,2})`)
			}
			seasonEpisodePattern = strings.ReplaceAll(seasonEpisodePattern, info.Episode, `(\d{1,2})`)
			pattern = strings.ReplaceAll(pattern, regexp.QuoteMeta(info.FullMatch), seasonEpisodePattern)
		}

		fmt.Println()
		fmt.Printf("被替换词: \n%s\n", pattern)
		if seasonOffset == 0 && info.Season != "01" {
			fmt.Printf("替换词: \n%s.%s.S\\1E\\2.%s.{tmdbid=%d;type=tv}\n",
				title, year, videoFormat, tmdbID)
		} else {
			fmt.Printf("替换词: \n%s.%s.S%sE\\1.%s.{tmdbid=%d;type=tv}\n",
				title, year, info.Season, videoFormat, tmdbID)
		}
	}
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
	fileName := getInput("请输入文件名: ")
	if fileName == "" {
		fmt.Println("文件名不能为空，程序退出")
		os.Exit(1)
	}

	fileInfo := parseFileName(fileName)

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

	var seasonOffset int
	var manuallyInputSeason bool
	if mediaType == MediaTypeTV {
		if fileInfo.Season == "" {
			manuallyInputSeason = true
			for {
				seasonInput := getInput("未从文件名解析出季数，请手动输入（支持00、0、01、1等格式）: ")
				seasonNum, err := strconv.Atoi(seasonInput)
				if err != nil || seasonNum < 0 {
					fmt.Println("无效的季数，请输入非负数")
					continue
				}
				fileInfo.Season = ensureTwoDigits(strconv.Itoa(seasonNum))
				break
			}
		}
		if fileInfo.Episode == "" {
			fileInfo.Episode = getInput("未从文件名解析出集数，请手动输入: ")
			fileInfo.Episode = ensureTwoDigits(fileInfo.Episode)
		}

		if !manuallyInputSeason {
			for {
				offsetInput := getInput("请输入季偏移量（直接回车跳过，或输入+/-数字）: ")
				if offsetInput == "" {
					break
				}

				var err error
				seasonOffset, err = strconv.Atoi(offsetInput)
				if err != nil {
					fmt.Println("无效的偏移量，请重新输入")
					continue
				}

				currentSeason, _ := strconv.Atoi(fileInfo.Season)
				newSeason := currentSeason + seasonOffset
				if newSeason <= 0 {
					fmt.Println("调整后的季数不能为负数或零，请重新输入")
					continue
				}
				fileInfo.Season = ensureTwoDigits(strconv.Itoa(newSeason))
				break
			}
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

	showRegexRules(fileName, title, year, fileInfo, mediaType, movie.ID, seasonOffset)

	fmt.Print("\n按回车键退出...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}
