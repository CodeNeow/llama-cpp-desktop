Three steps to run a local LLM: **install the runtime → download a model → start the service**. Each step takes a click or two, no command line required.

## Step 1: Install the llama.cpp runtime

llama.cpp is the inference engine that actually runs the model; the app launches the local service through it.

1. Open the "Runtime" page in the sidebar.
2. If it shows "Not found", click "Download llama.cpp" — the app fetches the latest release and extracts it into the download path.
3. When the download finishes, the status flips to "Installed" with the version and install path shown.

If you already have llama.cpp on disk (a directory containing the llama-server binary), click "Custom" and pick that directory instead — no download needed.

## Step 2: Download a model

Models are `.gguf` weight files; get them from the "Downloads" page:

1. Type a model name (e.g. `Qwen`, `DeepSeek`) into the search box.
2. Click a search result to open the model detail page.
3. Tick the `.gguf` files you want (if unsure, pick a mid-size Q4 quantization), then click "Download Selected".
4. Progress shows in the bottom-right task card and the download manager, with pause, resume and resumable transfers.

Finished models appear automatically on the "Models" page. On Chinese networks, switching the download source to the HF Mirror or ModelScope in Settings first usually gives steadier speeds.

## Step 3: Start the service

1. Open the "API Router" page and click "Start Server".
2. Once the status shows "Running", the service address appears on the page (default `http://127.0.0.1:8080`).
3. Now you can:
   - chat with the model right on the "Local Chat" page;
   - connect any OpenAI-compatible client to the service (see the curl example in the "API Service" section).

## The quick-start checklist on Home

On first use, the "System Info" page shows a "Quick Start" checklist card at the top:

- The card lists the three steps above; unfinished steps carry a "Set up" button that jumps straight to the relevant page;
- Each step gets ticked automatically as you complete it; once all are done the card hides itself;
- "Don't show again" dismisses the card for good.

The sidebar footer also carries a system status dot: it turns green ("System Ready") once llama.cpp is installed and at least one model exists.
