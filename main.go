package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yuin/goldmark"
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
	</head>
	<body>
		{{ .Content }}
	</body>
</html>`

	err := os.WriteFile(path, []byte(htmlTemplate), 0644)
	if err != nil {
		return err
	}

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

	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("Error: failed to read file.\n%v\n", err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	md := goldmark.New()
	err = md.Convert(content, &buf)
	if err != nil {
		fmt.Printf("Error: failed to parse markdown.\n%v\n", err)
		os.Exit(1)
	}

	tmpl, err := template.ParseFiles(htmlPath)
	if err != nil {
		fmt.Printf("Error: failed to parse html template.\n%v\n", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, struct {
			Content template.HTML
		}{
			Content: template.HTML(buf.String()),
		})
	})

	http.ListenAndServe(":6942", nil)

}
