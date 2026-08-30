#!/usr/bin/env bash
# Android emulator smoke test for the llama-desktop debug APK (CI smoke-android
# job, run inside the android-emulator-runner). Kept as a committed file so the
# logic is reviewable and the action's single-input `script` cannot mangle
# multi-line shell.
set -e

APK="$(ls apk/*.apk)"
echo "installing $APK"
adb install -r "$APK"
adb logcat -c
adb shell am start -W -n com.wails.app/com.wails.app.MainActivity || true

# Poll for a live process up to ~90s (Go runtime + WebView first boot).
PID=""
i=0
while [ "$i" -lt 30 ]; do
  PID="$(adb shell pidof com.wails.app | tr -d '\r' || true)"
  if [ -n "$PID" ]; then
    break
  fi
  i=$((i + 1))
  sleep 3
done
if [ -z "$PID" ]; then
  echo "::error::app process not running after launch"
  adb logcat -d | tail -120
  exit 1
fi
echo "app running with pid $PID"

# No crash may be recorded.
if adb logcat -d | grep -q "FATAL EXCEPTION"; then
  echo "::error::crash recorded in logcat"
  adb logcat -d | grep -B 5 -A 25 "FATAL EXCEPTION"
  exit 1
fi

# The storage anchor must resolve through the JNI bridge; this WARN is emitted
# by core/paths.go when it falls back to the read-only cwd-relative layout.
if adb logcat -d | grep -q "keeping cwd-relative app paths"; then
  echo "::error::Android storage anchor unresolved (JNI bridge), app fell back to read-only cwd paths"
  adb logcat -d | grep -i "llama-desktop" | tail -40
  exit 1
fi

adb exec-out screencap -p > emulator-screen.png || true
ls -la emulator-screen.png || true
echo "smoke passed"
