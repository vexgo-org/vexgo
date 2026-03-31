package public

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"vexgo/backend/model"
)

// PostTemplateData represents template data for post pages
type PostTemplateData struct {
	Post      model.Post
	Title     string
	MetaDesc  string
	Canonical string
}

// IndexTemplateData represents template data for the homepage
type IndexTemplateData struct {
	Posts     []model.Post
	Title     string
	MetaDesc  string
	Canonical string
}

// RenderPostHTML renders post page HTML
func RenderPostHTML(post model.Post, baseURL string) ([]byte, error) {
	tmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>{{.Title}}</title>
	<meta name="description" content="{{.MetaDesc}}">
	<link rel="canonical" href="{{.Canonical}}">
	<meta property="og:title" content="{{.Title}}">
	<meta property="og:description" content="{{.MetaDesc}}">
	<meta property="og:type" content="article">
	<meta property="og:url" content="{{.Canonical}}">
	{{if .Post.CoverImage}}
	<meta property="og:image" content="{{.Post.CoverImage}}">
	{{end}}
	<link rel="stylesheet" href="{{.IndexCSS}}">
</head>
<body>
	<div id="root"></div>
	<script>
		// Initialize frontend application with SSR data
		window.__INITIAL_DATA__ = {
			post: {{.PostJSON}},
			ssrRendered: true
		};
	</script>
	<script type="module" crossorigin src="{{.IndexJS}}"></script>
	<link rel="modulepreload" crossorigin href="{{.ReactVendorJS}}">
	<link rel="modulepreload" crossorigin href="{{.UIVendorJS}}">
	<link rel="modulepreload" crossorigin href="{{.UtilsVendorJS}}">
	<link rel="stylesheet" crossorigin href="{{.IndexCSS}}">
</body>
</html>`

	// Generate meta description
	metaDesc := post.Excerpt
	if metaDesc == "" {
		// If no excerpt, extract from content
		content := strings.ReplaceAll(post.Content, "\n", " ")
		if len(content) > 150 {
			metaDesc = content[:150] + "..."
		} else {
			metaDesc = content
		}
	}

	// Generate canonical URL
	canonical := fmt.Sprintf("%s/posts/%d", baseURL, post.ID)

	// Generate JSON data
	postJSON, err := model.ToJSON(post)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"Post":          post,
		"Title":         post.Title,
		"MetaDesc":      metaDesc,
		"Canonical":     canonical,
		"PostJSON":      template.JS(postJSON),
		"IndexCSS":      GetAssetURL("css", "index"),
		"IndexJS":       GetAssetURL("js", "index"),
		"ReactVendorJS": GetAssetURL("js", "react-vendor"),
		"UIVendorJS":    GetAssetURL("js", "ui-vendor"),
		"UtilsVendorJS": GetAssetURL("js", "utils-vendor"),
	}

	// Parse template
	t, err := template.New("post").Parse(tmpl)
	if err != nil {
		return nil, err
	}

	// Render template
	var buf bytes.Buffer
	err = t.Execute(&buf, data)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// RenderIndexHTML renders homepage HTML
func RenderIndexHTML(posts []model.Post, baseURL string) ([]byte, error) {
	tmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>{{.Title}}</title>
	<meta name="description" content="{{.MetaDesc}}">
	<link rel="canonical" href="{{.Canonical}}">
	<meta property="og:title" content="{{.Title}}">
	<meta property="og:description" content="{{.MetaDesc}}">
	<meta property="og:type" content="website">
	<meta property="og:url" content="{{.Canonical}}">
	<link rel="stylesheet" href="{{.IndexCSS}}">
</head>
<body>
	<div id="root"></div>
	<script>
		// Initialize frontend application with SSR data
		window.__INITIAL_DATA__ = {
			posts: {{.PostsJSON}},
			hasPosts: {{if .Posts}}true{{else}}false{{end}},
			ssrRendered: true
		};
	</script>
	<script type="module" crossorigin src="{{.IndexJS}}"></script>
	<link rel="modulepreload" crossorigin href="{{.ReactVendorJS}}">
	<link rel="modulepreload" crossorigin href="{{.UIVendorJS}}">
	<link rel="modulepreload" crossorigin href="{{.UtilsVendorJS}}">
	<link rel="stylesheet" crossorigin href="{{.IndexCSS}}">
</body>
</html>`

	// Custom template functions
	t := template.New("index").Funcs(template.FuncMap{
		"truncate": func(s string, max int) string {
			s = strings.ReplaceAll(s, "\n", " ")
			if len(s) > max {
				return s[:max] + "..."
			}
			return s
		},
	})

	// Parse template
	var err error
	t, err = t.Parse(tmpl)
	if err != nil {
		return nil, err
	}

	// Generate meta description
	metaDesc := "最新文章列表"
	if len(posts) > 0 {
		metaDesc = fmt.Sprintf("最新文章: %s", posts[0].Title)
		if len(posts) > 1 {
			metaDesc += fmt.Sprintf("、%s 等", posts[1].Title)
		}
	}

	// Generate canonical URL
	canonical := baseURL

	// Generate JSON data
	postsJSON, err := model.ToJSON(posts)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"Posts":         posts,
		"Title":         "博客首页",
		"MetaDesc":      metaDesc,
		"Canonical":     canonical,
		"PostsJSON":     template.JS(postsJSON),
		"IndexCSS":      GetAssetURL("css", "index"),
		"IndexJS":       GetAssetURL("js", "index"),
		"ReactVendorJS": GetAssetURL("js", "react-vendor"),
		"UIVendorJS":    GetAssetURL("js", "ui-vendor"),
		"UtilsVendorJS": GetAssetURL("js", "utils-vendor"),
	}

	// Render template
	var buf bytes.Buffer
	err = t.Execute(&buf, data)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
