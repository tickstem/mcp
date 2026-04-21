package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

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

	s := server.NewMCPServer(
		"tickstem",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	registerCronTools(s, cronClient)
	registerVerifyTools(s, verifyClient)

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
