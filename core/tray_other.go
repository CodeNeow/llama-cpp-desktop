//go:build !windows

package core

import "context"

// TrayIcon 保存托盘图标字节；非 Windows 平台不使用，仅为保证 main.go 跨平台
// 编译签名一致（main.go 在 wails.Run 前无条件赋值）。
var TrayIcon []byte

// InitTray 是非 Windows 平台的 no-op 存根：macOS / Linux 无系统托盘需求
// （关闭按钮保持直接退出行为），本函数仅保证跨平台编译签名一致。
func InitTray(_ context.Context, _ []byte) {}

// QuitTray 是非 Windows 平台的 no-op 存根。
func QuitTray() {}
