//go:build windows

package core

import (
	"fmt"
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ─── Headless llama-server start-failure alert ────────────────────
//
// fyne.io/systray exposes no balloon/notification API on Windows, and the
// headless process has neither a window nor a console: without an explicit
// alert a failed llama-server start would leave the tray looking normal
// while the OpenAI API is dead. A native MessageBox is the one channel
// guaranteed to reach the user right after they flip the mode switch.

var (
	user32Dll       = windows.NewLazySystemDLL("user32.dll")
	messageBoxWProc = user32Dll.NewProc("MessageBoxW")
)

// MessageBox button/message box styles (winuser.h).
const (
	mbOK            = 0x00000000
	mbIconWarning   = 0x00000030
	mbSetForeground = 0x00010000
	mbTopmost       = 0x00040000
)

// defaultHeadlessServerAlert is the Windows implementation of the headless
// start-failure notification: a topmost warning dialog spells out the error
// and points at the tray menu as the way back to the GUI. MessageBoxW blocks
// until dismissed, so it runs on its own goroutine — the headless lifecycle
// (tray + downloads) keeps running regardless of the dialog.
func defaultHeadlessServerAlert(err error) {
	go func() {
		text := fmt.Sprintf(tr("llama-server 启动失败：%v\n\n后台模式将继续运行（仅托盘，API 当前不可用）。可通过托盘菜单“显示主窗口”返回界面，检查 llama.cpp 安装或模型目录后重试。",
			"llama-server failed to start: %v\n\nHeadless mode keeps running (tray only, the API is currently unavailable). Use the tray menu \"Show Main Window\" to return to the GUI, check the llama.cpp installation or the model directory, then try again."), err)
		textPtr, errPtr := windows.UTF16PtrFromString(text)
		if errPtr != nil {
			log.Printf("[ERROR] build headless alert text: %v", errPtr)
			return
		}
		titlePtr, errTitle := windows.UTF16PtrFromString("Llama Desktop")
		if errTitle != nil {
			log.Printf("[ERROR] build headless alert title: %v", errTitle)
			return
		}
		messageBoxWProc.Call(
			0,
			uintptr(unsafe.Pointer(textPtr)),
			uintptr(unsafe.Pointer(titlePtr)),
			mbOK|mbIconWarning|mbSetForeground|mbTopmost,
		)
	}()
}
