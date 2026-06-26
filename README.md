# just

`just` is a premium, lightweight TUI-based command container and runner written in Go. It allows you to organize, store, and execute your daily command-line snippets and scripts from anywhere with clean aliases, without cluttering your system or configuration files.

Built on top of Charm's **Bubble Tea** and **Lipgloss**, `just` offers a modern, visual CLI interface that adapts dynamically to your terminal size.

![just screenshot](screenshot.png)

## Features

- 🖥️ **Alternate Screen Buffer Mode**: Full-screen interactive CLI dashboard that launches and exits cleanly without polluting your terminal scrollback history.
- 📐 **Dynamic Layout Wrapping**: Help banners, options, and boxes automatically wrap based on terminal viewport width.
- 📊 **Adaptive Commands Table**: Fully expands columns to terminal width. The `ALIAS` and `COMMAND` columns size dynamically to fit content (capped at 1/4 of the viewport size), allocating remaining space to the `DESCRIPTION` and `DIRECTORY` columns.
- 📂 **Directory Context Aware**: Run commands directly in the directories where they belong (e.g. current directory at time of creation, or a manually specified path with tilde `~` expansion).
- 💾 **Global Persistence**: Saves all configurations in a single global JSON file at `~/.config/just/just.json`.

---

## Installation

### Shell Script (via curl)

You can download and install the latest compiled binary automatically using:

```bash
curl -sSL https://raw.githubusercontent.com/SavNico/just/main/install.sh | sh
```

*Note: The script detects your OS (macOS/Linux) and CPU architecture, downloads the tarball, extracts it, and installs it to `/usr/local/bin/just`.*

### Building from Source

To compile and install manually from source, ensure you have **Go** installed:

```bash
# Clone the repository
git clone https://github.com/SavNico/just.git
cd just

# Compile the binary
go build -o just

# Move to your path (Linux/macOS)
sudo mv just /usr/local/bin/
```

---

## How It Works

1. Run **`just`** from your shell to open the interactive panel.
2. Select **Add Command** to name your command, set its execution directory, enter the shell command itself, add a description, and assign an **Alias**.
3. View or delete registered commands via the **List Commands** or **Delete Command** menus in the TUI, or directly from the CLI.
4. Execute any command from anywhere using its alias:
   ```bash
   just <alias> [extra args...]
   ```

---

## CLI Usage

```text
USAGE
  just <alias> [args...]    Run a registered command by its alias
  just [options]            Open the interactive TUI management panel
  just -l                   List all registered commands in a table
  just -d <alias>           Delete a registered command by its alias

OPTIONS
  -h, --help                Show the CLI help menu
  -v, --version             Show version information
  -l, --list                List all registered commands in a table
  -d, --delete              Delete a command by alias
```

---

## Configuration File

Your commands are saved in:
`~/.config/just/just.json`

Example file format:
```json
{
  "commands": [
    {
      "title": "deploy",
      "command": "docker-compose up -d --build",
      "description": "Starts local docker containers",
      "directory": "/Users/user/Projects/webapp"
    }
  ]
}
```

---

## License

This project is licensed under the **MIT License**. See the LICENSE file for details.
