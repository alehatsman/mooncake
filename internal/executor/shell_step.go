package executor

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/utils"
	"github.com/fatih/color"
)

func HandleShell(step config.Step, ec *ExecutionContext) error {
	shell := *step.Shell

	tag := color.New(color.BgMagenta).Sprintf(" shell ")
	message := fmt.Sprintf("Executing shell:")
	ec.Logger.Infof("%s %s", tag, message)

	shell = strings.Trim(shell, " ")
	shell = strings.Trim(shell, "\n")

	renderedCommand, err := utils.Render(shell, ec.Variables)
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
		logger.Errorf("Error: %v", err)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		logger.Errorf("Error: %v", err)
	}

	if err := command.Start(); err != nil {
		logger.Errorf("Error: %v", err)
	}

	go printCommandOutputPipe(stdout, ec.Logger)
	go printCommandOutputPipe(stderr, ec.Logger)

	if err := command.Wait(); err != nil {
		logger.Errorf("Error: %v", err)
	}

	return err
}

func printCommandOutputPipe(pipe io.Reader, logger *logger.Logger) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		logger.Codef("%v", scanner.Text())
	}
}
