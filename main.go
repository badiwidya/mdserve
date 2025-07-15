package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	host := runtime.GOOS

	homeEnv := func() string {
		if host == "windows" {
			return "%USERPROFILE%"
		}
		return "$HOME"
	}()

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("%s is not set.\n", homeEnv)
		os.Exit(1)
	} else {
		cachePath := filepath.Join(home, ".cache", "mdserve")
		err := os.MkdirAll(cachePath, 0755)
		if err != nil {
			fmt.Printf("Error: failed to make cache directory.\n")
			os.Exit(1)
		}
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
}
