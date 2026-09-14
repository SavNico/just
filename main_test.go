package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "just.json")

	// Test loading nonexistent file returns empty config
	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("expected no error for nonexistent file, got: %v", err)
	}
	if len(cfg.Commands) != 0 {
		t.Fatalf("expected 0 commands, got: %d", len(cfg.Commands))
	}

	// Test saving config
	sampleCmds := []CommandInfo{
		{
			Title:       "build",
			Command:     "go build -o app",
			Description: "Compile the application",
			Directory:   "/tmp",
		},
		{
			Title:       "test",
			Command:     "go test ./...",
			Description: "Run test suite",
			Directory:   "",
		},
	}
	cfg.Commands = sampleCmds
	if err := saveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Test loading saved config
	loadedCfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if len(loadedCfg.Commands) != 2 {
		t.Fatalf("expected 2 commands, got: %d", len(loadedCfg.Commands))
	}
	if loadedCfg.Commands[0].Title != "build" || loadedCfg.Commands[1].Title != "test" {
		t.Fatalf("unexpected commands in loaded config: %+v", loadedCfg.Commands)
	}
}

func TestCommandEditLogic(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "just.json")

	cfg := Config{
		Commands: []CommandInfo{
			{
				Title:       "deploy",
				Command:     "docker compose up -d",
				Description: "Starts containers",
				Directory:   "/var/www",
			},
			{
				Title:       "logs",
				Command:     "docker compose logs -f",
				Description: "Tail logs",
				Directory:   "/var/www",
			},
		},
	}

	if err := saveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save initial config: %v", err)
	}

	// Simulate editing "deploy" -> rename alias to "deploy-prod", update command and description
	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	origTitle := "deploy"
	newTitle := "deploy-prod"
	newCommand := "docker compose -f prod.yml up -d"
	newDesc := "Production deployment"
	newDir := "/var/www/prod"

	found := false
	for i, cmd := range loaded.Commands {
		if cmd.Title == origTitle {
			loaded.Commands[i] = CommandInfo{
				Title:       newTitle,
				Command:     newCommand,
				Description: newDesc,
				Directory:   newDir,
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("original command %s not found for edit", origTitle)
	}

	if err := saveConfig(configPath, loaded); err != nil {
		t.Fatalf("failed to save edited config: %v", err)
	}

	// Verify persistence of edited command
	updatedCfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	if len(updatedCfg.Commands) != 2 {
		t.Fatalf("expected 2 commands after edit, got %d", len(updatedCfg.Commands))
	}

	var editedCmd *CommandInfo
	for _, cmd := range updatedCfg.Commands {
		if cmd.Title == newTitle {
			c := cmd
			editedCmd = &c
		}
		if cmd.Title == origTitle {
			t.Fatalf("old title %s should no longer exist", origTitle)
		}
	}

	if editedCmd == nil {
		t.Fatalf("edited command with title %s not found", newTitle)
	}
	if editedCmd.Command != newCommand {
		t.Errorf("expected command %q, got %q", newCommand, editedCmd.Command)
	}
	if editedCmd.Description != newDesc {
		t.Errorf("expected desc %q, got %q", newDesc, editedCmd.Description)
	}
	if editedCmd.Directory != newDir {
		t.Errorf("expected dir %q, got %q", newDir, editedCmd.Directory)
	}
}

func TestInitialModel(t *testing.T) {
	m := initialModel()
	if m.state != stateMenu {
		t.Errorf("expected initial state stateMenu (%d), got %d", stateMenu, m.state)
	}

	expectedChoices := []string{"Add Command", "Edit Command", "List Commands", "Delete Command", "Exit"}
	if len(m.choices) != len(expectedChoices) {
		t.Fatalf("expected %d choices, got %d", len(expectedChoices), len(m.choices))
	}
	for i, ch := range expectedChoices {
		if m.choices[i] != ch {
			t.Errorf("choice %d expected %q, got %q", i, ch, m.choices[i])
		}
	}
}

func TestStartEditing(t *testing.T) {
	m := initialModel()
	cmd := CommandInfo{
		Title:       "myalias",
		Command:     "npm start",
		Description: "Run dev server",
		Directory:   "/projects/frontend",
	}

	updatedModel, _ := m.startEditing(cmd)
	if updatedModel.state != stateEditDirectoryInput {
		t.Errorf("expected state stateEditDirectoryInput, got %d", updatedModel.state)
	}
	if updatedModel.editingOriginalTitle != "myalias" {
		t.Errorf("expected editingOriginalTitle 'myalias', got %q", updatedModel.editingOriginalTitle)
	}
	if updatedModel.editDirectory != "/projects/frontend" {
		t.Errorf("expected editDirectory '/projects/frontend', got %q", updatedModel.editDirectory)
	}
	if updatedModel.editCommand != "npm start" {
		t.Errorf("expected editCommand 'npm start', got %q", updatedModel.editCommand)
	}
	if updatedModel.editTitle != "myalias" {
		t.Errorf("expected editTitle 'myalias', got %q", updatedModel.editTitle)
	}
	if updatedModel.editDesc != "Run dev server" {
		t.Errorf("expected editDesc 'Run dev server', got %q", updatedModel.editDesc)
	}
	if updatedModel.textInput.Value() != "/projects/frontend" {
		t.Errorf("expected textInput pre-filled with directory, got %q", updatedModel.textInput.Value())
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("unable to get user home dir")
	}

	res := expandTilde("~/Desktop")
	expected := filepath.Join(home, "Desktop")
	if res != expected {
		t.Errorf("expected %q, got %q", expected, res)
	}

	noTilde := "/var/log"
	if expandTilde(noTilde) != noTilde {
		t.Errorf("expected %q, got %q", noTilde, expandTilde(noTilde))
	}
}

func TestWrapText(t *testing.T) {
	text := "one two three four five"
	lines := wrapText(text, 10)
	if len(lines) == 0 {
		t.Fatal("expected wrapped lines")
	}
	for _, l := range lines {
		if len(l) > 10 {
			t.Errorf("line %q exceeds limit 10", l)
		}
	}
}

func TestPadRight(t *testing.T) {
	padded := padRight("test", 8)
	if len(padded) != 8 {
		t.Errorf("expected length 8, got %d", len(padded))
	}
	if !strings.HasPrefix(padded, "test") {
		t.Errorf("expected prefix 'test', got %q", padded)
	}
}

func TestFormatAliasesCompletion(t *testing.T) {
	cfg := Config{
		Commands: []CommandInfo{
			{
				Title:       "build",
				Command:     "go build",
				Description: "Build binary",
			},
			{
				Title:       "run",
				Command:     "go run main.go",
				Description: "", // empty description should fallback to Command
			},
		},
	}

	out := formatAliasesCompletion(cfg)
	expected := "build\tBuild binary\nrun\tgo run main.go\n"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestCompletionScripts(t *testing.T) {
	if !strings.Contains(zshCompletion, "_just_completion") || !strings.Contains(zshCompletion, "compdef") {
		t.Error("zshCompletion missing key definitions")
	}
	if !strings.Contains(bashCompletion, "_just_completion") || !strings.Contains(bashCompletion, "complete -F") {
		t.Error("bashCompletion missing key definitions")
	}
	if !strings.Contains(fishCompletion, "complete -c just") {
		t.Error("fishCompletion missing key definitions")
	}
}
