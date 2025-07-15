package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

func initCacheDir() (string, error) {
	var cachePath string
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	} else {
		cachePath = filepath.Join(home, ".cache", "mdserve")
		err := os.MkdirAll(cachePath, 0755)
		if err != nil {
			return "", err
		}
	}

	return cachePath, nil
}

func initHtmlTemplate(path string) error {
	htmlTemplate := `
<html>
	<head>
		<title>Markdown Renderer</title>
		<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/5.8.1/github-markdown-light.min.css" />
		<style>body { padding: 2rem; }</style>
	</head>
	<body>
		<article class="markdown-body">
			{{ .Content }}
		</article>
	</body>
</html>`

	err := os.WriteFile(path, []byte(htmlTemplate), 0644)
	if err != nil {
		return err
	}

	return nil
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
				if event.Op&fsnotify.Write == fsnotify.Write {
					fmt.Println("File changed, reloading....")
					onChange()
				}
			case err := <-watcher.Errors:
				fmt.Printf("watch error\n%v\n", err)
			}

		}
	}()

	return nil
}

func main() {
	cachePath, err := initCacheDir()
	if err != nil {
		fmt.Printf("Error: can't initialize cache dir.\n%v\n", err)
		os.Exit(1)
	}

	htmlPath := filepath.Join(cachePath, "index.html")
	err = initHtmlTemplate(htmlPath)
	if err != nil {
		fmt.Printf("Error: can't initialize html template.\n%v\n", err)
		os.Exit(1)
	}

	if len(os.Args) != 2 {
		fmt.Printf("USAGE: mdserve [markdown file]\n")
		os.Exit(1)
	}

	file := os.Args[1]

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
	}

	state := &AppState{}

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
			),
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
			),
		)
		err = md.Convert(content, &buf)
		if err != nil {
			fmt.Printf("Error: failed to parse markdown.\n%v\n", err)
			os.Exit(1)
		}

		tmpl, err := template.ParseFiles(htmlPath)
		if err != nil {
			fmt.Printf("Error: failed to parse html template.\n%v\n", err)
		}

		state.tmpl = tmpl
		state.renderedHTML = template.HTML(buf.String())
		state.version = strconv.FormatInt(time.Now().UnixNano(), 10)

		fmt.Printf("Reloaded!")
	}

	update()

	watchFile(file, update)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		state.tmpl.Execute(w, struct {
			Content template.HTML
		}{
			Content: state.renderedHTML,
		})
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

	err = http.ListenAndServe(":6942", nil)
	if err != nil {
		fmt.Printf("Server failed to start\n%v\n", err)
	}
}
