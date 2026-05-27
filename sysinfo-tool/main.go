package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	WS_OVERLAPPEDWINDOW = 0xCF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	SS_CENTER           = 0x00000001
	WS_CLIPSIBLINGS     = 0x04000000
	WS_BORDER           = 0x00800000
	WS_TABSTOP          = 0x00010000
	WS_VSCROLL          = 0x00200000
	WS_EX_CLIENTEDGE    = 0x00000200
	WS_EX_STATICEDGE    = 0x00020000

	IDC_EDIT_POLICE  = 101
	IDC_EDIT_DEPT    = 103
	IDC_BTN_EXPORT   = 116
	IDC_BTN_REFRESH  = 117

	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_COMMAND         = 0x0111
	WM_CLOSE           = 0x0010
	WM_CTLCOLORSTATIC  = 0x0138
	WM_CTLCOLOREDIT    = 0x0133
	WM_ERASEBKGND      = 0x0014
	WM_PAINT           = 0x000F
	BN_CLICKED         = 0
	SW_SHOWDEFAULT     = 10
	FW_NORMAL          = 400
	FW_BOLD            = 700
	FONT_SIZE          = 11
	HEADER_SIZE        = 12
	TITLE_SIZE         = 14
	COLOR_WHITE        = 0x00FFFFFF
	COLOR_BLACK        = 0x00000000
	COLOR_ACCENT       = 0x00333399
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")

	procGetModuleHandle         = kernel32.NewProc("GetModuleHandleW")
	procCreateFontIndirect      = gdi32.NewProc("CreateFontIndirectW")
	procCreateWindowEx          = user32.NewProc("CreateWindowExW")
	procDefWindowProc           = user32.NewProc("DefWindowProcW")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
	procDispatchMessage         = user32.NewProc("DispatchMessageW")
	procGetMessage              = user32.NewProc("GetMessageW")
	procGetWindowText           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength     = user32.NewProc("GetWindowTextLengthW")
	procLoadCursor              = user32.NewProc("LoadCursorW")
	procLoadIcon                = user32.NewProc("LoadIconW")
	procPostQuitMessage         = user32.NewProc("PostQuitMessage")
	procRegisterClassEx         = user32.NewProc("RegisterClassExW")
	procSetWindowText           = user32.NewProc("SetWindowTextW")
	procShowWindow              = user32.NewProc("ShowWindow")
	procUpdateWindow            = user32.NewProc("UpdateWindow")
	procSendMessage             = user32.NewProc("SendMessageW")
	procMessageBox              = user32.NewProc("MessageBoxW")
	procEnableWindow            = user32.NewProc("EnableWindow")
	procGetDC                   = user32.NewProc("GetDC")
	procReleaseDC               = user32.NewProc("ReleaseDC")
	procTranslateMessage        = user32.NewProc("TranslateMessage")
	procSetBkMode               = gdi32.NewProc("SetBkMode")
	procSetTextColor            = gdi32.NewProc("SetTextColor")
	procCreateSolidBrush        = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject            = gdi32.NewProc("DeleteObject")
	procGetSysColorBrush        = user32.NewProc("GetSysColorBrush")
	procSetBkColor              = gdi32.NewProc("SetBkColor")
)

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	WndProc       uintptr
	ClsExtra      int32
	WndExtra      int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	MenuName      *uint16
	ClassName     *uint16
	HIconSm       uintptr
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type LOGFONT struct {
	LfHeight         int32
	LfWidth          int32
	LfEscapement     int32
	LfOrientation    int32
	LfWeight         int32
	LfItalic         byte
	LfUnderline      byte
	LfStrikeOut      byte
	LfCharSet        byte
	LfOutPrecision   byte
	LfClipPrecision  byte
	LfQuality        byte
	LfPitchAndFamily byte
	LfFaceName       [32]uint16
}

type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type PAINTSTRUCT struct {
	Hdc        uintptr
	FErase     uint32
	RcPaint    RECT
	FRestore   uint32
	FIncUpdate uint32
	RgbReserved [32]byte
}

var (
	mainHwnd       uintptr
	g_font         uintptr
	g_boldFont     uintptr
	g_headerFont   uintptr
	g_titleFont    uintptr
	g_smallFont    uintptr
	g_brushWhite   uintptr
	g_brushSection uintptr
	g_brushLine    uintptr

	g_cpuModel       string
	g_manufactureDate string
	g_osInfo         string
	g_installDate    string
	g_browserVer     string
	g_ipAddr         string
	g_macAddr        string
	g_diskSN         string

	g_exportBtn  uintptr
	g_refreshBtn uintptr

	editPoliceHwnd uintptr
	editDeptHwnd   uintptr
	txtCPUHwnd     uintptr
	txtMftHwnd     uintptr
	txtOSHwnd      uintptr
	txtInstallHwnd uintptr
	txtBrowserHwnd uintptr
	txtIPHwnd      uintptr
	txtMACHwnd     uintptr
	txtDiskHwnd    uintptr
)

func init() {
	runtime.LockOSThread()
}

func main() {
	createMutex := kernel32.NewProc("CreateMutexW")
	createMutex.Call(0, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("SysInfoToolMutex"))))

	type ICCEX struct {
		Size uint32
		ICC  uint32
	}
	icc := ICCEX{Size: uint32(unsafe.Sizeof(ICCEX{})), ICC: 0x00008000}
	comctl32.NewProc("InitCommonControlsEx").Call(uintptr(unsafe.Pointer(&icc)))

	gatherSystemInfo()

	hInstance, _, _ := procGetModuleHandle.Call(0)

	// Create colored brushes
	g_brushWhite, _, _ = procCreateSolidBrush.Call(COLOR_WHITE)
	g_brushSection, _, _ = procCreateSolidBrush.Call(0x00EEF2FF) // light blue for section headers
	g_brushLine, _, _ = procCreateSolidBrush.Call(0x00D0D8E8)   // gray-blue for lines

	className := syscall.StringToUTF16Ptr("SysInfoWindowClass")
	IDC_ARROW := uintptr(32512)
	cursor, _, _ := procLoadCursor.Call(0, IDC_ARROW)
	IDI_APPLICATION := uintptr(32512)
	icon, _, _ := procLoadIcon.Call(0, IDI_APPLICATION)

	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		WndProc:       syscall.NewCallback(windowProc),
		HInstance:     hInstance,
		HCursor:       cursor,
		HIcon:         icon,
		HbrBackground: g_brushWhite,
		ClassName:     className,
	}
	classAtom, _, _ := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	if classAtom == 0 {
		procMessageBox.Call(0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("注册窗口类失败"))),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("错误"))), 0x10)
		return
	}
	_ = classAtom

	// Create fonts
	g_font = createFont(FONT_SIZE, FW_NORMAL, "微软雅黑", "Segoe UI", "Tahoma", "Arial")
	g_boldFont = createFont(FONT_SIZE, FW_BOLD, "微软雅黑", "Segoe UI", "Tahoma", "Arial")
	g_headerFont = createFont(HEADER_SIZE, FW_BOLD, "微软雅黑", "Segoe UI", "Tahoma", "Arial")
	g_titleFont = createFont(TITLE_SIZE, FW_BOLD, "微软雅黑", "Segoe UI", "Tahoma", "Arial")
	g_smallFont = createFont(9, FW_NORMAL, "微软雅黑", "Segoe UI", "Tahoma", "Arial")

	// Create main window - enlarged to accommodate all content
	style := uintptr(0x00CA0000) // WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX
	title := syscall.StringToUTF16Ptr("系统信息采集工具 V0.1")
	hwnd, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		style,
		CW_USEDEFAULT, CW_USEDEFAULT, 660, 900,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		procMessageBox.Call(0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("创建窗口失败"))),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("错误"))), 0x10)
		return
	}
	mainHwnd = hwnd

	centerWindow(hwnd)
	procShowWindow.Call(hwnd, SW_SHOWDEFAULT)
	procUpdateWindow.Call(hwnd)

	var msg MSG
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}

	// Cleanup brushes
	if g_brushWhite != 0 {
		procDeleteObject.Call(g_brushWhite)
	}
	if g_brushSection != 0 {
		procDeleteObject.Call(g_brushSection)
	}
	if g_brushLine != 0 {
		procDeleteObject.Call(g_brushLine)
	}
	if g_font != 0 {
		procDeleteObject.Call(g_font)
	}
	if g_boldFont != 0 {
		procDeleteObject.Call(g_boldFont)
	}
	if g_headerFont != 0 {
		procDeleteObject.Call(g_headerFont)
	}
	if g_titleFont != 0 {
		procDeleteObject.Call(g_titleFont)
	}
	if g_smallFont != 0 {
		procDeleteObject.Call(g_smallFont)
	}
}

const CW_USEDEFAULT = 0x80000000

func centerWindow(hwnd uintptr) {
	var wr RECT
	user32.NewProc("GetWindowRect").Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	ww := wr.Right - wr.Left
	wh := wr.Bottom - wr.Top
	sw := int32(GetSystemMetrics(0))
	sh := int32(GetSystemMetrics(1))
	x := (sw - ww) / 2
	y := (sh - wh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	user32.NewProc("SetWindowPos").Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, 0x0001|0x0004)
}

func GetSystemMetrics(nIndex int) int {
	ret, _, _ := user32.NewProc("GetSystemMetrics").Call(uintptr(nIndex))
	return int(ret)
}

func createFont(size int, weight int, fallbacks ...string) uintptr {
	hdc, _, _ := procGetDC.Call(0)
	logPixelsY := 96
	if hdc != 0 {
		val, _, _ := gdi32.NewProc("GetDeviceCaps").Call(hdc, 90)
		if int(val) > 0 {
			logPixelsY = int(val)
		}
		procReleaseDC.Call(0, hdc)
	}
	height := -(size * logPixelsY / 72)
	for _, name := range fallbacks {
		var lf LOGFONT
		lf.LfHeight = int32(height)
		lf.LfWidth = 0
		lf.LfWeight = int32(weight)
		lf.LfCharSet = 0x86
		lf.LfQuality = 0x02
		nameUTF16 := syscall.StringToUTF16(name)
		for i, c := range nameUTF16 {
			if i >= 31 {
				break
			}
			lf.LfFaceName[i] = c
		}
		font, _, _ := procCreateFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
		if font != 0 {
			return font
		}
	}
	return 0
}

func windowProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		onCreate(hwnd)
		return 0

	case WM_COMMAND:
		lowWord := wParam & 0xFFFF
		highWord := (wParam >> 16) & 0xFFFF
		if highWord == BN_CLICKED {
			switch lowWord {
			case IDC_BTN_EXPORT:
				onExport()
				return 0
			case IDC_BTN_REFRESH:
				onRefresh()
				return 0
			}
		}

	case WM_ERASEBKGND:
		var rc RECT
		user32.NewProc("GetClientRect").Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		user32.NewProc("FillRect").Call(wParam, uintptr(unsafe.Pointer(&rc)), g_brushWhite)
		return 1

	case WM_CTLCOLORSTATIC:
		procSetBkMode.Call(wParam, 1)
		procSetTextColor.Call(wParam, 0x00444444)
		return g_brushWhite

	case WM_CTLCOLOREDIT:
		procSetTextColor.Call(wParam, 0x00333333)
		return g_brushWhite

	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func convertNewlines(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' && (i == 0 || s[i-1] != '\r') {
			result = append(result, '\r', '\n')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

func makeEdit(parent uintptr, id int, text string, x, y, w, h int, readonly bool, font uintptr) uintptr {
	style := uintptr(WS_CHILD | WS_VISIBLE | WS_BORDER | WS_TABSTOP | WS_CLIPSIBLINGS)
	if readonly {
		style |= 0x0800
		style |= 0x0004
		style |= 0x0040
		style |= WS_VSCROLL
	} else {
		style |= 0x0080
	}
	displayText := convertNewlines(text)
	hwndCtrl, _, _ := procCreateWindowEx.Call(
		uintptr(WS_EX_CLIENTEDGE),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("EDIT"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(displayText))),
		style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, uintptr(id), 0, 0,
	)
	if font != 0 && hwndCtrl != 0 {
		procSendMessage.Call(hwndCtrl, 0x0030, font, 0)
	}
	return hwndCtrl
}

func makeStatic(parent uintptr, text string, x, y, w, h int, font uintptr) uintptr {
	hwndCtrl, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("STATIC"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		WS_CHILD|WS_VISIBLE,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, 0, 0, 0,
	)
	if font != 0 && hwndCtrl != 0 {
		procSendMessage.Call(hwndCtrl, 0x0030, font, 0)
	}
	return hwndCtrl
}

func makeStaticCenter(parent uintptr, text string, x, y, w, h int, font uintptr) uintptr {
	hwndCtrl, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("STATIC"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		WS_CHILD|WS_VISIBLE|SS_CENTER,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, 0, 0, 0,
	)
	if font != 0 && hwndCtrl != 0 {
		procSendMessage.Call(hwndCtrl, 0x0030, font, 0)
	}
	return hwndCtrl
}

func makeButton(parent uintptr, id int, text string, x, y, w, h int, font uintptr) uintptr {
	hwndCtrl, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("BUTTON"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		WS_CHILD|WS_VISIBLE|WS_TABSTOP,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, uintptr(id), 0, 0,
	)
	if font != 0 && hwndCtrl != 0 {
		procSendMessage.Call(hwndCtrl, 0x0030, font, 0)
	}
	return hwndCtrl
}

func onCreate(hwnd uintptr) {
	x := 20
	lw := 80
	ew := 500
	lh := 22
	y := 15

	// ========== Title (居中) ==========
	makeStaticCenter(hwnd, "系统信息采集工具", x, y, 600, 30, g_titleFont)
	y += 40

	// ========== Section 1: User Info ==========
	drawSectionHeader(hwnd, x, y, 600)
	y += 28
	makeStatic(hwnd, "▶ 使用人信息", x+5, y, 300, 22, g_headerFont)
	y += 28

	// Row: 责任民警
	makeStatic(hwnd, "责任民警：", x+10, y+4, lw, lh, g_boldFont)
	editPoliceHwnd = makeEdit(hwnd, IDC_EDIT_POLICE, "", x+lw+10, y, ew-10, 27, false, g_font)
	y += 34

	// Row: 所属单位
	makeStatic(hwnd, "所属单位：", x+10, y+4, lw, lh, g_boldFont)
	editDeptHwnd = makeEdit(hwnd, IDC_EDIT_DEPT, "", x+lw+10, y, ew-10, 27, false, g_font)
	y += 40

	// ========== Section 2: 硬件信息 (CPU+出厂日期) ==========
	drawSectionHeader(hwnd, x, y, 600)
	y += 28
	makeStatic(hwnd, "▶ 硬件信息", x+5, y, 300, 22, g_headerFont)
	y += 28

	// CPU
	makeStatic(hwnd, "CPU型号：", x+10, y+3, lw, lh, g_boldFont)
	txtCPUHwnd = makeEdit(hwnd, 0, g_cpuModel, x+lw+10, y, ew-10, 24, true, g_font)
	y += 28

	// 出厂日期
	makeStatic(hwnd, "出厂日期：", x+10, y+3, lw, lh, g_boldFont)
	txtMftHwnd = makeEdit(hwnd, 0, g_manufactureDate, x+lw+10, y, ew-10, 24, true, g_font)
	y += 30

	// ========== Section 3: System Info ==========
	drawSectionHeader(hwnd, x, y, 600)
	y += 28
	makeStatic(hwnd, "▶ 系统信息", x+5, y, 300, 22, g_headerFont)
	y += 28

	infoData := []struct {
		label string
		val   string
		hwnd  *uintptr
		ht    int
	}{
		{"操作系统：", g_osInfo, &txtOSHwnd, 24},
		{"安装时间：", g_installDate, &txtInstallHwnd, 24},
		{"浏 览 器：", g_browserVer, &txtBrowserHwnd, 48},
		{"IP 地 址：", g_ipAddr, &txtIPHwnd, 48},
		{"MAC地址：", g_macAddr, &txtMACHwnd, 48},
		{"硬盘序列号：", g_diskSN, &txtDiskHwnd, 96},
	}

	for _, item := range infoData {
		makeStatic(hwnd, item.label, x+10, y+3, lw, lh, g_boldFont)
		*item.hwnd = makeEdit(hwnd, 0, item.val, x+lw+10, y, ew-10, item.ht, true, g_font)
		y += item.ht + 3
	}

	y += 10

	// ========== Buttons ==========
	btnW := 140
	btnH := 32
	gap := 30
	totalW := btnW*2 + gap
	startX := x + (lw+10+ew-10-totalW)/2

	g_refreshBtn = makeButton(hwnd, IDC_BTN_REFRESH, "   刷新信息   ", startX, y, btnW, btnH, g_boldFont)
	g_exportBtn = makeButton(hwnd, IDC_BTN_EXPORT, "   导出Excel   ", startX+btnW+gap, y, btnW, btnH, g_boldFont)
	y += btnH + 25

	// ========== Footer ==========
	drawSectionHeader(hwnd, x+5, y-5, 585)
	y += 8
	makeStatic(hwnd, "作者：董士魁    版本：V0.1    适用：Windows XP ~ Windows 11    32位/64位", x+10, y, 580, 18, g_smallFont)
}

func drawSectionHeader(hwnd uintptr, x, y, w int) {
	makeStatic(hwnd, "──────────────────────────────────────────────────────────────────────────", x, y, w, 10, g_smallFont)
}

func getWinText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLength.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

func setWinText(hwnd uintptr, text string) {
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(convertNewlines(text)))))
}

func msgBox(title, text string, flags uint) {
	procMessageBox.Call(mainHwnd,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))),
		uintptr(flags))
}

func enableWin(hwnd uintptr, enable bool) {
	v := uintptr(0)
	if enable {
		v = 1
	}
	procEnableWindow.Call(hwnd, v)
}

func onExport() {
	police := getWinText(editPoliceHwnd)
	dept := getWinText(editDeptHwnd)
	if police == "" {
		msgBox("提示", "请先填写责任民警姓名", 0x40)
		return
	}
	enableWin(g_exportBtn, false)
	setWinText(g_exportBtn, "正在导出...")
	desktopPath := getDesktopPath()
	if desktopPath == "" {
		desktopPath = "C:\\Users\\Public\\Desktop"
	}
	fileName := fmt.Sprintf("%s_系统信息_%s.xlsx", police, getDateStr())
	filePath := desktopPath + "\\" + fileName
	err := exportToExcel(filePath, police, dept,
		g_cpuModel, g_manufactureDate, g_osInfo, g_installDate, g_browserVer,
		g_ipAddr, g_macAddr, g_diskSN)
	enableWin(g_exportBtn, true)
	setWinText(g_exportBtn, "   导出Excel   ")
	if err != nil {
		msgBox("导出失败", fmt.Sprintf("导出失败：%v\n\n请确认文件未被占用", err), 0x10)
	} else {
		msgBox("导出成功", fmt.Sprintf("导出成功！\n文件已保存到桌面：\n%s", fileName), 0x40)
	}
}

func onRefresh() {
	enableWin(g_refreshBtn, false)
	setWinText(g_refreshBtn, "刷新中...")
	gatherSystemInfo()
	setWinText(txtCPUHwnd, g_cpuModel)
	setWinText(txtMftHwnd, g_manufactureDate)
	setWinText(txtOSHwnd, g_osInfo)
	setWinText(txtInstallHwnd, g_installDate)
	setWinText(txtBrowserHwnd, g_browserVer)
	setWinText(txtIPHwnd, g_ipAddr)
	setWinText(txtMACHwnd, g_macAddr)
	setWinText(txtDiskHwnd, g_diskSN)
	enableWin(g_refreshBtn, true)
	setWinText(g_refreshBtn, "   刷新信息   ")
}

func getDateStr() string {
	t := getLocalTime()
	return fmt.Sprintf("%04d%02d%02d_%02d%02d%02d",
		t.Year, t.Month, t.Day, t.Hour, t.Minute, t.Second)
}
