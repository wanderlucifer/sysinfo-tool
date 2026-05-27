package main

import (
	"syscall"
	"unsafe"
)

type SYSTEMTIME struct {
	Year         uint16
	Month        uint16
	DayOfWeek    uint16
	Day          uint16
	Hour         uint16
	Minute       uint16
	Second       uint16
	Milliseconds uint16
}

func getLocalTime() SYSTEMTIME {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetLocalTime := kernel32.NewProc("GetLocalTime")
	var st SYSTEMTIME
	procGetLocalTime.Call(uintptr(unsafe.Pointer(&st)))
	return st
}
