package core

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// GetWindowsVersion returns the major, minor, and build numbers of the current Windows version.
// Uses RtlGetNtVersionNumbers which works on all Windows versions without manifest requirements.
func GetWindowsVersion() (major, minor, build uint32) {
	major, minor, build = windows.RtlGetNtVersionNumbers()
	// RtlGetNtVersionNumbers returns build with high bits set; mask them off
	build &= 0xFFFF
	return major, minor, build
}

// IsWindows10OrAbove checks if running on Windows 10 or later.
// Windows 10 is major version 10.
func IsWindows10OrAbove() bool {
	major, _, _ := GetWindowsVersion()
	return major >= 10
}

// IsWindows11OrAbove checks if running on Windows 11 or later.
// Windows 11 is identified by build >= 22000.
func IsWindows11OrAbove() bool {
	major, _, build := GetWindowsVersion()
	return major >= 10 && build >= 22000
}

// WindowsVersionString returns a human-readable Windows version string.
// Examples: "Windows 10 (Build 19045)", "Windows 11 (Build 22621)"
func WindowsVersionString() string {
	major, minor, build := GetWindowsVersion()

	var name string
	switch {
	case major == 10 && build >= 22000:
		name = "Windows 11"
	case major == 10:
		name = "Windows 10"
	case major == 6 && minor == 3:
		name = "Windows 8.1"
	case major == 6 && minor == 2:
		name = "Windows 8"
	case major == 6 && minor == 1:
		name = "Windows 7"
	case major == 6 && minor == 0:
		name = "Windows Vista"
	default:
		name = fmt.Sprintf("Windows %d.%d", major, minor)
	}

	return fmt.Sprintf("%s (Build %d)", name, build)
}
