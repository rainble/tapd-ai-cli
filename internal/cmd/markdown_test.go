package cmd

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownToHTML_EmptyString(t *testing.T) {
	got := markdownToHTML("")
	if got != "" {
		t.Errorf("markdownToHTML(\"\") = %q, want \"\"", got)
	}
}

func TestMarkdownToHTML_Heading(t *testing.T) {
	got := markdownToHTML("## 需求背景")
	if !strings.Contains(got, "<h2") {
		t.Errorf("markdownToHTML heading result should contain <h2>, got: %s", got)
	}
	if !strings.Contains(got, "需求背景") {
		t.Errorf("markdownToHTML heading result should contain '需求背景', got: %s", got)
	}
}

func TestMarkdownToHTML_Paragraph(t *testing.T) {
	got := markdownToHTML("这是一段普通文本。")
	if !strings.Contains(got, "<p>") {
		t.Errorf("markdownToHTML paragraph result should contain <p>, got: %s", got)
	}
	if !strings.Contains(got, "这是一段普通文本。") {
		t.Errorf("markdownToHTML paragraph result should contain original text, got: %s", got)
	}
}

func TestMarkdownToHTML_BoldAndItalic(t *testing.T) {
	got := markdownToHTML("支持 **粗体** 和 *斜体* 文本")
	if !strings.Contains(got, "<strong>粗体</strong>") {
		t.Errorf("expected <strong> tag, got: %s", got)
	}
	if !strings.Contains(got, "<em>斜体</em>") {
		t.Errorf("expected <em> tag, got: %s", got)
	}
}

func TestMarkdownToHTML_FencedCodeBlock(t *testing.T) {
	md := "```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"
	got := markdownToHTML(md)
	if !strings.Contains(got, "<pre>") || !strings.Contains(got, "<code") {
		t.Errorf("markdownToHTML code block should contain <pre> and <code>, got: %s", got)
	}
	if !strings.Contains(got, "func main()") {
		t.Errorf("markdownToHTML code block should contain code content, got: %s", got)
	}
}

func TestMarkdownToHTML_UnorderedList(t *testing.T) {
	md := "- 项目一\n- 项目二\n- 项目三"
	got := markdownToHTML(md)
	if !strings.Contains(got, "<ul>") {
		t.Errorf("expected <ul> tag, got: %s", got)
	}
	if !strings.Contains(got, "<li>") {
		t.Errorf("expected <li> tag, got: %s", got)
	}
}

func TestMarkdownToHTML_OrderedList(t *testing.T) {
	md := "1. 第一步\n2. 第二步\n3. 第三步"
	got := markdownToHTML(md)
	if !strings.Contains(got, "<ol>") {
		t.Errorf("expected <ol> tag, got: %s", got)
	}
}

func TestMarkdownToHTML_Table(t *testing.T) {
	md := "| 名称 | 描述 |\n|------|------|\n| A | B |"
	got := markdownToHTML(md)
	if !strings.Contains(got, "<table>") {
		t.Errorf("expected <table> tag, got: %s", got)
	}
}

func TestMarkdownToHTML_Blockquote(t *testing.T) {
	md := "> 这是一段引用"
	got := markdownToHTML(md)
	if !strings.Contains(got, "<blockquote>") {
		t.Errorf("expected <blockquote> tag, got: %s", got)
	}
}

func TestMarkdownToHTML_Link(t *testing.T) {
	md := "参考 [TAPD](https://www.tapd.cn)"
	got := markdownToHTML(md)
	if !strings.Contains(got, "<a href=\"https://www.tapd.cn\"") {
		t.Errorf("expected link, got: %s", got)
	}
}

func TestMarkdownToHTML_ComplexDocument(t *testing.T) {
	md := `## 需求背景

这是一个测试 **Markdown** 的需求。

### 功能要点

1. 支持**粗体**和*斜体*
2. 支持` + "`行内代码`" + `

> 引用文字

- 列表项 A
- 列表项 B`

	got := markdownToHTML(md)
	if !strings.Contains(got, "<h2") {
		t.Error("expected h2 tag")
	}
	if !strings.Contains(got, "<h3") {
		t.Error("expected h3 tag")
	}
	if !strings.Contains(got, "<ol>") {
		t.Error("expected ol tag")
	}
	if !strings.Contains(got, "<ul>") {
		t.Error("expected ul tag")
	}
	if !strings.Contains(got, "<blockquote>") {
		t.Error("expected blockquote tag")
	}
	if !strings.Contains(got, "<strong>") {
		t.Error("expected strong tag")
	}
}

func TestContainsBlockHTML_WithBlockTags(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"<p>text</p>", true},
		{"<h1>heading</h1>", true},
		{"<ul><li>item</li></ul>", true},
		{"<ol><li>item</li></ol>", true},
		{"<pre><code>code</code></pre>", true},
		{"<blockquote>quote</blockquote>", true},
		{"<table><tr><td>cell</td></tr></table>", true},
		{"<div>content</div>", true},
		{"plain text without tags", false},
		{"<span>inline only</span>", false},
		{"<em>emphasis</em>", false},
		{"", false},
	}

	for _, tt := range tests {
		got := containsBlockHTML(tt.input)
		if got != tt.want {
			t.Errorf("containsBlockHTML(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestResolveLocalImages_LocalFile(t *testing.T) {
	// 创建临时图片文件
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	imgData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG 文件头（简化）
	if err := os.WriteFile(imgPath, imgData, 0644); err != nil {
		t.Fatal(err)
	}

	content := "描述内容\n\n![测试图片](" + imgPath + ")\n\n更多内容"
	got := resolveLocalImages(content)

	expectedBase64 := base64.StdEncoding.EncodeToString(imgData)
	expectedURI := "data:image/png;base64," + expectedBase64
	if !strings.Contains(got, expectedURI) {
		t.Errorf("resolveLocalImages should embed base64 data URI, got: %s", got)
	}
	if !strings.Contains(got, "![测试图片](") {
		t.Errorf("resolveLocalImages should keep alt text, got: %s", got)
	}
}

func TestResolveLocalImages_SkipHTTPURL(t *testing.T) {
	content := "![img](https://example.com/image.png)"
	got := resolveLocalImages(content)
	if got != content {
		t.Errorf("resolveLocalImages should skip HTTP URLs, got: %s", got)
	}
}

func TestResolveLocalImages_SkipTFLPath(t *testing.T) {
	content := "![img](/tfl/captures/2026-05/tapd_123_base64_456.png)"
	got := resolveLocalImages(content)
	if got != content {
		t.Errorf("resolveLocalImages should skip /tfl/ paths, got: %s", got)
	}
}

func TestResolveLocalImages_SkipDataURI(t *testing.T) {
	content := "![img](data:image/png;base64,abc123)"
	got := resolveLocalImages(content)
	if got != content {
		t.Errorf("resolveLocalImages should skip data URIs, got: %s", got)
	}
}

func TestResolveLocalImages_NonexistentFile(t *testing.T) {
	content := "![img](/nonexistent/path/image.png)"
	got := resolveLocalImages(content)
	if got != content {
		t.Errorf("resolveLocalImages should keep original on file not found, got: %s", got)
	}
}

func TestResolveLocalImages_UnsupportedExtension(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtPath, []byte("not an image"), 0644); err != nil {
		t.Fatal(err)
	}

	content := "![img](" + txtPath + ")"
	got := resolveLocalImages(content)
	if got != content {
		t.Errorf("resolveLocalImages should skip non-image files, got: %s", got)
	}
}

func TestResolveLocalImages_MultipleImages(t *testing.T) {
	tmpDir := t.TempDir()
	img1 := filepath.Join(tmpDir, "a.jpg")
	img2 := filepath.Join(tmpDir, "b.png")
	if err := os.WriteFile(img1, []byte{0xFF, 0xD8}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(img2, []byte{0x89, 0x50}, 0644); err != nil {
		t.Fatal(err)
	}

	content := "![A](" + img1 + ")\n![B](" + img2 + ")"
	got := resolveLocalImages(content)

	if !strings.Contains(got, "data:image/jpeg;base64,") {
		t.Error("expected jpeg data URI")
	}
	if !strings.Contains(got, "data:image/png;base64,") {
		t.Error("expected png data URI")
	}
}

func TestImageMIME(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".jpg", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".JPG", "image/jpeg"},
		{".png", "image/png"},
		{".gif", "image/gif"},
		{".bmp", "image/bmp"},
		{".webp", "image/webp"},
		{".svg", "image/svg+xml"},
		{".txt", ""},
		{".go", ""},
	}
	for _, tt := range tests {
		got := imageMIME(tt.ext)
		if got != tt.want {
			t.Errorf("imageMIME(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}
