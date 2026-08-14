//go:build !windows

package core

import "context"

// InitTray 是非 Windows 平台的 no-op 存根：macOS / Linux 无系统托盘需求
// （关闭按钮保持直接退出行为），本函数仅保证跨平台编译签名一致。
func InitTray(_ context.Context, _ []byte) {}

// QuitTray 是非 Windows 平台的 no-op 存根。
func QuitTray() {}
