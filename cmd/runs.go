package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/agentd"
)

// agentdHTTPClient returns an http.Client wired to the local agentd's unix
// socket. The system flag selects the system-mode socket (root daemon) vs
// the per-user socket.
func agentdHTTPClient(systemMode bool) (*http.Client, string, error) {
	cfg, err := agentd.Default(systemMode)
	if err != nil {
		return nil, "", err
	}
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", cfg.SocketPath)
		},
	}
	return &http.Client{Transport: transport}, cfg.SocketPath, nil
}

// runsApplyCommand submits a config to the local agentd and streams events
// back to stdout with the same rendering as a foreground `mooncake apply`.
//
// This is the agentd analog of `apply`: same UX, daemon-backed execution.
// Vars files are passed via --vars (multi-valued).
func runsApplyCommand(c *cli.Context) error {
	resolvedPath, err := resolveConfigPath(c)
	if err != nil {
		if printNoConfigHintAndExit(err, "runs apply") {
			return nil
		}
		return err
	}
	configPath, err := absPath(resolvedPath)
	if err != nil {
		return err
	}

	varsAbs := make([]string, 0, len(c.StringSlice("vars")))
	for _, v := range c.StringSlice("vars") {
		ap, absErr := absPath(v)
		if absErr != nil {
			return absErr
		}
		varsAbs = append(varsAbs, ap)
	}

	systemMode := c.Bool("system")
	hc, sock, err := agentdHTTPClient(systemMode)
	if err != nil {
		return err
	}

	body := map[string]any{
		"plan_path":  configPath,
		"vars_files": varsAbs,
		"goal":       c.String("goal"),
	}
	if tagsCSV := c.String("tags"); tagsCSV != "" {
		body["tags"] = parseTags(tagsCSV)
	}
	if d := c.String("base-dir"); d != "" {
		bd, absErr := absPath(d)
		if absErr != nil {
			return absErr
		}
		body["base_dir"] = bd
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := hc.Post("http://localhost/v1/runs", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("submit run via %s: %w", sock, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("submit run: HTTP %d: %s", resp.StatusCode, string(b))
	}

	var submitResp struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&submitResp); decodeErr != nil {
		return fmt.Errorf("decode submit response: %w", decodeErr)
	}
	fmt.Fprintf(os.Stderr, "Run %s submitted; streaming events...\n\n", submitResp.RunID)

	return streamRunEvents(hc, submitResp.RunID)
}

// runsFollowCommand attaches to an already-submitted run and renders its
// events to stdout. If the run is already terminal, this still reads back
// all replayable events from the daemon's store and prints a recap.
func runsFollowCommand(c *cli.Context) error {
	runID := c.Args().First()
	if runID == "" {
		return fmt.Errorf("usage: mooncake runs follow <run_id>")
	}
	hc, _, err := agentdHTTPClient(c.Bool("system"))
	if err != nil {
		return err
	}
	return streamRunEvents(hc, runID)
}

func runsGetCommand(c *cli.Context) error {
	runID := c.Args().First()
	if runID == "" {
		return fmt.Errorf("usage: mooncake runs get <run_id>")
	}
	hc, _, err := agentdHTTPClient(c.Bool("system"))
	if err != nil {
		return err
	}
	resp, err := hc.Get("http://localhost/v1/runs/" + runID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
	return nil
}

func runsListCommand(c *cli.Context) error {
	hc, _, err := agentdHTTPClient(c.Bool("system"))
	if err != nil {
		return err
	}
	resp, err := hc.Get("http://localhost/v1/runs")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
	return nil
}

// streamRunEvents reads the SSE event stream for a run and renders each
// event the same way a foreground `mooncake apply` would. Returns when the
// run hits a terminal state (or the connection drops cleanly).
func streamRunEvents(hc *http.Client, runID string) error {
	resp, err := hc.Get("http://localhost/v1/runs/" + runID + "/events")
	if err != nil {
		return fmt.Errorf("open event stream: %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// SSE events can be larger than the default 64K (long stdout lines).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	runFailed := false
	// Remember each step's action by step_id from step.started so we can
	// fall back to it as a display name on step.completed/.failed/.skipped
	// events when the user didn't set a `name:`. Without this fallback
	// `mooncake runs apply` streamed blank rows for unnamed steps (#55).
	stepActions := map[string]string{}
	display := func(name, stepID string) string {
		if name != "" {
			return name
		}
		if a := stepActions[stepID]; a != "" {
			return "<" + a + ">"
		}
		return "<unnamed step>"
	}
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")

		var env struct {
			Seq  int             `json:"seq"`
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if jsonErr := json.Unmarshal([]byte(payload), &env); jsonErr != nil {
			continue
		}

		switch env.Type {
		case "step.started":
			var d struct {
				StepID string `json:"step_id"`
				Name   string `json:"name"`
				Action string `json:"action"`
			}
			_ = json.Unmarshal(env.Data, &d)
			if d.StepID != "" && d.Action != "" {
				stepActions[d.StepID] = d.Action
			}
			fmt.Printf("%s %s\n", color.CyanString("▶"), display(d.Name, d.StepID))
		case "step.completed":
			var d struct {
				StepID  string `json:"step_id"`
				Name    string `json:"name"`
				Changed bool   `json:"changed"`
			}
			_ = json.Unmarshal(env.Data, &d)
			name := display(d.Name, d.StepID)
			if d.Changed {
				fmt.Printf("%s %s\n", color.YellowString("~"), name)
			} else {
				fmt.Printf("%s %s\n", color.GreenString("✓"), name)
			}
		case "step.failed":
			runFailed = true
			var d struct {
				StepID       string `json:"step_id"`
				Name         string `json:"name"`
				ErrorMessage string `json:"error_message"`
			}
			_ = json.Unmarshal(env.Data, &d)
			fmt.Printf("%s %s\n  %s\n", color.RedString("✗"), display(d.Name, d.StepID), d.ErrorMessage)
		case "step.skipped":
			var d struct {
				StepID string `json:"step_id"`
				Name   string `json:"name"`
				Reason string `json:"reason"`
			}
			_ = json.Unmarshal(env.Data, &d)
			fmt.Printf("%s %s  %s\n", color.HiBlackString("-"), display(d.Name, d.StepID), d.Reason)
		case "run.completed":
			var d struct {
				OK       int    `json:"success_steps"`
				Changed  int    `json:"changed_steps"`
				Skipped  int    `json:"skipped_steps"`
				Failed   int    `json:"failed_steps"`
				Reverted int    `json:"reverted_steps"`
				Dur      int64  `json:"duration_ms"`
				Success  bool   `json:"success"`
				Error    string `json:"error_message"`
			}
			_ = json.Unmarshal(env.Data, &d)
			var recap string
			if d.Reverted > 0 {
				recap = fmt.Sprintf("RECAP  ok=%d  changed=%d  skipped=%d  failed=%d  reverted=%d  %dms",
					d.OK, d.Changed, d.Skipped, d.Failed, d.Reverted, d.Dur)
			} else {
				recap = fmt.Sprintf("RECAP  ok=%d  changed=%d  skipped=%d  failed=%d  %dms",
					d.OK, d.Changed, d.Skipped, d.Failed, d.Dur)
			}
			if d.Success {
				fmt.Printf("\n%s\n", color.GreenString(recap))
			} else {
				fmt.Printf("\n%s  %s %s\n", color.RedString(recap), color.RedString("✗"), d.Error)
			}
			if !d.Success {
				runFailed = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("event stream error: %w", err)
	}
	if runFailed {
		return cli.Exit("", 1)
	}
	return nil
}

func absPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Abs(p)
}
