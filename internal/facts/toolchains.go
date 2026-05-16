package facts

import (
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed tools.yml
var defaultToolsYAML []byte

// detectToolchains probes for common development tools.
func detectToolchains() (docker, git, golang string) {
	docker = detectToolchainVersion("docker", "--version", "Docker version ")
	git = detectToolchainVersion("git", "--version", "git version ")
	golang = detectToolchainVersion("go", "version", "go version go")
	return
}

// detectToolchainVersion runs command and extracts version.
func detectToolchainVersion(cmd, flag, prefix string) string {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return "" // Not installed
	}

	// #nosec G204 -- path validated via exec.LookPath
	out, err := probeCombined(path, flag)
	if err != nil {
		return ""
	}

	version := strings.TrimSpace(string(out))
	version = strings.TrimPrefix(version, prefix)

	// Extract first token (version number)
	fields := strings.Fields(version)
	if len(fields) > 0 {
		// Remove trailing commas (e.g., "24.0.7," -> "24.0.7")
		return strings.TrimSuffix(fields[0], ",")
	}

	return ""
}

// detectOllamaVersion attempts to detect Ollama version.
func detectOllamaVersion() string {
	path, err := exec.LookPath("ollama")
	if err != nil {
		return "" // Not installed
	}

	// #nosec G204 -- path validated via exec.LookPath
	out, err := probeCombined(path, "--version")
	if err != nil {
		return ""
	}

	// Output formats:
	//   "ollama version is 0.15.2"   (current)
	//   "Warning: client version is 0.15.2"  (older)
	//   "ollama version 0.1.47"      (oldest)
	output := strings.TrimSpace(string(out))

	// Match "version is X.Y.Z" (covers both "ollama version is" and "client version is")
	if idx := strings.Index(output, "version is "); idx >= 0 {
		rest := output[idx+len("version is "):]
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			return fields[0]
		}
	}

	// Try oldest format: "ollama version X.Y.Z"
	if strings.HasPrefix(output, "ollama version ") {
		version := strings.TrimPrefix(output, "ollama version ")
		fields := strings.Fields(version)
		if len(fields) > 0 {
			return fields[0]
		}
	}

	return ""
}

// detectOllamaModels attempts to list installed Ollama models.
func detectOllamaModels() []OllamaModel {
	path, err := exec.LookPath("ollama")
	if err != nil {
		return nil
	}

	// #nosec G204 -- path validated via exec.LookPath
	out, err := probeCombined(path, "list")
	if err != nil {
		return nil
	}

	var models []OllamaModel
	lines := strings.Split(string(out), "\n")

	for i, line := range lines {
		// Skip header line and empty lines
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// Parse tabular output: NAME SIZE MODIFIED
		// Example: "llama3.1:8b    4.7 GB  2 weeks ago"
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			model := OllamaModel{
				Name: fields[0],
				Size: fields[1],
			}

			// Handle "4.7 GB" format (size is two fields)
			if len(fields) >= 3 && (fields[2] == "GB" || fields[2] == "MB" || fields[2] == "KB") {
				model.Size = fields[1] + " " + fields[2]
			}

			// Modified time is the remaining fields (e.g., "2 weeks ago")
			if len(fields) >= 4 {
				model.ModifiedAt = strings.Join(fields[3:], " ")
			} else if len(fields) == 3 && fields[2] != "GB" && fields[2] != "MB" && fields[2] != "KB" {
				model.ModifiedAt = fields[2]
			}

			models = append(models, model)
		}
	}

	return models
}

// detectOllamaEndpoint determines the Ollama server endpoint.
func detectOllamaEndpoint() string {
	// Check OLLAMA_HOST environment variable
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		// If it doesn't start with http, add it
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			return "http://" + host
		}
		return host
	}

	// Default endpoint
	return "http://localhost:11434"
}

// ToolSpec describes how to probe one CLI tool for its version.
// This is the public form used in YAML and by callers.
type ToolSpec struct {
	Cmd         string   `yaml:"cmd"`
	Args        []string `yaml:"args"`
	Prefix      string   `yaml:"prefix,omitempty"`
	Parser      string   `yaml:"parser,omitempty"`       // "" or "json_field"
	JSONKey     string   `yaml:"json_key,omitempty"`     // key for json_field parser
	StripSuffix bool     `yaml:"strip_suffix,omitempty"` // strip -(build) suffixes
}

// toolsFile is the top-level structure of tools.yml.
type toolsFile struct {
	Tools []ToolSpec `yaml:"tools"`
}

// loadedSpecs caches the merged tool list after first load.
var (
	loadedSpecs     []ToolSpec
	loadedSpecsOnce sync.Once
)

// loadToolSpecs loads the embedded default tools.yml, then merges any entries
// from ~/.config/mooncake/tools.yml. User entries override defaults by cmd.
func loadToolSpecs() []ToolSpec {
	loadedSpecsOnce.Do(func() {
		// Parse embedded defaults
		var def toolsFile
		if err := yaml.Unmarshal(defaultToolsYAML, &def); err == nil {
			loadedSpecs = def.Tools
		}

		// Attempt user override file
		userFile := userToolsFile()
		// #nosec G304 -- userFile is constructed from os.UserHomeDir/os.UserConfigDir, not user input
		if data, err := os.ReadFile(userFile); err == nil {
			var user toolsFile
			if err := yaml.Unmarshal(data, &user); err == nil {
				loadedSpecs = mergeToolSpecs(loadedSpecs, user.Tools)
			}
		}
	})
	return loadedSpecs
}

// userToolsFile returns the path to the user's tools override file.
// Checks XDG_CONFIG_HOME first, then ~/.config/mooncake, then the OS default.
func userToolsFile() string {
	// XDG / Linux convention (also set on macOS by some setups)
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mooncake", "tools.yml")
	}

	// Fallback: ~/.config/mooncake/tools.yml (works everywhere)
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dotConfig := filepath.Join(home, ".config", "mooncake", "tools.yml")
	if _, statErr := os.Stat(dotConfig); statErr == nil {
		return dotConfig
	}

	// OS-native config dir (~/Library/Application Support on macOS)
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "mooncake", "tools.yml")
}

// mergeToolSpecs merges user entries into defaults: same cmd → user wins; new cmds appended.
func mergeToolSpecs(defaults, overrides []ToolSpec) []ToolSpec {
	index := make(map[string]int, len(defaults))
	merged := make([]ToolSpec, len(defaults))
	copy(merged, defaults)
	for i, s := range merged {
		index[s.Cmd] = i
	}
	for _, u := range overrides {
		if i, ok := index[u.Cmd]; ok {
			merged[i] = u
		} else {
			merged = append(merged, u)
		}
	}
	return merged
}

// detectExtendedTools probes for an extended set of dev tools, each with a 2s
// timeout. Returns a map of tool name → version (absent if not found).
func detectExtendedTools() map[string]string {
	tools := make(map[string]string)
	for _, spec := range loadToolSpecs() {
		v := probeToolVersion(spec)
		if v != "" {
			tools[spec.Cmd] = v
		}
	}
	return tools
}

// probeToolVersion runs a single tool probe with a 2s timeout.
func probeToolVersion(spec ToolSpec) string {
	path, err := exec.LookPath(spec.Cmd)
	if err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// #nosec G204 -- path validated via exec.LookPath
	cmd := exec.CommandContext(ctx, path, spec.Args...)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}

	raw := strings.TrimSpace(string(out))

	// json_field parser: extract a quoted value from JSON output
	if spec.Parser == "json_field" && spec.JSONKey != "" {
		// Try with and without space after colon (compact vs pretty JSON)
		v := parseJSONField(raw, spec.JSONKey)
		if v == "" {
			// Try compact variant (remove spaces around colon)
			compactKey := strings.ReplaceAll(spec.JSONKey, ": ", ":")
			v = parseJSONField(raw, compactKey)
		}
		return v
	}

	line := strings.SplitN(raw, "\n", 2)[0]
	line = strings.TrimPrefix(line, spec.Prefix)

	// Take first whitespace-separated token as the version
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	v := strings.TrimSuffix(fields[0], ",")

	// strip_suffix: drop -(build) and (N)-suffix patterns
	if spec.StripSuffix {
		if idx := strings.IndexAny(v, "-("); idx > 0 {
			v = v[:idx]
		}
	}
	return v
}

// parseJSONField extracts a quoted value after key from a JSON blob.
// This avoids importing encoding/json for a simple lookup.
func parseJSONField(s, key string) string {
	idx := strings.Index(s, key)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(key):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	v := rest[:end]
	return strings.TrimPrefix(v, "v")
}
