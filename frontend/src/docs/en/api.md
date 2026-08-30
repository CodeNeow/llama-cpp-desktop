The "API Router" page starts, stops and monitors the llama-server service (token speed, uptime and the service log). Once running it exposes an **OpenAI-compatible** endpoint that any client speaking that protocol can use.

## Start, stop, restart

- **Start Server**: scans the model directories, generates presets from each model's parameters, then launches llama-server. Starting fails with an error when the model directories contain no GGUF files at all — download a model first;
- **Stop Server**: terminates the llama-server process, freeing VRAM and memory;
- **Restart**: apply new model parameters or server configuration by restarting.

The status light shows the service address (default `http://127.0.0.1:8080`) and uptime. "Available Models" below lists the models recognized in the model directories.

## Server parameters

Click the gear icon to open the parameter panel:

| Parameter | Default | Meaning |
| --- | --- | --- |
| Port | 8080 | Service listening port, range 1024–65535 |
| Max Concurrent Models | 1 | How many models stay loaded in VRAM/memory at once |
| Prompt Cache (MiB) | 8192 | Prompt cache kept in system memory |

The panel is locked while the server is running ("stop it to modify") so configuration cannot change underneath a live service.

## Monitoring metrics

The lower half of the page refreshes every second:

- **Server Log**: llama-server's raw output — the first place to look when troubleshooting;
- **Token Speed**: prompt processing speed (prefill — how fast your input is ingested) and generation speed (decode — how fast the model writes); thinking and answering both count as decode. A decode-speed trend chart for the last 60s sits below.

## Calling the service from code

The service speaks the OpenAI Chat Completions protocol, e.g. with curl:

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"model-name\",
    \"messages\": [
      { \"role\": \"user\", \"content\": \"Hello, introduce yourself\" }
    ]
  }"
```

In third-party clients (chat frontends, IDE plugins, ...) set the API base URL to `http://127.0.0.1:8080/v1`; the API key can be any placeholder while no authentication is configured.

## LAN access and the API key

Both live on the "Preferences" page and take effect the next time the service starts:

- **Server Access Scope**: `Local (127.0.0.1)` restricts access to this machine; `LAN (0.0.0.0)` lets other devices on the same network connect — use this machine's LAN IP as the service address then;
- **API Key**: when set, every inference request must carry this Bearer token:

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{ \"messages\": [ { \"role\": \"user\", \"content\": \"Hello\" } ] }"
```

When exposing the service to the LAN, setting an API key as well is strongly recommended.
