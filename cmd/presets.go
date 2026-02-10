package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/presets"
	"github.com/alehatsman/mooncake/internal/registry"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// presetsCommand creates the presets command with subcommands
func presetsCommand() *cli.Command {
	return &cli.Command{
		Name:  "presets",
		Usage: "Interactive preset selector and manager",
		Description: `Discover and install presets interactively.

Without arguments, opens an interactive selector (requires fzf).
Use subcommands for non-interactive operations.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "ask-become-pass",
				Aliases: []string{"K"},
				Usage:   "Prompt for sudo password interactively (for interactive mode)",
			},
			&cli.StringFlag{
				Name:    "sudo-pass",
				Aliases: []string{"s"},
				Usage:   "Sudo password (requires --insecure-sudo-pass)",
			},
			&cli.StringFlag{
				Name:  "sudo-pass-file",
				Usage: "Read sudo password from file (must have 0600 permissions)",
			},
			&cli.BoolFlag{
				Name:  "insecure-sudo-pass",
				Usage: "Allow --sudo-pass flag (WARNING: password visible in shell history)",
			},
		},
		Subcommands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "Add a preset from URL, git repository, or local path",
				ArgsUsage: "<source>",
				Action:    addPresetAction,
				Description: `Add a preset from an external source to the local registry.

Sources can be:
  - URL:  https://example.com/presets/foo.yml
  - Git:  https://github.com/user/repo.git (coming in v2)
  - Path: /path/to/preset.yml or /path/to/preset/

The preset is cached in ~/.mooncake/cache/presets/ and installed to ~/.mooncake/presets/.

Examples:
  mooncake presets add https://raw.githubusercontent.com/user/repo/main/presets/foo.yml
  mooncake presets add ./local-presets/custom.yml`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "name",
						Usage: "Override preset name (extracted from file by default)",
					},
				},
			},
			{
				Name:   "list",
				Usage:  "List all available presets",
				Action: listPresetsAction,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "detailed",
						Aliases: []string{"d"},
						Usage:   "Show detailed information",
					},
				},
			},
			{
				Name:      "info",
				Usage:     "Show detailed information about a preset",
				ArgsUsage: "<preset-name>",
				Action:    presetInfoAction,
			},
			{
				Name:      "install",
				Usage:     "Install a preset",
				ArgsUsage: "<preset-name>",
				Action:    installPresetAction,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "ask-become-pass",
						Aliases: []string{"K"},
						Usage:   "Prompt for sudo password interactively (recommended)",
					},
					&cli.StringFlag{
						Name:    "sudo-pass",
						Aliases: []string{"s"},
						Usage:   "Sudo password (requires --insecure-sudo-pass)",
					},
					&cli.StringFlag{
						Name:  "sudo-pass-file",
						Usage: "Read sudo password from file (must have 0600 permissions)",
					},
					&cli.BoolFlag{
						Name:  "insecure-sudo-pass",
						Usage: "Allow --sudo-pass flag (WARNING: password visible in shell history)",
					},
					&cli.BoolFlag{
						Name:    "non-interactive",
						Aliases: []string{"n"},
						Usage:   "Skip parameter prompts, use defaults only (fails if required param without default)",
					},
					&cli.StringSliceFlag{
						Name:    "param",
						Aliases: []string{"p"},
						Usage:   "Set parameter value (format: key=value, can be used multiple times)",
					},
				},
			},
			{
				Name:      "status",
				Usage:     "Show status of preset(s)",
				ArgsUsage: "[preset-name]",
				Action:    presetStatusAction,
				Description: `Show detailed status of preset(s) including location and version.

If no preset name is provided, shows status of all presets.`,
			},
			{
				Name:      "uninstall",
				Usage:     "Execute preset's uninstall logic",
				ArgsUsage: "[preset-name]",
				Action:    uninstallPresetAction,
				Description: `Uninstalls a preset by executing it with state: absent.

Without arguments, opens an interactive selector (requires fzf).
With a preset name, uninstalls that specific preset.

This runs the preset's uninstall steps (e.g., stops services, removes packages).
It does not delete the preset files from disk.`,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "ask-become-pass",
						Aliases: []string{"K"},
						Usage:   "Prompt for sudo password interactively (recommended)",
					},
					&cli.StringFlag{
						Name:    "sudo-pass",
						Aliases: []string{"s"},
						Usage:   "Sudo password (requires --insecure-sudo-pass)",
					},
					&cli.StringFlag{
						Name:  "sudo-pass-file",
						Usage: "Read sudo password from file (must have 0600 permissions)",
					},
					&cli.BoolFlag{
						Name:  "insecure-sudo-pass",
						Usage: "Allow --sudo-pass flag (WARNING: password visible in shell history)",
					},
				},
			},
		},
		Action: interactiveSelectorAction,
	}
}

// interactiveSelectorAction runs the interactive preset selector
func interactiveSelectorAction(c *cli.Context) error {
	// Discover all presets
	allPresets, err := presets.DiscoverAllPresets()
	if err != nil {
		return fmt.Errorf("failed to discover presets: %w", err)
	}

	if len(allPresets) == 0 {
		fmt.Println("No presets found in search paths.")
		fmt.Println("\nSearch paths:")
		for _, path := range presets.PresetSearchPaths() {
			fmt.Printf("  - %s\n", path)
		}
		return nil
	}

	// Check for fzf
	if !hasFzf() {
		fmt.Println("fzf is not installed.")
		fmt.Print("\nWould you like to install fzf? (y/n): ")

		var response string
		if _, scanErr := fmt.Scanln(&response); scanErr != nil {
			return fmt.Errorf("failed to read input: %w", scanErr)
		}

		if strings.ToLower(strings.TrimSpace(response)) == "y" {
			fmt.Println("\nInstalling fzf...")
			// Check if fzf preset exists
			if _, loadErr := presets.LoadPreset("fzf"); loadErr == nil {
				return executePresetInstall(c, "fzf")
			}
			// Fallback to manual instructions
			fmt.Println("\nfzf preset not found. Install manually:")
			fmt.Println("  macOS:   brew install fzf")
			fmt.Println("  Linux:   apt install fzf")
			return nil
		}

		fmt.Println("\nAvailable presets:")
		for _, p := range allPresets {
			fmt.Printf("  %s - %s\n", p.Name, p.Description)
		}
		fmt.Println("\nUse: mooncake presets install <name>")
		return nil
	}

	// Use fzf to select preset
	selectedName, err := selectWithFzf(allPresets)
	if err != nil {
		return err
	}

	if selectedName == "" {
		return nil // User cancelled
	}

	// Install the selected preset
	return executePresetInstall(c, selectedName)
}

// hasFzf checks if fzf is available in PATH
func hasFzf() bool {
	_, err := exec.LookPath("fzf")
	return err == nil
}

// selectWithFzf uses fzf to select a preset from the list
func selectWithFzf(allPresets []presets.PresetInfo) (string, error) {
	// Format preset list for fzf (simple: name - description)
	lines := make([]string, 0, len(allPresets))
	for _, p := range allPresets {
		lines = append(lines, fmt.Sprintf("%s - %s", p.Name, p.Description))
	}

	// Run fzf
	cmd := exec.Command("fzf", "--prompt=Select preset: ")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		// User cancelled (Ctrl-C or ESC)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return "", nil
		}
		return "", fmt.Errorf("fzf failed: %w", err)
	}

	// Extract preset name (everything before " - ")
	selectedLine := strings.TrimSpace(string(output))
	if idx := strings.Index(selectedLine, " - "); idx > 0 {
		return selectedLine[:idx], nil
	}

	return selectedLine, nil
}

// listPresetsAction lists all available presets
func listPresetsAction(c *cli.Context) error {
	allPresets, err := presets.DiscoverAllPresets()
	if err != nil {
		return fmt.Errorf("failed to discover presets: %w", err)
	}

	if len(allPresets) == 0 {
		fmt.Println("No presets found.")
		return nil
	}

	detailed := c.Bool("detailed")

	if detailed {
		fmt.Printf("Found %d preset(s):\n\n", len(allPresets))
		for _, p := range allPresets {
			fmt.Printf("Name:        %s\n", p.Name)
			fmt.Printf("Description: %s\n", p.Description)
			fmt.Printf("Version:     %s\n", p.Version)
			fmt.Printf("Source:      %s\n", p.Source)
			fmt.Printf("Path:        %s\n", p.Path)
			fmt.Println()
		}
	} else {
		for _, p := range allPresets {
			fmt.Printf("%-20s  %s (v%s)\n", p.Name, p.Description, p.Version)
		}
	}

	return nil
}

// presetInfoAction shows detailed information about a specific preset
func presetInfoAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("preset name required\n\nUsage: mooncake presets info <preset-name>")
	}

	name := c.Args().First()

	// Load the preset
	preset, err := presets.LoadPreset(name)
	if err != nil {
		return fmt.Errorf("failed to load preset '%s': %w", name, err)
	}

	// Display preset information
	fmt.Printf("%s (v%s)\n\n", preset.Name, preset.Version)
	fmt.Printf("%s\n\n", preset.Description)

	if len(preset.Parameters) > 0 {
		fmt.Println("Parameters:")
		for name, param := range preset.Parameters {
			required := ""
			if param.Required {
				required = " (required)"
			}
			defaultVal := ""
			if param.Default != nil {
				defaultVal = fmt.Sprintf(" [default: %v]", param.Default)
			}
			enumVals := ""
			if len(param.Enum) > 0 {
				vals := make([]string, len(param.Enum))
				for i, v := range param.Enum {
					vals[i] = fmt.Sprintf("%v", v)
				}
				enumVals = fmt.Sprintf(" [%s]", strings.Join(vals, "|"))
			}

			fmt.Printf("  • %s: %s%s%s%s\n", name, param.Type, required, defaultVal, enumVals)
			if param.Description != "" {
				fmt.Printf("    %s\n", param.Description)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Steps: %d\n", len(preset.Steps))

	fmt.Println("\nUsage:")
	fmt.Println("  In your mooncake.yml:")
	fmt.Println()
	fmt.Printf("    - preset:\n")
	fmt.Printf("        name: %s\n", preset.Name)
	if len(preset.Parameters) > 0 {
		fmt.Printf("        with:\n")
		for name, param := range preset.Parameters {
			if param.Required {
				fmt.Printf("          %s: # %s\n", name, param.Type)
			}
		}
	}

	return nil
}

// installPresetAction installs a preset
func installPresetAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("preset name required\n\nUsage: mooncake preset install <preset-name>")
	}

	name := c.Args().First()
	return executePresetInstall(c, name)
}

// collectParameters prompts the user for preset parameters interactively.
// Returns a map of parameter values ready for validation.
func collectParameters(preset *config.PresetDefinition) (map[string]interface{}, error) {
	if len(preset.Parameters) == 0 {
		return map[string]interface{}{}, nil
	}

	fmt.Printf("\nPreset: %s (v%s)\n", preset.Name, preset.Version)
	if preset.Description != "" {
		fmt.Printf("%s\n", preset.Description)
	}
	fmt.Println("\nParameters:")

	params := make(map[string]interface{})
	reader := bufio.NewReader(os.Stdin)

	// Sort parameter names for consistent ordering
	paramNames := make([]string, 0, len(preset.Parameters))
	for name := range preset.Parameters {
		paramNames = append(paramNames, name)
	}
	sort.Strings(paramNames)

	for _, paramName := range paramNames {
		paramDef := preset.Parameters[paramName]

		// Skip if not required and has default
		if !paramDef.Required && paramDef.Default != nil {
			params[paramName] = paramDef.Default
			continue
		}

		// Show parameter prompt
		fmt.Printf("\n? %s", paramName)
		if paramDef.Required {
			fmt.Print(" (required)")
		} else {
			fmt.Print(" (optional)")
		}
		fmt.Printf(" [%s]", paramDef.Type)

		if paramDef.Description != "" {
			fmt.Printf("\n  %s", paramDef.Description)
		}

		if len(paramDef.Enum) > 0 {
			enumStrs := make([]string, len(paramDef.Enum))
			for i, v := range paramDef.Enum {
				enumStrs[i] = fmt.Sprintf("%v", v)
			}
			fmt.Printf("\n  Options: [%s]", strings.Join(enumStrs, ", "))
		}

		if paramDef.Default != nil {
			fmt.Printf("\n  Default: %v", paramDef.Default)
		}

		fmt.Print("\n  > ")

		// Read input
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)

		// Handle empty input
		if input == "" {
			if paramDef.Required {
				fmt.Println("  Error: Required parameter cannot be empty")
				return nil, fmt.Errorf("required parameter '%s' not provided", paramName)
			}
			if paramDef.Default != nil {
				params[paramName] = paramDef.Default
			}
			continue
		}

		// Parse based on type
		var value interface{}
		switch paramDef.Type {
		case "string":
			value = input
		case "bool":
			lower := strings.ToLower(input)
			if lower == "true" || lower == "t" || lower == "yes" || lower == "y" || lower == "1" {
				value = true
			} else if lower == "false" || lower == "f" || lower == "no" || lower == "n" || lower == "0" {
				value = false
			} else {
				fmt.Printf("  Error: Invalid boolean value '%s' (use: true/false, yes/no, y/n)\n", input)
				return nil, fmt.Errorf("invalid boolean value for parameter '%s'", paramName)
			}
		case "array":
			if input == "[]" || input == "" {
				value = []interface{}{}
			} else {
				parts := strings.Split(input, ",")
				arr := make([]interface{}, len(parts))
				for i, part := range parts {
					arr[i] = strings.TrimSpace(part)
				}
				value = arr
			}
		case "object":
			fmt.Println("  Warning: Object parameters not supported in interactive mode, skipping")
			continue
		default:
			return nil, fmt.Errorf("unknown parameter type: %s", paramDef.Type)
		}

		// Validate enum constraint
		if len(paramDef.Enum) > 0 {
			valid := false
			for _, allowed := range paramDef.Enum {
				if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", allowed) {
					valid = true
					break
				}
			}
			if !valid {
				fmt.Printf("  Error: Invalid value. Must be one of: %v\n", paramDef.Enum)
				return nil, fmt.Errorf("invalid value for parameter '%s'", paramName)
			}
		}

		params[paramName] = value
	}

	fmt.Println()
	return params, nil
}

// executePresetInstall executes a preset by name
func executePresetInstall(c *cli.Context, name string) error {
	// Validate password input methods (mutual exclusion)
	passwordMethods := 0
	if c.String("sudo-pass") != "" {
		passwordMethods++
	}
	if c.Bool("ask-become-pass") {
		passwordMethods++
	}
	if c.String("sudo-pass-file") != "" {
		passwordMethods++
	}

	if passwordMethods > 1 {
		return fmt.Errorf("only one password method can be specified (--sudo-pass, --ask-become-pass, --sudo-pass-file)")
	}

	// Security warning for --sudo-pass
	if c.String("sudo-pass") != "" && !c.Bool("insecure-sudo-pass") {
		return fmt.Errorf("--sudo-pass requires --insecure-sudo-pass flag (WARNING: password will be visible in shell history and process list)")
	}
	// Load the preset to validate it exists
	preset, err := presets.LoadPreset(name)
	if err != nil {
		return fmt.Errorf("failed to load preset '%s': %w", name, err)
	}

	// Parse CLI parameters if provided
	cliParams := make(map[string]interface{})
	for _, paramStr := range c.StringSlice("param") {
		parts := strings.SplitN(paramStr, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid parameter format: %s (expected key=value)", paramStr)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Try to parse as bool, then as array, then keep as string
		if lower := strings.ToLower(value); lower == "true" || lower == "false" {
			cliParams[key] = lower == "true"
		} else if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			// Simple array parsing: [a,b,c]
			inner := strings.Trim(value, "[]")
			if inner == "" {
				cliParams[key] = []interface{}{}
			} else {
				parts := strings.Split(inner, ",")
				arr := make([]interface{}, len(parts))
				for i, part := range parts {
					arr[i] = strings.TrimSpace(part)
				}
				cliParams[key] = arr
			}
		} else {
			cliParams[key] = value
		}
	}

	// Collect parameters based on mode
	var userParams map[string]interface{}
	if c.Bool("non-interactive") {
		// Non-interactive: use CLI params + defaults only
		userParams = cliParams
	} else {
		// Interactive: collect from user, but CLI params take precedence
		if len(preset.Parameters) > 0 {
			collected, err := collectParameters(preset)
			if err != nil {
				return fmt.Errorf("failed to collect parameters: %w", err)
			}
			userParams = collected
			// Override with CLI params
			for key, value := range cliParams {
				userParams[key] = value
			}
		} else {
			userParams = cliParams
		}
	}

	// Validate parameters
	validatedParams, err := presets.ValidateParameters(preset, userParams)
	if err != nil {
		return fmt.Errorf("invalid parameters: %w", err)
	}

	// Show what we're installing
	fmt.Printf("\nInstalling %s", preset.Name)
	if len(validatedParams) > 0 {
		fmt.Printf(" with parameters:\n")
		for key, value := range validatedParams {
			fmt.Printf("  - %s: %v\n", key, value)
		}
	} else {
		fmt.Println("...")
	}

	// Create temporary config file with preset invocation
	presetInvocation := map[string]interface{}{
		"name": name,
	}
	if len(validatedParams) > 0 {
		presetInvocation["with"] = validatedParams
	}

	tmpConfig := struct {
		Steps []map[string]interface{} `yaml:"steps"`
	}{
		Steps: []map[string]interface{}{
			{
				"preset": presetInvocation,
			},
		},
	}

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "mooncake-preset-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temp config: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	encoder := yaml.NewEncoder(tmpFile)
	if err := encoder.Encode(tmpConfig); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp config: %w", err)
	}

	// Setup event publisher and logger
	publisher := events.NewPublisher()
	defer publisher.Close()

	level := logger.InfoLevel
	subscriber := logger.NewConsoleSubscriber(level, "text")
	publisher.Subscribe(subscriber)

	internalLog := logger.NewLogger(level)

	// Execute preset
	if err := executor.Start(executor.StartConfig{
		ConfigFilePath:   tmpFile.Name(),
		SudoPass:         c.String("sudo-pass"),
		SudoPassFile:     c.String("sudo-pass-file"),
		AskBecomePass:    c.Bool("ask-become-pass"),
		InsecureSudoPass: c.Bool("insecure-sudo-pass"),
	}, internalLog, publisher); err != nil {
		return fmt.Errorf("preset installation failed: %w", err)
	}

	fmt.Printf("\n%s installed\n", preset.Name)

	return nil
}

// presetStatusAction shows status of preset(s)
func presetStatusAction(c *cli.Context) error {
	if c.NArg() == 0 {
		// Show status of all presets
		allPresets, err := presets.DiscoverAllPresets()
		if err != nil {
			return fmt.Errorf("failed to discover presets: %w", err)
		}

		if len(allPresets) == 0 {
			fmt.Println("No presets found.")
			return nil
		}

		fmt.Printf("Found %d preset(s):\n\n", len(allPresets))
		for _, p := range allPresets {
			sourceLabel := getSourceLabel(p.Source)
			fmt.Printf("%-20s  v%-10s  %s  %s\n", p.Name, p.Version, sourceLabel, p.Description)
		}
		return nil
	}

	// Show status of specific preset
	name := c.Args().First()

	// Discover all presets to find all instances
	allPresets, err := presets.DiscoverAllPresets()
	if err != nil {
		return fmt.Errorf("failed to discover presets: %w", err)
	}

	// Find all instances of this preset
	var instances []presets.PresetInfo
	for _, p := range allPresets {
		if p.Name == name {
			instances = append(instances, p)
		}
	}

	if len(instances) == 0 {
		return fmt.Errorf("preset '%s' not found", name)
	}

	// Show detailed status
	fmt.Printf("Preset: %s\n\n", name)

	if len(instances) == 1 {
		p := instances[0]
		fmt.Printf("Version:     %s\n", p.Version)
		fmt.Printf("Description: %s\n", p.Description)
		fmt.Printf("Location:    %s (%s)\n", p.Path, getSourceLabel(p.Source))
	} else {
		fmt.Printf("Multiple versions found:\n\n")
		for i, p := range instances {
			priority := ""
			if i == 0 {
				priority = " (active)"
			}
			fmt.Printf("%d. %s%s\n", i+1, getSourceLabel(p.Source), priority)
			fmt.Printf("   Version: %s\n", p.Version)
			fmt.Printf("   Path:    %s\n", p.Path)
			fmt.Println()
		}
		fmt.Println("Note: The first preset found is used when multiple versions exist.")
	}

	return nil
}

// uninstallPresetAction executes the preset's uninstall logic
func uninstallPresetAction(c *cli.Context) error {
	var name string

	// If no preset name provided, use interactive selector
	if c.NArg() == 0 {
		// Discover all presets
		allPresets, err := presets.DiscoverAllPresets()
		if err != nil {
			return fmt.Errorf("failed to discover presets: %w", err)
		}

		if len(allPresets) == 0 {
			fmt.Println("No presets found.")
			return nil
		}

		// Check for fzf
		if !hasFzf() {
			fmt.Println("fzf is not installed or no preset name provided.")
			fmt.Println("\nAvailable presets:")
			for _, p := range allPresets {
				fmt.Printf("  %s - %s\n", p.Name, p.Description)
			}
			fmt.Println("\nUsage: mooncake presets uninstall <preset-name>")
			fmt.Println("Or install fzf for interactive selection.")
			return nil
		}

		// Use fzf to select preset
		selectedName, err := selectWithFzf(allPresets)
		if err != nil {
			return err
		}

		if selectedName == "" {
			return nil // User cancelled
		}

		name = selectedName
	} else {
		name = c.Args().First()
	}

	// Validate password input methods (mutual exclusion)
	passwordMethods := 0
	if c.String("sudo-pass") != "" {
		passwordMethods++
	}
	if c.Bool("ask-become-pass") {
		passwordMethods++
	}
	if c.String("sudo-pass-file") != "" {
		passwordMethods++
	}

	if passwordMethods > 1 {
		return fmt.Errorf("only one password method can be specified (--sudo-pass, --ask-become-pass, --sudo-pass-file)")
	}

	// Security warning for --sudo-pass
	if c.String("sudo-pass") != "" && !c.Bool("insecure-sudo-pass") {
		return fmt.Errorf("--sudo-pass requires --insecure-sudo-pass flag (WARNING: password will be visible in shell history and process list)")
	}

	// Load the preset to validate it exists
	preset, err := presets.LoadPreset(name)
	if err != nil {
		return fmt.Errorf("failed to load preset '%s': %w", name, err)
	}

	fmt.Printf("Uninstalling %s...\n", preset.Name)

	// Create temporary config file with preset invocation (state: absent)
	tmpConfig := struct {
		Steps []map[string]interface{} `yaml:"steps"`
	}{
		Steps: []map[string]interface{}{
			{
				"preset": map[string]interface{}{
					"name": name,
					"with": map[string]interface{}{
						"state": "absent",
					},
				},
			},
		},
	}

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "mooncake-preset-uninstall-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temp config: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	encoder := yaml.NewEncoder(tmpFile)
	if err := encoder.Encode(tmpConfig); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp config: %w", err)
	}

	// Setup event publisher and logger
	publisher := events.NewPublisher()
	defer publisher.Close()

	level := logger.InfoLevel
	subscriber := logger.NewConsoleSubscriber(level, "text")
	publisher.Subscribe(subscriber)

	internalLog := logger.NewLogger(level)

	// Execute preset with state: absent
	if err := executor.Start(executor.StartConfig{
		ConfigFilePath:   tmpFile.Name(),
		SudoPass:         c.String("sudo-pass"),
		SudoPassFile:     c.String("sudo-pass-file"),
		AskBecomePass:    c.Bool("ask-become-pass"),
		InsecureSudoPass: c.Bool("insecure-sudo-pass"),
	}, internalLog, publisher); err != nil {
		return fmt.Errorf("preset uninstall failed: %w", err)
	}

	fmt.Printf("\n%s uninstalled\n", preset.Name)

	return nil
}

// addPresetAction adds a preset from an external source to the registry
func addPresetAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("source required\n\nUsage: mooncake presets add <source>")
	}

	source := c.Args().First()
	overrideName := c.String("name")

	// Get cache directory
	cacheDir, err := registry.DefaultCacheDir()
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	// Get user presets directory
	userDir, err := registry.UserPresetsDir()
	if err != nil {
		return fmt.Errorf("failed to get user presets directory: %w", err)
	}

	// Load manifest
	manifest, err := registry.LoadManifest(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Detect source type
	sourceType := registry.DetectSourceType(source)
	fmt.Printf("Detected source type: %s\n", sourceType)

	if sourceType == registry.SourceTypeGit {
		return fmt.Errorf("git sources not yet supported (coming in registry v2)")
	}

	// Generate temporary hash for fetching (will be replaced with actual hash)
	tmpHash := fmt.Sprintf("tmp-%d", os.Getpid())

	// Fetch source
	fmt.Printf("Fetching preset from %s...\n", source)
	cachedDir, err := registry.FetchSource(source, sourceType, cacheDir, tmpHash)
	if err != nil {
		return fmt.Errorf("failed to fetch source: %w", err)
	}

	// Find the preset file to determine name and calculate hash
	var presetFile string
	var presetName string

	// Check for flat format: *.yml
	entries, err := os.ReadDir(cachedDir)
	if err != nil {
		return fmt.Errorf("failed to read cached directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml")) {
			presetFile = filepath.Join(cachedDir, entry.Name())
			presetName = strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".yml"), ".yaml")
			break
		}
	}

	// If not found, check for directory format: */preset.yml
	if presetFile == "" {
		for _, entry := range entries {
			if entry.IsDir() {
				candidatePath := filepath.Join(cachedDir, entry.Name(), "preset.yml")
				if _, statErr := os.Stat(candidatePath); statErr == nil {
					presetFile = candidatePath
					presetName = entry.Name()
					break
				}
			}
		}
	}

	if presetFile == "" {
		_ = os.RemoveAll(cachedDir) // Clean up
		return fmt.Errorf("no preset file found in source (expected *.yml or */preset.yml)")
	}

	// Override name if provided
	if overrideName != "" {
		presetName = overrideName
	}

	fmt.Printf("Found preset: %s\n", presetName)

	// Calculate SHA256 of preset file
	sha256hash, err := registry.CalculateSHA256(presetFile)
	if err != nil {
		_ = os.RemoveAll(cachedDir) // Clean up
		return fmt.Errorf("failed to calculate SHA256: %w", err)
	}

	// Rename cache directory to use actual hash
	finalCacheDir := filepath.Join(cacheDir, sha256hash)
	if err := os.Rename(cachedDir, finalCacheDir); err != nil {
		_ = os.RemoveAll(cachedDir) // Clean up
		return fmt.Errorf("failed to move to final cache location: %w", err)
	}

	// Add to manifest
	entry := registry.ManifestEntry{
		Name:        presetName,
		Source:      source,
		Type:        string(sourceType),
		SHA256:      sha256hash,
		InstalledAt: time.Now(),
	}
	manifest.Add(entry)

	if err := manifest.Save(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// Install to user directory
	fmt.Printf("Installing to %s...\n", userDir)
	if err := registry.InstallToUserDir(presetName, cacheDir, userDir); err != nil {
		return fmt.Errorf("failed to install preset: %w", err)
	}

	fmt.Printf("\n✓ Preset '%s' added successfully\n", presetName)
	fmt.Printf("  Source:  %s\n", source)
	fmt.Printf("  SHA256:  %s\n", sha256hash[:16]+"...")
	fmt.Printf("  Cached:  %s\n", finalCacheDir)
	fmt.Printf("\nUse in your mooncake.yml:\n")
	fmt.Printf("  - preset: %s\n", presetName)

	return nil
}

// getSourceLabel returns a formatted label for preset source
func getSourceLabel(source string) string {
	switch source {
	case "local":
		return "[local]  "
	case "user":
		return "[user]   "
	case "system":
		return "[system] "
	default:
		return "[unknown]"
	}
}

// truncate truncates a string to maxLen, adding "..." if truncated
