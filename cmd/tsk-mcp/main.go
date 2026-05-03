package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tickstem/cron"
	"github.com/tickstem/verify"
)

func main() {
	apiKey := os.Getenv("TICKSTEM_API_KEY")
	if apiKey == "" {
		log.Fatal("TICKSTEM_API_KEY is not set")
	}

	baseURL := os.Getenv("TICKSTEM_BASE_URL")

	cronOpts := []cron.Option{}
	verifyOpts := []verify.Option{}
	if baseURL != "" {
		cronOpts = append(cronOpts, cron.WithBaseURL(baseURL))
		verifyOpts = append(verifyOpts, verify.WithBaseURL(baseURL))
	}

	cronClient := cron.New(apiKey, cronOpts...)
	verifyClient := verify.New(apiKey, verifyOpts...)

	apiBaseURL := "https://api.tickstem.dev/v1"
	if baseURL != "" {
		apiBaseURL = baseURL
	}
	uptimeClient := newUptimeClient(apiKey, apiBaseURL)
	heartbeatClient := newUptimeClient(apiKey, apiBaseURL)

	s := server.NewMCPServer(
		"tickstem",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	registerCronTools(s, cronClient)
	registerVerifyTools(s, verifyClient)
	registerUptimeTools(s, uptimeClient)
	registerHeartbeatTools(s, heartbeatClient)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}

// ── cron tools ─────────────────────────────────────────────────────────────────

func registerCronTools(s *server.MCPServer, client *cron.Client) {
	s.AddTool(mcp.NewTool("list_jobs",
		mcp.WithDescription("List all cron jobs in the account"),
	), makeListJobs(client))

	s.AddTool(mcp.NewTool("get_job",
		mcp.WithDescription("Get a cron job by ID"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("The job ID"),
		),
	), makeGetJob(client))

	s.AddTool(mcp.NewTool("register_job",
		mcp.WithDescription("Register a new cron job"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Human-readable label for the job"),
		),
		mcp.WithString("schedule",
			mcp.Required(),
			mcp.Description(`Standard 5-field cron expression, e.g. "0 9 * * 1-5"`),
		),
		mcp.WithString("endpoint",
			mcp.Required(),
			mcp.Description("URL that will be called on each execution"),
		),
		mcp.WithString("method",
			mcp.Description("HTTP method (GET, POST, PUT, PATCH, DELETE). Defaults to POST"),
		),
		mcp.WithString("description",
			mcp.Description("Optional human-readable note"),
		),
		mcp.WithNumber("timeout_secs",
			mcp.Description("Request timeout in seconds (1-300). Defaults to 30"),
		),
	), makeRegisterJob(client))

	s.AddTool(mcp.NewTool("update_job",
		mcp.WithDescription("Update an existing cron job. Only provided fields are changed"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("The job ID"),
		),
		mcp.WithString("name",
			mcp.Description("New name for the job"),
		),
		mcp.WithString("schedule",
			mcp.Description("New cron schedule"),
		),
		mcp.WithString("endpoint",
			mcp.Description("New endpoint URL"),
		),
		mcp.WithString("method",
			mcp.Description("New HTTP method"),
		),
		mcp.WithString("description",
			mcp.Description("New description"),
		),
		mcp.WithNumber("timeout_secs",
			mcp.Description("New timeout in seconds"),
		),
	), makeUpdateJob(client))

	s.AddTool(mcp.NewTool("pause_job",
		mcp.WithDescription("Pause a cron job so it no longer fires"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("The job ID"),
		),
	), makePauseJob(client))

	s.AddTool(mcp.NewTool("resume_job",
		mcp.WithDescription("Resume a paused or failing cron job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("The job ID"),
		),
	), makeResumeJob(client))

	s.AddTool(mcp.NewTool("delete_job",
		mcp.WithDescription("Permanently delete a cron job and its execution history"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("The job ID"),
		),
	), makeDeleteJob(client))

	s.AddTool(mcp.NewTool("list_executions",
		mcp.WithDescription("List execution history for a cron job, most recent first"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("The job ID"),
		),
	), makeListExecutions(client))
}

func makeListJobs(client *cron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobs, err := client.List(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(jobs)
	}
}

func makeGetJob(client *cron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID := mcp.ParseString(req, "job_id", "")
		if jobID == "" {
			return mcp.NewToolResultError("job_id is required"), nil
		}
		job, err := client.Get(ctx, jobID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(job)
	}
}

func makeRegisterJob(client *cron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := cron.RegisterParams{
			Name:        mcp.ParseString(req, "name", ""),
			Schedule:    mcp.ParseString(req, "schedule", ""),
			Endpoint:    mcp.ParseString(req, "endpoint", ""),
			Method:      mcp.ParseString(req, "method", ""),
			Description: mcp.ParseString(req, "description", ""),
			TimeoutSecs: mcp.ParseInt(req, "timeout_secs", 0),
		}
		job, err := client.Register(ctx, params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(job)
	}
}

func makeUpdateJob(client *cron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID := mcp.ParseString(req, "job_id", "")
		if jobID == "" {
			return mcp.NewToolResultError("job_id is required"), nil
		}

		existing, err := client.Get(ctx, jobID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		params := cron.RegisterParams{
			Name:        coalesce(mcp.ParseString(req, "name", ""), existing.Name),
			Schedule:    coalesce(mcp.ParseString(req, "schedule", ""), existing.Schedule),
			Endpoint:    coalesce(mcp.ParseString(req, "endpoint", ""), existing.Endpoint),
			Method:      coalesce(mcp.ParseString(req, "method", ""), existing.Method),
			Description: coalesce(mcp.ParseString(req, "description", ""), existing.Description),
			TimeoutSecs: coalesceInt(mcp.ParseInt(req, "timeout_secs", 0), existing.TimeoutSecs),
		}

		updated, err := client.Update(ctx, jobID, params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(updated)
	}
}

func makePauseJob(client *cron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID := mcp.ParseString(req, "job_id", "")
		if jobID == "" {
			return mcp.NewToolResultError("job_id is required"), nil
		}
		job, err := client.Pause(ctx, jobID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(job)
	}
}

func makeResumeJob(client *cron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID := mcp.ParseString(req, "job_id", "")
		if jobID == "" {
			return mcp.NewToolResultError("job_id is required"), nil
		}
		job, err := client.Resume(ctx, jobID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(job)
	}
}

func makeDeleteJob(client *cron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID := mcp.ParseString(req, "job_id", "")
		if jobID == "" {
			return mcp.NewToolResultError("job_id is required"), nil
		}
		if err := client.Delete(ctx, jobID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("job %s deleted", jobID)), nil
	}
}

func makeListExecutions(client *cron.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID := mcp.ParseString(req, "job_id", "")
		if jobID == "" {
			return mcp.NewToolResultError("job_id is required"), nil
		}
		executions, err := client.Executions(ctx, jobID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(executions)
	}
}

// ── verify tools ───────────────────────────────────────────────────────────────

func registerVerifyTools(s *server.MCPServer, vc *verify.Client) {
	s.AddTool(mcp.NewTool("verify_email",
		mcp.WithDescription("Verify an email address: checks syntax, MX records, disposable domain list, and role-based prefixes"),
		mcp.WithString("email",
			mcp.Required(),
			mcp.Description("The email address to verify"),
		),
	), makeVerifyEmail(vc))

	s.AddTool(mcp.NewTool("list_verify_history",
		mcp.WithDescription("List past email verification results for the account"),
		mcp.WithNumber("limit",
			mcp.Description("Number of results to return (1-100, default 20)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Offset for pagination"),
		),
	), makeListVerifyHistory(vc))
}

func makeVerifyEmail(vc *verify.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email := mcp.ParseString(req, "email", "")
		if email == "" {
			return mcp.NewToolResultError("email is required"), nil
		}
		result, err := vc.Verify(ctx, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func makeListVerifyHistory(vc *verify.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := vc.ListHistory(ctx, verify.ListHistoryParams{
			Limit:  mcp.ParseInt(req, "limit", 20),
			Offset: mcp.ParseInt(req, "offset", 0),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(page)
	}
}

// ── uptime tools ───────────────────────────────────────────────────────────────

type uptimeClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func newUptimeClient(apiKey, baseURL string) *uptimeClient {
	return &uptimeClient{apiKey: apiKey, baseURL: baseURL, http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *uptimeClient) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tickstem-mcp/1.0.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		var e struct{ Error string }
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, e.Error)
		}
		return nil, fmt.Errorf("API error %d", resp.StatusCode)
	}
	return data, nil
}

func registerUptimeTools(s *server.MCPServer, client *uptimeClient) {
	s.AddTool(mcp.NewTool("list_monitors",
		mcp.WithDescription("List all uptime monitors in the account. Returns each monitor's status (active/paused/failing), URL, check interval, SSL expiry date, and assertion rules."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.do(ctx, http.MethodGet, "/monitors", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("create_monitor",
		mcp.WithDescription("Create an uptime monitor that polls an HTTP/HTTPS endpoint on a schedule and alerts on failure. "+
			"Supports response assertions to validate that the response is correct — not just that the server responded. "+
			"Assertions use source (status_code, response_time, body) + comparison (eq, ne, lt, lte, gt, gte, contains, not_contains) + target. "+
			"When assertions are set they replace the default 2xx/3xx success logic. "+
			"For HTTPS endpoints, SSL certificate expiry is captured automatically and an alert is sent 30 days before expiry. "+
			"Pass assertions as a JSON string, e.g.: [{\"source\":\"status_code\",\"comparison\":\"eq\",\"target\":\"200\"},{\"source\":\"body\",\"comparison\":\"contains\",\"target\":\"\\\"status\\\":\\\"ok\\\"\"}]"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Human-readable label for the monitor"),
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("HTTP or HTTPS URL to check"),
		),
		mcp.WithNumber("interval_secs",
			mcp.Description("Check interval in seconds (60–86400). Free plan minimum is 300s, paid plans 60s. Defaults to 60."),
		),
		mcp.WithNumber("timeout_secs",
			mcp.Description("Request timeout in seconds (5–30, default 10)"),
		),
		mcp.WithString("assertions",
			mcp.Description("Optional JSON array of assertion objects. Each must have source, comparison, and target fields. "+
				"Sources: status_code, response_time, body. "+
				"Numeric comparisons (status_code, response_time): eq, ne, lt, lte, gt, gte — target must be an integer string. "+
				"Body comparisons: eq, ne, contains, not_contains — target is a plain string."),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body := map[string]any{
			"name": mcp.ParseString(req, "name", ""),
			"url":  mcp.ParseString(req, "url", ""),
		}
		if v := mcp.ParseInt(req, "interval_secs", 0); v > 0 {
			body["interval_secs"] = v
		}
		if v := mcp.ParseInt(req, "timeout_secs", 0); v > 0 {
			body["timeout_secs"] = v
		}
		if raw := mcp.ParseString(req, "assertions", ""); raw != "" {
			var assertions []map[string]string
			if err := json.Unmarshal([]byte(raw), &assertions); err != nil {
				return mcp.NewToolResultError("assertions must be a valid JSON array: " + err.Error()), nil
			}
			body["assertions"] = assertions
		}
		data, err := client.do(ctx, http.MethodPost, "/monitors", body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("get_monitor",
		mcp.WithDescription("Get a single uptime monitor by ID. Returns current status, SSL expiry date, assertion rules, and next scheduled check time."),
		mcp.WithString("monitor_id",
			mcp.Required(),
			mcp.Description("The monitor ID"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "monitor_id", "")
		if id == "" {
			return mcp.NewToolResultError("monitor_id is required"), nil
		}
		data, err := client.do(ctx, http.MethodGet, "/monitors/"+id, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("pause_monitor",
		mcp.WithDescription("Pause an uptime monitor so it stops polling. No alerts will fire while paused. Use resume_monitor to restart."),
		mcp.WithString("monitor_id",
			mcp.Required(),
			mcp.Description("The monitor ID"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "monitor_id", "")
		if id == "" {
			return mcp.NewToolResultError("monitor_id is required"), nil
		}
		data, err := client.do(ctx, http.MethodPatch, "/monitors/"+id, map[string]string{"status": "paused"})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("resume_monitor",
		mcp.WithDescription("Resume a paused uptime monitor. Polling and alerting restart immediately."),
		mcp.WithString("monitor_id",
			mcp.Required(),
			mcp.Description("The monitor ID"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "monitor_id", "")
		if id == "" {
			return mcp.NewToolResultError("monitor_id is required"), nil
		}
		data, err := client.do(ctx, http.MethodPatch, "/monitors/"+id, map[string]string{"status": "active"})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("delete_monitor",
		mcp.WithDescription("Permanently delete an uptime monitor and all its check history. This cannot be undone."),
		mcp.WithString("monitor_id",
			mcp.Required(),
			mcp.Description("The monitor ID"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "monitor_id", "")
		if id == "" {
			return mcp.NewToolResultError("monitor_id is required"), nil
		}
		if _, err := client.do(ctx, http.MethodDelete, "/monitors/"+id, nil); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("monitor %s deleted", id)), nil
	})

	s.AddTool(mcp.NewTool("list_monitor_checks",
		mcp.WithDescription("List recent check results for an uptime monitor, most recent first. "+
			"Each check includes: status (up/down/timeout), HTTP status code, response time in ms, error message, SSL certificate expiry date, and timestamp. "+
			"Use this to diagnose failures or verify that a monitor is healthy."),
		mcp.WithString("monitor_id",
			mcp.Required(),
			mcp.Description("The monitor ID"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Number of results to return (1–100, default 50)"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "monitor_id", "")
		if id == "" {
			return mcp.NewToolResultError("monitor_id is required"), nil
		}
		limit := mcp.ParseInt(req, "limit", 50)
		data, err := client.do(ctx, http.MethodGet, fmt.Sprintf("/monitors/%s/checks?limit=%d", id, limit), nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

// ── heartbeat tools ────────────────────────────────────────────────────────────

func registerHeartbeatTools(s *server.MCPServer, client *uptimeClient) {
	s.AddTool(mcp.NewTool("list_heartbeats",
		mcp.WithDescription("List all heartbeat monitors in the account. Each heartbeat has a token used for pinging and a status: active, paused, or failing."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := client.do(ctx, http.MethodGet, "/heartbeats", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("create_heartbeat",
		mcp.WithDescription("Create a heartbeat monitor (dead-man's switch). Your job should POST to the returned ping URL after each successful run. If the ping stops arriving within the interval + grace window, Tickstem sends an alert."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Human-readable label for the heartbeat"),
		),
		mcp.WithNumber("interval_secs",
			mcp.Description("Expected ping interval in seconds (60–86400, default 3600)"),
		),
		mcp.WithNumber("grace_secs",
			mcp.Description("Buffer after the deadline before alerting (0–86400, default 300)"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body := map[string]any{
			"name": mcp.ParseString(req, "name", ""),
		}
		if v := mcp.ParseInt(req, "interval_secs", 0); v > 0 {
			body["interval_secs"] = v
		}
		if v := mcp.ParseInt(req, "grace_secs", 0); v > 0 {
			body["grace_secs"] = v
		}
		data, err := client.do(ctx, http.MethodPost, "/heartbeats", body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("get_heartbeat",
		mcp.WithDescription("Get a single heartbeat monitor by ID. Returns its status, ping token, interval, grace window, and last pinged time."),
		mcp.WithString("heartbeat_id",
			mcp.Required(),
			mcp.Description("The heartbeat ID"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "heartbeat_id", "")
		if id == "" {
			return mcp.NewToolResultError("heartbeat_id is required"), nil
		}
		data, err := client.do(ctx, http.MethodGet, "/heartbeats/"+id, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("update_heartbeat",
		mcp.WithDescription("Update a heartbeat's name, interval, or grace window. Only provided fields are changed."),
		mcp.WithString("heartbeat_id",
			mcp.Required(),
			mcp.Description("The heartbeat ID"),
		),
		mcp.WithString("name",
			mcp.Description("New human-readable label"),
		),
		mcp.WithNumber("interval_secs",
			mcp.Description("New expected ping interval in seconds (60–86400)"),
		),
		mcp.WithNumber("grace_secs",
			mcp.Description("New grace window in seconds (0–86400)"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "heartbeat_id", "")
		if id == "" {
			return mcp.NewToolResultError("heartbeat_id is required"), nil
		}
		body := map[string]any{}
		if v := mcp.ParseString(req, "name", ""); v != "" {
			body["name"] = v
		}
		if v := mcp.ParseInt(req, "interval_secs", 0); v > 0 {
			body["interval_secs"] = v
		}
		if v := mcp.ParseInt(req, "grace_secs", 0); v >= 0 {
			body["grace_secs"] = v
		}
		data, err := client.do(ctx, http.MethodPatch, "/heartbeats/"+id, body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("pause_heartbeat",
		mcp.WithDescription("Pause a heartbeat monitor. Alerts are suppressed while paused — useful during planned downtime or deployments."),
		mcp.WithString("heartbeat_id",
			mcp.Required(),
			mcp.Description("The heartbeat ID"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "heartbeat_id", "")
		if id == "" {
			return mcp.NewToolResultError("heartbeat_id is required"), nil
		}
		data, err := client.do(ctx, http.MethodPost, "/heartbeats/"+id+"/pause", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("resume_heartbeat",
		mcp.WithDescription("Resume a paused heartbeat monitor. Alerting restarts immediately."),
		mcp.WithString("heartbeat_id",
			mcp.Required(),
			mcp.Description("The heartbeat ID"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "heartbeat_id", "")
		if id == "" {
			return mcp.NewToolResultError("heartbeat_id is required"), nil
		}
		data, err := client.do(ctx, http.MethodPost, "/heartbeats/"+id+"/resume", nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("delete_heartbeat",
		mcp.WithDescription("Permanently delete a heartbeat monitor and all its ping history. This cannot be undone."),
		mcp.WithString("heartbeat_id",
			mcp.Required(),
			mcp.Description("The heartbeat ID"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "heartbeat_id", "")
		if id == "" {
			return mcp.NewToolResultError("heartbeat_id is required"), nil
		}
		if _, err := client.do(ctx, http.MethodDelete, "/heartbeats/"+id, nil); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("heartbeat %s deleted", id)), nil
	})

	s.AddTool(mcp.NewTool("ping_heartbeat",
		mcp.WithDescription("Ping a heartbeat to signal a successful job run. The token is the credential — no API key needed. Call this at the end of each successful execution."),
		mcp.WithString("token",
			mcp.Required(),
			mcp.Description("The heartbeat ping token (returned when the heartbeat was created)"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		token := mcp.ParseString(req, "token", "")
		if token == "" {
			return mcp.NewToolResultError("token is required"), nil
		}
		pingURL := client.baseURL + "/heartbeats/" + token + "/ping"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, pingURL, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		httpReq.Header.Set("User-Agent", "tickstem-mcp/1.0.0")
		resp, err := client.http.Do(httpReq)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			return mcp.NewToolResultError(fmt.Sprintf("API error %d: %s", resp.StatusCode, string(data))), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcp.NewTool("list_heartbeat_pings",
		mcp.WithDescription("List recent pings for a heartbeat monitor, most recent first."),
		mcp.WithString("heartbeat_id",
			mcp.Required(),
			mcp.Description("The heartbeat ID"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Number of results to return (1–100, default 50)"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(req, "heartbeat_id", "")
		if id == "" {
			return mcp.NewToolResultError("heartbeat_id is required"), nil
		}
		limit := mcp.ParseInt(req, "limit", 50)
		data, err := client.do(ctx, http.MethodGet, fmt.Sprintf("/heartbeats/%s/pings?limit=%d", id, limit), nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	})
}

// ── helpers ────────────────────────────────────────────────────────────────────

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func coalesce(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

func coalesceInt(val, fallback int) int {
	if val != 0 {
		return val
	}
	return fallback
}
