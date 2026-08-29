"API route mode" (headless mode) turns the app into a pure background service: **the graphical interface closes and only the system tray plus llama-server remain**, continuing to serve the OpenAI-compatible API. Ideal when this machine acts as an always-on inference server for your home or studio.

## Turning it on and off

- **Enable**: flip the switch under "Preferences → API Route Mode". The app relaunches itself into headless mode and the window closes;
- **Return to the GUI**: right-click the tray icon → "Show Main Window"; the app relaunches once more, this time back into the full interface.

Prerequisites:

- **Windows** only;
- The **system tray** must be enabled first — the tray menu is the only way back from headless mode;
- Developers: this feature is unavailable under `wails dev` (the switch restarts the process and kills the dev session) — test it with a release build.

## Switching between GUI and headless never drops the service

Loading a model can take tens of seconds; if a mode switch restarted the service, every connected API client would be interrupted. The app avoids that with a **handover mechanism**:

1. On a mode switch, the exiting process writes the identity of the **still-running llama-server** (process ID, port, log path) into a handover record file;
2. The current process exits; the new process probes the record and, once it confirms the service is healthy (the port answers, the process is alive), **adopts** it instead of starting a new one;
3. Throughout the switch, llama-server never stops — **the model is not reloaded and API calls are not interrupted**; only the host process changes.

After adoption, the new process keeps tailing the service log, and monitor metrics such as token speed keep working. If the handover record is stale or the service has died, the app falls back to a fresh start rather than ever adopting an unrelated process.

## Behavior in headless mode

- The tray icon stays resident; its menu offers "Show Main Window" and quit;
- llama-server keeps serving in the background, and download tasks resume from the persisted queue;
- There is no window to show errors in — if llama-server fails to start while headless, a **native system message box** appears while the app itself keeps running, preserving the tray as the way back;
- Returning to the GUI from the tray adopts the service seamlessly again, exactly as before.

## When headless mode fits

- The machine mostly acts as an inference server and the window is rarely looked at;
- You want an always-on background service for other devices on the LAN (remember to switch the access scope to LAN and set an API key in Preferences);
- You do not want the app occupying the taskbar.
