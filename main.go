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

var htmlTemplate string = `
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

func getChromaCss(themeName string) string {
	style := styles.Get(themeName)
	if style == nil {
		style = styles.Fallback
	}

	formatter := chromahtml.New(chromahtml.WithClasses(true))

	var buf bytes.Buffer
	formatter.WriteCSS(&buf, style)
	return buf.String()
}

func watchFile(path string, onChange func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = watcher.Add(path)
	if err != nil {
		return err
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event := <-watcher.Events:
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
					fmt.Println("File changed, reloading....")

					time.Sleep(100 * time.Millisecond)
					onChange()

					err := watcher.Add(path)
					if err != nil {
						fmt.Printf("Error re-watching file\n%v\n", err)
					}
				}
			case err := <-watcher.Errors:
				fmt.Printf("Watch error\n%v\n", err)
			}

		}
	}()

	return nil
}

func main() {
	prefTheme := flag.String("theme", "system", "Set markdown preview theme.")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Printf("USAGE: mdserve [-theme=light|dark] [markdown file]\n")
	}

	file := flag.Arg(0)

	if _, err := os.Stat(file); os.IsNotExist(err) {
		fmt.Printf("Error: %s not found.\n", file)
		os.Exit(1)
	}

	if filepath.Ext(file) != ".md" {
		fmt.Printf("Error: %s is not a markdown file.\n", file)
		os.Exit(1)
	}

	type AppState struct {
		tmpl         *template.Template
		renderedHTML template.HTML
		version      string
		themeSuffix  string
		injectedCss  template.CSS
		mu           sync.Mutex
	}
	customCss := ""
	themeSuffix := ""
	chromaCss := ""

	switch *prefTheme {
	case "light":
		themeSuffix = "-light"
		chromaCss = getChromaCss("github")
	case "dark":
		themeSuffix = "-dark"
		chromaCss = getChromaCss("github-dark")
	default:
		customCss = `
		@media (prefers-color-scheme: dark) {
			body {
				background-color: #0d1117;
			}
		}
		`

		lightCode := getChromaCss("github")
		darkCode := getChromaCss("github-dark")

		chromaCss = lightCode + "\n@media (prefers-color-scheme: dark) {\n" + darkCode + "\n}"
	}

	injectedCss := customCss + "\n" + chromaCss

	state := &AppState{
		injectedCss: template.CSS(injectedCss),
		themeSuffix: themeSuffix,
	}

	update := func() {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error: failed to read file.\n%v\n", err)
			return
		}

		var buf bytes.Buffer
		md := goldmark.New(
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
		err = md.Convert(content, &buf)
		if err != nil {
			fmt.Printf("Error: failed to parse markdown.\n%v\n", err)
			os.Exit(1)
		}

		tmpl, err := template.New("index").Parse(htmlTemplate)
		if err != nil {
			fmt.Printf("Error: failed to parse html template.\n%v\n", err)
		}

		state.mu.Lock()
		defer state.mu.Unlock()

		state.tmpl = tmpl
		state.renderedHTML = template.HTML(buf.String())
		state.version = strconv.FormatInt(time.Now().UnixNano(), 10)

		fmt.Printf("Reloaded!\n")
	}

	update()

	watchFile(file, update)

	dir := filepath.Dir(file)
	fs := http.FileServer(http.Dir(dir))

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()

		fmt.Fprint(w, state.version)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			state.mu.Lock()
			defer state.mu.Unlock()

			state.tmpl.Execute(w, struct {
				Content     template.HTML
				InjectedCSS template.CSS
				ThemeSuffix string
			}{
				Content:     state.renderedHTML,
				InjectedCSS: state.injectedCss,
				ThemeSuffix: state.themeSuffix,
			})
			return
		}

		fs.ServeHTTP(w, r)
	})

	url := "http://localhost:6942"

	fmt.Printf("Markdown served on %s\n", url)
	fmt.Printf("CTRL + C to quit...\n")

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

	err := http.ListenAndServe(":6942", nil)
	if err != nil {
		fmt.Printf("Server failed to start\n%v\n", err)
	}
}
