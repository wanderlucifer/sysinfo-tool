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

// ===== OS Info =====
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
		// If not WOW64, check pointer size
		if unsafe.Sizeof(uintptr(0)) == 8 {
			return true
		}
		return false
	}
	return unsafe.Sizeof(uintptr(0)) == 8
}

// ===== OS Install Date =====
func getOSInstallDate() string {
	// Try multiple registry locations
	// IMPORTANT: For 32-bit builds on 64-bit OS, WOW64 redirects SOFTWARE to WOW6432Node
	// We must use KEY_WOW64_64KEY to access the 64-bit view where InstallDate actually exists
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

	// Try reading InstallTime (some Windows 10+ versions store it as FILETIME)
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

	// Try reading from registry as string (some systems store it differently)
	installDateStr := readRegistryStr("SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion", "InstallDate")
	if installDateStr == "" {
		installDateStr = readRegistryStr("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Installer", "InstallDate")
	}

	// Check firstboot in Setup key
	setupBoot := readRegistryStr("SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Setup", "SystemStartFirstBoot")
	if setupBoot != "" {
		return "请查看系统安装时间（首次启动）"
	}

	return "无法获取"
}

// ===== CPU Info =====
func getCPUInfo() string {
	// HKEY_LOCAL_MACHINE\HARDWARE\DESCRIPTION\System\CentralProcessor\0
	// ProcessorNameString e.g. "12th Gen Intel(R) Core(TM) i7-12700"
	cpuName := readRegistryStr("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorNameString")
	if cpuName == "" {
		cpuName = readRegistryStr64("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "ProcessorNameString")
	}
	if cpuName == "" {
		return "无法获取"
	}
	return cpuName
}

// ===== Manufacture Date =====
func getManufactureDate() string {
	// HKEY_LOCAL_MACHINE\HARDWARE\DESCRIPTION\System\BIOS
	// BIOSReleaseDate e.g. "01/03/2023"
	releaseDate := readRegistryStr("HARDWARE\\DESCRIPTION\\System\\BIOS", "BIOSReleaseDate")
	if releaseDate == "" {
		releaseDate = readRegistryStr64("HARDWARE\\DESCRIPTION\\System\\BIOS", "BIOSReleaseDate")
	}
	if releaseDate == "" {
		return "无法获取"
	}
	return releaseDate
}

// ===== Browser Version =====
// readRegistryStr64 reads from the 64-bit registry view (KEY_WOW64_64KEY)
// This is needed on 64-bit Windows when the app is 32-bit (WOW64 redirects to WOW6432Node)
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

// getFileVersionFromPath extracts version info from an EXE file
// Uses GetFileVersionInfo API with locale enumeration, or checks
// parent directory name if it looks like a version (e.g. "...\148.0.7778.179\chrome.exe")
func getFileVersionFromPath(exePath string) string {
	// GetFileVersionInfoSize/GetFileVersionInfo are in version.dll, NOT kernel32.dll
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
			// First get the list of available translations
			var transBuf uintptr
			var transLen uint32
			retT, _, _ := procVerQueryValue.Call(
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("\\VarFileInfo\\Translation"))),
				uintptr(unsafe.Pointer(&transBuf)),
				uintptr(unsafe.Pointer(&transLen)),
			)
			if retT != 0 && transLen >= 4 {
				// Try each translation
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

			// Fallback: try common locales
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

	// Fallback: check parent directory name for version pattern
	// (e.g., "C:\Program Files\Google\Chrome\Application\148.0.7778.179\chrome.exe")
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
		// Also check subdirectories of parent for version-named folders
		// Chrome/Edge may have the EXE next to a version folder, not inside it
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
					if ffd.DwFileAttributes&0x10 != 0 { // Directory
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

// getBrowserExePath gets the EXE path for a browser registered in StartMenuInternet
func getBrowserExePath(browserName string) string {
	// Try App Paths first (most reliable)
	appPath := readRegistryStr("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\"+browserName, "")
	if appPath != "" {
		return appPath
	}
	appPath = readRegistryStr64("SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\"+browserName, "")
	if appPath != "" {
		return appPath
	}
	// Try Client registry
	clientPath := readRegistryStr("SOFTWARE\\Clients\\StartMenuInternet\\"+browserName+"\\shell\\open\\command", "")
	if clientPath != "" {
		// Clean up the path - strip quotes and arguments
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

	// ==========================================
	// Internet Explorer - from registry directly
	// ==========================================
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

	// ==========================================
	// Google Chrome - detect by EXE + registry
	// ==========================================
	chromeExe := getBrowserExePath("chrome.exe")
	chromeVer := ""
	if chromeExe != "" {
		chromeVer = getFileVersionFromPath(chromeExe)
	}
	// Fall back to Uninstall registry
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

	// ==========================================
	// Microsoft Edge - detect by EXE + registry
	// ==========================================
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

	// ==========================================
	// Mozilla Firefox
	// ==========================================
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

// ===== IP Address =====
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

// ===== MAC Address =====
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

// ===== Disk Serial Number - Registry-based (safe, no IOCTL) =====
func getDiskSerialRegistry() string {
	// Run ALL methods concurrently, merge and deduplicate
	disks1 := getDiskInfoFromRegDeviceMap()
	disks2 := getDiskInfoFromEnum()
	disks3 := getDiskInfoFromIDE()
	disks4 := getDiskInfoFromWMIEnum()

	// Merge all disks, dedup by Model name (prefer entry with serial)
	seenModel := make(map[string]int) // model -> index in disks
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
				// Model already seen - prefer the one with serial
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

// Read disk info from HARDWARE\DEVICEMAP\Scsi (covers IDE/SATA/SCSI drives)
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

// Read disk info from SYSTEM\CurrentControlSet\Enum (covers all device types)
func getDiskInfoFromEnum() []DiskInfo {
	var disks []DiskInfo

	// Try SCSI disk enumeration
	scsiBase := "SYSTEM\\CurrentControlSet\\Enum\\SCSI\\Disk"
	disks = append(disks, enumSubKeyDisk(scsiBase, disks)...)

	// Try STORAGE volume enumeration
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

		// Now enumerate sub-keys under this model (serial numbers)
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
			// Read FriendlyName from this instance (actual model name)
			friendlyName := readRegistryStr(instancePath, "FriendlyName")
			modelForDisplay := cleanDiskModel(modelName)
			if friendlyName != "" {
				// FriendlyName is the clean model name (e.g. "WDC WD10EZEX-08WN4A1")
				modelForDisplay = friendlyName
			}
			// Serial is NOT stored in Enum sub-keys; they are instance IDs
			// Serial comes from HARDWARE\DEVICEMAP\Scsi which we already read
			d := DiskInfo{
				Index:  instanceIdx,
				Model:  modelForDisplay,
				Serial: "", // serial comes from RegDeviceMap
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

// Legacy IDE enumeration
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

// Read disk info from Enum\IDE (covers IDE/ATA drives via IDE channel enumeration)
func getDiskInfoFromWMIEnum() []DiskInfo {
	var disks []DiskInfo

	// Try Enum\IDE
	ideBases := []string{
		"SYSTEM\\CurrentControlSet\\Enum\\IDE",
	}
	for _, base := range ideBases {
		d := enumSubKeyDisk(base, disks)
		disks = append(disks, d...)
	}

	// Try Enum\PCIIDE
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

// ===== Registry Helpers =====
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

	// Unix time to Windows FILETIME
	// FILETIME = 100-nanosecond intervals since Jan 1, 1601
	// Unix time = seconds since Jan 1, 1970
	// Difference = 11644473600 seconds
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
