// Package cmd 中的 markdown.go 提供 Markdown 与 HTML 的双向转换辅助函数
package cmd

import (
	"encoding/base64"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// markdownToHTML 将 Markdown 文本转换为 HTML。
// 仅当转换结果包含块级 HTML 元素时才返回 HTML，否则返回原始文本。
// 这样可以避免对已经是纯文本的内容进行不必要的转换。
func markdownToHTML(md string) string {
	if md == "" {
		return md
	}

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs |
		parser.FencedCode | parser.Tables
	p := parser.NewWithExtensions(extensions)

	opts := html.RendererOptions{Flags: html.CommonFlags}
	renderer := html.NewRenderer(opts)

	htmlBytes := markdown.ToHTML([]byte(md), p, renderer)
	result := strings.TrimSpace(string(htmlBytes))

	// 安全检查：仅当输出包含块级 HTML 元素时才使用 HTML
	if containsBlockHTML(result) {
		return result
	}
	return md
}

// containsBlockHTML 检查 HTML 字符串是否包含块级元素标签
func containsBlockHTML(s string) bool {
	blockTags := []string{"<p>", "<p ", "<h1", "<h2", "<h3", "<h4", "<h5", "<h6",
		"<ul", "<ol", "<li", "<pre", "<blockquote", "<table", "<div"}
	lower := strings.ToLower(s)
	for _, tag := range blockTags {
		if strings.Contains(lower, tag) {
			return true
		}
	}
	return false
}

// htmlToMarkdown 将 HTML 文本转换为 Markdown。
// 空字符串直接返回；转换失败时返回原始 HTML。
func htmlToMarkdown(h string) string {
	if h == "" {
		return h
	}
	md, err := htmltomarkdown.ConvertString(h)
	if err != nil {
		return h
	}
	return md
}

// mdImageRe 匹配 Markdown 图片语法：![alt](path) 或 ![alt](path "title")
var mdImageRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)"]+)(?:\s+"[^"]*")?\)`)

// resolveLocalImages 将 Markdown 中引用的本地图片文件转换为 base64 data URI。
// 仅处理本地文件路径，跳过 HTTP URL、TAPD 内部路径和已有的 data URI。
func resolveLocalImages(content string) string {
	return mdImageRe.ReplaceAllStringFunc(content, func(match string) string {
		subs := mdImageRe.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		imgPath := strings.TrimSpace(subs[2])

		// 跳过非本地路径
		if strings.HasPrefix(imgPath, "http://") ||
			strings.HasPrefix(imgPath, "https://") ||
			strings.HasPrefix(imgPath, "data:") ||
			strings.HasPrefix(imgPath, "/tfl/") {
			return match
		}

		// 展开 ~ 为用户 home 目录
		if strings.HasPrefix(imgPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return match
			}
			imgPath = filepath.Join(home, imgPath[2:])
		}

		// 相对路径转绝对路径
		if !filepath.IsAbs(imgPath) {
			wd, err := os.Getwd()
			if err != nil {
				return match
			}
			imgPath = filepath.Join(wd, imgPath)
		}

		// 读取文件
		data, err := os.ReadFile(imgPath)
		if err != nil {
			return match
		}

		// 确定 MIME 类型
		mimeType := imageMIME(filepath.Ext(imgPath))
		if mimeType == "" {
			return match
		}

		// 构造 data URI 并替换
		dataURI := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
		return "![" + subs[1] + "](" + dataURI + ")"
	})
}

// imageMIME 根据文件扩展名返回图片 MIME 类型
func imageMIME(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		// 尝试系统 MIME 类型检测
		t := mime.TypeByExtension(ext)
		if strings.HasPrefix(t, "image/") {
			return t
		}
		return ""
	}
}
