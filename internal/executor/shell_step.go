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
	"github.com/fatih/color"
)

func HandleShell(step config.Step, ec *ExecutionContext) error {
	shell := *step.Shell

	tag := color.New(color.BgMagenta).Sprintf(" shell ")
	message := fmt.Sprintf("Executing shell:")
	ec.Logger.Infof("%s %s", tag, message)

	shell = strings.Trim(shell, " ")
	shell = strings.Trim(shell, "\n")

	renderedCommand, err := ec.Template.Render(shell, ec.Variables)
	if err != nil {
		return err
	}

	ec.Logger.Codef(renderedCommand)

	var command *exec.Cmd

	if step.Become {
		if ec.SudoPass == "" {
			command = exec.Command("sudo", "bash")
			command.Stdin = bytes.NewBuffer([]byte(renderedCommand))
		} else {
			command = exec.Command("sudo", "-S", "--", "bash", "-c", renderedCommand)
			command.Stdin = bytes.NewBuffer([]byte(ec.SudoPass))
		}
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

	var wg sync.WaitGroup
	wg.Add(2)
	go printCommandOutputPipe(stdout, ec.Logger, &wg)
	go printCommandOutputPipe(stderr, ec.Logger, &wg)

	// Wait for all output to be processed
	wg.Wait()

	if err := command.Wait(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

func printCommandOutputPipe(pipe io.Reader, log logger.Logger, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		log.Codef("%v", scanner.Text())
	}
}
