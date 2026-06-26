package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const version = "0.1.0"

type CommandInfo struct {
	Title       string `json:"title"`
	Command     string `json:"command"`
	Description string `json:"description"`
	Directory   string `json:"directory"`
}

type Config struct {
	Commands []CommandInfo `json:"commands"`
}

func getGlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "just", "just.json"), nil
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func saveConfig(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

var (
	// Colors
	purple = lipgloss.Color("99")  // Vibrant Purple
	gray   = lipgloss.Color("244") // Mid Gray for explanations
	cyan   = lipgloss.Color("86")  // Aqua/Cyan for emphasis
	white  = lipgloss.Color("255") // Pure White

	// CLI Help Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			Background(purple).
			Padding(0, 1).
			MarginBottom(0)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(gray).
			Italic(true).
			MarginBottom(0)

	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(purple).
				MarginTop(0).
				MarginBottom(0)

	cmdKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan)

	optKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan).
			Width(16)

	valStyle = lipgloss.NewStyle().
			Foreground(white)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(0, 2).
			MarginTop(0).
			MarginBottom(0)

	footerStyle = lipgloss.NewStyle().
			Foreground(gray).
			MarginTop(1)
)

// TUI Model definition
type listMode int

const (
	modeList listMode = iota
	modeDelete
)

type sessionState int

const (
	stateMenu sessionState = iota
	stateList
	stateAddSaveLocation
	stateAddPathInput
	stateAddCommandInput
	stateAddTitleInput
	stateAddDescInput
)

type model struct {
	state         sessionState
	listMode      listMode
	choices       []string
	cursor        int
	saveLocCursor int
	listCursor    int
	commands      []CommandInfo
	commandToRun  string
	width         int

	// Form data
	selectedLocType string
	savePath        string
	newCommand      string
	newTitle        string
	newDesc         string

	textInput textinput.Model
	err       error
}

func initialModel() model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 60

	return model{
		state:        stateMenu,
		listMode:     modeList,
		choices:      []string{"Add Command", "List Commands", "Delete Command", "Exit"},
		cursor:       0,
		listCursor:   0,
		commands:     []CommandInfo{},
		commandToRun: "",
		width:        80,
		textInput:    ti,
		err:          nil,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// If there is an error, only Esc can clear it and go back to menu
	if m.err != nil {
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.Type == tea.KeyEsc {
				m.err = nil
				m.state = stateMenu
				return m, nil
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.textInput.Width = msg.Width - 25
		if m.textInput.Width < 20 {
			m.textInput.Width = 20
		}
	}

	switch m.state {
	case stateMenu:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.choices)-1 {
					m.cursor++
				}
			case "enter":
				selected := m.choices[m.cursor]
				switch selected {
				case "Exit":
					return m, tea.Quit
				case "Add Command":
					m.state = stateAddSaveLocation
					m.saveLocCursor = 0
					return m, nil
				case "List Commands", "Delete Command":
					m.state = stateList
					m.listCursor = 0
					if selected == "List Commands" {
						m.listMode = modeList
					} else {
						m.listMode = modeDelete
					}

					globalPath, err := getGlobalConfigPath()
					if err == nil {
						cfg, err := loadConfig(globalPath)
						if err == nil {
							m.commands = cfg.Commands
							sort.Slice(m.commands, func(i, j int) bool {
								return m.commands[i].Title < m.commands[j].Title
							})
						}
					}
					return m, nil
				}
			}
		}

	case stateList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.listCursor > 0 {
					m.listCursor--
				}
			case "down", "j":
				if m.listCursor < len(m.commands)-1 {
					m.listCursor++
				}
			case "enter":
				if len(m.commands) > 0 && m.listCursor >= 0 && m.listCursor < len(m.commands) {
					switch m.listMode {
					case modeList:
						// Set the command to run and exit the TUI
						m.commandToRun = m.commands[m.listCursor].Title
						return m, tea.Quit
					case modeDelete:
						// Delete command
						titleToDelete := m.commands[m.listCursor].Title
						globalPath, _ := getGlobalConfigPath()
						cfg, err := loadConfig(globalPath)
						if err == nil {
							var newCmds []CommandInfo
							for _, cmd := range cfg.Commands {
								if cmd.Title != titleToDelete {
									newCmds = append(newCmds, cmd)
								}
							}
							cfg.Commands = newCmds
							err = saveConfig(globalPath, cfg)
							if err == nil {
								m.commands = newCmds
								sort.Slice(m.commands, func(i, j int) bool {
									return m.commands[i].Title < m.commands[j].Title
								})
								if m.listCursor >= len(m.commands) {
									m.listCursor = len(m.commands) - 1
								}
								if m.listCursor < 0 {
									m.listCursor = 0
								}
							} else {
								m.err = err
							}
						}
					}
				}
			case "d", "x":
				// Allow d or x as alternative deletion shortcut
				if len(m.commands) > 0 && m.listCursor >= 0 && m.listCursor < len(m.commands) {
					titleToDelete := m.commands[m.listCursor].Title
					globalPath, _ := getGlobalConfigPath()
					cfg, err := loadConfig(globalPath)
					if err == nil {
						var newCmds []CommandInfo
						for _, cmd := range cfg.Commands {
							if cmd.Title != titleToDelete {
								newCmds = append(newCmds, cmd)
							}
						}
						cfg.Commands = newCmds
						err = saveConfig(globalPath, cfg)
						if err == nil {
							m.commands = newCmds
							sort.Slice(m.commands, func(i, j int) bool {
								return m.commands[i].Title < m.commands[j].Title
							})
							if m.listCursor >= len(m.commands) {
								m.listCursor = len(m.commands) - 1
							}
							if m.listCursor < 0 {
								m.listCursor = 0
							}
						} else {
							m.err = err
						}
					}
				}
			case "esc", "q":
				m.state = stateMenu
				return m, nil
			}
		}

	case stateAddSaveLocation:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.saveLocCursor > 0 {
					m.saveLocCursor--
				}
			case "down", "j":
				if m.saveLocCursor < 1 {
					m.saveLocCursor++
				}
			case "enter":
				if m.saveLocCursor == 0 {
					m.selectedLocType = "current"
					pwd, err := os.Getwd()
					if err != nil {
						m.err = err
						return m, nil
					}
					m.savePath = pwd
					m.state = stateAddCommandInput
					m.textInput.SetValue("")
					m.textInput.Placeholder = "e.g. docker-compose up -d"
					m.textInput.Prompt = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("Command ❯ ")
					m.textInput.Focus()
					return m, textinput.Blink
				} else {
					m.selectedLocType = "manual"
					m.state = stateAddPathInput
					m.textInput.SetValue("")
					m.textInput.Placeholder = "e.g. ~/Desktop or /path/to/dir"
					m.textInput.Prompt = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("Directory Path ❯ ")
					m.textInput.Focus()
					return m, textinput.Blink
				}
			case "esc":
				m.state = stateMenu
				return m, nil
			}
		}

	case stateAddPathInput:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				m.state = stateAddSaveLocation
				return m, nil
			case tea.KeyEnter:
				val := strings.TrimSpace(m.textInput.Value())
				if val == "" {
					val = "."
				}
				val = expandTilde(val)
				absVal, err := filepath.Abs(val)
				if err != nil {
					m.err = err
					return m, nil
				}
				m.savePath = absVal
				m.state = stateAddCommandInput
				m.textInput.SetValue("")
				m.textInput.Placeholder = "e.g. docker-compose up -d"
				m.textInput.Prompt = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("Command ❯ ")
				m.textInput.Focus()
				return m, textinput.Blink
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)

	case stateAddCommandInput:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				if m.selectedLocType == "manual" {
					m.state = stateAddPathInput
					m.textInput.SetValue(m.savePath)
					m.textInput.Prompt = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("Directory Path ❯ ")
				} else {
					m.state = stateAddSaveLocation
				}
				return m, nil
			case tea.KeyEnter:
				val := strings.TrimSpace(m.textInput.Value())
				if val != "" {
					m.newCommand = val
					m.state = stateAddTitleInput
					m.textInput.SetValue("")
					m.textInput.Placeholder = "e.g. deploy (no spaces allowed)"
					m.textInput.Prompt = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("Alias ❯ ")
					m.textInput.Focus()
					return m, textinput.Blink
				}
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)

	case stateAddTitleInput:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == " " {
				return m, nil
			}
			switch msg.Type {
			case tea.KeyEsc:
				m.state = stateAddCommandInput
				m.textInput.SetValue(m.newCommand)
				m.textInput.Prompt = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("Command ❯ ")
				return m, nil
			case tea.KeyEnter:
				val := strings.TrimSpace(m.textInput.Value())
				if strings.Contains(val, " ") {
					return m, nil
				}
				if val != "" {
					m.newTitle = val
					m.state = stateAddDescInput
					m.textInput.SetValue("")
					m.textInput.Placeholder = "e.g. Starts deployment of containers"
					m.textInput.Prompt = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("Description ❯ ")
					m.textInput.Focus()
					return m, textinput.Blink
				}
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)

	case stateAddDescInput:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				m.state = stateAddTitleInput
				m.textInput.SetValue(m.newTitle)
				m.textInput.Prompt = lipgloss.NewStyle().Foreground(cyan).Bold(true).Render("Alias ❯ ")
				return m, nil
			case tea.KeyEnter:
				val := strings.TrimSpace(m.textInput.Value())
				m.newDesc = val

				globalPath, err := getGlobalConfigPath()
				if err != nil {
					m.err = err
					return m, nil
				}

				cfg, err := loadConfig(globalPath)
				if err != nil {
					m.err = err
					return m, nil
				}

				found := false
				for i, cmd := range cfg.Commands {
					if cmd.Title == m.newTitle {
						cfg.Commands[i] = CommandInfo{
							Title:       m.newTitle,
							Command:     m.newCommand,
							Description: m.newDesc,
							Directory:   m.savePath,
						}
						found = true
						break
					}
				}
				if !found {
					cfg.Commands = append(cfg.Commands, CommandInfo{
						Title:       m.newTitle,
						Command:     m.newCommand,
						Description: m.newDesc,
						Directory:   m.savePath,
					})
				}

				err = saveConfig(globalPath, cfg)
				if err != nil {
					m.err = err
					return m, nil
				}

				m.state = stateMenu
				return m, nil
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	helpText := getHelpContent(m.width)

	var content string
	var footer string
	footerStyleCopy := footerStyle.Copy().Width(m.width - 4)

	if m.err != nil {
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).Width(m.width - 4).Render(fmt.Sprintf("Error: %v", m.err))
		footer = footerStyleCopy.Render("Press Esc to return to menu.")
	} else {
		switch m.state {
		case stateMenu:
			var lines []string
			lines = append(lines, sectionHeaderStyle.Render("MENU SELECTION:"))
			for i, choice := range m.choices {
				if i == m.cursor {
					lines = append(lines, lipgloss.NewStyle().Foreground(cyan).Bold(true).Render(fmt.Sprintf("  ❯ %s", choice)))
				} else {
					lines = append(lines, lipgloss.NewStyle().Foreground(gray).Render(fmt.Sprintf("    %s", choice)))
				}
			}
			content = strings.Join(lines, "\n")
			footer = footerStyleCopy.Render("Use ↑/↓ or j/k to navigate. Press Enter to select.")

		case stateList:
			tableStr, err := formatCommandsTable(m.listCursor, m.width)
			if err != nil {
				content = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).Render(fmt.Sprintf("Error loading table: %v", err))
			} else {
				var header string
				if m.listMode == modeList {
					header = sectionHeaderStyle.Render("REGISTERED COMMANDS (Select command to run):")
				} else {
					header = sectionHeaderStyle.Render("DELETE COMMAND (Select command to delete):")
				}
				content = lipgloss.JoinVertical(
					lipgloss.Left,
					header,
					tableStr,
				)
			}
			if m.listMode == modeList {
				footer = footerStyleCopy.Render("Use ↑/↓ or j/k to navigate. Press Enter to run, Esc or 'q' to return.")
			} else {
				footer = footerStyleCopy.Render("Use ↑/↓ or j/k to navigate. Press Enter to delete, Esc or 'q' to return.")
			}

		case stateAddSaveLocation:
			var lines []string
			lines = append(lines, sectionHeaderStyle.Render("ADD NEW COMMAND: EXECUTION LOCATION"))
			lines = append(lines, "Where do you want to run this command context from?")

			options := []string{"Current Directory (pwd)", "Write Manually (specify path)"}
			for i, opt := range options {
				if i == m.saveLocCursor {
					lines = append(lines, lipgloss.NewStyle().Foreground(cyan).Bold(true).Render(fmt.Sprintf("  ❯ %s", opt)))
				} else {
					lines = append(lines, lipgloss.NewStyle().Foreground(gray).Render(fmt.Sprintf("    %s", opt)))
				}
			}
			content = strings.Join(lines, "\n")
			footer = footerStyleCopy.Render("Use ↑/↓ to choose. Enter to confirm. Esc to cancel.")

		case stateAddPathInput:
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				sectionHeaderStyle.Render("ADD NEW COMMAND: DIRECTORY PATH"),
				"Enter the directory path where this command should run (e.g. ~/Desktop):",
				m.textInput.View(),
			)
			footer = footerStyleCopy.Render("Enter to confirm. Esc to go back.")

		case stateAddCommandInput:
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				sectionHeaderStyle.Render("ADD NEW COMMAND: ENTER COMMAND"),
				"Type the command you want to add to your container:",
				m.textInput.View(),
			)
			footer = footerStyleCopy.Render("Enter to confirm. Esc to go back.")

		case stateAddTitleInput:
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				sectionHeaderStyle.Render("ADD NEW COMMAND: ENTER ALIAS"),
				"Type an alias for this command (no spaces allowed):",
				m.textInput.View(),
			)
			footer = footerStyleCopy.Render("Enter to confirm. Esc to go back.")

		case stateAddDescInput:
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				sectionHeaderStyle.Render("ADD NEW COMMAND: ENTER DESCRIPTION"),
				"Type a short description of what this command does:",
				m.textInput.View(),
			)
			footer = footerStyleCopy.Render("Enter to save. Esc to go back.")
		}
	}

	tuiOutput := lipgloss.JoinVertical(
		lipgloss.Left,
		helpText,
		content,
		footer,
	)

	return lipgloss.NewStyle().MarginLeft(2).Render(tuiOutput)
}

func main() {
	args := os.Args[1:]

	// If no arguments, launch TUI
	if len(args) == 0 {
		p := tea.NewProgram(initialModel(), tea.WithAltScreen())
		m, err := p.Run()
		if err != nil {
			fmt.Printf("Error starting TUI: %v\n", err)
			os.Exit(1)
		}

		if finalModel, ok := m.(model); ok {
			if finalModel.commandToRun != "" {
				executeCommand(finalModel.commandToRun, nil)
			}
		}
		return
	}

	// Handle special flags
	switch args[0] {
	case "-v", "--version":
		fmt.Printf("just version %s\n", version)
		return
	case "-h", "--help":
		printHelp()
		return
	case "-l", "--list":
		listCommands()
		return
	case "-d", "--delete":
		if len(args) < 2 {
			fmt.Println("Error: please specify the command alias to delete.")
			fmt.Println("Usage: just -d <alias>")
			os.Exit(1)
		}
		deleteCommand(args[1])
		return
	}

	// Execute command by title
	title := args[0]
	extraArgs := args[1:]
	executeCommand(title, extraArgs)
}

func printHelp() {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		w = 80
	}
	fmt.Println(lipgloss.NewStyle().MarginLeft(2).Render(getHelpContent(w)))
}

func renderUsageRow(cmd, arg string, availWidth int) string {
	cmdStr := cmdKeyStyle.Render(cmd)
	argWidth := availWidth - 7
	if argWidth < 10 {
		argWidth = 10
	}
	argStr := valStyle.Copy().Width(argWidth).Render(arg)
	return lipgloss.JoinHorizontal(lipgloss.Top, "  ", cmdStr, " ", argStr)
}

func renderOptionRow(opt, desc string, availWidth int) string {
	optStr := optKeyStyle.Render(opt)
	descWidth := availWidth - 19
	if descWidth < 10 {
		descWidth = 10
	}
	descStr := lipgloss.NewStyle().Foreground(white).Width(descWidth).Render(desc)
	return lipgloss.JoinHorizontal(lipgloss.Top, "  ", optStr, " ", descStr)
}

func getHelpContent(w int) string {
	availWidth := w - 4
	if availWidth < 30 {
		availWidth = 30
	}

	// Banner/Header
	title := titleStyle.Render("JUST")
	subtitle := subtitleStyle.Copy().Width(availWidth).Render("A premium TUI-based command container & runner.")

	// Usage section
	usageHeader := sectionHeaderStyle.Render("USAGE")
	usageContent := strings.Join([]string{
		renderUsageRow("just", "<command> [args...]", availWidth),
		renderUsageRow("just", "[options]", availWidth),
		renderUsageRow("just", "-l", availWidth),
		renderUsageRow("just", "-d <alias>", availWidth),
	}, "\n")

	// Options section
	optionsHeader := sectionHeaderStyle.Render("OPTIONS")
	optionsContent := strings.Join([]string{
		renderOptionRow("-h, --help", "Show this help menu", availWidth),
		renderOptionRow("-v, --version", "Show version information", availWidth),
		renderOptionRow("-l, --list", "List all registered commands in a table", availWidth),
		renderOptionRow("-d, --delete", "Delete a command by alias", availWidth),
	}, "\n")

	// App flow explanation
	infoTitle := lipgloss.NewStyle().Bold(true).Foreground(purple).Render("How it works:")
	infoBody := fmt.Sprintf(
		"1. Run %s to open the interactive TUI management panel.\n"+
			"2. Follow the prompt to name, configure, and save your command.\n"+
			"3. Run %s to see a table of your saved commands.\n"+
			"4. Execute them from anywhere by simply running %s.",
		lipgloss.NewStyle().Bold(true).Foreground(cyan).Render("just"),
		lipgloss.NewStyle().Bold(true).Foreground(cyan).Render("just -l"),
		lipgloss.NewStyle().Bold(true).Foreground(cyan).Render("just <command>"),
	)

	contentWidth := availWidth - 6
	if contentWidth < 20 {
		contentWidth = 20
	}
	boxContent := lipgloss.JoinVertical(
		lipgloss.Left,
		infoTitle,
		"",
		lipgloss.NewStyle().MaxWidth(contentWidth).Render(infoBody),
	)
	infoBox := boxStyle.Copy().Render(boxContent)

	// Assemble all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		subtitle,
		usageHeader,
		usageContent,
		optionsHeader,
		optionsContent,
		infoBox,
	)
}

func padRight(s string, w int) string {
	if len(s) < w {
		return s + strings.Repeat(" ", w-len(s))
	}
	return s
}

func wrapText(text string, limit int) []string {
	if len(text) == 0 {
		return []string{""}
	}
	var lines []string
	var current string

	words := strings.Split(text, " ")
	for _, word := range words {
		if len(word) == 0 {
			continue
		}
		// If a single word is longer than the limit (e.g. a long path or command), split it
		for len(word) > limit {
			part := word[:limit]
			if len(current) > 0 {
				lines = append(lines, current)
				current = ""
			}
			lines = append(lines, part)
			word = word[limit:]
		}

		if len(current) == 0 {
			current = word
		} else if len(current)+1+len(word) > limit {
			lines = append(lines, current)
			current = word
		} else {
			current += " " + word
		}
	}
	if len(current) > 0 {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func formatCommandsTable(selectedIdx int, termWidth int) (string, error) {
	globalPath, err := getGlobalConfigPath()
	if err != nil {
		return "", err
	}
	cfg, err := loadConfig(globalPath)
	if err != nil {
		return "", err
	}
	commands := cfg.Commands
	if len(commands) == 0 {
		return "No commands registered yet.\n", nil
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Title < commands[j].Title
	})

	// Calculate column widths based on content size
	wTitle := 5 // length of "ALIAS"
	wCmd := 7   // length of "COMMAND"

	for _, cmd := range commands {
		if len(cmd.Title) > wTitle {
			wTitle = len(cmd.Title)
		}
		if len(cmd.Command) > wCmd {
			wCmd = len(cmd.Command)
		}
	}

	if termWidth < 40 {
		termWidth = 40
	}
	availColWidth := termWidth - 15

	// Cap wTitle and wCmd at 1/4 of terminal width
	limitTC := termWidth / 4
	if limitTC < 10 {
		limitTC = 10
	}

	if wTitle > limitTC {
		wTitle = limitTC
	}
	if wCmd > limitTC {
		wCmd = limitTC
	}

	// Remaining width is split between DESCRIPTION and DIRECTORY (60/40)
	remWidth := availColWidth - wTitle - wCmd
	if remWidth < 20 {
		remWidth = 20
	}

	wDesc := int(float64(remWidth) * 0.60)
	if wDesc < 12 {
		wDesc = 12
	}
	wDir := remWidth - wDesc
	if wDir < 8 {
		wDir = 8
	}

	// Stylize table elements
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(purple)
	titleColStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	cmdColStyle := lipgloss.NewStyle().Foreground(white)
	descColStyle := lipgloss.NewStyle().Foreground(gray)
	dirColStyle := lipgloss.NewStyle().Foreground(gray).Italic(true)
	borderStyle := lipgloss.NewStyle().Foreground(purple)

	// Create borders
	topBorder := borderStyle.Render("┌" + strings.Repeat("─", wTitle+2) + "┬" + strings.Repeat("─", wCmd+2) + "┬" + strings.Repeat("─", wDesc+2) + "┬" + strings.Repeat("─", wDir+2) + "┐")
	midBorder := borderStyle.Render("├" + strings.Repeat("─", wTitle+2) + "┼" + strings.Repeat("─", wCmd+2) + "┼" + strings.Repeat("─", wDesc+2) + "┼" + strings.Repeat("─", wDir+2) + "┤")
	botBorder := borderStyle.Render("└" + strings.Repeat("─", wTitle+2) + "┴" + strings.Repeat("─", wCmd+2) + "┴" + strings.Repeat("─", wDesc+2) + "┴" + strings.Repeat("─", wDir+2) + "┘")

	var sb strings.Builder
	sb.WriteString(topBorder)
	sb.WriteString("\n")

	headerRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		borderStyle.Render("│ "),
		headerStyle.Width(wTitle).Render("ALIAS"),
		borderStyle.Render(" │ "),
		headerStyle.Width(wCmd).Render("COMMAND"),
		borderStyle.Render(" │ "),
		headerStyle.Width(wDesc).Render("DESCRIPTION"),
		borderStyle.Render(" │ "),
		headerStyle.Width(wDir).Render("DIRECTORY"),
		borderStyle.Render(" │"),
	)
	sb.WriteString(headerRow)
	sb.WriteString("\n")
	sb.WriteString(midBorder)
	sb.WriteString("\n")

	for idx, cmd := range commands {
		titleLines := wrapText(cmd.Title, wTitle)
		cmdLines := wrapText(cmd.Command, wCmd)
		descLines := wrapText(cmd.Description, wDesc)
		dirLines := wrapText(cmd.Directory, wDir)

		maxLines := len(titleLines)
		if len(cmdLines) > maxLines {
			maxLines = len(cmdLines)
		}
		if len(descLines) > maxLines {
			maxLines = len(descLines)
		}
		if len(dirLines) > maxLines {
			maxLines = len(dirLines)
		}

		for i := 0; i < maxLines; i++ {
			var tLine, cLine, dLine, rLine string
			if i < len(titleLines) {
				tLine = titleLines[i]
			}
			if i < len(cmdLines) {
				cLine = cmdLines[i]
			}
			if i < len(descLines) {
				dLine = descLines[i]
			}
			if i < len(dirLines) {
				rLine = dirLines[i]
			}

			var (
				tCell = titleColStyle.Render(padRight(tLine, wTitle))
				cCell = cmdColStyle.Render(padRight(cLine, wCmd))
				dCell = descColStyle.Render(padRight(dLine, wDesc))
				rCell = dirColStyle.Render(padRight(rLine, wDir))
			)

			if idx == selectedIdx {
				highlightStyle := lipgloss.NewStyle().Background(purple).Foreground(white).Bold(true)
				tCell = highlightStyle.Render(padRight(tLine, wTitle))
				cCell = highlightStyle.Render(padRight(cLine, wCmd))
				dCell = highlightStyle.Render(padRight(dLine, wDesc))
				rCell = highlightStyle.Render(padRight(rLine, wDir))
			}

			sb.WriteString(fmt.Sprintf("%s %s %s %s %s %s %s %s %s\n",
				borderStyle.Render("│"),
				tCell,
				borderStyle.Render("│"),
				cCell,
				borderStyle.Render("│"),
				dCell,
				borderStyle.Render("│"),
				rCell,
				borderStyle.Render("│"),
			))
		}
	}
	sb.WriteString(botBorder)

	return sb.String(), nil
}

func listCommands() {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		w = 80
	}
	tableStr, err := formatCommandsTable(-1, w)
	if err != nil {
		fmt.Printf("Error formatting table: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(tableStr)
}

func deleteCommand(title string) {
	globalPath, err := getGlobalConfigPath()
	if err != nil {
		fmt.Printf("Error getting config path: %v\n", err)
		os.Exit(1)
	}

	cfg, err := loadConfig(globalPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	found := false
	var newCmds []CommandInfo
	for _, cmd := range cfg.Commands {
		if cmd.Title == title {
			found = true
		} else {
			newCmds = append(newCmds, cmd)
		}
	}

	if !found {
		fmt.Printf("Error: command '%s' not found.\n", title)
		os.Exit(1)
	}

	cfg.Commands = newCmds
	err = saveConfig(globalPath, cfg)
	if err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Command '%s' deleted successfully.\n", title)
}

func executeCommand(title string, extraArgs []string) {
	globalPath, err := getGlobalConfigPath()
	if err != nil {
		fmt.Printf("Error getting config path: %v\n", err)
		os.Exit(1)
	}
	cfg, err := loadConfig(globalPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	var targetCmd *CommandInfo
	for _, cmd := range cfg.Commands {
		if cmd.Title == title {
			targetCmd = &cmd
			break
		}
	}

	if targetCmd == nil {
		fmt.Printf("Error: command '%s' not found.\n", title)
		fmt.Println("Run 'just -l' to see all available commands.")
		os.Exit(1)
	}

	// Construct final shell command
	fullCmd := targetCmd.Command
	if len(extraArgs) > 0 {
		fullCmd += " " + strings.Join(extraArgs, " ")
	}

	// Execute via shell
	c := exec.Command("sh", "-c", fullCmd)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	// Set execution directory context if specified
	if targetCmd.Directory != "" {
		c.Dir = targetCmd.Directory
	}

	err = c.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Printf("Error executing command: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
