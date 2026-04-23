# tickstem/mcp

[![Glama](https://glama.ai/mcp/servers/tickstem/mcp/badge)](https://glama.ai/mcp/servers/tickstem/mcp)

MCP server for [Tickstem](https://tickstem.dev) — exposes cron job management and email verification as native tools for AI coding assistants (Claude, Cursor, Copilot, and any MCP-compatible agent).

Pairs with the [Go SDK](https://github.com/tickstem/cron) and [Node.js SDK](https://github.com/tickstem/node) — let your AI assistant register jobs and verify emails while you write the app code.

## Install

```bash
go install github.com/tickstem/mcp/cmd/tsk-mcp@latest
```

Or download a pre-built binary from [Releases](https://github.com/tickstem/mcp/releases).

## Quick start

```bash
export TICKSTEM_API_KEY=tsk_your_key_here
tsk-mcp
```

The server speaks the [Model Context Protocol](https://modelcontextprotocol.io) over stdio — connect it to any MCP-compatible client.

## Claude Code

Add to your `~/.claude/claude_desktop_config.json` (or equivalent):

```json
{
  "mcpServers": {
    "tickstem": {
      "command": "tsk-mcp",
      "env": {
        "TICKSTEM_API_KEY": "tsk_your_key_here"
      }
    }
  }
}
```

## Available tools

### Cron jobs

| Tool | Description |
|------|-------------|
| `list_jobs` | List all cron jobs in the account |
| `get_job` | Get a cron job by ID |
| `register_job` | Register a new cron job (name, schedule, endpoint) |
| `update_job` | Update an existing job — only provided fields change |
| `pause_job` | Pause a job so it no longer fires |
| `resume_job` | Resume a paused or failing job |
| `delete_job` | Permanently delete a job and its execution history |
| `list_executions` | List execution history for a job, most recent first |

### Email verification

| Tool | Description |
|------|-------------|
| `verify_email` | Check syntax, MX records, disposable domain, and role-based prefix |
| `list_verify_history` | List past verification results for the account |

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `TICKSTEM_API_KEY` | Yes | API key from [app.tickstem.dev](https://app.tickstem.dev) |
| `TICKSTEM_BASE_URL` | No | Override API base URL (e.g. `http://localhost:8080/v1` for local dev) |

## Local development

```bash
export TICKSTEM_API_KEY=tsk_your_key_here
export TICKSTEM_BASE_URL=http://localhost:8080/v1
go run ./cmd/tsk-mcp
```

Use [tsk-local](https://github.com/tickstem/cron) to run the Tickstem API locally.

## Get an API key

[app.tickstem.dev](https://app.tickstem.dev) — free tier includes 1,000 cron executions and 500 email verifications per month.

## License

MIT
