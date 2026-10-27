//go:build !linux

package java

func linuxMemAvailableMB() int { return 0 }
