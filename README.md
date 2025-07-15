# mdserve - Serve Markdown

A simple markdown renderer written in Go.

## Features

- Live reloading [<sup>\[1\]</sup>](#credits)
- Github Flavored Markdown (GFM) support [<sup>\[2\]</sup>](#credits)
- Github-like visual [<sup>\[3\]</sup>](#credits)
- Zero configuration

It renders your Markdown file to the browser, nothing more, nothing less.

## Usage

```bash
mdserve path/to/markdown.md
```

Then open your browser and go to: `http://localhost:6942`

## Installation

### Install using `go install`:

```bash
go install github.com/badiwidya/mdserve@latest
```

Make sure your Go binary path (`$GOPATH/bin`) is included in your system's environment `$PATH`.

1. **Linux/macOs**

Add the following line to your shell config (`~/.zshrc`, `~/.bashrc`, or anything you use).

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

Then apply changes by restarting your terminal or running:

```bash
source ~/.zshrc

# or

source ~/.bashrc

# or etc.
```

2. **Windows**

- Open **Start Menu** and search for `env`, then select **Edit the system environment variables.**
- In the System Properties window, click **Environment Variables....**
- Under **System variables** (or User variables), find and select the variable named `Path`, then click **Edit**.
- Click **New**, then enter:

```
%USERPROFILE%\go\bin
```

- Click `OK` to save and apply changes.
- Restart your terminal (e.g., PowerShell, CMD, or Git Bash).

### Manual installation

1. Clone this repository:

```bash
git clone https://github.com/badiwidya/mdserve.git
cd mdserve
```

2. Build the project:

```bash
go build .
```

This will generate a `mdserve` (or `mdserve.exe` on Windows) binary in the current directory.

3. (Optional) Move the binary to your `$PATH`

On **Linux/macOS**:

```bash
mv mdserve ~/.local/bin
```

On **Windows**:

```cmd
move mdserve.exe %USERPROFILE%\bin
```

> NOTE: Make sure `%USERPROFILE%\bin` is added to your `PATH`.

## Credits

- [goldmark](https://github.com/yuin/goldmark)
- [github-markdown-css](https://github.com/sindresorhus/github-markdown-css)
- [fsnotify](https://github.com/fsnotify/fsnotify)

## License

See [LICENSE](LICENSE)
