package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"strings"
)

type xlsxFile struct {
	files map[string][]byte
}

func newXlsx() *xlsxFile {
	return &xlsxFile{files: make(map[string][]byte)}
}

func (x *xlsxFile) addFile(name, content string) {
	x.files[name] = []byte(content)
}

func (x *xlsxFile) writeTo(path string) error {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	for name, data := range x.files {
		f, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

func xmlHdr() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
}

func escXML(s string) string {
	a := "&" + "amp;"
	l := "&" + "lt;"
	g := "&" + "gt;"
	q := "&" + "quot;"
	p := "&" + "apos;"

	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString(a)
		case '<':
			b.WriteString(l)
		case '>':
			b.WriteString(g)
		case '"':
			b.WriteString(q)
		case '\'':
			b.WriteString(p)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func exportToExcel(filePath, police, dept, cpuModel, manufactureDate, osInfo, installDate, browserVer, ipAddr, macAddr, diskSN string) error {
	x := newXlsx()

	x.addFile("[Content_Types].xml", xmlHdr()+`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
</Types>`)

	x.addFile("_rels/.rels", xmlHdr()+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`)

	x.addFile("xl/_rels/workbook.xml.rels", xmlHdr()+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
</Relationships>`)

	x.addFile("xl/workbook.xml", xmlHdr()+`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="系统信息" sheetId="1" r:id="rId1"/></sheets>
</workbook>`)

	styles := xmlHdr() + `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="3">
<font><sz val="12"/><name val="微软雅黑"/></font>
<font><b/><sz val="12"/><name val="微软雅黑"/></font>
<font><b/><sz val="14"/><name val="微软雅黑"/></font>
</fonts>
<fills count="2">
<fill><patternFill patternType="none"/></fill>
<fill><patternFill patternType="gray125"/></fill>
</fills>
<borders count="2">
<border><left/><right/><top/><bottom/><diagonal/></border>
<border>
<left style="thin"><color auto="1"/></left>
<right style="thin"><color auto="1"/></right>
<top style="thin"><color auto="1"/></top>
<bottom style="thin"><color auto="1"/></bottom>
<diagonal/>
</border>
</borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="4">
<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
<xf numFmtId="0" fontId="1" fillId="0" borderId="1" xfId="0" applyFont="1" applyBorder="1"/>
<xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1"/>
<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/>
</cellXfs>
</styleSheet>`
	x.addFile("xl/styles.xml", styles)

	// 导出时间使用当前系统时间
	now := getLocalTime()
	exportTime := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
		now.Year, now.Month, now.Day, now.Hour, now.Minute, now.Second)

	// 所有需要写入Excel的字符串
	allStrs := []string{
		"系统信息采集表",
		"责任民警", police,
		"所属单位", dept,
		"CPU型号", cpuModel,
		"出厂日期", manufactureDate,
		"操作系统", osInfo,
		"系统安装时间", installDate,
		"浏览器版本", browserVer,
		"IP地址", ipAddr,
		"MAC地址", macAddr,
		"硬盘序列号", diskSN,
		"导出时间", exportTime,
	}

	strIdx := make(map[string]int)
	var uniq []string
	for _, s := range allStrs {
		if _, ok := strIdx[s]; !ok {
			strIdx[s] = len(uniq)
			uniq = append(uniq, s)
		}
	}

	var ss strings.Builder
	ss.WriteString(xmlHdr())
	fmt.Fprintf(&ss, `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`, len(allStrs), len(uniq))
	for _, s := range uniq {
		ss.WriteString("<si><t>")
		ss.WriteString(escXML(s))
		ss.WriteString("</t></si>")
	}
	ss.WriteString("</sst>")
	x.addFile("xl/sharedStrings.xml", ss.String())

	getIdx := func(s string) string {
		if idx, ok := strIdx[s]; ok {
			return fmt.Sprintf("%d", idx)
		}
		return "-1"
	}

	var ws strings.Builder
	ws.WriteString(xmlHdr())
	ws.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	ws.WriteString(`<cols><col min="1" max="1" width="18" customWidth="1"/><col min="2" max="2" width="60" customWidth="1"/></cols>`)
	ws.WriteString("<sheetData>")

	ws.WriteString(fmt.Sprintf(`<row r="1" ht="30" customHeight="1"><c r="A1" t="s" s="3"><v>%s</v></c></row>`, getIdx("系统信息采集表")))

	ws.WriteString(fmt.Sprintf(`<row r="2">
		<c r="A2" t="s" s="1"><v>%s</v></c>
		<c r="B2" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("责任民警"), getIdx(police)))

	ws.WriteString(fmt.Sprintf(`<row r="3">
		<c r="A3" t="s" s="1"><v>%s</v></c>
		<c r="B3" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("所属单位"), getIdx(dept)))

	ws.WriteString(`<row r="4"/>`)

	ws.WriteString(fmt.Sprintf(`<row r="5">
		<c r="A5" t="s" s="1"><v>%s</v></c>
		<c r="B5" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("CPU型号"), getIdx(cpuModel)))

	ws.WriteString(fmt.Sprintf(`<row r="6">
		<c r="A6" t="s" s="1"><v>%s</v></c>
		<c r="B6" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("出厂日期"), getIdx(manufactureDate)))

	ws.WriteString(fmt.Sprintf(`<row r="7">
		<c r="A7" t="s" s="1"><v>%s</v></c>
		<c r="B7" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("操作系统"), getIdx(osInfo)))

	ws.WriteString(fmt.Sprintf(`<row r="8">
		<c r="A8" t="s" s="1"><v>%s</v></c>
		<c r="B8" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("系统安装时间"), getIdx(installDate)))

	ws.WriteString(fmt.Sprintf(`<row r="9">
		<c r="A9" t="s" s="1"><v>%s</v></c>
		<c r="B9" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("浏览器版本"), getIdx(browserVer)))

	ws.WriteString(fmt.Sprintf(`<row r="10">
		<c r="A10" t="s" s="1"><v>%s</v></c>
		<c r="B10" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("IP地址"), getIdx(ipAddr)))

	ws.WriteString(fmt.Sprintf(`<row r="11">
		<c r="A11" t="s" s="1"><v>%s</v></c>
		<c r="B11" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("MAC地址"), getIdx(macAddr)))

	ws.WriteString(fmt.Sprintf(`<row r="12">
		<c r="A12" t="s" s="1"><v>%s</v></c>
		<c r="B12" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("硬盘序列号"), getIdx(diskSN)))

	ws.WriteString(`<row r="13"/>`)

	ws.WriteString(fmt.Sprintf(`<row r="14">
		<c r="A14" t="s" s="1"><v>%s</v></c>
		<c r="B14" t="s" s="2"><v>%s</v></c>
	</row>`, getIdx("导出时间"), getIdx(exportTime)))

	ws.WriteString("</sheetData></worksheet>")
	x.addFile("xl/worksheets/sheet1.xml", ws.String())

	return x.writeTo(filePath)
}
