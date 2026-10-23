package render

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"llml/internal/leaderboard"
)

func testModels() []leaderboard.Model {
	v := func(f float64) *float64 { return &f }
	i := func(n int) *int { return &n }
	s := func(str string) *string { return &str }
	return []leaderboard.Model{
		{Slug: "a", Name: "模型甲", ModelCreatorName: "Acme", ContextWindowTokens: i(1_000_000),
			IntelligenceIndex: v(57.5), CostPerTask: v(5.98), MedianOutputTokensPerSec: v(1541.35),
			MedianTimeToFirstTokenSec: v(0.55), MedianEndToEndResponseTimeSec: v(6.5),
			HLE: v(0.618), GPQA: v(0.83), MMMUPro: v(0.88),
			IsReasoning: true, ParamClass: s("large")},
		{Slug: "b", Name: "Model B", ModelCreatorName: "Beta", ContextWindowTokens: i(1048576),
			IntelligenceIndex: v(42.0)},
	}
}

func TestDisplayWidth(t *testing.T) {
	cases := map[string]int{
		"": 0, "abc": 3, "模型": 4, "智能指数": 8,
		"中英mixed": 9, "$入/1M": 6,
	}
	for in, want := range cases {
		if got := displayWidth(in); got != want {
			t.Errorf("displayWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestFormatters(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	if got := fmtNum(f(1541.35)); got != "1541" {
		t.Errorf("fmtNum(1541.35) = %q, want 1541", got)
	}
	if got := fmtNum(f(0.0502)); got != "0.0502" {
		t.Errorf("fmtNum(0.0502) = %q, want 0.0502", got)
	}
	if got := fmtNum(nil); got != "-" {
		t.Errorf("fmtNum(nil) = %q", got)
	}
	if got := fmtPct(f(0.618)); got != "61.8" {
		t.Errorf("fmtPct(0.618) = %q, want 61.8", got)
	}
	i := func(n int) *int { return &n }
	if got := fmtCtx(i(1000000)); got != "1M" {
		t.Errorf("fmtCtx(1000000) = %q, want 1M", got)
	}
	if got := fmtCtx(nil); got != "-" {
		t.Errorf("fmtCtx(nil) = %q", got)
	}
	if got := fmtIndex(leaderboard.Model{IntelligenceIndex: f(57.5), IntelligenceIndexEstimated: true}); got != "57.5*" {
		t.Errorf("fmtIndex estimated = %q, want 57.5*", got)
	}
	if got := mark(true); got != "是" {
		t.Errorf("mark(true) = %q", got)
	}
	if got := orDash(nil); got != "-" {
		t.Errorf("orDash(nil) = %q", got)
	}
}

func TestHeaderRowArrow(t *testing.T) {
	cols := []column{{"rank", "#"}, {"name", "模型"}, {"speed", "速度"}}
	row := headerRow(cols, "speed", false)
	if row[2] != "速度↓" {
		t.Errorf("desc arrow = %q, want 速度↓", row[2])
	}
	row = headerRow(cols, "speed", true)
	if row[2] != "速度↑" {
		t.Errorf("asc arrow = %q, want 速度↑", row[2])
	}
	row = headerRow(cols, "speed", false)
	if row[0] != "#" || row[1] != "模型" {
		t.Errorf("non-sorted columns should stay plain: %v", row)
	}
}

// 所有数据行的显示宽度必须与表头一致，否则终端列会错位。
func TestPrintTableAlignment(t *testing.T) {
	restore := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	PrintTable(testModels(), "intelligenceIndex", false)
	w.Close()
	os.Stdout = restore
	out, _ := io.ReadAll(r)

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	widths := map[int]bool{}
	for _, ln := range lines {
		if ln == "" {
			continue
		}
		widths[displayWidth(ln)] = true
	}
	if len(widths) != 1 {
		t.Errorf("table lines have inconsistent widths %v:\n%s", widths, out)
	}
	if !bytes.Contains(out, []byte("智能指数↓")) {
		t.Error("sort arrow missing in output")
	}
}

func TestPrintFullAlignment(t *testing.T) {
	restore := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	PrintFull(testModels(), "medianOutputTokensPerSecond", true)
	w.Close()
	os.Stdout = restore
	out, _ := io.ReadAll(r)

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	sepWidths := widthsOf(lines, func(i int) bool { return strings.HasPrefix(lines[i], "-") })
	lineWidths := widthsOf(lines, func(i int) bool { return !strings.HasPrefix(lines[i], "-") && lines[i] != "" })
	if len(sepWidths) != 1 {
		t.Errorf("separator widths inconsistent: %v", sepWidths)
	}
	if len(lineWidths) != 1 {
		t.Errorf("row widths inconsistent: %v\n%s", lineWidths, out)
	}
	if !bytes.Contains(out, []byte("中位数(tok/s)↑")) {
		t.Error("sort arrow missing in full output")
	}
	if n := bytes.Count(out, []byte("\n----")); n != 3 { // 表头下 1 条 + 模型间 1 条 + 表尾 1 条
		t.Errorf("separator count = %d, want 3", n)
	}
}

func widthsOf(lines []string, pick func(int) bool) map[int]bool {
	ws := map[int]bool{}
	for i, ln := range lines {
		if pick(i) {
			ws[displayWidth(ln)] = true
		}
	}
	return ws
}
