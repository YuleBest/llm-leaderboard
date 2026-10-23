package leaderboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// flightFixture 构造模拟页面：把 flight 流内容按 JS 字符串规则分块塞进 self.__next_f.push。
func flightFixture(chunks ...string) []byte {
	var html []byte
	for _, c := range chunks {
		html = append(html, `<script>self.__next_f.push([1,"`...)
		html = append(html, c...)
		html = append(html, `"])</script>`...)
	}
	return html
}

func TestParseFlightStream(t *testing.T) {
	// 数组被拆成两块推送；第二条记录的 name 含转义引号（L0 层 A\"q\" -> JSON 值 A"q"）
	html := flightFixture(
		`1:[\"$\",\"div\",null,{\"initialModels\":[{\"slug\":\"m1\",\"name\":\"Model One\",\"shortName\":\"M1\",\"modelCreatorName\":\"Acme\",\"intelligenceIndex\":10.5,\"medianReasoningTimeSeconds\":null},`,
		`{\"slug\":\"m2\",\"name\":\"A\\\"q\\\"\",\"modelCreatorName\":\"Beta\",\"intelligenceIndex\":20.5,\"medianReasoningTimeSeconds\":2}]}`,
	)
	models, err := parse(html)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].Name != `A"q"` {
		t.Errorf("escaped name = %q, want %q", models[0].Name, `A"q"`)
	}
	// 按智能指数降序排名：Beta(20.5) 第 1，Acme(10.5) 第 2
	if models[0].ModelCreatorName != "Beta" || models[0].Rank != 1 {
		t.Errorf("rank/order wrong: %+v", models[0])
	}
	if models[1].Rank != 2 {
		t.Errorf("models[1].Rank = %d, want 2", models[1].Rank)
	}
}

func TestParseMissingData(t *testing.T) {
	if _, err := parse([]byte(`<html>no flight data</html>`)); err == nil {
		t.Fatal("want error for page without initialModels")
	}
}

func TestFilter(t *testing.T) {
	models := []Model{
		{Slug: "m-1", Name: "Alpha Pro", ShortName: "AP", ModelCreatorName: "Acme"},
		{Slug: "m-2", Name: "Beta Max", ShortName: "BM", ModelCreatorName: "Beta"},
		{Slug: "m-3", Name: "Gamma", ShortName: "alpha-lite", ModelCreatorName: "Beta"},
	}
	cases := []struct {
		name     string
		keywords []string
		want     []string // slug 列表
	}{
		{"无关键词返回全部", nil, []string{"m-1", "m-2", "m-3"}},
		{"按 name 大小写不敏感", []string{"ALPHA pro"}, []string{"m-1"}},
		{"按 shortName 命中", []string{"alpha"}, []string{"m-1", "m-3"}},
		{"按 creator 命中", []string{"beta"}, []string{"m-2", "m-3"}},
		{"多关键词为或", []string{"gamma", "acme"}, []string{"m-1", "m-3"}},
		{"字段边界不拼接误匹配", []string{"pro max"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Filter(models, c.keywords)
			if len(got) != len(c.want) {
				t.Fatalf("got %d, want %d (%v)", len(got), len(c.want), got)
			}
			for i, m := range got {
				if m.Slug != c.want[i] {
					t.Errorf("got[%d] = %s, want %s", i, m.Slug, c.want[i])
				}
			}
		})
	}
}

func TestSort(t *testing.T) {
	v1, v2, v3 := 1.0, 2.0, 3.0
	models := []Model{
		{Name: "b", IntelligenceIndex: &v2},
		{Name: "nil"},
		{Name: "c", IntelligenceIndex: &v3},
		{Name: "a", IntelligenceIndex: &v1},
	}

	if err := Sort(models, "intelligenceIndex", false); err != nil {
		t.Fatal(err)
	}
	want := []string{"c", "b", "a", "nil"}
	for i, m := range models {
		if m.Name != want[i] {
			t.Fatalf("desc: got[%d]=%s, want %s", i, m.Name, want[i])
		}
	}

	if err := Sort(models, "intelligenceIndex", true); err != nil {
		t.Fatal(err)
	}
	want = []string{"a", "b", "c", "nil"} // 空值始终排最后
	for i, m := range models {
		if m.Name != want[i] {
			t.Fatalf("asc: got[%d]=%s, want %s", i, m.Name, want[i])
		}
	}

	if err := Sort(models, "no-such-key", false); err == nil {
		t.Fatal("want error for unknown key")
	}
}

func TestCanonicalSortKey(t *testing.T) {
	cases := map[string]string{
		"speed":   "medianOutputTokensPerSecond",
		"SPEED":   "medianOutputTokensPerSecond",
		"ttft":    "medianTimeToFirstTokenSeconds",
		"latency": "medianTimeToFirstTokenSeconds",
		"cost":    "intelligenceIndexCostPerTask",
		"ctx":     "contextWindowTokens",
		"index":   "intelligenceIndex",
		"e2e":     "medianEndToEndResponseTimeSeconds",
		"gpqa":    "gpqa",
		"zzz":     "",
	}
	for in, want := range cases {
		got, ok := CanonicalSortKey(in)
		if want == "" {
			if ok {
				t.Errorf("CanonicalSortKey(%q) = %q, want not found", in, got)
			}
			continue
		}
		if got != want {
			t.Errorf("CanonicalSortKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCacheRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	v := 42.5
	models := []Model{{Slug: "a", Name: "A", IntelligenceIndex: &v}}

	writeCache(path, models)
	got, at, ok := readCache(path)
	if !ok {
		t.Fatal("readCache should hit a fresh cache")
	}
	if len(got) != 1 || got[0].Name != "A" {
		t.Fatalf("cache content wrong: %+v", got)
	}
	if got[0].Rank != 1 {
		t.Errorf("Rank should be rebuilt from order, got %d", got[0].Rank)
	}
	if time.Since(at) > time.Minute {
		t.Errorf("fetchedAt too old: %v", at)
	}

	// 过期缓存不吃
	if err := os.Chtimes(path, time.Now().Add(-7*time.Hour), time.Now().Add(-7*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := readCache(path); ok {
		t.Error("expired cache should miss")
	}

	// 损坏缓存不吃
	if err := os.WriteFile(path, []byte("{\"broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := readCache(path); ok {
		t.Error("corrupted cache should miss")
	}
}

// 确保 Model 序列化用于缓存/JSON 时字段名与网站数据一致。
func TestModelJSONTags(t *testing.T) {
	data, err := json.Marshal(Model{Slug: "x"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"slug"`, `"intelligenceIndexCostPerTask"`, `"medianEndToEndResponseTimeSeconds"`} {
		if !contains(string(data), key) {
			t.Errorf("marshaled model missing %s", key)
		}
	}
	if contains(string(data), `"rank"`) {
		t.Error("rank should not be serialized")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
