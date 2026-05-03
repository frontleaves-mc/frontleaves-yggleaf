package logic

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata/email_templates/*.html
var testTemplateFS embed.FS

// TestEmailTemplateRendering 验证 bamboo-base-go email 插件 v1.0.0-202605032001 的模板加载和渲染链路。
//
// 复刻 newTemplateManager + AddDir + Render（Lookup → AddParseTree → ExecuteTemplate("base")），
// 确保外部模板的 define 名与 Lookup 查找的名称一致。
func TestEmailTemplateRendering(t *testing.T) {
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("创建临时模板目录失败: %v", err)
	}

	testTemplates := map[string]string{
		"issue_create.html": `{{define "issue_create"}}
<h2>新问题提交通知</h2>
<p>用户 <strong>{{.Username}}</strong> 提交了新问题: {{.Title}}</p>
{{end}}`,
		"issue_reply.html": `{{define "issue_reply"}}
<h2>问题回复通知</h2>
<p>{{.ReplyUser}} 回复了 {{.Title}}: {{.Content}}</p>
{{end}}`,
		"issue_status.html": `{{define "issue_status"}}
<h2>问题状态变更通知</h2>
<p>{{.Title}} 从 {{.OldStatus}} 变为 {{.NewStatus}}</p>
{{end}}`,
	}

	for name, content := range testTemplates {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("写入测试模板 %s 失败: %v", name, err)
		}
	}

	// Step 1: ParseFS 内置模板
	tmpl, err := template.New("").ParseFS(testTemplateFS, "testdata/email_templates/*.html")
	if err != nil {
		t.Fatalf("ParseFS 内置模板失败: %v", err)
	}

	t.Log("=== ParseFS 后注册的模板名 ===")
	for _, t2 := range tmpl.Templates() {
		t.Logf("  %q", t2.Name())
	}

	// Step 2: AddDir
	cloned, err := tmpl.Clone()
	if err != nil {
		t.Fatalf("Clone 模板失败: %v", err)
	}

	pattern := templateDir + "/*.html"
	if _, err = cloned.ParseGlob(pattern); err != nil {
		t.Fatalf("ParseGlob 外部模板失败: %v", err)
	}

	t.Log("=== AddDir 后注册的所有模板名 ===")
	for _, t2 := range cloned.Templates() {
		t.Logf("  %q", t2.Name())
	}

	// Step 3: 新版 Render 逻辑 — Lookup → AddParseTree("content") → ExecuteTemplate("base")
	render := func(tmpl *template.Template, name string, data any) (string, error) {
		contentTmpl := tmpl.Lookup(name)
		if contentTmpl == nil {
			return "", fmt.Errorf("模板 %s 不存在", name)
		}
		tmp, err := tmpl.Clone()
		if err != nil {
			return "", fmt.Errorf("克隆模板失败: %w", err)
		}
		tmp, err = tmp.AddParseTree("content", contentTmpl.Tree)
		if err != nil {
			return "", fmt.Errorf("组合模板失败: %w", err)
		}
		var buf strings.Builder
		if err := tmp.ExecuteTemplate(&buf, "base", data); err != nil {
			return "", fmt.Errorf("渲染模板 %s 失败: %w", name, err)
		}
		return buf.String(), nil
	}

	t.Run("渲染 issue_create 模板", func(t *testing.T) {
		result, err := render(cloned, "issue_create", map[string]string{
			"Username": "TestPlayer",
			"Title":    "无法登录游戏",
		})
		if err != nil {
			t.Fatalf("渲染 issue_create 失败: %v", err)
		}
		if !strings.Contains(result, "TestPlayer") {
			t.Errorf("渲染结果应包含用户名 TestPlayer, 实际: %s", result)
		}
		if !strings.Contains(result, "无法登录游戏") {
			t.Errorf("渲染结果应包含标题, 实际: %s", result)
		}
		if !strings.Contains(result, "</html>") {
			t.Errorf("渲染结果应包含 base 布局, 实际: %s", result)
		}
	})

	t.Run("渲染 issue_reply 模板", func(t *testing.T) {
		result, err := render(cloned, "issue_reply", map[string]string{
			"ReplyUser":    "Admin",
			"Title":        "无法登录游戏",
			"Content":      "请尝试重置密码",
			"IsAdminReply": "true",
		})
		if err != nil {
			t.Fatalf("渲染 issue_reply 失败: %v", err)
		}
		if !strings.Contains(result, "Admin") {
			t.Errorf("渲染结果应包含回复者 Admin, 实际: %s", result)
		}
		if !strings.Contains(result, "</html>") {
			t.Errorf("渲染结果应包含 base 布局, 实际: %s", result)
		}
	})

	t.Run("渲染 issue_status 模板", func(t *testing.T) {
		result, err := render(cloned, "issue_status", map[string]string{
			"Title":     "无法登录游戏",
			"OldStatus": "registered",
			"NewStatus": "processing",
		})
		if err != nil {
			t.Fatalf("渲染 issue_status 失败: %v", err)
		}
		if !strings.Contains(result, "registered") || !strings.Contains(result, "processing") {
			t.Errorf("渲染结果应包含状态变更信息, 实际: %s", result)
		}
	})

	t.Run("渲染不存在的模板应报错", func(t *testing.T) {
		_, err := render(cloned, "nonexistent", nil)
		if err == nil {
			t.Fatal("渲染不存在的模板应返回错误")
		}
	})
}
