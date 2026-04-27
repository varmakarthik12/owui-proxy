# owui-proxy

[![Build](https://github.com/varmakarthik12/owui-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/varmakarthik12/owui-proxy/actions)
[![Release](https://img.shields.io/github/v/release/varmakarthik12/owui-proxy)](https://github.com/varmakarthik12/owui-proxy/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/varmakarthik12/owui-proxy)](go.mod)

**Local Ollama-compatible API proxy for Open WebUI** — use any Ollama client with all your Open WebUI models.

---

## Overview

`owui-proxy` is a lightweight, production-grade CLI tool that acts as a **local Ollama-compatible API server**. It accepts requests in Ollama's native API format, translates them to Open WebUI's OpenAI-compatible API format, calls Open WebUI, and translates the response back to Ollama format.

**Clients never know they are talking to Open WebUI.** To them, `owui-proxy` looks and behaves exactly like a native Ollama server.

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  Any Ollama Client                                                                                      │
│  (ollama CLI, GitHub Copilot (Bring Your Own Model), Continue.dev, Aider, LM Studio, scripts)           │
└────────────────────────┬────────────────────────────────────────────────────────────────────────────────┘
                         │
                         │  Ollama REST API (port 11434)  /api/*
                         │  OpenAI REST API (port 11434)  /v1/*
                         │  NDJSON / SSE streaming
                         ▼
              ┌─────────────────┐
              │   owui-proxy    │
              │   :11434        │
              └────────┬────────┘
                       │  ← translates Ollama ↔ OpenAI formats
                       │  ← proxies /v1/* directly
                       │  ← injects Authorization header
                       │
                       │  OpenAI-compatible REST API
                       │  SSE streaming
                       ▼
              ┌─────────────────┐
              │   Open WebUI    │
              │   /api/...      │
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────────────────────────────┐
              │  All Models                             │
              │  Ollama, Claude, GPT-4o, Gemini,        │
              │  Pipelines, Functions, ...              │
              └─────────────────────────────────────────┘
```

---

## Why owui-proxy?

- **Use any Ollama tool with any Open WebUI model** — GitHub Copilot (Bring Your Own Model), Continue.dev, Aider, LM Studio, shell scripts, and more all speak the Ollama API. Open WebUI exposes all models (not just local Ollama ones) via its OpenAI-compatible API. `owui-proxy` bridges the two.

- **Also speaks the OpenAI `/v1/` API** — tools that use the OpenAI wire format (GitHub Copilot's "Bring Your Own Model" in OpenAI mode, LiteLLM clients, etc.) can also point directly at the proxy via `POST /v1/chat/completions` and `GET /v1/models`.

- **Access all your models in one place** — local Ollama models, Claude, GPT-4o, Gemini, custom Pipelines — everything your Open WebUI instance can reach, now accessible through any Ollama client.

- **Transparent auth injection** — clients don't need to handle authentication. The proxy injects the Bearer token on every upstream request.

- **Localhost-only by default** — your API key stays safe. The proxy binds to `127.0.0.1` unless you explicitly ask for network exposure.

- **Zero configuration on clients** — just point `OLLAMA_HOST` at the proxy. Everything works.

- **Real streaming** — SSE streams from Open WebUI are translated to Ollama's NDJSON format in real time. No buffering.

---

## Requirements

- **Go 1.22+** (to build from source)
- OR a **prebuilt binary** (no Go needed)

---

## Installation

### Go install

```bash
go install github.com/varmakarthik12/owui-proxy@latest
```

### Homebrew (macOS/Linux)

The formula is served directly from this repository — no separate tap repo needed.

```bash
brew tap varmakarthik12/owui-proxy https://github.com/varmakarthik12/owui-proxy
brew install owui-proxy
```

### Binary — Linux amd64

```bash
curl -Lo owui-proxy \
  https://github.com/varmakarthik12/owui-proxy/releases/latest/download/owui-proxy_linux_amd64
chmod +x owui-proxy
sudo mv owui-proxy /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/varmakarthik12/owui-proxy
cd owui-proxy
make build
# binary: ./bin/owui-proxy
```

---

## Quick Start

```bash
# Set your Open WebUI endpoint and API key
export OWUI_ENDPOINT="https://your-openwebui.example.com"
export OWUI_TOKEN="sk-your-key-here"

# Start the proxy
owui-proxy serve

# --- In another terminal ---

# Verify it's running
curl http://localhost:11434/api/version
# {"version":"0.6.5"}

# List all models from Open WebUI
curl http://localhost:11434/api/tags
# {"models":[...all your Open WebUI models...]}

# Chat with a model
curl http://localhost:11434/api/chat -d '{
  "model": "llama3.1",
  "messages": [{"role": "user", "content": "Hello!"}],
  "stream": false
}'
```

---

## Full Configuration Reference

### Required

| Flag         | Env Var         | Description                                   |
| ------------ | --------------- | --------------------------------------------- |
| `--endpoint` | `OWUI_ENDPOINT` | Base URL of your Open WebUI instance          |
| `--token`    | `OWUI_TOKEN`    | Bearer token (Open WebUI API key). Prefer env var — CLI flag is visible in `ps aux` |

### Server

| Flag            | Env Var            | Default     | Description                                     |
| --------------- | ------------------ | ----------- | ----------------------------------------------- |
| `--port`        | `OWUI_PROXY_PORT`  | `11434`     | Local port to listen on                         |
| `--listen-addr` | `OWUI_LISTEN_ADDR` | `127.0.0.1` | Bind address (localhost-only by default)         |
| `--bind-all`    | `OWUI_BIND_ALL`    | `false`     | Bind to `0.0.0.0` — exposes on the network      |
| `--tls-cert`    | `OWUI_TLS_CERT`    | —           | TLS cert file (enables HTTPS; requires `--tls-key`) |
| `--tls-key`     | `OWUI_TLS_KEY`     | —           | TLS key file (requires `--tls-cert`)            |
| `--rate-limit`  | `OWUI_RATE_LIMIT`  | `0`         | Max requests/sec per client IP (0 = disabled)   |
| `--max-body-size` | `OWUI_MAX_BODY_SIZE` | `104857600` | Max request body in bytes (100 MB)          |
| `--timeout`     | `OWUI_TIMEOUT`     | `300s`      | HTTP client timeout for upstream requests       |

### Translation

| Flag           | Env Var           | Default | Description                                              |
| -------------- | ----------------- | ------- | -------------------------------------------------------- |
| `--api-prefix` | `OWUI_API_PREFIX` | `/api`  | Open WebUI API path prefix (change if behind a subpath) |
| `--mock-version` | `OWUI_MOCK_VERSION` | `0.6.5` | Ollama version string returned by `GET /api/version` |

### Model behaviour

| Flag                           | Env Var                            | Default        | Description |
| ------------------------------ | ---------------------------------- | -------------- | ----------- |
| `--model-prefix`               | `OWUI_MODEL_PREFIX`                | `owui-proxy/`  | Prefix added to all model IDs in `/api/tags`, `/v1/models`. Stripped before forwarding requests to Open WebUI. Set to empty string to disable. |
| `--no-default-capabilities`    | `OWUI_NO_DEFAULT_CAPABILITIES`     | `false`        | Disable auto-appending `completion`, `vision`, `tools`, `thinking` capabilities to every model in `/api/show` responses. |
| `--default-context-length`     | `OWUI_DEFAULT_CONTEXT_LENGTH`      | `262144` (256K) | Minimum context window size (tokens) to report in `/api/show` `model_info`. Any upstream `*.context_length` value smaller than this is raised to this value; if no context key exists, `general.context_length` is injected. |
| `--no-context-length-override` | `OWUI_NO_CONTEXT_LENGTH_OVERRIDE`  | `false`        | Disable the context length floor — leave `model_info` exactly as Open WebUI reports it. |

### Logging

| Flag            | Env Var          | Default  | Description                                  |
| --------------- | ---------------- | -------- | -------------------------------------------- |
| `--log-level`   | `OWUI_LOG_LEVEL` | `info`   | Log verbosity: `debug`, `info`, `warn`, `error` |
| `--log-format`  | `OWUI_LOG_FORMAT`| `text`   | Log format: `text` or `json`                 |
| `--no-color`    | `NO_COLOR`       | `false`  | Disable coloured terminal output             |

---

## Usage Examples

### Minimal

```bash
owui-proxy serve --endpoint https://openwebui.example.com --token sk-abc123
```

### Custom port + version mock

```bash
owui-proxy serve --endpoint https://openwebui.example.com --token sk-abc123 --port 11435 --mock-version 0.5.1
```

### Network-exposed with TLS + rate limit

```bash
owui-proxy serve --endpoint https://openwebui.example.com --token sk-abc123 --bind-all --tls-cert /etc/ssl/proxy.crt --tls-key /etc/ssl/proxy.key --rate-limit 20
```

### Debug logging to see translation in action

```bash
owui-proxy serve --endpoint https://openwebui.example.com --token sk-abc123 --log-level debug --log-format json
```

### Custom API prefix (if Open WebUI is behind a subpath)

```bash
owui-proxy serve --endpoint https://openwebui.example.com --api-prefix /internal/api --token sk-abc123
```

### Disable model prefix (expose raw OWUI model IDs)

```bash
owui-proxy serve --endpoint https://openwebui.example.com --token sk-abc123 --model-prefix ""
```

### Disable context length override (trust upstream values as-is)

```bash
owui-proxy serve --endpoint https://openwebui.example.com --token sk-abc123 --no-context-length-override
```

---

## Integrations

### Ollama CLI

```bash
# Set the Ollama host to point at owui-proxy
export OLLAMA_HOST=http://localhost:11434

# Or use an alias
alias ollama='OLLAMA_HOST=http://localhost:11434 ollama'

# Now use ollama normally — it talks to Open WebUI via the proxy
ollama list              # lists all Open WebUI models
ollama run llama3.1      # runs via Open WebUI
```

### Aider

```bash
aider --model ollama/claude-sonnet-4 \
      --ollama-api-base http://localhost:11434
```

### Continue.dev

Add to `~/.continue/config.json`:

```json
{
  "models": [
    {
      "title": "All OWUI Models",
      "provider": "ollama",
      "model": "llama3.1",
      "apiBase": "http://localhost:11434"
    }
  ]
}
```

### LM Studio

Set the Ollama-compatible server URL to `http://localhost:11434`.

### GitHub Copilot (Bring Your Own Model)

GitHub Copilot now supports local models via Ollama. You can use `owui-proxy` to connect Copilot to _any_ model in your Open WebUI instance.
Follow these steps in VS Code:

1. Open the Copilot Chat sidebar (top-right icon)

2. Click the model selector dropdown → Manage Models

3. Click Add Models → select Ollama

4. All models from your Open WebUI instance will appear (prefixed with `owui-proxy/` by default) — click Unhide on any model to activate it

5. Select Local at the bottom of the Copilot Chat panel to route requests through owui-proxy

   > **Tip:** The `github.copilot.chat.byok.ollamaEndpoint` setting lets you point Copilot at a non-default address — useful inside devcontainers or when running owui-proxy on a custom port. Set it to `http://host.docker.internal:11434` when working inside Docker.

### GitHub Copilot via OpenAI-compatible endpoint

If your Copilot plan or extension version uses the OpenAI wire format directly, point it at the proxy's `/v1/` base URL:

```json
// VS Code settings.json
{
  "github.copilot.advanced": {
    "apiUrl": "http://localhost:11434/v1"
  }
}
```

Models will appear as `owui-proxy/<model-id>` in the picker. The proxy strips the prefix before forwarding to Open WebUI.

### Shell scripts

```bash
# Any script using curl against the Ollama API works unchanged
curl -s http://localhost:11434/api/generate -d '{
  "model": "gpt-4o",
  "prompt": "Write a haiku about proxies",
  "stream": false
}' | jq -r '.response'
```

---

## Supported Endpoints

### Ollama API (`/api/*`)

| Endpoint               | Status                    | Open WebUI Call                   |
| ---------------------- | ------------------------- | --------------------------------- |
| `GET /`                | ✅ Health check           | Returns `"Ollama is running"`     |
| `GET /api/version`     | ✅ Mocked                 | Returns `--mock-version` value    |
| `GET /api/tags`        | ✅ Translated             | `GET /api/models`                 |
| `POST /api/chat`       | ✅ Translated + streaming | `POST /api/chat/completions`      |
| `POST /api/generate`   | ✅ Translated + streaming | `POST /api/chat/completions`      |
| `POST /api/show`       | ✅ Translated             | `GET /api/models`                 |
| `POST /api/embeddings` | ✅ Translated             | `POST /api/embeddings`            |
| `POST /api/embed`      | ✅ Translated             | `POST /api/embeddings`            |
| `GET /api/ps`          | ✅ Mocked                 | Returns empty list                |
| `POST /api/pull`       | ⚠️ 501                    | Not supported                     |
| `POST /api/push`       | ⚠️ 501                    | Not supported                     |
| `DELETE /api/delete`   | ⚠️ 501                    | Not supported                     |
| `POST /api/copy`       | ⚠️ 501                    | Not supported                     |
| `POST /api/create`     | ⚠️ 501                    | Not supported                     |
| `/api/blobs/*`         | ⚠️ 501                    | Not supported                     |

### OpenAI-compatible API (`/v1/*`)

| Endpoint                      | Status                    | Open WebUI Call              |
| ----------------------------- | ------------------------- | ---------------------------- |
| `GET /v1/models`              | ✅ Translated             | `GET /api/models`            |
| `POST /v1/chat/completions`   | ✅ Proxied + streaming    | `POST /api/chat/completions` |

The `/v1/` endpoints speak the standard OpenAI wire format — JSON for non-streaming, SSE (`data: {...}\n\n`) for streaming. Tool calling, function definitions, and all OpenAI request fields are supported.

> **Model IDs and the prefix**: By default `owui-proxy` adds the prefix `owui-proxy/` to all model IDs it exposes (both `/api/tags` and `/v1/models`). It strips this prefix before forwarding any request to Open WebUI. This prevents tools from treating the proxy's models as real OpenAI/Claude endpoints. Configure with `--model-prefix` or set to empty string to disable.

---

## Tool Calling & Full Capability Support

`owui-proxy` fully supports Ollama's tool calling, thinking/reasoning, and rich model metadata features:

- **Tool calling**: `/api/chat` forwards `tools`, `tool_choice`, and `think` fields to Open WebUI. Tool calls from the model are returned in `message.tool_calls` for both streaming and non-streaming responses.
- **Thinking/reasoning**: When the model provides reasoning content (e.g. DeepSeek-R1), it is mapped to `message.thinking` in responses.
- **Images**: Multi-modal messages with images are forwarded as base64-encoded `image_url` content parts.
- **Format**: Both `"json"` string format and JSON Schema objects are supported via `format`.
- **Live model metadata**: Model metadata (`family`, `parameter_size`, `quantization_level`, `size`, `digest`, `capabilities`, `model_info`) is sourced from Open WebUI's `GET /api/models` response — specifically the `ollama` sub-object for Ollama-backed models and `info.meta` for OWUI-level capabilities. Never hardcoded or inferred.
- **Capabilities**: All capabilities (`vision`, `tools`, `thinking`, `embedding`) are reflected accurately in `/api/show` and `/api/tags`, sourced from both the `ollama.capabilities` array and `info.meta.capabilities` map in the OWUI model data.

### Tool calling example

```bash
# Request with tools
curl http://localhost:11434/api/chat -d '{
  "model": "llama3.1",
  "messages": [{"role": "user", "content": "What is the weather in Paris?"}],
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "description": "Get the weather for a location",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string", "description": "City name"}
        },
        "required": ["location"]
      }
    }
  }],
  "stream": false
}'

# Response with tool_calls
# {
#   "model": "llama3.1",
#   "message": {
#     "role": "assistant",
#     "content": "",
#     "tool_calls": [{
#       "function": {
#         "name": "get_weather",
#         "arguments": {"location": "Paris"}
#       }
#     }]
#   },
#   "done": true,
#   "done_reason": "tool_calls"
# }
```

---

## Security

### Localhost-only by default

The proxy binds to `127.0.0.1` by default. Only processes on your machine can reach it. This keeps your Open WebUI API key safe.

### Token masking

The API token is **never** logged in full. All log output masks it as `sk-****`. The token is also never reflected in error responses.

### `--bind-all` warning

When you use `--bind-all` or set `OWUI_BIND_ALL=true`, the proxy binds to `0.0.0.0` and is accessible from the network. The proxy will print a warning at startup. **Use TLS and rate limiting** if exposing on a network.

### CLI token flag warning

If you pass `--token` as a CLI flag, it will be visible in process listings (`ps aux`). The proxy prints a warning recommending the `OWUI_TOKEN` environment variable instead.

### TLS

Enable HTTPS on the local proxy by providing `--tls-cert` and `--tls-key`. This is recommended if exposing the proxy on a network.

### Rate limiting

Enable per-IP rate limiting with `--rate-limit <requests-per-second>`. Disabled by default (`0`). Uses a token bucket algorithm.

### Request body size limit

Request bodies are capped at 100MB by default. Configure with `--max-body-size`.

---

## Run as a Service

### macOS launchd

Save as `~/Library/LaunchAgents/com.owui.proxy.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.owui.proxy</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/owui-proxy</string>
    <string>serve</string>
    <string>--endpoint</string>
    <string>https://your-owui.example.com</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>OWUI_TOKEN</key>
    <string>sk-your-key-here</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/owui-proxy.stdout.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/owui-proxy.stderr.log</string>
</dict>
</plist>
```

Load with:

```bash
launchctl load ~/Library/LaunchAgents/com.owui.proxy.plist
```

### Linux systemd

Save as `/etc/systemd/system/owui-proxy.service`:

```ini
[Unit]
Description=owui-proxy — Ollama API shim for Open WebUI
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/owui-proxy serve --endpoint https://your-owui.example.com
EnvironmentFile=/etc/owui-proxy/env
Restart=always
RestartSec=5
User=owui-proxy
Group=owui-proxy

[Install]
WantedBy=multi-user.target
```

Create the environment file at `/etc/owui-proxy/env`:

```
OWUI_TOKEN=sk-your-key-here
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now owui-proxy
sudo journalctl -u owui-proxy -f
```

### Windows (NSSM)

Use [NSSM](https://nssm.cc/) to run as a Windows service:

```powershell
nssm install owui-proxy "C:\path\to\owui-proxy.exe" "serve --endpoint https://your-owui.example.com"
nssm set owui-proxy AppEnvironmentExtra "OWUI_TOKEN=sk-your-key-here"
nssm start owui-proxy
```

---

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run checks:
   ```bash
   make fmt
   make vet
   make lint
   make test
   ```
5. Commit with a descriptive message
6. Push and open a Pull Request

### Code style

- Follow standard Go conventions
- Use `go vet` and `golangci-lint`
- Write tests for new functionality
- Keep handlers as standalone functions, not methods on a god struct

---

## License

MIT License — see [LICENSE](LICENSE) for details.
