package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallOpenCodeMCPConfigCreatesConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".config", "opencode", "opencode.json")
	mnemoPath := "/opt/homebrew/bin/mnemo"

	if err := installOpenCodeMCPConfig(configPath, mnemoPath); err != nil {
		t.Fatalf("installOpenCodeMCPConfig() error = %v", err)
	}

	config := readJSONFile(t, configPath)
	mnemo := config["mcp"].(map[string]interface{})["mnemo"].(map[string]interface{})

	if got, want := mnemo["type"], "local"; got != want {
		t.Fatalf("mnemo.type = %v, want %v", got, want)
	}
	if got, want := mnemo["enabled"], true; got != want {
		t.Fatalf("mnemo.enabled = %v, want %v", got, want)
	}
	if got, want := stringSlice(mnemo["command"]), []string{mnemoPath, "serve"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mnemo.command = %v, want %v", got, want)
	}
}

func TestInstallOpenCodeMCPConfigPreservesExistingConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}

	existing := []byte(`{
  "$schema": "https://opencode.ai/config.json",
  "theme": "system",
  "mcp": {
    "github": {
      "type": "local",
      "command": ["gh", "mcp", "server"],
      "enabled": false
    }
  }
}`)
	if err := os.WriteFile(configPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	if err := installOpenCodeMCPConfig(configPath, "/usr/local/bin/mnemo"); err != nil {
		t.Fatalf("installOpenCodeMCPConfig() error = %v", err)
	}

	config := readJSONFile(t, configPath)
	if got, want := config["$schema"], "https://opencode.ai/config.json"; got != want {
		t.Fatalf("$schema = %v, want %v", got, want)
	}
	if got, want := config["theme"], "system"; got != want {
		t.Fatalf("theme = %v, want %v", got, want)
	}

	mcp := config["mcp"].(map[string]interface{})
	github := mcp["github"].(map[string]interface{})
	if got, want := stringSlice(github["command"]), []string{"gh", "mcp", "server"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("github.command = %v, want %v", got, want)
	}
	if _, ok := mcp["mnemo"]; !ok {
		t.Fatal("mnemo MCP server was not added")
	}
}

func TestInstallOpenCodeMCPConfigUpdatesMnemoEntry(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	existing := []byte(`{
  "mcp": {
    "mnemo": {
      "type": "local",
      "command": ["old-mnemo", "serve"],
      "enabled": false
    }
  }
}`)
	if err := os.WriteFile(configPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	if err := installOpenCodeMCPConfig(configPath, "/new/mnemo"); err != nil {
		t.Fatalf("installOpenCodeMCPConfig() error = %v", err)
	}

	config := readJSONFile(t, configPath)
	mnemo := config["mcp"].(map[string]interface{})["mnemo"].(map[string]interface{})
	if got, want := stringSlice(mnemo["command"]), []string{"/new/mnemo", "serve"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mnemo.command = %v, want %v", got, want)
	}
	if got, want := mnemo["enabled"], true; got != want {
		t.Fatalf("mnemo.enabled = %v, want %v", got, want)
	}
}

func readJSONFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func stringSlice(v interface{}) []string {
	values, ok := v.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.(string))
	}
	return result
}
