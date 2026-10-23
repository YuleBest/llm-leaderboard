// Package render 负责把模型列表渲染为对齐表格或 JSON。
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"llml/internal/leaderboard"
)

func fmtNum(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.4g", *v) // %.3g 会把 4 位数显示成科学计数法
}

func fmtPct(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.1f", *v*100)
}

func mark(b bool) string {
	if b {
		return "是" // 不用 ✓：其终端宽度有歧义（CJK 字体常画成全宽），会破坏列对齐
	}
	return "-"
}

func orDash(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

func fmtIndex(m leaderboard.Model) string {
	if m.IntelligenceIndex == nil {
		return "-"
	}
	s := fmt.Sprintf("%.1f", *m.IntelligenceIndex)
	if m.IntelligenceIndexEstimated {
		s += "*"
	}
	return s
}

func fmtCtx(v *int) string {
	if v == nil {
		return "-"
	}
	switch n := *v; {
	case n >= 1_000_000 && n%1_000_000 == 0:
		return fmt.Sprintf("%dM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%dK", n/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// column 描述表格中的一列：数据字段键名（数值列才参与排序）与显示标题。
type column struct {
	key   string
	title string
}

// KeyTitles 是排序键（规范名）对应的中文列名，供标题行使用。
var KeyTitles = map[string]string{
	"rank":                              "#",
	"contextWindowTokens":               "上下文窗口",
	"intelligenceIndex":                 "智能指数",
	"hle":                               "HLE(%)",
	"gpqa":                              "GPQA(%)",
	"mmmuPro":                           "MMMU-Pro(%)",
	"price1mInputTokens":                "$入/1M",
	"price1mOutputTokens":               "$出/1M",
	"intelligenceIndexCostPerTask":      "每任务($)",
	"medianOutputTokensPerSecond":       "中位数(tok/s)",
	"medianTimeToFirstTokenSeconds":     "首块延迟(s)",
	"medianEndToEndResponseTimeSeconds": "端到端(s)",
}

// headerRow 生成表头行文本，当前排序列的标题后追加方向箭头。
func headerRow(cols []column, sortKey string, asc bool) []string {
	arrow := "↓"
	if asc {
		arrow = "↑"
	}
	row := make([]string, len(cols))
	for i, c := range cols {
		row[i] = c.title
		if c.key != "" && c.key == sortKey {
			row[i] += arrow
		}
	}
	return row
}

// PrintTable 输出对齐表格，列与网页未点「展开列」时的默认列一致
// （网页最后的「进一步分析」是操作列，无数据意义，不展示）。
func PrintTable(models []leaderboard.Model, sortKey string, asc bool) {
	cols := []column{
		{"rank", "#"}, {"name", "模型"}, {"contextWindowTokens", "上下文窗口"},
		{"modelCreatorName", "开发商"}, {"intelligenceIndex", "智能指数"},
		{"intelligenceIndexCostPerTask", "每任务($)"}, {"medianOutputTokensPerSecond", "中位数(tok/s)"},
		{"medianTimeToFirstTokenSeconds", "首块延迟(s)"}, {"medianEndToEndResponseTimeSeconds", "端到端(s)"},
	}
	header := headerRow(cols, sortKey, asc)
	rows := make([][]string, 0, len(models))
	for _, m := range models {
		rows = append(rows, []string{
			fmt.Sprint(m.Rank), m.Name, fmtCtx(m.ContextWindowTokens), m.ModelCreatorName,
			fmtIndex(m), fmtNum(m.CostPerTask), fmtNum(m.MedianOutputTokensPerSec),
			fmtNum(m.MedianTimeToFirstTokenSec), fmtNum(m.MedianEndToEndResponseTimeSec),
		})
	}

	// 数字列右对齐，其余左对齐
	rightAlign := make([]bool, len(header))
	rightAlign[0] = true
	for i := 4; i < len(header); i++ {
		rightAlign[i] = true
	}

	w := make([]int, len(header))
	for i, h := range header {
		w[i] = displayWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if dw := displayWidth(cell); dw > w[i] {
				w[i] = dw
			}
		}
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		for i, cell := range cells {
			pad := w[i] - displayWidth(cell)
			if i > 0 {
				b.WriteString("  ")
			}
			if rightAlign[i] {
				b.WriteString(strings.Repeat(" ", pad) + cell)
			} else {
				b.WriteString(cell + strings.Repeat(" ", pad))
			}
		}
		b.WriteByte('\n')
	}
	writeRow(header)
	for _, row := range rows {
		writeRow(row)
	}
	b.WriteByte('\n')
	writeRow(header) // 表末再打一遍表头，便于对照长表底部的列含义
	fmt.Print(b.String())
}

// tableRow 是非 full 模式下 JSON 输出的精简视图，字段与默认表格列一致。
type tableRow struct {
	Name     string   `json:"name"`
	Creator  string   `json:"modelCreatorName"`
	Ctx      *int     `json:"contextWindowTokens"`
	Index    *float64 `json:"intelligenceIndex"`
	IndexEst bool     `json:"intelligenceIndexIsEstimated"`
	Cost     *float64 `json:"intelligenceIndexCostPerTask"`
	Speed    *float64 `json:"medianOutputTokensPerSecond"`
	TTFT     *float64 `json:"medianTimeToFirstTokenSeconds"`
	E2E      *float64 `json:"medianEndToEndResponseTimeSeconds"`
}

// PrintJSON 以 JSON 输出：full 时输出全部字段，否则只输出默认表格列对应的字段。
func PrintJSON(models []leaderboard.Model, full bool) {
	var out any
	if full {
		out = models
	} else {
		rows := make([]tableRow, len(models))
		for i, m := range models {
			rows[i] = tableRow{
				Name: m.Name, Creator: m.ModelCreatorName, Ctx: m.ContextWindowTokens,
				Index: m.IntelligenceIndex, IndexEst: m.IntelligenceIndexEstimated,
				Cost: m.CostPerTask, Speed: m.MedianOutputTokensPerSec,
				TTFT: m.MedianTimeToFirstTokenSec, E2E: m.MedianEndToEndResponseTimeSec,
			}
		}
		out = rows
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(data))
}

// PrintFull 输出全部列。列太多一行放不下，每个模型占两行、每行 8 列，
// 排名单独占一列；两行共享同一组列边界（每列宽度取两行内容的最大值），
// 因此表头与数据在两行间上下对齐。
func PrintFull(models []leaderboard.Model, sortKey string, asc bool) {
	colsA := []column{
		{"rank", "#"}, {"name", "模型"}, {"modelCreatorName", "开发商"},
		{"contextWindowTokens", "上下文窗口"}, {"intelligenceIndex", "智能指数"},
		{"hle", "HLE(%)"}, {"gpqa", "GPQA(%)"}, {"mmmuPro", "MMMU-Pro(%)"}, {"", "推理"},
	}
	colsB := []column{
		{"", ""}, {"price1mInputTokens", "$入/1M"}, {"price1mOutputTokens", "$出/1M"},
		{"intelligenceIndexCostPerTask", "每任务($)"}, {"medianOutputTokensPerSecond", "中位数(tok/s)"},
		{"medianTimeToFirstTokenSeconds", "首块延迟(s)"}, {"medianEndToEndResponseTimeSeconds", "端到端(s)"},
		{"", "规模"}, {"", "开源"},
	}
	headerA := headerRow(colsA, sortKey, asc)
	headerB := headerRow(colsB, sortKey, asc)
	right := []bool{true, false, false, true, true, true, true, true, true}

	rowsA := make([][]string, len(models))
	rowsB := make([][]string, len(models))
	for i, m := range models {
		rowsA[i] = []string{
			fmt.Sprint(m.Rank), m.Name, m.ModelCreatorName, fmtCtx(m.ContextWindowTokens),
			fmtIndex(m), fmtPct(m.HLE), fmtPct(m.GPQA), fmtPct(m.MMMUPro), mark(m.IsReasoning),
		}
		rowsB[i] = []string{
			"", fmtNum(m.Price1MInputTokens), fmtNum(m.Price1MOutputTokens), fmtNum(m.CostPerTask),
			fmtNum(m.MedianOutputTokensPerSec), fmtNum(m.MedianTimeToFirstTokenSec),
			fmtNum(m.MedianEndToEndResponseTimeSec), orDash(m.ParamClass), mark(m.IsOpenWeights),
		}
	}

	w := colWidths(headerA, append(append([][]string{headerB}, rowsA...), rowsB...))
	total := 2 * (len(w) - 1)
	for _, cw := range w {
		total += cw
	}
	line := strings.Repeat("-", total) + "\n"

	var b strings.Builder
	writeGridRow(&b, w, right, "", headerA)
	writeGridRow(&b, w, right, "", headerB)
	b.WriteString(line)
	for i := range models {
		writeGridRow(&b, w, right, "", rowsA[i])
		writeGridRow(&b, w, right, "", rowsB[i])
		if i < len(models)-1 {
			b.WriteString(line)
		}
	}
	b.WriteByte('\n')
	writeGridRow(&b, w, right, "", headerA) // 表末再打一遍表头
	writeGridRow(&b, w, right, "", headerB)
	b.WriteString(line)
	fmt.Print(b.String())
}

func colWidths(header []string, rows [][]string) []int {
	w := make([]int, len(header))
	for i, h := range header {
		w[i] = displayWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if dw := displayWidth(cell); dw > w[i] {
				w[i] = dw
			}
		}
	}
	return w
}

func writeGridRow(b *strings.Builder, w []int, right []bool, prefix string, cells []string) {
	b.WriteString(prefix)
	for i, cell := range cells {
		pad := w[i] - displayWidth(cell)
		if i > 0 {
			b.WriteString("  ")
		}
		if right[i] {
			b.WriteString(strings.Repeat(" ", pad) + cell)
		} else {
			b.WriteString(cell + strings.Repeat(" ", pad))
		}
	}
	b.WriteByte('\n')
}

// displayWidth 返回字符串在终端中的显示宽度（CJK 等全宽字符按 2 计）。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
			r >= 0x2E80 && r <= 0xA4CF, // CJK 部首、注音、CJK 符号与汉文
			r >= 0xAC00 && r <= 0xD7A3, // 韩文音节
			r >= 0xF900 && r <= 0xFAFF, // CJK 兼容表意
			r >= 0xFE30 && r <= 0xFE4F,
			r >= 0xFF00 && r <= 0xFF60, // 全角形式
			r >= 0xFFE0 && r <= 0xFFE6,
			r >= 0x20000 && r <= 0x3FFFD:
			w += 2
		default:
			w++
		}
	}
	return w
}
