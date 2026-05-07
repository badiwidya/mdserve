package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/fsnotify/fsnotify"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/mermaid"
)

const htmlTemplate = `
<html>
<head>
<title>Markdown Renderer</title>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/5.8.1/github-markdown{{ .ThemeSuffix }}.min.css" />
<style>
	body {
		padding: 2rem;
	}

	{{ .InjectedCSS }}
</style>
<script>
	window._version = "";
	setInterval(() => {
		fetch("/ping")
			.then(r => r.text())
			.then(v => {
				if (window._version === '') {
					window._version = v;
				} else if (v !== window._version) {
					location.reload();
				}
			})
	}, 1000);
</script>
</head>
<body>
<article class="markdown-body">
	{{ .Content }}
</article>
</body>
</html>`

func main() {
	prefTheme := flag.String("theme", "system", "Set markdown preview theme (light|dark|system)")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Println("USAGE: mdserve [-theme=light|dark] [markdown file]")
		os.Exit(1)
	}

	file := flag.Arg(0)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		fmt.Printf("Error: file '%s' not found.\n", file)
		os.Exit(1)
	}
	if filepath.Ext(file) != ".md" {
		fmt.Printf("Error: '%s' is not a markdown file.\n", file)
		os.Exit(1)
	}

	injectedCSS, themeSuffix := resolveTheme(*prefTheme)
	app := &appState{
		themeSuffix: themeSuffix,
		injectedCSS: injectedCSS,
		mdEngine:    initMarkdownEngine(),
	}

	app.reload(file)

	watchFile(file, func() { app.reload(file) })

	dir := filepath.Dir(file)
	fileServer := http.FileServer(http.Dir(dir))

	http.HandleFunc("/ping", app.handlePing)
	http.HandleFunc("/", app.handleRoot(fileServer))

	port := "6942"
	url := fmt.Sprintf("http://localhost:%s", port)

	fmt.Printf("Markdown served on %s\n", url)
	fmt.Println("CTRL + C to quit...")

	openBrowser(url)

	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}

type appState struct {
	tmpl         *template.Template
	renderedHTML template.HTML
	version      string
	themeSuffix  string
	injectedCSS  template.CSS
	mdEngine     goldmark.Markdown
	mu           sync.RWMutex
}

func (a *appState) reload(filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error: failed to read file.\n%v\n", err)
		return
	}

	var buf bytes.Buffer
	if err := a.mdEngine.Convert(content, &buf); err != nil {
		fmt.Printf("Error: failed to parse markdown.\n%v\n", err)
		return
	}

	tmpl, err := template.New("index").Parse(htmlTemplate)
	if err != nil {
		fmt.Printf("Error: failed to parse html template.\n%v\n", err)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.tmpl = tmpl
	a.renderedHTML = template.HTML(buf.String())
	a.version = strconv.FormatInt(time.Now().UnixNano(), 10)

	fmt.Println("File reloaded successfully!")
}

func (a *appState) handlePing(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	fmt.Fprintf(w, "%s", a.version)
}

func (a *appState) handleRoot(fs http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			a.mu.RLock()
			defer a.mu.RUnlock()

			if a.tmpl != nil {
				a.tmpl.Execute(w, struct {
					Content     template.HTML
					InjectedCSS template.CSS
					ThemeSuffix string
				}{
					Content:     a.renderedHTML,
					InjectedCSS: a.injectedCSS,
					ThemeSuffix: a.themeSuffix,
				})
			}
			return
		}
		fs.ServeHTTP(w, r)
	}
}

func resolveTheme(prefTheme string) (template.CSS, string) {
	var customCSS, themeSuffix, chromaCSS string

	switch prefTheme {
	case "light":
		themeSuffix = "-light"
		chromaCSS = getChromaCSS("github")
	case "dark":
		themeSuffix = "-dark"
		chromaCSS = getChromaCSS("github-dark")
	default:
		customCSS = `
	@media (prefers-color-scheme: dark) {
		body { background-color: #0d1117 }
	}`
		lightCode := getChromaCSS("github")
		darkCode := getChromaCSS("github-dark")
		chromaCSS = fmt.Sprintf("%s\n@media (prefers-color-scheme: dark) {\n%s\n}", lightCode, darkCode)
	}

	return template.CSS(customCSS + "\n" + chromaCSS), themeSuffix
}

func getChromaCSS(themeName string) string {
	style := styles.Get(themeName)
	if style == nil {
		style = styles.Fallback
	}

	formatter := chromahtml.New(chromahtml.WithClasses(true))
	var buf bytes.Buffer
	formatter.WriteCSS(&buf, style)
	return buf.String()
}

func initMarkdownEngine() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&mermaid.Extender{},
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
}

func watchFile(path string, onChange func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Error: failed to create watcher.\n%v\n", err)
		return
	}

	if err := watcher.Add(path); err != nil {
		fmt.Printf("Error: failed to watch file.\n%v\n", err)
		return
	}

	go func() {
		defer watcher.Close()
		var lastEvent time.Time
		for {
			select {
			case event := <-watcher.Events:
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
					if time.Since(lastEvent) < 200*time.Millisecond {
						continue
					}
					lastEvent = time.Now()
					time.Sleep(50 * time.Millisecond)
					onChange()
					if err := watcher.Add(path); err != nil {
						fmt.Printf("Error: failed to watch file.\n%v\n", err)
					}
				}
			case err := <-watcher.Errors:
				fmt.Printf("Watch error: %v\n", err)
			}
		}
	}()
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
