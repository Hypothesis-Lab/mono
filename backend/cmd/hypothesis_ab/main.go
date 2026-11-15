package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type result struct {
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
}

type options struct {
	mode, baseURL, hypothesisID, tokenFile, token string
	wait                                          bool
	timeout, poll, requestTimeout                 time.Duration
}

type httpStatusError struct {
	status     int
	body       string
	workflowID string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.status, e.body)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: hypothesis_ab launch|status|stop [flags]")
	}

	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printUsage()
		return nil
	}

	command := args[0]
	if !validCommand(command) {
		return fmt.Errorf("unknown command %q", command)
	}

	cfg, err := parseFlags(command, args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	if err := validate(cfg, command); err != nil {
		return err
	}

	token, err := resolveToken(cfg.token, cfg.tokenFile)
	if err != nil {
		return err
	}

	endpoint := hypothesisURL(cfg.baseURL, cfg.hypothesisID)
	return execute(command, endpoint, token, cfg)
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: hypothesis_ab launch|status|stop [flags]")
	fmt.Fprintln(os.Stdout, "Use 'hypothesis_ab <command> --help' for command flags.")
}

func validCommand(command string) bool {
	return command == "launch" || command == "status" || command == "stop"
}

func parseFlags(command string, args []string) (options, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	cfg := options{}
	fs.StringVar(&cfg.mode, "mode", "b", "launch mode: a (sync) or b (async)")
	fs.StringVar(&cfg.baseURL, "base-url", "http://127.0.0.1:8000", "backend base URL")
	fs.StringVar(&cfg.hypothesisID, "hypothesis-id", "", "hypothesis identifier")
	fs.StringVar(&cfg.tokenFile, "token-file", "", "file containing API bearer token")
	fs.StringVar(&cfg.token, "token", "", "API bearer token (prefer HYPOTHESIS_AB_TOKEN)")
	fs.BoolVar(&cfg.wait, "wait", false, "poll until completed or failed")
	fs.DurationVar(&cfg.timeout, "timeout", 10*time.Minute, "overall wait timeout")
	fs.DurationVar(&cfg.poll, "poll-interval", 2*time.Second, "status polling interval")
	fs.DurationVar(&cfg.requestTimeout, "request-timeout", 10*time.Minute, "launch request timeout")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	if fs.NArg() != 0 {
		return options{}, errors.New("unexpected positional arguments")
	}

	return cfg, nil
}

func validate(cfg options, command string) error {
	if cfg.hypothesisID == "" {
		return errors.New("--hypothesis-id is required")
	}

	if command == "launch" && cfg.mode != "a" && cfg.mode != "b" {
		return errors.New("--mode must be a or b")
	}

	if cfg.timeout <= 0 || cfg.poll <= 0 || cfg.requestTimeout <= 0 {
		return errors.New("timeouts and poll interval must be positive")
	}

	if cfg.token != "" && cfg.tokenFile != "" {
		return errors.New("use only one of --token and --token-file")
	}

	return nil
}

func resolveToken(token, tokenFile string) (string, error) {
	if token != "" {
		return token, nil
	}

	if tokenFile != "" {
		// The local CLI user explicitly selects this token-file path.
		b, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("read token file: %w", err)
		}

		return strings.TrimSpace(string(b)), nil
	}

	return os.Getenv("HYPOTHESIS_AB_TOKEN"), nil
}

func hypothesisURL(baseURL, id string) string {
	return strings.TrimRight(baseURL, "/") + "/api/v1/hypothesis/" + url.PathEscape(id)
}

func execute(command, endpoint, token string, cfg options) error {
	if token == "" {
		return errors.New("provide API token with HYPOTHESIS_AB_TOKEN or --token-file")
	}

	switch command {
	case "stop":
		return request(endpoint+"/stop", http.MethodPost, "", token, 30*time.Second, nil)
	case "status":
		return printStatus(endpoint+"/launch-status", token, 30*time.Second, "")
	default:
		return launch(endpoint, token, cfg)
	}

}

func launch(endpoint, token string, cfg options) error {
	mode := map[string]string{"a": "sync", "b": "async"}[cfg.mode]
	var launched result
	started := time.Now()
	err := request(endpoint+"/launch?mode="+mode, http.MethodPost, "", token, cfg.requestTimeout, &launched)
	fmt.Fprintf(os.Stderr, "HTTP launch elapsed=%s\n", time.Since(started).Round(time.Millisecond))
	if err != nil {
		return launchError(err, cfg)
	}

	if err := writeJSON(launched); err != nil {
		return err
	}

	if cfg.mode == "a" {
		switch launched.Status {
		case "completed":
			return nil
		case "failed":
			return errors.New("workflow failed")
		case "failed_with_residuals":
			return errors.New("preparation stopped; residual or uncertain resources are preserved; a new explicit attempt is permitted")
		case "reconciliation_required":
			return errors.New("workflow requires reconciliation; outcome unresolved")
		case "queued", "running", "rolling_back":
		default:
			return fmt.Errorf("unknown workflow status %q", launched.Status)
		}
	}

	if !cfg.wait {
		return nil
	}

	return waitForCompletion(endpoint, token, launched, cfg)
}

func launchError(err error, cfg options) error {
	if cfg.mode != "a" {
		return err
	}

	var statusErr *httpStatusError
	if errors.As(err, &statusErr) && statusErr.status >= 400 && statusErr.status < 500 {
		if statusErr.workflowID != "" {
			return fmt.Errorf("launch rejected by server; workflow_id=%s: %w", statusErr.workflowID, err)
		}

		return fmt.Errorf("launch rejected by server: %w", err)
	}

	if errors.As(err, &statusErr) && statusErr.workflowID != "" {
		return fmt.Errorf("launch outcome unknown; server response may follow a commit; workflow_id=%s. Do not resubmit. Check: hypothesis_ab status --hypothesis-id %s: %w", statusErr.workflowID, cfg.hypothesisID, err)
	}

	if errors.As(err, &statusErr) && statusErr.status >= 500 {
		return fmt.Errorf("launch outcome unknown; server error may follow a commit. Do not resubmit. Check: hypothesis_ab status --hypothesis-id %s: %w", cfg.hypothesisID, err)
	}

	return fmt.Errorf("launch outcome unknown; do not resubmit. Check: hypothesis_ab status --hypothesis-id %s: %w", cfg.hypothesisID, err)
}

func waitForCompletion(endpoint, token string, launched result, cfg options) error {
	deadline := time.Now().Add(cfg.timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("wait timed out; check hypothesis_ab status --hypothesis-id %s", cfg.hypothesisID)
		}

		var current result
		if err := request(endpoint+"/launch-status", http.MethodGet, "", token, min(30*time.Second, remaining), &current); err != nil {
			return err
		}

		if current.WorkflowID != launched.WorkflowID {
			return errors.New("status workflow_id differs from launched workflow")
		}

		if err := writeJSON(current); err != nil {
			return err
		}

		switch current.Status {
		case "completed":
			return nil
		case "failed":
			return errors.New("workflow failed")
		case "failed_with_residuals":
			return errors.New("preparation stopped; residual or uncertain resources are preserved; a new explicit attempt is permitted")
		case "reconciliation_required":
			return errors.New("workflow requires reconciliation; outcome unresolved")
		case "queued", "running", "rolling_back":
		default:
			return fmt.Errorf("unknown workflow status %q", current.Status)
		}

		remaining = time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("wait timed out; check hypothesis_ab status --hypothesis-id %s", cfg.hypothesisID)
		}

		time.Sleep(min(cfg.poll, remaining))
	}

}

func request(endpoint, method, body, token string, timeout time.Duration, out *result) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// The endpoint is assembled from the explicitly configured local CLI base URL.
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	// This request targets the user-selected backend base URL by design.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed after %s: %w", time.Since(start).Round(time.Millisecond), err)
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "HTTP %d elapsed=%s\n", resp.StatusCode, time.Since(start).Round(time.Millisecond))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var rejected result
		_ = json.Unmarshal(data, &rejected)
		return &httpStatusError{status: resp.StatusCode, body: strings.TrimSpace(string(data)), workflowID: rejected.WorkflowID}
	}

	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		if out.WorkflowID == "" || out.Status == "" {
			return errors.New("response missing workflow_id or status")
		}

	}

	return nil
}

func printStatus(endpoint, token string, timeout time.Duration, expected string) error {
	var got result
	if err := request(endpoint, http.MethodGet, "", token, timeout, &got); err != nil {
		return err
	}

	if expected != "" && expected != got.WorkflowID {
		return errors.New("status workflow_id differs from launched workflow")
	}

	if !validStatus(got.Status) {
		return fmt.Errorf("unknown workflow status %q", got.Status)
	}

	return writeJSON(got)
}

func validStatus(status string) bool {
	switch status {
	case "queued", "running", "rolling_back", "completed", "failed", "failed_with_residuals", "reconciliation_required":
		return true
	default:
		return false
	}

}

func writeJSON(v any) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}
