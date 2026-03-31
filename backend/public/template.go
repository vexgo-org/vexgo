package public

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"vexgo/backend/model"
)

// PostTemplateData 文章页面模板数据
type PostTemplateData struct {
	Post      model.Post
	Title     string
	MetaDesc  string
	Canonical string
}

// IndexTemplateData 首页模板数据
type IndexTemplateData struct {
	Posts     []model.Post
	Title     string
	MetaDesc  string
	Canonical string
}

// RenderPostHTML 渲染文章页面HTML
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
	<link rel="stylesheet" href="/assets/index-4WleuXJq.css">
	<link rel="stylesheet" href="/assets/index-B34w2e4C.css">
</head>
<body>
	<div id="root"></div>
	<script>
		// 初始化前端应用
		window.__INITIAL_DATA__ = {
			post: {{.PostJSON}}
		};
	</script>
	<script type="module" crossorigin src="/assets/index-DrBv4CXU.js"></script>
	<link rel="modulepreload" crossorigin href="/assets/react-vendor-BmqGXi6J.js">
	<link rel="modulepreload" crossorigin href="/assets/ui-vendor-CEsCEvQe.js">
	<link rel="modulepreload" crossorigin href="/assets/utils-vendor-42ANG6Sg.js">
	<link rel="stylesheet" crossorigin href="/assets/index-BTvxqpsA.css">
</body>
</html>`

	// 生成摘要
	metaDesc := post.Excerpt
	if metaDesc == "" {
		// 如果没有摘要，从内容中提取
		content := strings.ReplaceAll(post.Content, "\n", " ")
		if len(content) > 150 {
			metaDesc = content[:150] + "..."
		} else {
			metaDesc = content
		}
	}

	// 生成规范URL
	canonical := fmt.Sprintf("%s/posts/%d", baseURL, post.ID)

	// 生成JSON数据
	postJSON, err := model.ToJSON(post)
	if err != nil {
		return nil, err
	}

	data := PostTemplateData{
		Post:      post,
		Title:     post.Title,
		MetaDesc:  metaDesc,
		Canonical: canonical,
	}

	// 解析模板
	t, err := template.New("post").Parse(tmpl)
	if err != nil {
		return nil, err
	}

	// 渲染模板
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]interface{}{
		"Post":      data.Post,
		"Title":     data.Title,
		"MetaDesc":  data.MetaDesc,
		"Canonical": data.Canonical,
		"PostJSON":  template.JS(postJSON),
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// RenderIndexHTML 渲染首页HTML
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
	<link rel="stylesheet" href="/assets/index-4WleuXJq.css">
	<link rel="stylesheet" href="/assets/index-B34w2e4C.css">
</head>
<body>
	<div id="root">
		{{if .Posts}}
		<div class="post-list">
			{{range .Posts}}
			<article class="post-item">
				<h2><a href="/posts/{{.ID}}">{{.Title}}</a></h2>
				<div class="post-meta">
					<span>作者: {{.Author.Username}}</span>
					<span>发布时间: {{.CreatedAt.Format "2006-01-02"}}</span>
					<span>阅读: {{.ViewCount}}</span>
				</div>
				{{if .CoverImage}}
				<div class="post-cover">
					<img src="{{.CoverImage}}" alt="{{.Title}}">
				</div>
				{{end}}
				<div class="post-excerpt">{{if .Excerpt}}{{.Excerpt}}{{else}}{{.Content | truncate 200}}{{end}}</div>
				<div class="post-tags">
					{{range .Tags}}
						<span class="tag">{{.Name}}</span>
					{{end}}
				</div>
			</article>
			{{end}}
		</div>
		{{else}}
		<div class="empty-posts-message">
			<p>暂无文章</p>
		</div>
		{{end}}
	</div>
	<script>
		// 初始化前端应用
		window.__INITIAL_DATA__ = {
			posts: {{.PostsJSON}},
			hasPosts: {{if .Posts}}true{{else}}false{{end}}
		};
	</script>
	<script type="module" crossorigin src="/assets/index-DrBv4CXU.js"></script>
	<link rel="modulepreload" crossorigin href="/assets/react-vendor-BmqGXi6J.js">
	<link rel="modulepreload" crossorigin href="/assets/ui-vendor-CEsCEvQe.js">
	<link rel="modulepreload" crossorigin href="/assets/utils-vendor-42ANG6Sg.js">
	<link rel="stylesheet" crossorigin href="/assets/index-BTvxqpsA.css">
</body>
</html>`

	// 自定义模板函数
	t := template.New("index").Funcs(template.FuncMap{
		"truncate": func(s string, max int) string {
			s = strings.ReplaceAll(s, "\n", " ")
			if len(s) > max {
				return s[:max] + "..."
			}
			return s
		},
	})

	// 解析模板
	var err error
	t, err = t.Parse(tmpl)
	if err != nil {
		return nil, err
	}

	// 生成摘要
	metaDesc := "最新文章列表"
	if len(posts) > 0 {
		metaDesc = fmt.Sprintf("最新文章: %s", posts[0].Title)
		if len(posts) > 1 {
			metaDesc += fmt.Sprintf("、%s 等", posts[1].Title)
		}
	}

	// 生成规范URL
	canonical := baseURL

	// 生成JSON数据
	postsJSON, err := model.ToJSON(posts)
	if err != nil {
		return nil, err
	}

	data := IndexTemplateData{
		Posts:     posts,
		Title:     "博客首页",
		MetaDesc:  metaDesc,
		Canonical: canonical,
	}

	// 渲染模板
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]interface{}{
		"Posts":     data.Posts,
		"Title":     data.Title,
		"MetaDesc":  data.MetaDesc,
		"Canonical": data.Canonical,
		"PostsJSON": template.JS(postsJSON),
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
