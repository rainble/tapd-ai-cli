# 测试需求文档

## 需求背景

本文档用于测试 Markdown 转 HTML 的各种语法支持，包括标题、表格、代码段和图片引用。

### 功能要点

1. 支持**粗体**和*斜体*文本
2. 支持`行内代码`
3. 支持本地图片自动转 base64 嵌入

## 技术方案

以下是实现的核心逻辑：

```go
func resolveLocalImages(content string) string {
    return mdImageRe.ReplaceAllStringFunc(content, func(match string) string {
        // 读取本地图片并转为 base64 data URI
        data, err := os.ReadFile(imgPath)
        if err != nil {
            return match
        }
        return "![" + alt + "](data:" + mime + ";base64," + encoded + ")"
    })
}
```

### 支持的图片格式

| 扩展名 | MIME 类型 | 说明 |
|--------|-----------|------|
| .jpg/.jpeg | image/jpeg | JPEG 格式 |
| .png | image/png | PNG 格式 |
| .gif | image/gif | GIF 动图 |
| .bmp | image/bmp | BMP 位图 |
| .webp | image/webp | WebP 格式 |
| .svg | image/svg+xml | 矢量图 |

### 处理流程

> 用户输入 Markdown -> 解析本地图片路径 -> 读取文件转 base64 -> 替换为 data URI -> 转换为 HTML -> 提交 TAPD API

## 效果展示

以下是通过本地路径引用的图片：

![AI示例图片](testdata/AI.jpg)

#### 四级标题示例

- 无序列表项 A
- 无序列表项 B
- 无序列表项 C

##### 五级标题

这是一段普通文本，用于测试段落渲染。

###### 六级标题

测试完毕。
