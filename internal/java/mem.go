package java

import "runtime"

// AutoMemoryMB mirrors the 5.31 rule: size Xmx from free RAM, clamped to
// 500–1300 MB. Manual mode in the UI bypasses this.
func AutoMemoryMB() int {
	free := freeRAMMB()
	if free <= 0 {
		return 1024
	}
	// Leave some RAM for the system, same spirit as the original hint.
	use := free / 2
	if use < 500 {
		use = 500
	}
	if use > 1300 {
		use = 1300
	}
	return use
}

func freeRAMMB() int {
	switch runtime.GOOS {
	case "linux":
		return linuxMemAvailableMB()
	default:
		return 0
	}
}
