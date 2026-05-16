package pilot

import (
	"bytes"
	"context"
	"errors"
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
	if opts.AutoApply {
		fmt.Fprintln(os.Stderr, AutoApplyWarning)
	}

	iterNum, err := NextIterationNumber(opts.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get next iteration number: %w", err)
	}

	planBytes, err := loadPlan(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load plan: %w", err)
	}

	planBytes = stripMarkdownFences(planBytes)

	// Hash the LLM's raw output, not the wrapped form — RunLoop's
	// no-progress check wants to detect "the model emitted the same
	// plan again", which is a property of the model's output, not
	// pilot's deterministic wrap.
	planHash := ComputePlanHash(planBytes)

	wrappedBytes, err := WrapInTransaction(planBytes)
	if err != nil {
		return nil, writeFailureLog(opts.RepoRoot, iterNum, opts.Goal, planHash, fmt.Errorf("transaction wrap failed: %w", err))
	}

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

	if _, writeErr := tmpFile.Write(wrappedBytes); writeErr != nil {
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

	// Plan-confirm gate (spec-67 §10). Skipped when --auto-apply.
	// Single-shot mode: reject and abort both terminate the run; only
	// apply proceeds to executor.Start.
	if !opts.AutoApply {
		if err := EnsureInteractive(os.Stdin); err != nil {
			return nil, err
		}
		result, gateErr := ConfirmPlan(os.Stdin, os.Stderr, planBytes)
		if gateErr != nil {
			return nil, gateErr
		}
		switch result.Outcome {
		case OutcomeReject:
			return nil, errors.New("plan rejected at confirm gate")
		case OutcomeAbort:
			return nil, errors.New("plan aborted at confirm gate")
		case OutcomeApply:
			if !bytes.Equal(result.PlanBytes, planBytes) {
				planBytes = result.PlanBytes
				wrappedBytes, err = WrapInTransaction(planBytes)
				if err != nil {
					return nil, fmt.Errorf("re-wrap edited plan: %w", err)
				}
				if err := os.WriteFile(tmpFile.Name(), wrappedBytes, 0o600); err != nil {
					return nil, fmt.Errorf("rewrite tempfile with edited plan: %w", err)
				}
			}
		}
	}

	publisher := events.NewPublisher()
	defer publisher.Close()

	log := logger.NewLogger(logger.ErrorLevel)

	// F016: agent.Run doesn't currently carry a context; the agent loop
	// is invoked from CLI and CLI-level signal handling tears down the
	// process. Use Background until a follow-up plumbs ctx through Run.
	execErr := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: tmpFile.Name(),
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
