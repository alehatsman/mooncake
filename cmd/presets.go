package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/presets"
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
		Subcommands: []*cli.Command{
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
			},
		},
		Action: interactiveSelectorAction,
	}
}

// interactiveSelectorAction runs the interactive preset selector
func interactiveSelectorAction(_ *cli.Context) error {
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
				return executePresetInstall("fzf")
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
	return executePresetInstall(selectedName)
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
	return executePresetInstall(name)
}

// executePresetInstall executes a preset by name
func executePresetInstall(name string) error {
	// Load the preset to validate it exists
	preset, err := presets.LoadPreset(name)
	if err != nil {
		return fmt.Errorf("failed to load preset '%s': %w", name, err)
	}

	fmt.Printf("Installing %s...\n", preset.Name)

	// Create temporary config file with preset invocation
	tmpConfig := struct {
		Steps []map[string]interface{} `yaml:"steps"`
	}{
		Steps: []map[string]interface{}{
			{
				"preset": map[string]interface{}{
					"name": name,
					// TODO: Collect parameters from user
				},
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
		ConfigFilePath: tmpFile.Name(),
	}, internalLog, publisher); err != nil {
		return fmt.Errorf("preset installation failed: %w", err)
	}

	fmt.Printf("\n%s installed\n", preset.Name)

	return nil
}

// truncate truncates a string to maxLen, adding "..." if truncated
