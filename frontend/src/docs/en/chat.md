The "Local Chat" page is a ready-to-use chat interface talking straight to your local model — no data ever leaves the machine.

## Starting a chat

1. Pick a model in the capsule at the top (clicking it opens the list of models recognized on this machine — the service does not need to be running yet);
2. Type a message into the floating input bar — **Enter sends, Shift+Enter makes a new line** — or hit the round gradient send button;
3. If the service is not running yet, sending **starts it automatically** and loads the selected model on demand before streaming begins — no trip to the "API Router" page needed (that page still offers manual start/stop and monitoring);
4. Replies stream in token by token; while generating, the send button turns into a red stop button you can hit to interrupt; when a reply finishes, its generation speed (tok/s) is shown under the message.

Before the auto-start, a guided check runs: with no usable models you are pointed to the Models page to download or import one; if the llama.cpp runtime is missing you are pointed to the "Runtime Environment" tab of the System Environment page. Fill the gap and you are ready to chat.

Sending a message first unloads every OTHER loaded model so the selected one is the only one in memory; load / unload changes show up in the task dock in real time, and models can be unloaded from there at any moment. The dock hugs the bottom-right corner by default and can be dragged to snap to either the left or right edge.

> Android note: the official Android llama-server runs in **direct mode** — one service process hosts exactly one model. Switching the chat model automatically restarts the service to load the new one (first launch is slower on phone storage, please be patient); in-memory models cannot be unloaded manually (the task dock shows no unload button) — stop the service to free resources. Desktop platforms keep router mode with on-demand loading and unloading of multiple models, unchanged.

Multimodal models (with an mmproj file) can also take images: use the paperclip button next to the input bar to attach one and let the model describe it.

## Tuning chat parameters

Click the gear icon at the right end of the top toolbar to open the parameter panel:

- **Temperature**: higher is more creative, lower is more deterministic;
- **Top P / Top K**: how wide the sampling pool is;
- **Repeat penalty**: reduces repetitive rambling;
- **Max tokens**: the length cap for a single reply;
- **System prompt**: persona or task instructions for the model; left empty, nothing is injected.

"Reset defaults" restores every parameter. Changes apply from the next request on.

## Thinking process

Reasoning models (e.g. the DeepSeek-R1 series) emit a thinking phase; the chat page folds it into a "Thinking" block you can expand, keeping only the final answer in the body — the two phases get separate speed readouts.

## Chat history

- "Clear chat" deletes the current conversation;
- Conversations are stored locally (browser local storage); closing and reopening the app keeps your last conversation.

Every request goes to the local service — nothing is uploaded to any cloud.
