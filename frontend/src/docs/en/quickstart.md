Three steps to run a local LLM: **install the runtime → download a model → start the service**. Each step takes a click or two, no command line required.

## Step 1: Install the llama.cpp runtime

llama.cpp is the inference engine that actually runs the model; the app launches the local service through it.

1. Open the System Environment page in the sidebar and find the "Runtime Environment" tab.
2. If it shows "Not found", click "Download llama.cpp" — the app fetches the latest release and extracts it into the download path.
3. When the download finishes, the status flips to "Installed" with the version and install path shown.

If you already have llama.cpp on disk (a directory containing the llama-server binary), click "Custom" and pick that directory instead — no download needed.

## Step 2: Download a model

Models are `.gguf` weight files; get them from the "Download" tab of the Models page:

1. Type a model name (e.g. `Qwen`, `DeepSeek`) into the search box.
2. Click a search result to open the model detail page.
3. Tick the `.gguf` files you want (if unsure, pick a mid-size Q4 quantization), then click "Download Selected".
4. Progress shows in the bottom-right task card and the download manager, with pause, resume and resumable transfers.

Finished models appear automatically on the "My Models" tab of the Models page. On Chinese networks, switching the download source to the HF Mirror or ModelScope in Settings first usually gives steadier speeds.

## Step 3: Start the service

1. Open the "API Router" page and click "Start Server".
2. Once the status shows "Running", the service address appears on the page (default `http://127.0.0.1:8080`) and any OpenAI-compatible client can connect to the endpoint (see the curl example in the "API Service" section).
3. Only here to chat? This step is skippable: open the "Local Chat" page and send a message — the app starts the service automatically and loads the selected model on demand.

## The quick-start checklist on Home

On first use, the System Environment page shows a "Quick Start" checklist card at the top of the System Info tab:

- The card lists the three steps above; unfinished steps carry a "Set up" button that jumps straight to the relevant page;
- Each step gets ticked automatically as you complete it; once all are done the card hides itself;
- "Don't show again" dismisses the card for good.

The sidebar footer also carries a system status dot: it turns green ("System Ready") once llama.cpp is installed and at least one model exists.
