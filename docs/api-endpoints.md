# API Endpoints

> **Note:** The HTTP API is implemented in [scout-and-wave-web](https://github.com/blackwell-systems/scout-and-wave-web), which imports this engine as a library. This document describes the API surface that wraps the engine's functionality.

## Base URL

```
http://localhost:7432/api
```

## Endpoints

### `POST /api/scout`

Generate an IMPL doc from a user prompt using the Scout agent.

**Request:**

```json
{
  "prompt": "Add user authentication to the API",
  "repo_path": "/path/to/repo",
  "scout_model": "claude-sonnet-4-6"
}
```

**Response:**

```json
{
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-add-user-auth.md",
  "status": "complete"
}
```

**SSE Events:** Published to `/api/events` with session ID in `X-Session-ID` header.

**Engine Call:** `engine.RunScout(ctx, opts)`

---

### `POST /api/wave/start`

Start a wave (launches all agents in parallel).

**Request:**

```json
{
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-feature.md",
  "wave_number": 1,
  "wave_model": "claude-sonnet-4-6"
}
```

**Response:**

```json
{
  "status": "started",
  "agents": ["A", "B", "C"]
}
```

**SSE Events:** Published to `/api/events` for wave lifecycle, agent progress, and completion reports.

**Engine Call:** `orchestrator.StartWave(ctx, waveNumber)`

---

### `POST /api/wave/merge`

Merge a wave's completed agent branches to main.

**Request:**

```json
{
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-feature.md",
  "wave_number": 1
}
```

**Response:**

```json
{
  "status": "merged",
  "conflicts": []
}
```

**Conflict Response:**

```json
{
  "status": "conflict",
  "conflicts": [
    {
      "agent": "B",
      "files": ["src/auth.go", "src/middleware.go"]
    }
  ]
}
```

**Engine Call:** `orchestrator.MergeWave(ctx, waveNumber)`

---

### `POST /api/verification`

Run verification suite (build, tests, protocol invariants).

**Request:**

```json
{
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-feature.md",
  "repo_path": "/path/to/repo"
}
```

**Response:**

```json
{
  "status": "passed",
  "build": "passed",
  "tests": "passed",
  "invariants": "passed"
}
```

**Failure Response:**

```json
{
  "status": "failed",
  "build": "failed",
  "build_output": "error: undefined reference to `authenticate`",
  "tests": "passed",
  "invariants": "passed"
}
```

**Engine Call:** `orchestrator.RunVerification(ctx)`

---

### `POST /api/chat`

Chat with Claude using a specified model (no IMPL doc, standalone conversation).

**Request:**

```json
{
  "message": "How do I implement JWT authentication in Go?",
  "model": "claude-sonnet-4-6",
  "repo_path": "/path/to/repo"
}
```

**Response (streaming):**

SSE stream of text chunks, terminated with `[DONE]`.

**Engine Call:** `engine.RunChat(ctx, opts)`

---

### `GET /api/events`

Subscribe to Server-Sent Events (SSE) for real-time orchestrator updates.

**Query Parameters:**
- `session_id` (optional) — Filter events for a specific session

**Response:** SSE stream (see `docs/sse-events.md` for event schema).

**Engine Integration:** The web server wraps `orchestrator.SSEBroker` and publishes events from the engine's orchestrator.

---

### `GET /api/config`

Get current configuration (Scout/Wave/Chat models).

**Response:**

```json
{
  "agent": {
    "scout_model": "claude-sonnet-4-6",
    "wave_model": "claude-sonnet-4-6",
    "chat_model": "claude-sonnet-4-6"
  }
}
```

**Source:** Loaded from `saw.config.json` in the repo root.

---

### `POST /api/config`

Update configuration (Scout/Wave/Chat models).

**Request:**

```json
{
  "agent": {
    "scout_model": "bedrock:claude-sonnet-4-5",
    "wave_model": "ollama:qwen2.5-coder:32b",
    "chat_model": "claude-opus-4-6"
  }
}
```

**Response:**

```json
{
  "status": "saved"
}
```

**Validation:** Model names are validated with regex `^[a-zA-Z0-9:._/-]+$` (max 200 chars) to prevent command injection.

---

## Model Name Format

Model names support provider-prefix routing:

| Prefix | Provider | Example |
|--------|----------|---------|
| `anthropic:` | Anthropic API | `anthropic:claude-opus-4-6` |
| `bedrock:` | AWS Bedrock | `bedrock:claude-sonnet-4-5` |
| `openai:` | OpenAI | `openai:gpt-4o` |
| `ollama:` | Ollama (local) | `ollama:qwen2.5-coder:32b` |
| `lmstudio:` | LM Studio (local) | `lmstudio:phi-4` |
| `cli:` | Claude Code CLI | `cli:claude-sonnet-4-6` |
| (none) | Default (Anthropic API) | `claude-sonnet-4-6` |

**Engine Mapping:** `orchestrator.parseProviderPrefix()` splits the prefix and routes to the appropriate backend.

---

## Error Responses

All endpoints return standard error JSON:

```json
{
  "error": "failed to parse IMPL doc: missing wave 1 header"
}
```

HTTP status codes:
- `400` — Bad request (invalid input, validation failure)
- `404` — Not found (IMPL doc, wave, agent)
- `500` — Internal server error (engine crash, git failure)

---

## Authentication

**Current:** None. The API is intended for localhost development only.

**Future:** API key or OAuth for remote deployments (see `scout-and-wave-web` roadmap).
