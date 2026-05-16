// Package pilot provides autonomous agent functionality for iterative plan generation and execution.
package pilot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pilot/llm"
	"github.com/alehatsman/mooncake/internal/snapshot"
)

const defaultMaxIterations = 5

type LoopResult struct {
	Iterations []IterationLog
	StopReason StopReason
	FinalLog   *IterationLog
}

func RunLoop(opts RunOptions) (*LoopResult, error) {
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = defaultMaxIterations
	}

	client, err := llm.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude client: %w", err)
	}

	var iterations []IterationLog
	var lastIteration *IterationSummary

	for i := 1; i <= opts.MaxIterations; i++ {
		iterNum, err := NextIterationNumber(opts.RepoRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to get iteration number: %w", err)
		}

		snap, err := snapshot.Collect(opts.RepoRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to collect snapshot: %w", err)
		}

		snapJSON, err := json.Marshal(snap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal snapshot: %w", err)
		}

		systemPrompt, userPrompt, err := BuildPrompt(PlanInput{
			Goal:          opts.Goal,
			Snapshot:      snapJSON,
			LastIteration: lastIteration,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build prompt: %w", err)
		}

		rawPlan, err := client.GeneratePlan(context.Background(), systemPrompt, userPrompt, opts.Model)
		if err != nil {
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, "", "generation_failed", err.Error())
			iterations = append(iterations, *log)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopFailed,
				FinalLog:   log,
			}, err
		}

		planBytes, err := SanitizePlan(rawPlan)
		if err != nil {
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, "", "sanitization_failed", err.Error())
			iterations = append(iterations, *log)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopFailed,
				FinalLog:   log,
			}, err
		}

		planHash := ComputePlanHash(planBytes)

		if lastIteration != nil && planHash == lastIteration.PlanHash {
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "no_progress", "plan identical to previous iteration")
			iterations = append(iterations, *log)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopNoProgress,
				FinalLog:   log,
			}, nil
		}

		wrappedBytes, err := WrapInTransaction(planBytes)
		if err != nil {
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "wrap_failed", err.Error())
			iterations = append(iterations, *log)
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopFailed,
				FinalLog:   log,
			}, err
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
		if err != nil || config.HasErrors(diagnostics) {
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			} else {
				errMsg = config.FormatDiagnostics(diagnostics)
			}
			log := writeLoopFailureLog(opts.RepoRoot, iterNum, opts, planHash, "validation_failed", errMsg)
			iterations = append(iterations, *log)
			lastIteration = &IterationSummary{
				Iteration:    iterNum,
				PlanHash:     planHash,
				Status:       "validation_failed",
				ErrorMessage: errMsg,
			}
			continue
		}

		publisher := events.NewPublisher()
		log := logger.NewLogger(logger.ErrorLevel)

		execErr := executor.Start(context.Background(), executor.StartConfig{
			ConfigFilePath: tmpFile.Name(),
		}, log, publisher)

		publisher.Close()

		changedFiles, _ := CollectChangedFiles(opts.RepoRoot)
		diffStat, _ := CollectDiffStat(opts.RepoRoot)

		planPath, savePlanErr := savePlan(opts.RepoRoot, iterNum, planBytes)
		if savePlanErr != nil {
			// Plan already executed via the tempfile path; failure to
			// persist the artifact is non-fatal to the iteration. Surface
			// it so the operator notices instead of seeing a silent
			// missing-artifact in the iteration log (F039).
			log.Errorf("pilot: save plan iteration %d: %v", iterNum, savePlanErr)
		}

		iterLog := &IterationLog{
			Iteration:    iterNum,
			Goal:         opts.Goal,
			PlanHash:     planHash,
			Provider:     opts.Provider,
			Model:        opts.Model,
			ChangedFiles: changedFiles,
			DiffStat:     diffStat,
		}

		if planPath != "" {
			iterLog.Artifacts = append(iterLog.Artifacts, planPath)
		}

		if execErr != nil {
			iterLog.Status = "execution_failed"
			iterLog.ExecutionError = execErr.Error()
			_, _ = WriteIterationLog(opts.RepoRoot, iterLog)
			iterations = append(iterations, *iterLog)
			lastIteration = &IterationSummary{
				Iteration:    iterNum,
				PlanHash:     planHash,
				Status:       "execution_failed",
				ChangedFiles: changedFiles,
				ErrorMessage: execErr.Error(),
			}
			continue
		}

		iterLog.Status = "success"
		_, _ = WriteIterationLog(opts.RepoRoot, iterLog)
		iterations = append(iterations, *iterLog)

		if len(changedFiles) == 0 {
			return &LoopResult{
				Iterations: iterations,
				StopReason: StopSuccess,
				FinalLog:   iterLog,
			}, nil
		}

		lastIteration = &IterationSummary{
			Iteration:    iterNum,
			PlanHash:     planHash,
			Status:       "success",
			ChangedFiles: changedFiles,
		}
	}

	finalLog := &iterations[len(iterations)-1]
	return &LoopResult{
		Iterations: iterations,
		StopReason: StopMaxReached,
		FinalLog:   finalLog,
	}, nil
}

// SavePlan persists an LLM-generated plan to `<repoRoot>/.mooncake/
// iterations/<n>.plan.yml` and returns the path. The directory is
// created at 0700 and the file at 0600 — plan artifacts can contain
// resolved !secret values (F037 resolves markers at plan time before
// the YAML is serialized) and the operator's goal/prompt, so world-
// readable permissions on a shared host (CI runner, dev box with
// multiple users) would leak them. Matches the 0700/0600 convention
// already in use by internal/agentd/store.go for run state (F039).
//
// Returns the saved path on success, or an empty string and an error
// on failure. The error is non-fatal to the loop iteration (the plan
// already ran via the tempfile path); callers should log+continue.
func SavePlan(repoRoot string, iterNum int, planBytes []byte) (string, error) {
	dir := filepath.Join(repoRoot, ".mooncake", "iterations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create iterations dir: %w", err)
	}
	filename := filepath.Join(dir, fmt.Sprintf("%05d.plan.yml", iterNum))
	if err := os.WriteFile(filename, planBytes, 0o600); err != nil {
		return "", fmt.Errorf("write plan: %w", err)
	}
	return filename, nil
}

// savePlan is the internal call site used by RunLoop. Kept as a thin
// wrapper so the caller doesn't have to import the same package as
// itself when testing the loop with a different persistence policy.
func savePlan(repoRoot string, iterNum int, planBytes []byte) (string, error) {
	return SavePlan(repoRoot, iterNum, planBytes)
}

func writeLoopFailureLog(repoRoot string, iterNum int, opts RunOptions, planHash, status, errMsg string) *IterationLog {
	log := &IterationLog{
		Iteration:       iterNum,
		Goal:            opts.Goal,
		PlanHash:        planHash,
		Status:          status,
		Provider:        opts.Provider,
		Model:           opts.Model,
		ChangedFiles:    []string{},
		DiffStat:        DiffStat{},
		ValidationError: errMsg,
	}
	_, _ = WriteIterationLog(repoRoot, log)
	return log
}
