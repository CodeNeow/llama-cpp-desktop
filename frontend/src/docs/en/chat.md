The "Local Chat" page is a ready-to-use chat interface talking straight to your local model — no data ever leaves the machine.

## Starting a chat

1. **Start the service** on the "API Router" page first — the chat page relies on it for inference. While the service is off, the page shows a notice card with a shortcut to the API page;
2. Back on the chat page, pick a model in the dropdown at the top (the list comes from the models the service recognized);
3. Type a message in the input box — **Enter sends, Shift+Enter makes a new line**;
4. Replies stream in token by token; when a reply finishes, its generation speed (tok/s) is shown under the message.

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
