package main

import (
	"syscall"
	"unsafe"
)

// getDesktopPath 返回当前用户桌面的文件夹路径
// 使用 SHGetFolderPathW 函数（Windows XP 及以上版本支持）
func getDesktopPath() string {
	// CSIDL_DESKTOP = 0x0000, CSIDL_DESKTOPDIRECTORY = 0x0010
	// 使用 DESKTOPDIRECTORY 获取实际文件系统路径
	const CSIDL_DESKTOPDIRECTORY = 0x0010

	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("SHGetFolderPathW")

	// SHGFP_TYPE_CURRENT = 0
	var buf [260]uint16
	ret, _, _ := proc.Call(
		0,
		CSIDL_DESKTOPDIRECTORY,
		0, // token（0 = 当前用户）
		0, // SHGFP_TYPE_CURRENT
		uintptr(unsafe.Pointer(&buf[0])),
	)

	if ret != 0 { // S_OK = 0
		// 回退方案：尝试 CSIDL_DESKTOP
		ret, _, _ = proc.Call(
			0,
			0x0000, // CSIDL_DESKTOP
			0,
			0,
			uintptr(unsafe.Pointer(&buf[0])),
		)
		if ret != 0 {
			// 最后尝试 SHGetKnownFolderPath（Vista 及以上）
			return getDesktopPathFallback()
		}
	}

	return syscall.UTF16ToString(buf[:])
}

func getDesktopPathFallback() string {
	// 使用 SHGetKnownFolderPath（Vista 及以上）
	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("SHGetKnownFolderPath")

	// FOLDERID_Desktop = {B4BFCC3A-DB2C-424C-B029-7FE99A87C641}
	var folderID = GUID{
		Data1: 0xB4BFCC3A,
		Data2: 0xDB2C,
		Data3: 0x424C,
		Data4: [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41},
	}

	var path *uint16
	ret, _, _ := proc.Call(
		uintptr(unsafe.Pointer(&folderID)),
		0,
		0,
		uintptr(unsafe.Pointer(&path)),
	)

	if ret != 0 || path == nil {
		// 最终回退路径
		return "C:\\Users\\Public\\Desktop"
	}

	result := ptrToString(path)

	// 释放内存
	procCoTaskMemFree := syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree")
	if procCoTaskMemFree.Find() == nil {
		procCoTaskMemFree.Call(uintptr(unsafe.Pointer(path)))
	}

	return result
}

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

func ptrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	chars := []uint16{}
	for i := 0; ; i++ {
		c := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i*2)))
		if c == 0 {
			break
		}
		chars = append(chars, c)
	}
	return syscall.UTF16ToString(chars)
}
