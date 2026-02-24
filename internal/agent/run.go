package agent

import (
	"fmt"
	"os"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/snapshot"
)

func Run(opts RunOptions) (*IterationLog, error) {
	iterNum, err := NextIterationNumber(opts.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get next iteration number: %w", err)
	}

	planBytes, err := loadPlan(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load plan: %w", err)
	}

	planBytes = stripMarkdownFences(planBytes)

	planHash := ComputePlanHash(planBytes)

	_, err = snapshot.Collect(opts.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to collect snapshot: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "mooncake-plan-*.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, writeErr := tmpFile.Write(planBytes); writeErr != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", writeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", closeErr)
	}

	_, diagnostics, err := config.ReadConfigWithValidation(tmpFile.Name())
	if err != nil {
		return nil, writeFailureLog(opts.RepoRoot, iterNum, opts.Goal, planHash, fmt.Errorf("config validation failed: %w", err))
	}

	if config.HasErrors(diagnostics) {
		return nil, writeFailureLog(opts.RepoRoot, iterNum, opts.Goal, planHash, fmt.Errorf("config has validation errors: %s", config.FormatDiagnostics(diagnostics)))
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	log := logger.NewLogger(logger.ErrorLevel)

	execErr := executor.Start(executor.StartConfig{
		ConfigFilePath: tmpFile.Name(),
		DryRun:         false,
	}, log, publisher)

	if execErr != nil {
		return nil, writeFailureLog(opts.RepoRoot, iterNum, opts.Goal, planHash, execErr)
	}

	changedFiles, err := CollectChangedFiles(opts.RepoRoot)
	if err != nil {
		changedFiles = []string{}
	}

	diffStat, err := CollectDiffStat(opts.RepoRoot)
	if err != nil {
		diffStat = DiffStat{}
	}

	iterLog := &IterationLog{
		Iteration:    iterNum,
		Goal:         opts.Goal,
		PlanHash:     planHash,
		Status:       "success",
		ChangedFiles: changedFiles,
		DiffStat:     diffStat,
		Artifacts:    []string{},
	}

	logPath, err := WriteIterationLog(opts.RepoRoot, iterLog)
	if err != nil {
		return nil, fmt.Errorf("failed to write iteration log: %w", err)
	}

	iterLog.Artifacts = append(iterLog.Artifacts, logPath)

	return iterLog, nil
}

func loadPlan(opts RunOptions) ([]byte, error) {
	if opts.UseStdin {
		return os.ReadFile("/dev/stdin")
	}

	if opts.PlanPath == "" {
		return nil, fmt.Errorf("either --plan or --stdin must be specified")
	}

	return os.ReadFile(opts.PlanPath)
}

func stripMarkdownFences(data []byte) []byte {
	content := string(data)
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "```yaml") || strings.HasPrefix(content, "```yml") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			lines = lines[1 : len(lines)-1]
			content = strings.Join(lines, "\n")
		}
	} else if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			lines = lines[1 : len(lines)-1]
			content = strings.Join(lines, "\n")
		}
	}

	return []byte(content)
}

func writeFailureLog(repoRoot string, iterNum int, goal, planHash string, execErr error) error {
	log := &IterationLog{
		Iteration:    iterNum,
		Goal:         goal,
		PlanHash:     planHash,
		Status:       "failed",
		ChangedFiles: []string{},
		DiffStat:     DiffStat{},
		Artifacts:    []string{},
	}

	if _, err := WriteIterationLog(repoRoot, log); err != nil {
		return fmt.Errorf("execution failed: %v; failed to write failure log: %w", execErr, err)
	}

	return execErr
}
