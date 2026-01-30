package executor

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
)

func HandleShell(step config.Step, ec *ExecutionContext) error {
	shell := *step.Shell

	shell = strings.Trim(shell, " ")
	shell = strings.Trim(shell, "\n")

	renderedCommand, err := ec.Template.Render(shell, ec.Variables)
	if err != nil {
		return err
	}

	// Check for dry-run mode
	if ec.DryRun {
		ec.Logger.Infof("  [DRY-RUN] Would execute: %s", renderedCommand)
		if step.Become {
			ec.Logger.Infof("  [DRY-RUN] With sudo privileges")
		}
		return nil
	}

	ec.Logger.Debugf("  Executing: %s", renderedCommand)

	var command *exec.Cmd

	if step.Become {
		if ec.SudoPass == "" {
			return fmt.Errorf("step requires sudo but no password provided. Use --sudo-pass flag or --raw mode for interactive sudo")
		}
		command = exec.Command("sudo", "-S", "--", "bash", "-c", renderedCommand)
		command.Stdin = bytes.NewBuffer([]byte(ec.SudoPass + "\n"))
	} else {
		command = exec.Command("bash", "-c", renderedCommand)
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Capture output
	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go captureOutput(stdout, &stdoutBuf, ec.Logger, true, &wg)
	go captureOutput(stderr, &stderrBuf, ec.Logger, false, &wg)

	// Wait for all output to be processed
	wg.Wait()

	if err := command.Wait(); err != nil {
		// On error, show captured output for debugging
		if stdoutBuf.Len() > 0 {
			ec.Logger.Errorf("Command output:\n%s", stdoutBuf.String())
		}
		if stderrBuf.Len() > 0 {
			ec.Logger.Errorf("Error output:\n%s", stderrBuf.String())
		}
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

func captureOutput(pipe io.Reader, buf *bytes.Buffer, log logger.Logger, isStdout bool, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line + "\n")

		// Only show in debug mode (both stdout and stderr)
		log.Debugf(" %v", line)
	}
}
