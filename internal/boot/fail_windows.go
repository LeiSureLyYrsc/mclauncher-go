//go:build windows

package boot

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func Fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	title, _ := syscall.UTF16PtrFromString("mclauncher")
	text, _ := syscall.UTF16PtrFromString(err.Error())
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	const mbIconError = 0x10
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbIconError)
	os.Exit(1)
}
