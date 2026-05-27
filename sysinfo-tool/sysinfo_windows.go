package main

import (
	"fmt"
	"syscall"
	"unsafe"
	"strings"
)

func gatherSystemInfo() {
	g_cpuModel = getCPUInfo()
	g_manufactureDate = getManufactureDate()
	g_osInfo = getOSInfo()
	g_installDate = getOSInstallDate()
	g_browserVer = getBrowserVersion()
	g_ipAddr = getIPAddress()
	g_macAddr = getMACAddress()
	g_diskSN = getDiskSerialRegistry()
}

// ===== 操作系统信息 =====
func getOSInfo() string {
	type OSVERSIONINFOW struct {
		DwOSVersionInfoSize uint32
		DwMajorVersion      uint32
		DwMinorVersion      uint32
		DwBuildNumber       uint32
		DwPlatformId        uint32
		SzCSDVersion        [128]uint16
	}

	var verInfo OSVERSIONINFOW
	verInfo.DwOSVersionInfoSize = uint32(unsafe.Sizeof(verInfo))

	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procRtlGetVersion := ntdll.NewProc("RtlGetVersion")
	ret, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&verInfo)))
	if ret != 0 {
		return "无法获取"
	}

	major := verInfo.DwMajorVersion
	minor := verInfo.DwMinorVersion
	build := verInfo.DwBuildNumber

	var osName string
	switch {
	case major == 5 && minor == 0:
		osName = "Windows 2000"
	case major == 5 && minor == 1:
		osName = "Windows XP"
	case major == 5 && minor == 2:
		osName = "Windows Server 2003"
	case major == 6 && minor == 0:
		osName = "Windows Vista / Server 2008"
	case major == 6 && minor == 1:
		osName = "Windows 7 / Server 2008 R2"
	case major == 6 && minor == 2:
		osName = "Windows 8 / Server 2012"
	case major == 6 && minor == 3:
		osName = "Windows 8.1 / Server 2012 R2"
	case major == 10 && minor == 0 && build < 22000:
		osName = "Windows 10"
	case major == 10 && minor == 0 && build >= 22000:
		osName = "Windows 11"
	default:
		osName = fmt.Sprintf("Windows %d.%d", major, minor)
	}

	productName := readRegistryStr("SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion", "ProductName")
	if productName != "" {
		osName = productName
	}

	spVersion := syscall.UTF16ToString(verInfo.SzCSDVersion[:])
	if nullIdx := strings.IndexRune(spVersion, 0); nullIdx >= 0 {
		spVersion = spVersion[:nullIdx]
	}

	arch := "32位"
	is64Bit := is64BitOS()
	if is64Bit {
		arch = "64位"
	}

	result := fmt.Sprintf("%s (版本 %d.%d, 构建 %d)", osName, major, minor, build)
	if spVersion != "" {
		result += " " + spVersion
	}
	result += " [" + arch + "]"

	return result
}

func is64BitOS() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procIsWow64 := kernel32.NewProc("IsWow64Process")
	if procIsWow64.Find() == nil {
		var wow64 uint32
		currentProcess, _, _ := kernel32.NewProc("GetCurrentProcess").Call()
		ret, _, _ := procIsWow64.Call(currentProcess, uintptr(unsafe.Pointer(&wow64)))
		if ret != 0 && wow64 != 0 {
			return true
		}
		// 如果不是 WOW64，检查指针大小
		if unsafe.Sizeof(uintptr(0)) == 8 {
			return true
		}
		return false
	}
	return unsafe.Sizeof(uintptr(0)) == 8
}

// ===== 系统安装时间 =====
func getOSInstallDate() string {
	// 尝试多个注册表位置
	// 重要提示：对于在64位系统上运行的32位程序，WOW64会将 SOFTWARE 重定向到 WOW6432Node
	// 必须使用 KEY_WOW64_64KEY 来访问64位视图，那里才有真实的 InstallDate
	installDate := readRegistryDword64("SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion", "InstallDate")
	if installDate == 0 {
		installDate = readRegistryDword("SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion", "InstallDate")
	}
	if installDate == 0 {
		installDate = readRegistryDword("SOFTWARE\\WOW6432Node\\Microsoft\\Windows NT\\CurrentVersion", "InstallDate")
	}
	if installDate == 0 {
		installDate = readRegistryDword("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Installer", "InstallDate")
	}
	if installDate != 0 {
		t := unixTimeToLocalTime(uint64(installDate))
		return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
			t.Year, t.Month, t.Day, t.Hour, t.Minute, t.Second)
	}

	// 尝试读取 InstallTime（某些 Windows 10+ 版本以 FILETIME 格式存储）
	installTime := readRegistryInt64("SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion", "InstallTime")
	if installTime != 0 {
		var ft, lft int64
		ft = installTime
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		procFileTimeToLocalFileTime := kernel32.NewProc("FileTimeToLocalFileTime")
		procFileTimeToSystemTime := kernel32.NewProc("FileTimeToSystemTime")
		procFileTimeToLocalFileTime.Call(uintptr(unsafe.Pointer(&ft)), uintptr(unsafe.Pointer(&lft)))
		var st SYSTEMTIME
		procFileTimeToSystemTime.Call(uintptr(unsafe.Pointer(&lft)), uintptr(unsafe.Pointer(&st)))
		return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
			st.Year, st.Month, st.Day, st.Hour, st.Minute, st.Second)
	}

	// 尝试从注册表读取字符串格式（某些系统以不同方式存储）
	installDateStr := readRegistryStr("SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion", "InstallDate")
	if installDateStr == "" {
		installDateStr = readRegistryStr("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Installer", "InstallDate")
	}

	// 检查 Setup 键中的首次启动标记
	setupBoot := readRegistryStr("SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Setup", "SystemStartFirstBoot")
	if setupBoot != "" {
		return "请查看系统安装时间（首次启动）"
	}

	return "无法获取"
}

// ===== CPU 信息 =====
func getCPUInfo() string {
	// HKEY_LOCAL_MACHINE\HARDWARE\DESCRIPTION\System\CentralProcessor\0
	// ProcessorNameString 例如 "12th Gen Intel(R) Core(TM) i7-12700"
	cpuName := readRegistryStr("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorNameString")
	if cpuName == "" {
		cpuName = readRegistryStr64("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorNameString")
	}
	if cpuName == "" {
		return "无法获取"
	}
	return cpuName
}

// ===== 出厂日期（优先读取CPU相关日期，回退到BIOS日期） =====
func getManufactureDate() string {
	// 尝试多种方式获取CPU/系统日期

	// 方式1：尝试读取 ProcessorReleaseDate（可能存在于部分OEM系统中）
	cpuDate := readRegistryStr("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorReleaseDate")
	if cpuDate == "" {
		cpuDate = readRegistryStr64("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorReleaseDate")
	}
	if cpuDate != "" {
		return formatDateStr(cpuDate)
	}

	// 方式2：尝试读取 ProcessorReleaseDate 作为 DWORD 类型（某些系统以DWORD存储日期）
	cpuDateDword := readRegistryDword("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorReleaseDate")
	if cpuDateDword == 0 {
		cpuDateDword = readRegistryDword64("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorReleaseDate")
	}
	if cpuDateDword != 0 {
		return dateDwordToStr(cpuDateDword)
	}

	// 方式3：读取BIOS日期
	biosDate := readRegistryStr("HARDWARE\\DESCRIPTION\\System\\BIOS", "BIOSReleaseDate")
	if biosDate == "" {
		biosDate = readRegistryStr64("HARDWARE\\DESCRIPTION\\System\\BIOS", "BIOSReleaseDate")
	}
	if biosDate != "" {
		return formatDateStr(biosDate)
	}

	// 方式4：尝试读取 SystemBiosDate（某些系统用此键名）
	biosDate2 := readRegistryStr("HARDWARE\\DESCRIPTION\\System\\BIOS", "SystemBiosDate")
	if biosDate2 == "" {
		biosDate2 = readRegistryStr64("HARDWARE\\DESCRIPTION\\System\\BIOS", "SystemBiosDate")
	}
	if biosDate2 != "" {
		return formatDateStr(biosDate2)
	}

	// 方式5：尝试读取 BaseBoard 制造日期（主板日期，对品牌机可作参考）
	boardDate := readRegistryStr("HARDWARE\\DESCRIPTION\\System\\BIOS", "BaseBoardManufactureDate")
	if boardDate == "" {
		boardDate = readRegistryStr64("HARDWARE\\DESCRIPTION\\System\\BIOS", "BaseBoardManufactureDate")
	}
	if boardDate != "" {
		return formatDateStr(boardDate)
	}

	return "无法获取"
}

// dateDwordToStr 将DWORD格式的日期转换为 YYYY-MM-DD
// DWORD日期格式可能为 0xYYYYMMDD 或 0xMMDDYYYY
func dateDwordToStr(dword uint32) string {
	if dword == 0 {
		return ""
	}
	// 尝试按 YYYYMMDD 解析
	year := (dword >> 16) & 0xFFFF
	if year > 1900 && year <= 2099 {
		month := (dword >> 8) & 0xFF
		day := dword & 0xFF
		if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		}
	}
	// 尝试反向字节序
	b0 := byte(dword & 0xFF)
	b1 := byte((dword >> 8) & 0xFF)
	b2 := byte((dword >> 16) & 0xFF)
	b3 := byte((dword >> 24) & 0xFF)
	// 可能格式：MM DD YYYY 或 DD MM YYYY
	_ = b0
	_ = b1
	_ = b2
	_ = b3
	yearUint := uint32(b3)*100 + uint32(b2)
	if yearUint > 0 {
		year2 := 2000 + yearUint
		if b0 >= 1 && b0 <= 12 && b1 >= 1 && b1 <= 31 {
			return fmt.Sprintf("%04d-%02d-%02d", year2, b0, b1)
		}
	}
	return fmt.Sprintf("%d", dword)
}

// formatDateStr 将各种日期格式统一转换为 YYYY-MM-DD
// 支持的格式：
//   中文：2023年01月15日、23年1月5日
//   斜杠：2023/01/15、1/15/2023、01/15/23、2023/1/5
//   横线：2023-01-15、1-15-2023、01-15-23、2023-1-5
//   点号：2023.01.15、01.15.2023、2023.1.5、1.15.2023
//   空格分隔：2023 01 15、1 15 2023
//   纯数字：20230115、230115
//   ISO带T：2023-01-15T00:00:00、2023-01-15 00:00:00
func formatDateStr(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	// 1. 去除前后空格
	s := strings.TrimSpace(dateStr)

	// 2. 处理含中文的日期格式（如 "2023年01月15日"）
	if strings.Contains(s, "年") || strings.Contains(s, "月") || strings.Contains(s, "日") {
		var y, m, d int
		// 匹配 "2023年01月15日" 或 "23年1月5日"
		if _, err := fmt.Sscanf(s, "%d年%d月%d日", &y, &m, &d); err == nil {
			if m >= 1 && m <= 12 && d >= 1 && d <= 31 {
				// 补全短年份
				if y < 100 {
					y += 2000
				}
				if y >= 1900 && y <= 2099 {
					return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
				}
			}
		}
		// 中文替换成横线，继续尝试
		s = strings.Replace(s, "年", "-", 1)
		s = strings.Replace(s, "月", "-", 1)
		s = strings.Replace(s, "日", "", 1)
	}

	// 3. 处理 ISO 格式带T（如 "2023-01-15T00:00:00"）
	if tIdx := strings.Index(s, "T"); tIdx >= 6 && tIdx <= 10 {
		s = s[:tIdx]
	}

	// 4. 统一分隔符
	// 将所有 / . \ 和连续空格都替换为 -
	s = strings.Replace(s, "/", "-", -1)
	s = strings.Replace(s, ".", "-", -1)
	s = strings.Replace(s, "\\", "-", -1)
	for strings.Contains(s, "  ") {
		s = strings.Replace(s, "  ", " ", -1)
	}
	s = strings.Replace(s, " ", "-", -1)

	// 5. 尝试按 "-" 分割解析
	if strings.Contains(s, "-") {
		parts := strings.Split(s, "-")
		// 过滤掉空字串
		clean := make([]string, 0, len(parts))
		for _, p := range parts {
			if p != "" {
				clean = append(clean, p)
			}
		}
		if len(clean) == 3 {
			y, m, d := tryParseDateParts(clean)
			if y != 0 {
				return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
			}
		}
	}

	// 6. 提取数字部分，去除所有非数字字符
	digits := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}
	cleanNum := string(digits)

	// 尝试8位数字：yyyyMMdd
	if len(cleanNum) >= 8 {
		y := parseIntSubstr(cleanNum, 0, 4)
		m := parseIntSubstr(cleanNum, 4, 2)
		d := parseIntSubstr(cleanNum, 6, 2)
		if y >= 1900 && y <= 2099 && m >= 1 && m <= 12 && d >= 1 && d <= 31 {
			return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
		}
	}

	// 尝试6位数字：yyMMdd -> 20yy-MM-dd
	if len(cleanNum) == 6 {
		y := parseIntSubstr(cleanNum, 0, 2)
		m := parseIntSubstr(cleanNum, 2, 2)
		d := parseIntSubstr(cleanNum, 4, 2)
		if y >= 0 && y <= 99 && m >= 1 && m <= 12 && d >= 1 && d <= 31 {
			return fmt.Sprintf("20%02d-%02d-%02d", y, m, d)
		}
	}

	// 无法识别的格式，原样返回
	return dateStr
}


// tryParseDateParts 尝试解析三部分日期，自动识别是 yyyy/MM/dd 还是 MM/dd/yyyy
// 返回 y, m, d（如果解析失败返回 0,0,0）
func tryParseDateParts(parts []string) (int, int, int) {
	if len(parts) != 3 {
		return 0, 0, 0
	}
	v0 := parseIntSubstr(parts[0], 0, len(parts[0]))
	v1 := parseIntSubstr(parts[1], 0, len(parts[1]))
	v2 := parseIntSubstr(parts[2], 0, len(parts[2]))
	if v0 == 0 && v1 == 0 && v2 == 0 {
		return 0, 0, 0
	}
	// 规则1：如果第一部分 > 31，一定是年份 => yyyy/MM/dd
	if v0 > 31 {
		if v1 >= 1 && v1 <= 12 && v2 >= 1 && v2 <= 31 {
			return v0, v1, v2
		}
		return 0, 0, 0
	}
	// 规则2：如果第三部分 > 31，一定是年份 => MM/dd/yyyy
	if v2 > 31 {
		if v0 >= 1 && v0 <= 12 && v1 >= 1 && v1 <= 31 {
			return v2, v0, v1
		}
		return 0, 0, 0
	}
	// 规则3：如果第三部分是两位数（如 23），补全为 2023
	if v2 < 100 && v2 >= 0 {
		v2 += 2000
		if v0 >= 1 && v0 <= 12 && v1 >= 1 && v1 <= 31 {
			return v2, v0, v1
		}
	}
	// 规则4：如果第一部分 <= 12 且第二部分 <= 31，推测为 MM/dd/yyyy
	if v0 >= 1 && v0 <= 12 && v1 >= 1 && v1 <= 31 && v2 >= 1900 && v2 <= 2099 {
		return v2, v0, v1
	}
	// 规则5：推测为 dd/MM/yyyy（欧洲格式）
	if v1 >= 1 && v1 <= 12 && v0 >= 1 && v0 <= 31 && v2 >= 1900 && v2 <= 2099 {
		return v2, v1, v0
	}
	return 0, 0, 0
}

// parseIntSubstr 安全地将字符串子串解析为整数，失败返回0
func parseIntSubstr(s string, start, length int) int {
	if start < 0 || start >= len(s) {
		return 0
	}
	end := start + length
	if end > len(s) {
		end = len(s)
	}
	var val int
	for i := start; i < end; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0
		}
		val = val*10 + int(c-'0')
	}
	return val
}


// ===== 浏览器版本 =====

// readRegistryStr64 从64位注册表视图中读取字符串值（使用 KEY_WOW64_64KEY）
// 当32位程序在64位 Windows 上运行时，WOW64 会将注册表重定向到 WOW6432Node，需要此函数
func readRegistryStr64(keyPath, valueName string) string {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueEx := advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")

	HKEY_LOCAL_MACHINE := uintptr(0x80000002)
	// KEY_READ | KEY_WOW64_64KEY = 0x20019 | 0x100 = 0x00020119
	samDesired := uintptr(0x00020119)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(keyPath))),
		0,
		samDesired,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return ""
	}
	defer procRegCloseKey.Call(hKey)

	var valueType uint32
	var valueLen uint32
	procRegQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(valueName))),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		0,
		uintptr(unsafe.Pointer(&valueLen)),
	)
	if valueLen == 0 {
		return ""
	}

	valueBuf := make([]uint16, valueLen/2+1)
	ret, _, _ = procRegQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(valueName))),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		uintptr(unsafe.Pointer(&valueBuf[0])),
		uintptr(unsafe.Pointer(&valueLen)),
	)
	if ret != 0 {
		return ""
	}

	return syscall.UTF16ToString(valueBuf)
}

// getFileVersionFromPath 从EXE文件中提取版本信息
// 使用 GetFileVersionInfo API 并枚举区域设置，或检查父目录名是否看起来像版本号
// 例如： "...\148.0.7778.179\chrome.exe"
func getFileVersionFromPath(exePath string) string {
	// GetFileVersionInfoSize/GetFileVersionInfo 位于 version.dll 中，而非 kernel32.dll
	versionDll := syscall.NewLazyDLL("version.dll")
	procGetFileVersionInfoSize := versionDll.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfo := versionDll.NewProc("GetFileVersionInfoW")
	procVerQueryValue := versionDll.NewProc("VerQueryValueW")

	size, _, _ := procGetFileVersionInfoSize.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(exePath))),
		0,
	)
	if size > 0 {
		buf := make([]byte, size)
		ret, _, _ := procGetFileVersionInfo.Call(
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(exePath))),
			0,
			size,
			uintptr(unsafe.Pointer(&buf[0])),
		)
		if ret != 0 {
			// 首先获取可用的翻译列表
			var transBuf uintptr
			var transLen uint32
			retT, _, _ := procVerQueryValue.Call(
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("\\VarFileInfo\\Translation"))),
				uintptr(unsafe.Pointer(&transBuf)),
				uintptr(unsafe.Pointer(&transLen)),
			)
			if retT != 0 && transLen >= 4 {
				// 尝试每个翻译
				transArr := (*[1 << 20]uint16)(unsafe.Pointer(transBuf))[:transLen/2]
				for i := 0; i < len(transArr); i += 2 {
					lang := transArr[i]
					codepage := transArr[i+1]
					localeStr := fmt.Sprintf("%04X%04X", lang, codepage)
					queryPath := "\\StringFileInfo\\" + localeStr + "\\FileVersion"
					var verBuf uintptr
					var verLen uint32
					retV, _, _ := procVerQueryValue.Call(
						uintptr(unsafe.Pointer(&buf[0])),
						uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(queryPath))),
						uintptr(unsafe.Pointer(&verBuf)),
						uintptr(unsafe.Pointer(&verLen)),
					)
					if retV != 0 && verLen > 2 {
						verStr := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(verBuf))[:verLen/2])
						if verStr != "" {
							return verStr
						}
					}
				}
			}

			// 回退方案：尝试常用区域设置
			commonLocales := []string{
				"040904B0", "040904E4", "04090480",
				"080404B0", "080404E4", "08040480",
				"0C0404B0", "000004B0",
			}
			for _, loc := range commonLocales {
				var verBuf uintptr
				var verLen uint32
				qPath := "\\StringFileInfo\\" + loc + "\\FileVersion"
				retV, _, _ := procVerQueryValue.Call(
					uintptr(unsafe.Pointer(&buf[0])),
					uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(qPath))),
					uintptr(unsafe.Pointer(&verBuf)),
					uintptr(unsafe.Pointer(&verLen)),
				)
				if retV != 0 && verLen > 2 {
					verStr := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(verBuf))[:verLen/2])
					if verStr != "" {
						return verStr
					}
				}
			}
		}
	}

	// 回退方案：检查父目录名中是否包含版本号模式
	// （例如 "C:\Program Files\Google\Chrome\Application\148.0.7778.179\chrome.exe"）
	lastSlash := strings.LastIndex(exePath, "\\")
	if lastSlash >= 0 {
		parentDir := exePath[:lastSlash]
		lastSlash2 := strings.LastIndex(parentDir, "\\")
		if lastSlash2 >= 0 {
			dirName := parentDir[lastSlash2+1:]
			if isVersionString(dirName) {
				return dirName
			}
		}
		// 也检查父目录的子目录中是否有版本命名的文件夹
		// Chrome/Edge 的EXE可能位于版本文件夹旁边，而不是在内部
		subDirs := []string{"", "Application\\"}
		for _, sd := range subDirs {
			checkDir := parentDir + "\\" + sd
			procFindFirstFile := kernel32.NewProc("FindFirstFileW")
			procFindNextFile := kernel32.NewProc("FindNextFileW")
			procFindClose := kernel32.NewProc("FindClose")
			type WIN32_FIND_DATAW struct {
				DwFileAttributes   uint32
				FtCreationTime     [2]uint32
				FtLastAccessTime   [2]uint32
				FtLastWriteTime    [2]uint32
				NFileSizeHigh      uint32
				NFileSizeLow       uint32
				DwReserved0        uint32
				DwReserved1        uint32
				CFileName          [260]uint16
				CAlternateFileName [14]uint16
			}
			var ffd WIN32_FIND_DATAW
			searchPath := checkDir + "\\*"
			hFind, _, _ := procFindFirstFile.Call(
				uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(searchPath))),
				uintptr(unsafe.Pointer(&ffd)),
			)
			if hFind != uintptr(0xFFFFFFFF) {
				for {
					if ffd.DwFileAttributes&0x10 != 0 { // 目录
						dirName := syscall.UTF16ToString(ffd.CFileName[:])
						if dirName != "." && dirName != ".." && isVersionString(dirName) {
							procFindClose.Call(hFind)
							return dirName
						}
					}
					retF, _, _ := procFindNextFile.Call(hFind, uintptr(unsafe.Pointer(&ffd)))
					if retF == 0 {
						break
					}
				}
				procFindClose.Call(hFind)
			}
		}
	}
	return ""
}

func isVersionString(s string) bool {
	if s == "" {
		return false
	}
	dotCount := 0
	for _, c := range s {
		if c == '.' {
			dotCount++
		} else if c < '0' || c > '9' {
			return false
		}
	}
	return dotCount >= 2 && dotCount <= 3
}

// getBrowserExePath 获取在 StartMenuInternet 中注册的浏览器的EXE路径
func getBrowserExePath(browserName string) string {
	// 首先尝试 App Paths（最可靠）
	appPath := readRegistryStr("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\"+browserName, "")
	if appPath != "" {
		return appPath
	}
	appPath = readRegistryStr64("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\"+browserName, "")
	if appPath != "" {
		return appPath
	}
	// 尝试 Client 注册表
	clientPath := readRegistryStr("SOFTWARE\\Clients\\StartMenuInternet\\"+browserName+"\\shell\\open\\command", "")
	if clientPath != "" {
		// 清理路径 - 去除引号和参数
		clientPath = strings.Trim(clientPath, "\"")
		spaceIdx := strings.Index(clientPath, "\" ")
		if spaceIdx >= 0 {
			clientPath = clientPath[:strings.LastIndex(clientPath, ".exe")+4]
		}
	}
	return clientPath
}

func getBrowserVersion() string {
	var parts []string

	// Internet Explorer - 直接从注册表读取
	ieVer := readRegistryStr64("SOFTWARE\\Microsoft\\Internet Explorer", "svcVersion")
	if ieVer == "" {
		ieVer = readRegistryStr64("SOFTWARE\\Microsoft\\Internet Explorer", "Version")
	}
	if ieVer == "" {
		ieVer = readRegistryStr("SOFTWARE\\Microsoft\\Internet Explorer", "svcVersion")
	}
	if ieVer == "" {
		ieVer = readRegistryStr("SOFTWARE\\Microsoft\\Internet Explorer", "Version")
	}
	if ieVer == "" {
		ieVer = readRegistryStr("SOFTWARE\\WOW6432Node\\Microsoft\\Internet Explorer", "svcVersion")
	}
	if ieVer == "" {
		ieVer = readRegistryStr("SOFTWARE\\WOW6432Node\\Microsoft\\Internet Explorer", "Version")
	}
	if ieVer != "" {
		parts = append(parts, "Internet Explorer: "+ieVer)
	}

	// Google Chrome - 通过EXE和注册表检测
	chromeExe := getBrowserExePath("chrome.exe")
	chromeVer := ""
	if chromeExe != "" {
		chromeVer = getFileVersionFromPath(chromeExe)
	}
	// 回退到 Uninstall 注册表
	if chromeVer == "" {
		chromeVer = readRegistryStr64("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Google Chrome", "DisplayVersion")
	}
	if chromeVer == "" {
		chromeVer = readRegistryStr("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Google Chrome", "DisplayVersion")
	}
	if chromeVer == "" {
		chromeVer = readRegistryStr64("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Google Chrome", "version")
	}
	if chromeVer == "" {
		chromeVer = readRegistryStr("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Google Chrome", "version")
	}
	if chromeVer == "" {
		chromeVer = readRegistryStr("SOFTWARE\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Google Chrome", "DisplayVersion")
	}
	if chromeVer != "" {
		parts = append(parts, "Google Chrome: "+chromeVer)
	}

	// Microsoft Edge - 通过EXE和注册表检测
	edgeExe := getBrowserExePath("msedge.exe")
	edgeVer := ""
	if edgeExe != "" {
		edgeVer = getFileVersionFromPath(edgeExe)
	}
	if edgeVer == "" {
		edgeVer = readRegistryStr64("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Microsoft Edge", "DisplayVersion")
	}
	if edgeVer == "" {
		edgeVer = readRegistryStr("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Microsoft Edge", "DisplayVersion")
	}
	if edgeVer == "" {
		edgeVer = readRegistryStr("SOFTWARE\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Microsoft Edge", "DisplayVersion")
	}
	if edgeVer == "" {
		edgeVer = readRegistryStr64("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Microsoft Edge", "version")
	}
	if edgeVer == "" {
		edgeVer = readRegistryStr("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Microsoft Edge", "version")
	}
	if edgeVer == "" {
		edgeVer = readRegistryStr("SOFTWARE\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Microsoft Edge", "version")
	}
	if edgeVer != "" {
		parts = append(parts, "Microsoft Edge: "+edgeVer)
	}

	// Mozilla Firefox
	firefoxVer := readRegistryStr64("SOFTWARE\\Mozilla\\Mozilla Firefox", "CurrentVersion")
	if firefoxVer == "" {
		firefoxVer = readRegistryStr("SOFTWARE\\Mozilla\\Mozilla Firefox", "CurrentVersion")
	}
	if firefoxVer == "" {
		firefoxVer = readRegistryStr("SOFTWARE\\WOW6432Node\\Mozilla\\Mozilla Firefox", "CurrentVersion")
	}
	if firefoxVer != "" {
		parts = append(parts, "Firefox: "+firefoxVer)
	}

	if len(parts) == 0 {
		return "无法获取"
	}

	return strings.Join(parts, "；\n")
}

// ===== IP 地址 =====
func getIPAddress() string {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	procRegEnumKeyEx := advapi32.NewProc("RegEnumKeyExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")
	procRegQueryInfoKey := advapi32.NewProc("RegQueryInfoKeyW")

	HKEY_LOCAL_MACHINE := uintptr(0x80000002)
	interfacesKey := "SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters\\Interfaces"

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(interfacesKey))),
		0,
		0x00020019,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return "无法获取"
	}
	defer procRegCloseKey.Call(hKey)

	var subKeyCount uint32
	var maxSubKeyLen uint32
	procRegQueryInfoKey.Call(
		hKey,
		0, 0, 0,
		uintptr(unsafe.Pointer(&subKeyCount)),
		uintptr(unsafe.Pointer(&maxSubKeyLen)),
		0, 0, 0, 0, 0, 0,
	)

	if subKeyCount == 0 {
		return "无法获取"
	}

	var ips []string

	for i := uint32(0); i < subKeyCount; i++ {
		subKeyBuf := make([]uint16, maxSubKeyLen+2)
		var nameLen uint32 = maxSubKeyLen + 1

		ret, _, _ = procRegEnumKeyEx.Call(
			hKey,
			uintptr(i),
			uintptr(unsafe.Pointer(&subKeyBuf[0])),
			uintptr(unsafe.Pointer(&nameLen)),
			0, 0, 0, 0,
		)
		if ret != 0 {
			continue
		}

		guidStr := syscall.UTF16ToString(subKeyBuf)
		subKeyPath := interfacesKey + "\\" + guidStr

		dhcpIP := readRegistryStr(subKeyPath, "DhcpIPAddress")
		if dhcpIP != "" && !strings.HasPrefix(dhcpIP, "0.") && !strings.HasPrefix(dhcpIP, "169.") {
			if !contains(ips, dhcpIP) {
				ips = append(ips, dhcpIP)
			}
			continue
		}

		ipAddr := readRegistryStr(subKeyPath, "IPAddress")
		if ipAddr != "" {
			ipList := strings.FieldsFunc(ipAddr, func(r rune) bool {
				return r == ' ' || r == ',' || r == rune(0)
			})
			for _, ip := range ipList {
				if ip != "" && !strings.HasPrefix(ip, "0.") && !strings.HasPrefix(ip, "169.") {
					if !contains(ips, ip) {
						ips = append(ips, ip)
					}
				}
			}
		}
	}

	if len(ips) == 0 {
		return "未检测到网络连接"
	}

	return strings.Join(ips, "；\n")
}

// ===== MAC 地址 =====
func getMACAddress() string {
	iphlpapi := syscall.NewLazyDLL("iphlpapi.dll")
	procGetAdaptersInfo := iphlpapi.NewProc("GetAdaptersInfo")

	type IP_ADAPTER_INFO struct {
		Next                uintptr
		ComboIndex          uint32
		AdapterName         [260]byte
		Description         [132]byte
		AddressLength       uint32
		Address             [8]byte
		Index               uint32
		Type                uint32
		DhcpEnabled         uint32
		CurrentIpAddress    uintptr
		IpAddressList       [16]byte
		GatewayList         [16]byte
		DhcpServer          [16]byte
		HaveWins            uint32
		PrimaryWinsServer   [16]byte
		SecondaryWinsServer [16]byte
		LeaseObtained       int64
		LeaseExpires        int64
	}

	bufSize := uint32(0)
	procGetAdaptersInfo.Call(0, uintptr(unsafe.Pointer(&bufSize)))
	if bufSize == 0 {
		return "无法获取"
	}

	buf := make([]byte, bufSize+1024)
	ret, _, _ := procGetAdaptersInfo.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufSize)))
	if ret != 0 {
		return "无法获取"
	}

	var macs []string
	p := (*IP_ADAPTER_INFO)(unsafe.Pointer(&buf[0]))
	for {
		desc := string(p.Description[:])
		if nullIdx := strings.IndexByte(desc, 0); nullIdx >= 0 {
			desc = desc[:nullIdx]
		}
		descUpper := strings.ToUpper(desc)

		if p.AddressLength > 0 && p.AddressLength <= 8 &&
			!strings.Contains(descUpper, "LOOPBACK") &&
			!strings.Contains(descUpper, "VMWARE") &&
			!strings.Contains(descUpper, "VIRTUAL") &&
			!strings.Contains(descUpper, "TUNNEL") &&
			!strings.Contains(descUpper, "PANGP") &&
			!strings.Contains(descUpper, "BLUETOOTH") &&
			!strings.Contains(descUpper, "WAN") {
			mac := fmt.Sprintf("%02X-%02X-%02X-%02X-%02X-%02X",
				p.Address[0], p.Address[1], p.Address[2],
				p.Address[3], p.Address[4], p.Address[5])
			if !contains(macs, mac) && mac != "00-00-00-00-00-00" {
				macs = append(macs, mac)
			}
		}

		if p.Next == 0 {
			break
		}
		p = (*IP_ADAPTER_INFO)(unsafe.Pointer(p.Next))
	}

	if len(macs) == 0 {
		return "无法获取"
	}

	return strings.Join(macs, "；\n")
}

// ===== 硬盘序列号 - 基于注册表（安全，无IOCTL） =====
func getDiskSerialRegistry() string {
	// 同时使用所有方法，合并并去重
	disks1 := getDiskInfoFromRegDeviceMap()
	disks2 := getDiskInfoFromEnum()
	disks3 := getDiskInfoFromIDE()
	disks4 := getDiskInfoFromWMIEnum()

	// 合并所有磁盘，按型号去重（优先保留有序列号的条目）
	seenModel := make(map[string]int) // 型号 -> 在disks中的索引
	seenKey := make(map[string]bool)
	var disks []DiskInfo
	appendUnique := func(list []DiskInfo) {
		for _, d := range list {
			if d.Model == "" {
				continue
			}
			key := d.Model + "|" + d.Serial
			if seenKey[key] {
				continue
			}
			if idx, exists := seenModel[d.Model]; exists {
				// 型号已存在 - 优先保留有序列号的
				if d.Serial != "" && disks[idx].Serial == "" {
					disks[idx].Serial = d.Serial
				}
				seenKey[key] = true
			} else {
				seenModel[d.Model] = len(disks)
				seenKey[key] = true
				d.Index = len(disks)
				disks = append(disks, d)
			}
		}
	}
	appendUnique(disks1)
	appendUnique(disks2)
	appendUnique(disks3)
	appendUnique(disks4)

	if len(disks) == 0 {
		return "无法获取"
	}

	var parts []string
	for _, d := range disks {
		if d.Serial != "" {
			parts = append(parts, fmt.Sprintf("磁盘%d: %s (SN: %s)", d.Index, d.Model, d.Serial))
		} else {
			parts = append(parts, fmt.Sprintf("磁盘%d: %s", d.Index, d.Model))
		}
	}

	return strings.Join(parts, "；\n")
}

type DiskInfo struct {
	Index  int
	Model  string
	Serial string
}

// getDiskInfoFromRegDeviceMap 从 HARDWARE\DEVICEMAP\Scsi 读取磁盘信息（涵盖 IDE/SATA/SCSI 驱动器）
func getDiskInfoFromRegDeviceMap() []DiskInfo {
	var disks []DiskInfo
	basePath := "HARDWARE\\DEVICEMAP\\Scsi"

	for port := 0; port < 16; port++ {
		for bus := 0; bus < 16; bus++ {
			for target := 0; target < 16; target++ {
				for lun := 0; lun < 8; lun++ {
					keyPath := fmt.Sprintf("%s\\Scsi Port %d\\Scsi Bus %d\\Target Id %d\\Logical Unit Id %d",
						basePath, port, bus, target, lun)

					identifier := readRegistryStr(keyPath, "Identifier")
					if identifier == "" || strings.Contains(strings.ToUpper(identifier), "SCSI") ||
						strings.Contains(strings.ToUpper(identifier), "CONTROLLER") {
						continue
					}

					serial := readRegistryStr(keyPath, "SerialNumber")
					d := DiskInfo{
						Index:  len(disks),
						Model:  strings.TrimSpace(identifier),
						Serial: strings.TrimSpace(serial),
					}
					disks = append(disks, d)
				}
			}
		}
	}
	return disks
}

// getDiskInfoFromEnum 从 SYSTEM\CurrentControlSet\Enum 读取磁盘信息（涵盖所有设备类型）
func getDiskInfoFromEnum() []DiskInfo {
	var disks []DiskInfo

	// 尝试 SCSI 磁盘枚举
	scsiBase := "SYSTEM\\CurrentControlSet\\Enum\\SCSI\\Disk"
	disks = append(disks, enumSubKeyDisk(scsiBase, disks)...)

	// 尝试 STORAGE 卷枚举
	storageBase := "SYSTEM\\CurrentControlSet\\Enum\\STORAGE\\Volume"
	disks = append(disks, enumSubKeyStorage(storageBase, disks)...)

	return disks
}

func enumSubKeyDisk(baseKey string, existing []DiskInfo) []DiskInfo {
	var disks []DiskInfo

	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")
	procRegEnumKeyEx := advapi32.NewProc("RegEnumKeyExW")
	procRegQueryInfoKey := advapi32.NewProc("RegQueryInfoKeyW")

	HKEY_LOCAL_MACHINE := uintptr(0x80000002)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(baseKey))),
		0,
		0x00020019,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return disks
	}
	defer procRegCloseKey.Call(hKey)

	var subKeyCount uint32
	var maxSubKeyLen uint32
	procRegQueryInfoKey.Call(
		hKey, 0, 0, 0,
		uintptr(unsafe.Pointer(&subKeyCount)),
		uintptr(unsafe.Pointer(&maxSubKeyLen)),
		0, 0, 0, 0, 0, 0,
	)

	for i := uint32(0); i < subKeyCount; i++ {
		subKeyBuf := make([]uint16, maxSubKeyLen+2)
		var nameLen uint32 = maxSubKeyLen + 1
		ret, _, _ = procRegEnumKeyEx.Call(
			hKey, uintptr(i),
			uintptr(unsafe.Pointer(&subKeyBuf[0])),
			uintptr(unsafe.Pointer(&nameLen)),
			0, 0, 0, 0,
		)
		if ret != 0 {
			continue
		}

		modelName := syscall.UTF16ToString(subKeyBuf)
		subKeyPath := baseKey + "\\" + modelName

		// 枚举该型号下的子项（序列号）
		var hKey2 uintptr
		ret2, _, _ := procRegOpenKeyEx.Call(
			HKEY_LOCAL_MACHINE,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(subKeyPath))),
			0, 0x00020019,
			uintptr(unsafe.Pointer(&hKey2)),
		)
		if ret2 != 0 {
			continue
		}

		var subKeyCount2 uint32
		var maxSubKeyLen2 uint32
		procRegQueryInfoKey.Call(
			hKey2, 0, 0, 0,
			uintptr(unsafe.Pointer(&subKeyCount2)),
			uintptr(unsafe.Pointer(&maxSubKeyLen2)),
			0, 0, 0, 0, 0, 0,
		)

		var instanceIdx int
		for j := uint32(0); j < subKeyCount2; j++ {
			buf2 := make([]uint16, maxSubKeyLen2+2)
			var nameLen2 uint32 = maxSubKeyLen2 + 1
			ret3, _, _ := procRegEnumKeyEx.Call(
				hKey2, uintptr(j),
				uintptr(unsafe.Pointer(&buf2[0])),
				uintptr(unsafe.Pointer(&nameLen2)),
				0, 0, 0, 0,
			)
			if ret3 != 0 {
				continue
			}
			instancePath := subKeyPath + "\\" + syscall.UTF16ToString(buf2)
			// 从该实例读取 FriendlyName（实际型号名称）
			friendlyName := readRegistryStr(instancePath, "FriendlyName")
			modelForDisplay := cleanDiskModel(modelName)
			if friendlyName != "" {
				// FriendlyName 是干净的型号名称（例如 "WDC WD10EZEX-08WN4A1"）
				modelForDisplay = friendlyName
			}
			// 序列号不存储在 Enum 子项中，它们存储的是实例ID
			// 序列号来自 HARDWARE\DEVICEMAP\Scsi，我们已经读取过了
			d := DiskInfo{
				Index:  instanceIdx,
				Model:  modelForDisplay,
				Serial: "", // 序列号来自 RegDeviceMap
			}
			disks = append(disks, d)
			instanceIdx++
		}
		procRegCloseKey.Call(hKey2)
	}
	return disks
}

func enumSubKeyStorage(baseKey string, existing []DiskInfo) []DiskInfo {
	var disks []DiskInfo

	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")
	procRegEnumKeyEx := advapi32.NewProc("RegEnumKeyExW")
	procRegQueryInfoKey := advapi32.NewProc("RegQueryInfoKeyW")

	HKEY_LOCAL_MACHINE := uintptr(0x80000002)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(baseKey))),
		0,
		0x00020019,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return disks
	}
	defer procRegCloseKey.Call(hKey)

	var subKeyCount uint32
	var maxSubKeyLen uint32
	procRegQueryInfoKey.Call(
		hKey, 0, 0, 0,
		uintptr(unsafe.Pointer(&subKeyCount)),
		uintptr(unsafe.Pointer(&maxSubKeyLen)),
		0, 0, 0, 0, 0, 0,
	)

	seen := make(map[string]bool)

	for i := uint32(0); i < subKeyCount; i++ {
		buf := make([]uint16, maxSubKeyLen+2)
		var nameLen uint32 = maxSubKeyLen + 1
		ret, _, _ = procRegEnumKeyEx.Call(
			hKey, uintptr(i),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&nameLen)),
			0, 0, 0, 0,
		)
		if ret != 0 {
			continue
		}
		name := syscall.UTF16ToString(buf)
		if strings.Contains(strings.ToUpper(name), "HARDDISK") || strings.Contains(strings.ToUpper(name), "PHYSICAL") {
			subPath := baseKey + "\\" + name
			friendly := readRegistryStr(subPath, "FriendlyName")
			if friendly != "" {
				diskName := cleanDiskModel(friendly)
				if !seen[diskName] {
					seen[diskName] = true
					serial := readRegistryStr(subPath, "DeviceSerialNumber")
					d := DiskInfo{
						Index:  len(existing) + len(disks) + 1,
						Model:  diskName,
						Serial: serial,
					}
					disks = append(disks, d)
				}
			}
		}
	}
	return disks
}

// getDiskInfoFromIDE 旧版 IDE 枚举
func getDiskInfoFromIDE() []DiskInfo {
	var disks []DiskInfo
	idePaths := []string{
		"HARDWARE\\DEVICEMAP\\Scsi\\Scsi Port 0\\Scsi Bus 0\\Target Id 0\\Logical Unit Id 0",
		"HARDWARE\\DEVICEMAP\\Scsi\\Scsi Port 0\\Scsi Bus 0\\Target Id 1\\Logical Unit Id 0",
		"HARDWARE\\DEVICEMAP\\Scsi\\Scsi Port 1\\Scsi Bus 0\\Target Id 0\\Logical Unit Id 0",
		"HARDWARE\\DEVICEMAP\\Scsi\\Scsi Port 1\\Scsi Bus 0\\Target Id 1\\Logical Unit Id 0",
	}
	for _, keyPath := range idePaths {
		identifier := readRegistryStr(keyPath, "Identifier")
		if identifier != "" {
			serial := readRegistryStr(keyPath, "SerialNumber")
			d := DiskInfo{
				Index:  len(disks),
				Model:  strings.TrimSpace(identifier),
				Serial: strings.TrimSpace(serial),
			}
			disks = append(disks, d)
		}
	}
	return disks
}

// getDiskInfoFromWMIEnum 从 Enum\IDE 读取磁盘信息（通过IDE通道枚举涵盖IDE/ATA驱动器）
func getDiskInfoFromWMIEnum() []DiskInfo {
	var disks []DiskInfo

	// 尝试 Enum\IDE
	ideBases := []string{
		"SYSTEM\\CurrentControlSet\\Enum\\IDE",
	}
	for _, base := range ideBases {
		d := enumSubKeyDisk(base, disks)
		disks = append(disks, d...)
	}

	// 尝试 Enum\PCIIDE
	pciBase := "SYSTEM\\CurrentControlSet\\Enum\\PCIIDE"
	d2 := enumSubKeyDisk(pciBase, disks)
	disks = append(disks, d2...)

	return disks
}

func cleanDiskModel(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Replace(s, "\x00", "", -1)
	return s
}

// ===== 注册表辅助函数 =====
func readRegistryStr(keyPath, valueName string) string {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueEx := advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")

	HKEY_LOCAL_MACHINE := uintptr(0x80000002)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(keyPath))),
		0,
		0x00020019,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return ""
	}
	defer procRegCloseKey.Call(hKey)

	var valueType uint32
	var valueLen uint32
	procRegQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(valueName))),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		0,
		uintptr(unsafe.Pointer(&valueLen)),
	)
	if valueLen == 0 {
		return ""
	}

	valueBuf := make([]uint16, valueLen/2+1)
	ret, _, _ = procRegQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(valueName))),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		uintptr(unsafe.Pointer(&valueBuf[0])),
		uintptr(unsafe.Pointer(&valueLen)),
	)
	if ret != 0 {
		return ""
	}

	return syscall.UTF16ToString(valueBuf)
}

func readRegistryDword64(keyPath, valueName string) uint32 {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueEx := advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")

	HKEY_LOCAL_MACHINE := uintptr(0x80000002)
	samDesired := uintptr(0x00020119) // KEY_READ | KEY_WOW64_64KEY

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(keyPath))),
		0,
		samDesired,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return 0
	}
	defer procRegCloseKey.Call(hKey)

	var valueType uint32
	var valueLen uint32 = 4
	var value uint32
	ret, _, _ = procRegQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(valueName))),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&valueLen)),
	)
	if ret != 0 {
		return 0
	}
	return value
}

func readRegistryDword(keyPath, valueName string) uint32 {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueEx := advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")

	HKEY_LOCAL_MACHINE := uintptr(0x80000002)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(keyPath))),
		0,
		0x00020019,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return 0
	}
	defer procRegCloseKey.Call(hKey)

	var valueType uint32
	var valueLen uint32 = 4
	var value uint32
	ret, _, _ = procRegQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(valueName))),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&valueLen)),
	)
	if ret != 0 {
		return 0
	}
	return value
}

func readRegistryInt64(keyPath, valueName string) int64 {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueEx := advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")

	HKEY_LOCAL_MACHINE := uintptr(0x80000002)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(keyPath))),
		0,
		0x00020019,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return 0
	}
	defer procRegCloseKey.Call(hKey)

	var valueType uint32
	var valueLen uint32 = 8
	var value int64
	ret, _, _ = procRegQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(valueName))),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&valueLen)),
	)
	if ret != 0 {
		return 0
	}
	return value
}

func contains(s []string, e string) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func unixTimeToLocalTime(unixtime uint64) SYSTEMTIME {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procFileTimeToLocalFileTime := kernel32.NewProc("FileTimeToLocalFileTime")
	procFileTimeToSystemTime := kernel32.NewProc("FileTimeToSystemTime")

	// Unix 时间转 Windows FILETIME
	// FILETIME = 自 1601年1月1日 以来的 100纳秒 间隔数
	// Unix 时间 = 自 1970年1月1日 以来的秒数
	// 差值 = 11644473600 秒
	const EPOCH_DIFFERENCE uint64 = 11644473600
	fileTime := int64((unixtime + EPOCH_DIFFERENCE) * 10000000)

	var localFileTime int64
	procFileTimeToLocalFileTime.Call(
		uintptr(unsafe.Pointer(&fileTime)),
		uintptr(unsafe.Pointer(&localFileTime)),
	)

	var st SYSTEMTIME
	procFileTimeToSystemTime.Call(
		uintptr(unsafe.Pointer(&localFileTime)),
		uintptr(unsafe.Pointer(&st)),
	)

	return st
}
