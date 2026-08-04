package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gsprobe/internal/model"
)

type provinceLatencyRow struct {
	Code     string  `json:"code"`
	Short    string  `json:"short"`
	Name     string  `json:"name"`
	Carrier  string  `json:"carrier"`
	Host     string  `json:"host"`
	Ms       float64 `json:"ms,omitempty"`
	Text     string  `json:"text"`
	Loss     float64 `json:"loss"`
	LossText string  `json:"lossText,omitempty"`
}

type routeMapNode struct {
	Region string `json:"region"`
	Label  string `json:"label"`
}

type routeRow struct {
	City     string         `json:"city"`
	Carrier  string         `json:"carrier"`
	Line     string         `json:"line"`
	PingMS   float64        `json:"pingMs,omitempty"`
	PingText string         `json:"pingText"`
	Labels   []string       `json:"labels"`
	Path     []routeMapNode `json:"path,omitempty"`
	Hops     []string       `json:"hops,omitempty"`
	Output   string         `json:"output,omitempty"`
}

func findSection(sections []model.Section, id string) (model.Section, bool) {
	for _, s := range sections {
		if s.ID == id {
			return s, true
		}
	}
	return model.Section{}, false
}

func sectionMetricText(s model.Section, name, fallback string) string {
	for _, m := range s.Metrics {
		if m.Name == name {
			if t := strings.TrimSpace(formatMetric(m)); t != "" && t != "—" {
				return t
			}
		}
	}
	return fallback
}

func parseReturnRoutes(s model.Section) []routeRow {
	raw, ok := s.Details["returnRoutes"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var rows []routeRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil
	}
	return rows
}

func parseProvinceLatency(s model.Section) []provinceLatencyRow {
	raw, ok := s.Details["chinaLatency"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var rows []provinceLatencyRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil
	}
	return rows
}

func routeLineText(r routeRow) string {
	if len(r.Labels) > 0 {
		return strings.Join(r.Labels, "→")
	}
	return r.Line
}

func provinceCellText(row provinceLatencyRow) string {
	text := strings.TrimSpace(row.Text)
	if text == "" {
		text = "—"
	}
	if row.LossText != "" {
		text += row.LossText
	}
	return text
}

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && p != "—" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

func buildSystemColumn(sections []model.Section, width int) []string {
	sys, _ := findSection(sections, "system")
	cpu, _ := findSection(sections, "cpu")
	mem, _ := findSection(sections, "memory")
	disk, _ := findSection(sections, "disk")
	hwTotal := cpu.Score + mem.Score + disk.Score
	mb := detailMap(sys, "motherboard")

	var lines []string
	lines = appendScoreLine(lines, width, "硬件总分", hwTotal)
	lines = append(lines, "")

	scoreTbl := newMatrixTable([]string{"项目", "评分"})
	scoreTbl.addRow("CPU", colorScore(cpu.Score))
	scoreTbl.addRow("内存", colorScore(mem.Score))
	scoreTbl.addRow("磁盘", colorScore(disk.Score))
	lines = appendTable(lines, width, scoreTbl)
	lines = append(lines, "")

	lines = appendHeading(lines, "系统信息")
	sysTbl := newKVTable(8)
	for _, row := range [][2]string{
		{"系统", sectionMetricText(sys, "操作系统", sys.Summary)},
		{"虚拟化", sectionMetricText(sys, "虚拟化", "")},
		{"架构", sectionMetricText(sys, "架构", "")},
		{"核心", sectionMetricText(sys, "处理器核心", "")},
		{"内存", sectionMetricText(mem, "内存总量", mem.Summary)},
		{"运行", sectionMetricText(sys, "运行时长", "")},
		{"负载", sectionMetricText(sys, "系统负载", "")},
		{"时区", sectionMetricText(sys, "区域设置", "")},
		{"容器", sectionMetricText(sys, "容器", "")},
		{"BBR", sectionMetricText(sys, "BBR", "")},
		{"TCP", sectionMetricText(sys, "TCP 拥塞算法", "")},
		{"调度", sectionMetricText(sys, "队列调度算法", "")},
		{"CPU限额", sectionMetricText(sys, "CPU 限额", "")},
		{"CPU状态", sectionMetricText(sys, "CPU 状态", "")},
		{"内存限额", sectionMetricText(sys, "内存限额", "")},
		{"内存状态", sectionMetricText(sys, "内存状态", "")},
		{"I/O限额", sectionMetricText(sys, "I/O 限额", "")},
		{"BIOS", mapString(mb, "bios")},
		{"芯片组", mapString(mb, "chipset")},
		{"网卡", mapString(mb, "network")},
	} {
		if strings.TrimSpace(row[1]) == "" {
			continue
		}
		sysTbl.addKV(row[0], row[1])
	}
	lines = appendTable(lines, width, sysTbl)
	lines = append(lines, "")

	lines = appendHeading(lines, "性能摘要")
	perfTbl := newKVTable(8)
	for _, row := range [][2]string{
		{"CPU", sectionMetricText(cpu, "型号", cpu.Summary)},
		{"规格", joinNonEmpty(
			sectionMetricText(cpu, "物理核心", ""),
			sectionMetricText(cpu, "逻辑线程", ""),
			sectionMetricText(cpu, "频率", ""),
			sectionMetricText(cpu, "利用率", ""),
		)},
		{"偷取时间", sectionMetricText(cpu, "偷取时间", "")},
		{"缓存", sectionMetricText(cpu, "缓存", "")},
		{"SHA256", sectionMetricText(cpu, "SHA-256", "")},
		{"gzip", sectionMetricText(cpu, "gzip", "")},
		{"复制", sectionMetricText(mem, "复制", "")},
		{"延迟", colorizeMsInText(sectionMetricText(mem, "随机延迟", ""))},
		{"已用", sectionMetricText(mem, "已用内存", "")},
		{"Swap", strings.TrimSpace(sectionMetricText(mem, "Swap 已用", "") + " / " + sectionMetricText(mem, "Swap 总量", ""))},
		{"内存气球", sectionMetricText(mem, "内存气球", "")},
		{"KSM", sectionMetricText(mem, "KSM", "")},
		{"顺序写", sectionMetricText(disk, "顺序写", "")},
		{"顺序读", sectionMetricText(disk, "顺序读", "")},
		{"4K读", sectionMetricText(disk, "4K 随机读", "")},
		{"4K写", sectionMetricText(disk, "4K 随机写", "")},
		{"fsync P50", colorizeMsInText(sectionMetricText(disk, "fsync P50", ""))},
		{"fsync P99", colorizeMsInText(sectionMetricText(disk, "fsync P99", ""))},
		{"磁盘", sectionMetricText(disk, "测试设备", "")},
		{"文件系统", sectionMetricText(disk, "文件系统", "")},
		{"块设备", sectionMetricText(disk, "块设备", "")},
		{"I/O调度", sectionMetricText(disk, "I/O 调度器", "")},
		{"总容量", sectionMetricText(disk, "磁盘总容量", "")},
		{"可用", sectionMetricText(disk, "磁盘可用容量", "")},
		{"使用率", sectionMetricText(disk, "磁盘使用率", "")},
		{"Inode", sectionMetricText(disk, "Inode 使用率", "")},
		{"显卡", detailString(disk.Details["gpu"])},
	} {
		val := strings.TrimSpace(strings.Trim(row[1], "/ "))
		if val == "" || val == "—" {
			continue
		}
		perfTbl.addKV(row[0], val)
	}
	for _, row := range diskProfilesLines(disk) {
		perfTbl.addKV(row[0], row[1])
	}
	if flags := cpuFlagsLine(cpu); flags != "" {
		perfTbl.addKV("指令集", flags)
	}
	lines = appendTable(lines, width, perfTbl)
	return lines
}

func buildIPColumn(sections []model.Section, width int) []string {
	ip, _ := findSection(sections, "ip-quality")
	svc, _ := findSection(sections, "services")
	base := detailMap(ip, "base")
	mail := detailMap(ip, "mail")
	bl := detailMap(ip, "blacklist")
	riskScores := detailMap(ip, "riskScores")

	var lines []string
	lines = appendScoreLine(lines, width, "评分", ip.Score)
	if summary := terminalIPSummary(ip, base); summary != "" {
		lines = append(lines, summary)
	}
	lines = append(lines, "")

	lines = appendHeading(lines, "基础信息")
	basicTbl := newKVTable(10)
	basicRows := [][2]string{
		{"IP", terminalPublicIP(base)},
		{"ASN", mapString(base, "asn")},
		{"组织", mapString(base, "organization")},
		{"ISP", mapString(base, "isp")},
		{"位置", sectionMetricText(ip, "位置", mapString(base, "city"))},
		{"坐标", mapString(base, "coordinates")},
		{"使用地区", mapString(base, "country")},
		{"注册地区", mapString(base, "regCountry")},
		{"时区", mapString(base, "timezone")},
		{"IP类型", colorStatusValue(mapString(base, "ipType"))},
	}
	if purity := sectionMetricText(ip, "IP纯净度", ""); purity != "" {
		basicRows = append(basicRows, [2]string{"IP纯净度", colorPurityValue(purity)})
	}
	basicRows = append(basicRows,
		[2]string{"代理", colorStatusValue(sectionMetricText(ip, "代理", ""))},
		[2]string{"机房", colorStatusValue(sectionMetricText(ip, "机房", ""))},
	)
	for _, row := range basicRows {
		if strings.TrimSpace(stripANSI(row[1])) == "" || row[1] == "—" {
			continue
		}
		basicTbl.addKV(row[0], row[1])
	}
	if cats := formatIPCategories(base); cats != "" {
		basicTbl.addKV("用途分类", cats)
	}
	lines = appendTable(lines, width, basicTbl)

	if rs := formatRiskScores(riskScores); len(rs) > 0 {
		lines = append(lines, "")
		lines = appendHeading(lines, "风险评分")
		riskTbl := newKVTable(12)
		for _, line := range rs {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				riskTbl.addKV(parts[0], colorStatusValue(parts[1]))
			}
		}
		lines = appendTable(lines, width, riskTbl)
	}

	lines = append(lines, "")
	lines = appendHeading(lines, "邮件与黑名单")
	mailTbl := newKVTable(10)
	mailTbl.addKV("25端口", colorStatusValue(sectionMetricText(ip, "25端口", "")))
	mailTbl.addKV("DNSBL", colorStatusValue(sectionMetricText(ip, "DNSBL", "")))
	if s := formatMailSummary(mail); s != "" {
		mailTbl.addKV("邮件", s)
	}
	if s := formatBlacklistSummary(bl); s != "" {
		mailTbl.addKV("黑名单", s)
	}
	lines = appendTable(lines, width, mailTbl)

	lines = append(lines, "")
	lines = appendHeading(lines, "流媒体 & AI")
	lines = appendScoreLine(lines, width, "评分", svc.Score)
	svcTbl := newMatrixTable([]string{"服务", "状态"})
	if items := parseServiceItems(svc); len(items) > 0 {
		for _, it := range items {
			svcTbl.addRow(it.Name, colorStatusValue(serviceStatusText(it)))
		}
	} else {
		for _, m := range svc.Metrics {
			svcTbl.addRow(m.Name, colorStatusValue(formatMetric(m)))
		}
	}
	lines = appendTable(lines, width, svcTbl)
	return lines
}

func buildNetworkColumn(sections []model.Section, width int) []string {
	net, _ := findSection(sections, "network")
	china, _ := findSection(sections, "china-latency")
	route, _ := findSection(sections, "route")
	returns := parseReturnRoutes(route)

	var lines []string
	lines = appendScoreLine(lines, width, "评分", net.Score)
	if net.Summary != "" {
		lines = append(lines, padRight(net.Summary, width))
	}
	lines = append(lines, "")

	lines = appendHeading(lines, "本地策略")
	policyTbl := newKVTable(6)
	for _, row := range [][2]string{
		{"NAT", sectionMetricText(net, "NAT", "")},
		{"拥塞", sectionMetricText(net, "TCP 拥塞", "")},
		{"BBR", colorStatusValue(sectionMetricText(net, "BBR", ""))},
	} {
		policyTbl.addKV(row[0], row[1])
	}
	lines = appendTable(lines, width, policyTbl)
	lines = append(lines, "")

	lines = appendHeading(lines, "基础连通")
	connTbl := newKVTable(8)
	for _, row := range [][2]string{
		{"IPv4", colorBoolValue(sectionMetricText(net, "IPv4", ""))},
		{"IPv6", colorBoolValue(sectionMetricText(net, "IPv6", ""))},
		{"DNS", colorizeMsInText(sectionMetricText(net, "DNS", ""))},
		{"TCP", colorizeMsInText(sectionMetricText(net, "TCP 1.1.1.1", ""))},
		{"UDP", colorizeMsInText(sectionMetricText(net, "UDP 出站", ""))},
		{"UDP NAT", sectionMetricText(net, "UDP NAT", "")},
		{"QUIC", sectionMetricText(net, "QUIC", "")},
		{"下载", sectionMetricText(net, "下载", "")},
		{"上传", sectionMetricText(net, "上传", "")},
		{"队列", sectionMetricText(net, "队列调度", "")},
	} {
		connTbl.addKV(row[0], row[1])
	}
	lines = appendTable(lines, width, connTbl)

	if intl := parseIntlLatency(net); len(intl) > 0 {
		lines = append(lines, "")
		lines = appendHeading(lines, "国际节点")
		intlTbl := newMatrixTable([]string{"节点", "延迟"})
		for _, row := range intl {
			val := strings.TrimSpace(row.Text)
			if val == "" {
				val = "—"
			}
			intlTbl.addRow(row.City, colorLatency(row.Ms, val))
		}
		lines = appendTable(lines, width, intlTbl)
	}

	if len(returns) > 0 {
		lines = append(lines, "")
		lines = appendHeading(lines, "三网延迟")
		cities := []string{"北京", "上海", "广州"}
		carriers := []string{"电信", "联通", "移动"}
		pingTbl := newMatrixTable(append([]string{""}, cities...))
		for _, carrier := range carriers {
			row := []string{carrier}
			for _, city := range cities {
				val := "—"
				var ms float64
				for _, r := range returns {
					if r.City == city && r.Carrier == carrier {
						val = strings.TrimSpace(r.PingText)
						ms = r.PingMS
						break
					}
				}
				if val == "" {
					val = "—"
				}
				row = append(row, colorLatency(ms, val))
			}
			pingTbl.addRow(row...)
		}
		lines = appendTable(lines, width, pingTbl)
	}

	lines = append(lines, "")
	lines = appendHeading(lines, "全国延迟")
	lines = appendScoreLine(lines, width, "评分", china.Score)
	if china.Summary != "" {
		lines = append(lines, padRight(china.Summary, width))
	}
	lines = append(lines, provinceMatrixLines(china, width)...)
	return lines
}

func provinceMatrixLines(s model.Section, width int) []string {
	rows := parseProvinceLatency(s)
	if len(rows) == 0 {
		return nil
	}
	carriers := []string{"电信", "联通", "移动"}
	type provRow struct {
		short string
		cells map[string]provinceLatencyRow
	}
	byCode := make(map[string]*provRow)
	var order []string
	for _, row := range rows {
		if _, ok := byCode[row.Code]; !ok {
			byCode[row.Code] = &provRow{short: row.Short, cells: map[string]provinceLatencyRow{}}
			order = append(order, row.Code)
		}
		byCode[row.Code].cells[row.Carrier] = row
	}

	headers := []string{""}
	for _, c := range carriers {
		headers = append(headers, c)
	}
	matrix := newMatrixTable(headers)
	for _, code := range order {
		p := byCode[code]
		label := p.short
		if label == "" {
			label = code
		}
		tableRow := []string{label}
		for _, carrier := range carriers {
			hit, ok := p.cells[carrier]
			if !ok {
				tableRow = append(tableRow, paint(colorMuted, "—"))
				continue
			}
			tableRow = append(tableRow, colorLatency(hit.Ms, provinceCellText(hit)))
		}
		matrix.addRow(tableRow...)
	}
	return tableLines(width, matrix)
}

func buildRouteColumn(sections []model.Section, width int) []string {
	route, ok := findSection(sections, "route")
	if !ok {
		return []string{paint(colorMuted, "无回程数据")}
	}
	returns := parseReturnRoutes(route)

	var lines []string
	lines = appendScoreLine(lines, width, "评分", route.Score)
	if route.Summary != "" {
		lines = append(lines, padRight(route.Summary, width))
	}
	if len(returns) == 0 {
		lines = append(lines, "")
		tbl := newKVTable(8)
		for _, m := range route.Metrics {
			tbl.addKV(m.Name, formatMetric(m))
		}
		lines = appendTable(lines, width, tbl)
		return lines
	}

	lines = append(lines, "")
	lines = appendHeading(lines, "简评总结")
	cities := []string{"北京", "上海", "广州"}
	carriers := []string{"电信", "联通", "移动"}
	summaryTbl := newMatrixTable(append([]string{""}, cities...))
	for _, carrier := range carriers {
		row := []string{carrier}
		for _, city := range cities {
			line := "—"
			for _, r := range returns {
				if r.City == city && r.Carrier == carrier {
					line = routeLineText(r)
					break
				}
			}
			row = append(row, colorRouteLabel(line))
		}
		summaryTbl.addRow(row...)
	}
	lines = appendTable(lines, width, summaryTbl)

	lines = append(lines, "")
	lines = appendHeading(lines, "节点详情")
	routeTbl := newMatrixTable([]string{"节点", "路线图", "延迟"})
	for _, r := range returns {
		label := r.City + r.Carrier
		ping := strings.TrimSpace(r.PingText)
		if ping == "" {
			ping = "—"
		}
		routeTbl.addRow(label, routeMapText(r), colorLatency(r.PingMS, ping))
	}
	lines = appendTable(lines, width, routeTbl)

	if hopsRows := routeHopLines(returns); len(hopsRows) > 0 {
		lines = append(lines, "")
		lines = appendHeading(lines, "IP 路径")
		hopTbl := newMatrixTable([]string{"节点", "IP 路径"})
		for _, row := range hopsRows {
			hopTbl.addRow(row[0], row[1])
		}
		lines = appendTable(lines, width, hopTbl)
	}
	return lines
}

func routeHopLines(returns []routeRow) [][2]string {
	var out [][2]string
	for _, r := range returns {
		path := formatRouteHops(r)
		if path == "" {
			continue
		}
		out = append(out, [2]string{r.City + r.Carrier, path})
	}
	return out
}

func printLatencyLegend(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s\n", paint(colorDim, "延迟图例: "+paint(colorExcellent, "≤50ms")+"  "+paint(colorGood, "51-100ms")+"  "+paint(colorFair, "101-200ms")+"  "+paint(colorWarn, "201-250ms")+"  "+paint(colorHigh, ">250ms")+"  "+paint(colorBad, "超时")))
}

func printFourColumns(w io.Writer, sections []model.Section) {
	printReportTables(w, sections)
}

func printReportTables(w io.Writer, sections []model.Section) {
	width := terminalWidth()
	blocks := []struct {
		title string
		lines []string
	}{
		{"系统报告", buildSystemColumn(sections, width)},
		{"IP 质量", buildIPColumn(sections, width)},
		{"网络质量", buildNetworkColumn(sections, width)},
		{"回程路由", buildRouteColumn(sections, width)},
	}
	for _, block := range blocks {
		fmt.Fprintln(w)
		fmt.Fprintln(w, colorHeading("▓▓ "+block.title+" ▓▓"))
		for _, line := range block.lines {
			fmt.Fprintln(w, line)
		}
	}
	printLatencyLegend(w)
}
