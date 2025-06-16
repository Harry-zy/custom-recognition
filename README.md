# TMDB 视频文件名格式化工具

这是一个命令行工具，用于生成符合特定格式的视频文件名。通过TMDB ID获取影视信息，并生成相应的正则替换规则。

## 功能特点

- 支持电影和电视剧
- 自动识别视频分辨率和HDR信息
- 自动识别季数和集数（多种格式）
- 生成标准化的文件名格式
- 提供精确的正则替换规则

## 支持的格式

### 季集号格式
- S01E01 格式
- 第1季第1集 格式
- Season 1 Episode 1 格式
- E01 格式
- 第01集 格式
- Ep01/Ep.01 格式
- Episode01 格式

### 视频格式
- 分辨率：1080p, 720p, 2160p, 4k, 8k, 480p
- 特性：HDR

## 使用方法

1. 直接运行可执行文件：
```bash
# Windows
custom-recognition.exe

# macOS/Linux
./custom-recognition
```

2. 按提示输入：
   - 视频文件名
   - 媒体类型（电影/电视剧）
   - TMDB ID
   - API密钥

## 输出示例

### 电影
输入：
```
The.Matrix.1999.1080p.BluRay.x264
```

输出：
```
=== 正则替换规则 ===
原始文件名: The.Matrix.1999.1080p.BluRay.x264

要替换成:
黑客帝国.1999.1080p.{tmdbid=603}

替换规则:
被替换词: The\.Matrix\.1999\.1080p\.BluRay\.x264
替换词: 黑客帝国.1999.1080p.{tmdbid=603}
```

### 电视剧
输入：
```
The.Grand.Mansion.Gate.Ⅲ.2013.Ep34.WEB-DL.4K.HEVC.AAC-CHDWEB.mp4
```

输出：
```
=== 正则替换规则 ===
原始文件名: The.Grand.Mansion.Gate.Ⅲ.2013.Ep34.WEB-DL.4K.HEVC.AAC-CHDWEB.mp4

要替换成:
大宅门.2013.S01E34.4k.{tmdbid=xxx}

替换规则:
被替换词: The\.Grand\.Mansion\.Gate\.Ⅲ\.2013\.Ep(\d{1,2})\.WEB-DL\.4K\.HEVC\.AAC-CHDWEB\.mp4
替换词: 大宅门.2013.S01E\1.4k.{tmdbid=xxx}
```

## 下载

请从 [Releases](https://github.com/xxx/custom-recognition/releases) 页面下载对应系统的可执行文件：

- Windows: `custom-recognition.exe`
- macOS: `custom-recognition-darwin` (Intel/ARM)
- Linux: `custom-recognition-linux`

## 编译要求

- Go 1.16 或更高版本
- TMDB API密钥（从 [TMDB官网](https://www.themoviedb.org/settings/api) 获取）

## 从源码编译

```bash
# 编译当前平台版本
go build -o custom-recognition

# 跨平台编译
# Windows
GOOS=windows GOARCH=amd64 go build -o custom-recognition.exe

# macOS
GOOS=darwin GOARCH=amd64 go build -o custom-recognition-darwin-amd64
GOOS=darwin GOARCH=arm64 go build -o custom-recognition-darwin-arm64

# Linux
GOOS=linux GOARCH=amd64 go build -o custom-recognition-linux
``` 