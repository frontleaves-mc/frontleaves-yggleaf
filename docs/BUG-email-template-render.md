# BUG: `TemplateManager.Render` 模板名查找与注册名不匹配

> **模块**: `plugins/email/template.go`
> **版本**: `v1.0.0-202605031030`
> **Go**: ≥1.25
> **严重程度**: High

## 现象

调用 `SendTemplate` 时报错：

```
html/template: "template/issue_create.html" is undefined
```

内置模板（如 `verification`）和外部模板均无法通过 `Render` 方法正常渲染。

## 复现

```go
tmpl, _ := newTemplateManager("")

// ❌ 失败 — 这才是用户实际调用的路径
_, err := tmpl.Render("verification", map[string]string{"Code": "123"})
// err: html/template: "template/verification.html" is undefined

// ✅ 成功 — 但这是绕过 Render 直接操作内部字段
tmpl.templates.ExecuteTemplate(&buf, "base", data)
```

## 根因

`Render` 硬编码了 `"template/"` 前缀（template.go:49）：

```go
func (t *TemplateManager) Render(name string, data any) (string, error) {
    tmplName := "template/" + name + ".html" // ← 构造为 "template/verification.html"
    t.templates.ExecuteTemplate(&buf, tmplName, data)
}
```

但 Go 标准库 `ParseFS` / `ParseGlob` **不会在模板名中保留 glob 匹配路径的目录前缀**：

```go
// template.go:25
tmpl, _ := template.New("").ParseFS(templateFS, "template/*.html")

// 实际注册的模板名：
//   "verification.html"   ← 不含 "template/" 前缀
//   "_base.html"
//   "base"                ← {{define "base"}}
//   "content"             ← {{define "content"}}
```

`ExecuteTemplate("template/verification.html")` 查无此模板，因为注册名是 `"verification.html"`。

同理，`AddDir` 使用 `ParseGlob` 加载外部模板：

```go
// template.go:78-79
pattern := dir + "/*.html"
externalTmpl.ParseGlob(pattern)
```

`ParseGlob("/abs/path/template/*.html")` 注册的模板名也只是 `"issue_create.html"`，不带任何路径前缀。

## 为什么现有测试没覆盖到

`TestTemplateManagerRender`（client_test.go:43）**没有调用 `Render` 方法**：

```go
// 直接用 define 名 "base" 绕过了 Render 的 "template/" + name + ".html" 逻辑
tmpl.templates.ExecuteTemplate(&buf, "base", data)
```

## 修复建议

**推荐方案**: 去掉 `Render` 中硬编码的 `"template/"` 前缀

```go
func (t *TemplateManager) Render(name string, data any) (string, error) {
    var buf strings.Builder
    tmplName := name + ".html" // 去掉 "template/" 前缀
    if err := t.templates.ExecuteTemplate(&buf, tmplName, data); err != nil {
        return "", fmt.Errorf("渲染模板 %s 失败: %w", name, err)
    }
    return buf.String(), nil
}
```

同步修改 `extractTemplateNames`（template.go:88-107），去掉 `CutPrefix("template/")` 相关逻辑。

## 建议补充的测试

```go
func TestRenderBuiltinTemplate(t *testing.T) {
    tmpl, err := newTemplateManager("")
    if err != nil {
        t.Fatalf("创建模板管理器失败: %v", err)
    }

    _, err = tmpl.Render("verification", map[string]string{
        "Code":   "123456",
        "Expire": "5分钟",
    })
    if err != nil {
        t.Fatalf("Render 内置模板失败: %v", err)
    }
}

func TestRenderExternalTemplate(t *testing.T) {
    tmpDir := t.TempDir()
    os.WriteFile(filepath.Join(tmpDir, "custom.html"), []byte(
        `{{define "custom.html"}}<p>{{.Msg}}</p>{{end}}`,
    ), 0o644)

    tmpl, err := newTemplateManager(tmpDir)
    if err != nil {
        t.Fatalf("创建模板管理器失败: %v", err)
    }

    result, err := tmpl.Render("custom", map[string]string{"Msg": "hello"})
    if err != nil {
        t.Fatalf("Render 外部模板失败: %v", err)
    }
    if !strings.Contains(result, "hello") {
        t.Fatalf("渲染结果应包含 hello, 实际: %s", result)
    }
}
```
