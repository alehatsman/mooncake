package executor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
)

const (
	platformDarwin = "darwin"
	platformLinux  = "linux"
)

// HandleOllama executes an Ollama installation/management action.
func HandleOllama(step config.Step, ec *ExecutionContext) error {
	action := step.Ollama
	if action == nil {
		return &StepValidationError{
			Field:   "ollama",
			Message: "ollama action is nil",
		}
	}

	// Validate state (only present/absent for installation)
	validStates := []string{"present", "absent"}
	if !contains(validStates, action.State) {
		return &StepValidationError{
			Field:   "state",
			Message: fmt.Sprintf("invalid state: %s (must be 'present' or 'absent')", action.State),
		}
	}

	// Validate installation method if specified
	if action.Method != "" {
		validMethods := []string{"auto", "script", "package"}
		if !contains(validMethods, action.Method) {
			return &StepValidationError{
				Field:   "method",
				Message: fmt.Sprintf("invalid method: %s", action.Method),
			}
		}
	}

	// Create result
	result := NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		ec.CurrentResult = result
	}()

	// Dry-run
	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		dryRun.LogOllamaOperation(action, step.Become)
		dryRun.LogRegister(step)
	}) {
		// Register result even in dry-run mode
		if step.Register != "" {
			result.RegisterTo(ec.Variables, step.Register)
		}
		return nil
	}

	// Track operations for event emission
	operations := []string{}
	modelsPulled := []string{}
	modelsSkipped := []string{}

	// Dispatch to state handler
	var err error
	switch action.State {
	case "present":
		// Install Ollama
		installed, installMethod, installErr := handleOllamaInstall(action, step, ec, result)
		if installErr != nil {
			return installErr
		}
		if installed {
			operations = append(operations, "installed")
			ec.Logger.Infof("Ollama installed via %s", installMethod)
		} else {
			ec.Logger.Infof("Ollama already installed")
		}

		// Configure and start service if requested
		if action.Service != nil && *action.Service {
			serviceConfigured, serviceErr := configureOllamaService(action, step, ec, result)
			if serviceErr != nil {
				return serviceErr
			}
			if serviceConfigured {
				operations = append(operations, "service_configured")
				result.Changed = true
			}
		}

		// Pull models if specified
		if len(action.Pull) > 0 {
			pulled, skipped, pullErr := handleModelPull(action, step, ec, result)
			if pullErr != nil {
				return pullErr
			}
			modelsPulled = pulled
			modelsSkipped = skipped
			if len(pulled) > 0 {
				operations = append(operations, "models_pulled")
				result.Changed = true
			}
		}

	case "absent":
		removed, removeErr := handleOllamaUninstall(action, step, ec, result)
		if removeErr != nil {
			return removeErr
		}
		if removed {
			operations = append(operations, "uninstalled")
			result.Changed = true
		} else {
			ec.Logger.Infof("Ollama already absent")
		}
	}

	// Register result
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
	}

	// Emit event
	ec.EmitEvent(events.EventOllamaManaged, events.OllamaData{
		State:          action.State,
		ServiceEnabled: action.Service != nil && *action.Service,
		Method:         action.Method,
		ModelsDir:      action.ModelsDir,
		ModelsPulled:   modelsPulled,
		ModelsSkipped:  modelsSkipped,
		Operations:     operations,
	})

	return err
}

// handleOllamaInstall installs Ollama if not already present.
// Returns: (installed, method, error)
func handleOllamaInstall(action *config.OllamaAction, step config.Step, ec *ExecutionContext, result *Result) (bool, string, error) {
	// Check if already installed
	ollamaPath, err := exec.LookPath("ollama")
	if err == nil {
		// Already installed
		ec.Logger.Debugf("Ollama already installed at %s", ollamaPath)
		return false, "", nil
	}

	// Determine installation method
	method := action.Method
	if method == "" {
		method = "auto"
	}

	var installErr error
	var installMethod string

	switch method {
	case "auto":
		// Try package manager first, fallback to script
		installMethod, installErr = installViaPackageManager(step, ec)
		if installErr != nil {
			ec.Logger.Debugf("Package manager installation failed: %v, trying script", installErr)
			installMethod, installErr = installViaScript(step, ec)
		}

	case "package":
		// Package manager only
		installMethod, installErr = installViaPackageManager(step, ec)

	case "script":
		// Official script only
		installMethod, installErr = installViaScript(step, ec)
	}

	if installErr != nil {
		return false, "", &SetupError{
			Component: "ollama_install",
			Issue:     fmt.Sprintf("installation via %s failed", method),
			Cause:     installErr,
		}
	}

	result.Changed = true
	return true, installMethod, nil
}

// installViaPackageManager attempts to install Ollama via system package manager.
func installViaPackageManager(step config.Step, ec *ExecutionContext) (string, error) {
	switch runtime.GOOS {
	case platformDarwin:
		// Check for Homebrew
		if _, err := exec.LookPath("brew"); err == nil {
			ec.Logger.Infof("Installing Ollama via Homebrew")

			// Build command
			cmdStr := "brew install ollama"

			if step.Become {
				return "", fmt.Errorf("brew installation should not use sudo")
			}

			// #nosec G204 -- command string is controlled by this function
			cmd := exec.Command("bash", "-c", cmdStr)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("brew install failed: %v: %s", err, string(output))
			}

			ec.Logger.Debugf("Brew output: %s", string(output))
			return "brew", nil
		}
		return "", fmt.Errorf("homebrew not found")

	case platformLinux:
		// Check for package managers in order of preference
		packageManagers := []struct {
			name    string
			check   string
			install string
		}{
			{"apt", "apt-get", "apt-get update && apt-get install -y ollama"},
			{"dnf", "dnf", "dnf install -y ollama"},
			{"yum", "yum", "yum install -y ollama"},
			{"pacman", "pacman", "pacman -S --noconfirm ollama"},
			{"zypper", "zypper", "zypper install -y ollama"},
			{"apk", "apk", "apk add ollama"},
		}

		for _, pm := range packageManagers {
			if _, err := exec.LookPath(pm.check); err == nil {
				ec.Logger.Infof("Installing Ollama via %s", pm.name)

				var cmdErr error
				if step.Become {
					cmdErr = executeSudoCommand(pm.install, step, ec)
				} else {
					// #nosec G204 -- command string is controlled by this function
					cmd := exec.Command("bash", "-c", pm.install)
					output, err := cmd.CombinedOutput()
					cmdErr = err
					if err != nil {
						cmdErr = fmt.Errorf("%v: %s", err, string(output))
					}
				}

				if cmdErr != nil {
					return "", fmt.Errorf("%s install failed: %v", pm.name, cmdErr)
				}

				return pm.name, nil
			}
		}
		return "", fmt.Errorf("no supported package manager found")

	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// installViaScript installs Ollama via the official installation script.
func installViaScript(step config.Step, ec *ExecutionContext) (string, error) {
	ec.Logger.Infof("Installing Ollama via official script")

	switch runtime.GOOS {
	case platformDarwin, platformLinux:
		cmdStr := "curl -fsSL https://ollama.com/install.sh | bash"

		var cmdErr error
		if step.Become {
			cmdErr = executeSudoCommand(cmdStr, step, ec)
		} else {
			// #nosec G204 -- command string is controlled by this function
			cmd := exec.Command("bash", "-c", cmdStr)
			output, err := cmd.CombinedOutput()
			cmdErr = err
			if err != nil {
				ec.Logger.Debugf("Script output: %s", string(output))
				cmdErr = fmt.Errorf("%v: %s", err, string(output))
			}
		}

		if cmdErr != nil {
			return "", fmt.Errorf("installation script failed: %v", cmdErr)
		}

		return "script", nil

	default:
		return "", fmt.Errorf("script installation not supported on %s", runtime.GOOS)
	}
}

// configureOllamaService configures and starts the Ollama service.
// Returns: (configured, error)
func configureOllamaService(action *config.OllamaAction, step config.Step, ec *ExecutionContext, _ *Result) (bool, error) {
	switch runtime.GOOS {
	case platformLinux:
		return configureSystemdService(action, step, ec)
	case platformDarwin:
		return configureLaunchdService(action, step, ec)
	default:
		return false, fmt.Errorf("service management not supported on %s", runtime.GOOS)
	}
}

// configureSystemdService configures Ollama as a systemd service.
func configureSystemdService(action *config.OllamaAction, step config.Step, ec *ExecutionContext) (bool, error) {
	changed := false

	// Check if systemd service exists
	checkCmd := "systemctl list-unit-files ollama.service"
	// #nosec G204 -- command string is controlled
	cmd := exec.Command("bash", "-c", checkCmd)
	output, err := cmd.CombinedOutput()
	serviceExists := err == nil && strings.Contains(string(output), "ollama.service")

	if !serviceExists {
		ec.Logger.Infof("Ollama systemd service not found - may need to be created by installation method")
	}

	// Create drop-in configuration if env vars or custom settings provided
	if len(action.Env) > 0 || action.Host != "" || action.ModelsDir != "" {
		dropinDir := "/etc/systemd/system/ollama.service.d"
		dropinPath := dropinDir + "/10-mooncake.conf"

		// Build drop-in content
		var content strings.Builder
		content.WriteString("[Service]\n")

		if action.Host != "" {
			content.WriteString(fmt.Sprintf("Environment=\"OLLAMA_HOST=%s\"\n", action.Host))
		}
		if action.ModelsDir != "" {
			content.WriteString(fmt.Sprintf("Environment=\"OLLAMA_MODELS=%s\"\n", action.ModelsDir))
		}
		for key, value := range action.Env {
			content.WriteString(fmt.Sprintf("Environment=\"%s=%s\"\n", key, value))
		}

		// Check if drop-in already exists with same content
		existingContent, readErr := os.ReadFile(dropinPath)
		needsUpdate := readErr != nil || string(existingContent) != content.String()

		if needsUpdate {
			ec.Logger.Infof("Creating systemd drop-in configuration at %s", dropinPath)

			// Create directory
			mkdirCmd := fmt.Sprintf("mkdir -p %s", dropinDir)
			if err := executeSudoCommand(mkdirCmd, step, ec); err != nil {
				return false, &FileOperationError{
					Operation: "create",
					Path:      dropinDir,
					Cause:     err,
				}
			}

			// Write drop-in file
			if err := createFileWithBecome(dropinPath, []byte(content.String()), 0644, step, ec); err != nil {
				return false, &FileOperationError{
					Operation: "write",
					Path:      dropinPath,
					Cause:     err,
				}
			}

			// Daemon reload
			if err := executeSudoCommand("systemctl daemon-reload", step, ec); err != nil {
				return false, &CommandError{
					ExitCode: 1,
					Cause:    err,
				}
			}

			changed = true
		}
	}

	// Enable service
	if serviceExists {
		enableCmd := "systemctl enable ollama"
		if err := executeSudoCommand(enableCmd, step, ec); err != nil {
			ec.Logger.Debugf("Enable failed (may already be enabled): %v", err)
		} else {
			changed = true
		}

		// Start service
		// Check current state
		stateCmd := "systemctl is-active ollama"
		// #nosec G204 -- command controlled
		stateCheckCmd := exec.Command("bash", "-c", stateCmd)
		stateOutput, _ := stateCheckCmd.CombinedOutput()
		isActive := strings.TrimSpace(string(stateOutput)) == "active"

		if !isActive {
			startCmd := "systemctl start ollama"
			if err := executeSudoCommand(startCmd, step, ec); err != nil {
				return false, &CommandError{
					ExitCode: 1,
					Cause:    fmt.Errorf("failed to start ollama service: %v", err),
				}
			}
			ec.Logger.Infof("Ollama service started")
			changed = true
		} else {
			ec.Logger.Debugf("Ollama service already active")
		}
	}

	return changed, nil
}

// configureLaunchdService configures Ollama as a launchd service on macOS.
func configureLaunchdService(_ *config.OllamaAction, _ config.Step, ec *ExecutionContext) (bool, error) {
	// Check if installed via Homebrew (which provides service management)
	if _, err := exec.LookPath("brew"); err == nil {
		// Try to use brew services
		ec.Logger.Infof("Starting Ollama via Homebrew services")

		cmdStr := "brew services start ollama"
		// #nosec G204 -- command controlled
		cmd := exec.Command("bash", "-c", cmdStr)
		output, err := cmd.CombinedOutput()

		if err != nil {
			// Check if already running
			if strings.Contains(string(output), "already started") {
				ec.Logger.Debugf("Ollama service already running")
				return false, nil
			}
			return false, &CommandError{
				ExitCode: 1,
				Cause:    fmt.Errorf("brew services start failed: %v: %s", err, string(output)),
			}
		}

		return true, nil
	}

	// For non-Homebrew installations, we'd need to create a custom plist
	// This is more complex and depends on where Ollama was installed
	ec.Logger.Infof("Service management for non-Homebrew Ollama installations not yet implemented")
	return false, nil
}

// handleModelPull pulls the specified Ollama models.
// Returns: (pulled, skipped, error)
func handleModelPull(action *config.OllamaAction, _ config.Step, ec *ExecutionContext, _ *Result) ([]string, []string, error) {
	pulled := []string{}
	skipped := []string{}

	// Get list of existing models
	existingModels := make(map[string]bool)
	ollamaPath, err := exec.LookPath("ollama")
	if err != nil {
		return nil, nil, &SetupError{
			Component: "ollama",
			Issue:     "ollama command not found after installation",
			Cause:     err,
		}
	}

	// #nosec G204 -- path validated via LookPath
	listCmd := exec.Command(ollamaPath, "list")
	listOutput, listErr := listCmd.CombinedOutput()
	if listErr == nil {
		lines := strings.Split(string(listOutput), "\n")
		for i, line := range lines {
			if i == 0 || strings.TrimSpace(line) == "" {
				continue // Skip header
			}
			fields := strings.Fields(line)
			if len(fields) > 0 {
				existingModels[fields[0]] = true
			}
		}
	}

	// Pull each model
	for _, model := range action.Pull {
		// Check if already exists (unless force flag is set)
		if existingModels[model] && !action.Force {
			ec.Logger.Infof("Model %s already exists (skipping)", model)
			skipped = append(skipped, model)
			continue
		}

		ec.Logger.Infof("Pulling model: %s", model)

		// #nosec G204 -- path validated via LookPath, model validated by schema
		pullCmd := exec.Command(ollamaPath, "pull", model)
		pullOutput, pullErr := pullCmd.CombinedOutput()

		if pullErr != nil {
			return pulled, skipped, &CommandError{
				ExitCode: 1,
				Cause:    fmt.Errorf("failed to pull model %s: %v: %s", model, pullErr, string(pullOutput)),
			}
		}

		ec.Logger.Debugf("Pull output: %s", string(pullOutput))
		pulled = append(pulled, model)
	}

	return pulled, skipped, nil
}

// handleOllamaUninstall removes Ollama from the system.
// Returns: (removed, error)
//
//nolint:unparam // error always nil - best-effort removal, consistent with handler signature
func handleOllamaUninstall(action *config.OllamaAction, step config.Step, ec *ExecutionContext, _ *Result) (bool, error) {
	// Check if Ollama is installed
	_, err := exec.LookPath("ollama")
	if err != nil {
		// Already not installed
		ec.Logger.Debugf("Ollama not installed")
		return false, nil
	}

	removed := false

	// Phase 1: Stop and remove service
	switch runtime.GOOS {
	case platformLinux:
		ec.Logger.Infof("Stopping and disabling Ollama systemd service")

		// Stop service
		stopErr := executeSudoCommand("systemctl stop ollama", step, ec)
		if stopErr != nil {
			ec.Logger.Debugf("Stop service failed (may not exist): %v", stopErr)
		}

		// Disable service
		disableErr := executeSudoCommand("systemctl disable ollama", step, ec)
		if disableErr != nil {
			ec.Logger.Debugf("Disable service failed: %v", disableErr)
		}

		// Remove drop-in
		dropinPath := "/etc/systemd/system/ollama.service.d/10-mooncake.conf"
		if _, statErr := os.Stat(dropinPath); statErr == nil {
			removeCmd := fmt.Sprintf("rm -f %s", dropinPath)
			if rmErr := executeSudoCommand(removeCmd, step, ec); rmErr != nil {
				ec.Logger.Debugf("Remove drop-in failed: %v", rmErr)
			}
		}

		// Daemon reload
		_ = executeSudoCommand("systemctl daemon-reload", step, ec)

	case platformDarwin:
		// Check for Homebrew service
		if _, brewErr := exec.LookPath("brew"); brewErr == nil {
			ec.Logger.Infof("Stopping Ollama Homebrew service")
			stopCmd := "brew services stop ollama"
			// #nosec G204 -- command controlled
			cmd := exec.Command("bash", "-c", stopCmd)
			_, _ = cmd.CombinedOutput() // Ignore errors if service wasn't running
		}
	}

	// Phase 2: Remove binary
	ec.Logger.Infof("Removing Ollama binary")

	switch runtime.GOOS {
	case platformDarwin:
		// Check if installed via Homebrew
		if _, err := exec.LookPath("brew"); err == nil {
			uninstallCmd := "brew uninstall ollama"
			// #nosec G204 -- command controlled
			cmd := exec.Command("bash", "-c", uninstallCmd)
			output, uninstallErr := cmd.CombinedOutput()
			if uninstallErr != nil {
				ec.Logger.Debugf("Brew uninstall failed: %v: %s", uninstallErr, string(output))
			} else {
				removed = true
			}
		}

	case platformLinux:
		// Try package manager removal first
		packageManagers := []struct {
			name   string
			check  string
			remove string
		}{
			{"apt", "apt-get", "apt-get remove -y ollama"},
			{"dnf", "dnf", "dnf remove -y ollama"},
			{"yum", "yum", "yum remove -y ollama"},
			{"pacman", "pacman", "pacman -R --noconfirm ollama"},
			{"zypper", "zypper", "zypper remove -y ollama"},
			{"apk", "apk", "apk del ollama"},
		}

		var removeErr error
		for _, pm := range packageManagers {
			if _, checkErr := exec.LookPath(pm.check); checkErr == nil {
				ec.Logger.Infof("Removing Ollama via %s", pm.name)
				removeErr = executeSudoCommand(pm.remove, step, ec)
				if removeErr == nil {
					removed = true
					break
				}
			}
		}

		// If package manager removal failed, try removing binary directly
		if !removed {
			ec.Logger.Debugf("Package manager removal failed, trying direct removal")
			paths := []string{"/usr/local/bin/ollama", "/usr/bin/ollama"}
			for _, path := range paths {
				if _, statErr := os.Stat(path); statErr == nil {
					removeCmd := fmt.Sprintf("rm -f %s", path)
					if rmErr := executeSudoCommand(removeCmd, step, ec); rmErr == nil {
						removed = true
					}
				}
			}
		}
	}

	// Phase 3: Remove models (only if force flag is set)
	if action.Force {
		ec.Logger.Infof("Removing Ollama models directory (force flag set)")

		modelsDir := action.ModelsDir
		if modelsDir == "" {
			// Default models location
			homeDir, _ := os.UserHomeDir()
			modelsDir = homeDir + "/.ollama/models"
		}

		if _, statErr := os.Stat(modelsDir); statErr == nil {
			removeCmd := fmt.Sprintf("rm -rf %s", modelsDir)
			if step.Become {
				if rmErr := executeSudoCommand(removeCmd, step, ec); rmErr != nil {
					ec.Logger.Infof("Failed to remove models directory: %v", rmErr)
				}
			} else {
				if rmErr := os.RemoveAll(modelsDir); rmErr != nil {
					ec.Logger.Infof("Failed to remove models directory: %v", rmErr)
				}
			}
		}
	}

	return removed, nil
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
