The "Local Chat" page is a ready-to-use chat interface talking straight to your local model — no data ever leaves the machine.

## Starting a chat

1. Pick a model in the dropdown at the top (the list comes from the models recognized on this machine — the service does not need to be running yet);
2. Type a message in the input box — **Enter sends, Shift+Enter makes a new line**;
3. If the service is not running yet, sending **starts it automatically** and loads the selected model on demand before streaming begins — no trip to the "API Router" page needed (that page still offers manual start/stop and monitoring);
4. Replies stream in token by token; when a reply finishes, its generation speed (tok/s) is shown under the message.

Before the auto-start, a guided check runs: with no usable models you are pointed to the Models page to download or import one; if the llama.cpp runtime is missing you are pointed to the "Runtime Environment" tab of the System Environment page. Fill the gap and you are ready to chat.

Sending a message first unloads every OTHER loaded model so the selected one is the only one in memory; load / unload changes show up in the task dock in real time, and models can be unloaded from there at any moment.

Multimodal models (with an mmproj file) can also take images: use the paperclip button next to the input box to attach one and let the model describe it.

## Tuning chat parameters

Click the gear icon in the toolbar to open the parameter panel:

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
