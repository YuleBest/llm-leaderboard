// Package leaderboard 负责从 Artificial Analysis 模型排行榜页面抓取并解析模型数据。
package leaderboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const URL = "https://artificialanalysis.ai/zh/leaderboards/models"

const cacheTTL = 6 * time.Hour

type Model struct {
	Slug                          string   `json:"slug"`
	Name                          string   `json:"name"`
	ShortName                     string   `json:"shortName"`
	Deprecated                    bool     `json:"deprecated"`
	IsReasoning                   bool     `json:"isReasoning"`
	IsOpenWeights                 bool     `json:"isOpenWeights"`
	ParamClass                    *string  `json:"paramClass"`
	PriceClass                    string   `json:"priceClass"`
	ModelCreatorName              string   `json:"modelCreatorName"`
	ContextWindowTokens           *int     `json:"contextWindowTokens"`
	IntelligenceIndex             *float64 `json:"intelligenceIndex"`
	IntelligenceIndexEstimated    bool     `json:"intelligenceIndexIsEstimated"`
	HLE                           *float64 `json:"hle"`
	GPQA                          *float64 `json:"gpqa"`
	MMMUPro                       *float64 `json:"mmmuPro"`
	Price1MInputTokens            *float64 `json:"price1mInputTokens"`
	Price1MOutputTokens           *float64 `json:"price1mOutputTokens"`
	CostPerTask                   *float64 `json:"intelligenceIndexCostPerTask"`
	MedianOutputTokensPerSec      *float64 `json:"medianOutputTokensPerSecond"`
	MedianTimeToFirstTokenSec     *float64 `json:"medianTimeToFirstTokenSeconds"`
	MedianEndToEndResponseTimeSec *float64 `json:"medianEndToEndResponseTimeSeconds"`

	Rank int `json:"-"` // 总表中的排名（按智能指数降序），本地计算，非网站数据
}

// Fetch 抓取排行榜页面 HTML。
func fetch() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	client := &http.Client{Timeout: 60 * time.Second} // 默认 Transport 已按 HTTP(S)_PROXY 环境变量走代理
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// Parse 从页面 HTML 中提取主表格的模型列表，按智能指数降序排列。
func parse(html []byte) ([]Model, error) {
	arr, err := extractModelsArray(concatFlightStream(html))
	if err != nil {
		return nil, err
	}
	// 整段内容包上引号按 JSON 字符串解码，即完成反转义，得到真正的 JSON 数组
	var unescaped string
	if err := json.Unmarshal(append(append([]byte{'"'}, arr...), '"'), &unescaped); err != nil {
		return nil, fmt.Errorf("反转义数据失败: %w", err)
	}
	var models []Model
	if err := json.Unmarshal([]byte(unescaped), &models); err != nil {
		return nil, fmt.Errorf("解析模型数据失败: %w", err)
	}
	sort.SliceStable(models, func(a, b int) bool {
		x, y := models[a].IntelligenceIndex, models[b].IntelligenceIndex
		if x != nil && y != nil {
			if *x != *y {
				return *x > *y
			}
			return models[a].Name < models[b].Name
		}
		return x != nil
	})
	for i := range models {
		models[i].Rank = i + 1
	}
	return models, nil
}

// Filter 返回 slug/name/shortName/modelCreatorName 任一字段包含任一关键词
// （大小写不敏感）的模型；无关键词时原样返回。
func Filter(models []Model, keywords []string) []Model {
	if len(keywords) == 0 {
		return models
	}
	out := make([]Model, 0, len(models))
	for _, m := range models {
		hay := strings.ToLower(m.Slug + "\x00" + m.Name + "\x00" + m.ShortName + "\x00" + m.ModelCreatorName)
		for _, kw := range keywords {
			if strings.Contains(hay, strings.ToLower(kw)) {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// Load 返回模型列表及其抓取时间：缓存（6 小时内）可用时直接读缓存，
// 否则抓取页面解析并写入缓存。noCache 为 true 时跳过缓存强制抓取，并刷新缓存。
func Load(noCache bool) ([]Model, time.Time, error) {
	path, cacheErr := cacheFile()
	if cacheErr == nil && !noCache {
		if models, at, ok := readCache(path); ok {
			return models, at, nil
		}
	}
	html, err := fetch()
	if err != nil {
		return nil, time.Time{}, err
	}
	models, err := parse(html)
	if err != nil {
		return nil, time.Time{}, err
	}
	at := time.Now()
	if cacheErr == nil {
		writeCache(path, models)
	}
	return models, at, nil
}

// cacheFile 返回缓存文件路径（$XDG_CACHE_HOME 或 ~/.cache 下的 llml/models.json）。
func cacheFile() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "llml", "models.json"), nil
}

// readCache 读取未过期且内容完好的缓存，返回抓取时间（即缓存文件写入时间）；
// 缓存中不含 Rank，按存储顺序（已排序）回填。
func readCache(path string) ([]Model, time.Time, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) >= cacheTTL {
		return nil, time.Time{}, false
	}
	var models []Model
	if json.Unmarshal(data, &models) != nil {
		return nil, time.Time{}, false
	}
	for i := range models {
		models[i].Rank = i + 1
	}
	return models, info.ModTime(), true
}

func writeCache(path string, models []Model) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 无法创建缓存目录: %v\n", err)
		return
	}
	data, err := json.Marshal(models)
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 无法写入缓存: %v\n", err)
	}
}

// sortAliases 把排序键别名（小写）映射为规范字段名。
var sortAliases = map[string]string{
	"rank":                              "rank",
	"ctx":                               "contextWindowTokens",
	"contextwindowtokens":               "contextWindowTokens",
	"index":                             "intelligenceIndex",
	"intelligenceindex":                 "intelligenceIndex",
	"hle":                               "hle",
	"gpqa":                              "gpqa",
	"mmmupro":                           "mmmuPro",
	"pricein":                           "price1mInputTokens",
	"price1minputtokens":                "price1mInputTokens",
	"priceout":                          "price1mOutputTokens",
	"price1moutputtokens":               "price1mOutputTokens",
	"cost":                              "intelligenceIndexCostPerTask",
	"costpertask":                       "intelligenceIndexCostPerTask",
	"intelligenceindexcostpertask":      "intelligenceIndexCostPerTask",
	"speed":                             "medianOutputTokensPerSecond",
	"medianoutputtokenspersecond":       "medianOutputTokensPerSecond",
	"ttft":                              "medianTimeToFirstTokenSeconds",
	"latency":                           "medianTimeToFirstTokenSeconds",
	"mediantimetofirsttokenseconds":     "medianTimeToFirstTokenSeconds",
	"e2e":                               "medianEndToEndResponseTimeSeconds",
	"endtoend":                          "medianEndToEndResponseTimeSeconds",
	"medianendtoendresponsetimeseconds": "medianEndToEndResponseTimeSeconds",
}

var sortGetters = map[string]func(Model) *float64{
	"rank":                              func(m Model) *float64 { v := float64(m.Rank); return &v },
	"contextWindowTokens":               func(m Model) *float64 { return intPtrToFloat(m.ContextWindowTokens) },
	"intelligenceIndex":                 func(m Model) *float64 { return m.IntelligenceIndex },
	"hle":                               func(m Model) *float64 { return m.HLE },
	"gpqa":                              func(m Model) *float64 { return m.GPQA },
	"mmmuPro":                           func(m Model) *float64 { return m.MMMUPro },
	"price1mInputTokens":                func(m Model) *float64 { return m.Price1MInputTokens },
	"price1mOutputTokens":               func(m Model) *float64 { return m.Price1MOutputTokens },
	"intelligenceIndexCostPerTask":      func(m Model) *float64 { return m.CostPerTask },
	"medianOutputTokensPerSecond":       func(m Model) *float64 { return m.MedianOutputTokensPerSec },
	"medianTimeToFirstTokenSeconds":     func(m Model) *float64 { return m.MedianTimeToFirstTokenSec },
	"medianEndToEndResponseTimeSeconds": func(m Model) *float64 { return m.MedianEndToEndResponseTimeSec },
}

func intPtrToFloat(p *int) *float64 {
	if p == nil {
		return nil
	}
	v := float64(*p)
	return &v
}

// CanonicalSortKey 把用户输入的排序键（含别名、大小写不敏感）归一为规范字段名。
func CanonicalSortKey(k string) (string, bool) {
	c, ok := sortAliases[strings.ToLower(k)]
	return c, ok
}

// SortableKeys 返回全部可排序的规范键名，用于错误提示。
func SortableKeys() []string {
	keys := make([]string, 0, len(sortGetters))
	for k := range sortGetters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Sort 按指定规范键排序 models；空值（无数据的模型）始终排在末尾。
func Sort(models []Model, key string, asc bool) error {
	get, ok := sortGetters[key]
	if !ok {
		return fmt.Errorf("不支持的排序键: %s（可选: %s）", key, strings.Join(SortableKeys(), ", "))
	}
	sort.SliceStable(models, func(a, b int) bool {
		x, y := get(models[a]), get(models[b])
		if x == nil || y == nil {
			return x != nil // 空值排最后，与方向无关
		}
		if asc {
			return *x < *y
		}
		return *x > *y
	})
	return nil
}

// concatFlightStream 把页面里所有 self.__next_f.push([1,"..."]) 的字符串内容
// 按顺序拼接成完整的 RSC flight 流。数据数组可能被任意截断分块，必须先拼接。
func concatFlightStream(html []byte) []byte {
	marker := []byte(`self.__next_f.push([1,"`)
	var stream []byte
	for pos := 0; ; {
		idx := bytes.Index(html[pos:], marker)
		if idx < 0 {
			break
		}
		i := pos + idx + len(marker)
		j := i
		for j < len(html) && html[j] != '"' {
			if html[j] == '\\' {
				j++ // 跳过转义对
			}
			j++
		}
		stream = append(stream, html[i:j]...)
		pos = j
	}
	return stream
}

// extractModelsArray 在 flight 流中定位 initialModels 数组并返回其内容。
// 流中的 JSON 还带一层 JS 字符串转义（\" 为字符串边界、\\ 为内容转义、
// \/ 为 script 安全转义），括号匹配需按该层处理。
func extractModelsArray(stream []byte) ([]byte, error) {
	key := []byte(`initialModels\":[`)
	start := bytes.Index(stream, key)
	if start < 0 {
		return nil, fmt.Errorf("页面中未找到 initialModels 数据（网站结构可能已变更）")
	}
	i := start + len(key) - 1 // 指向 '['
	depth, inStr := 0, false
	for i < len(stream) {
		c := stream[i]
		if c == '\\' {
			switch next := stream[i+1]; next {
			case '\\': // 内容层转义序列：跳过 \\ 加被转义字符（\uXXXX 再加 4 位）
				i += 2
				if i < len(stream) && stream[i] == 'u' {
					i += 4
				}
				i++
			case '"': // 字符串边界
				inStr = !inStr
				i += 2
			default: // 例如 \/
				i += 2
			}
			continue
		}
		if !inStr {
			if c == '[' || c == '{' {
				depth++
			} else if c == ']' || c == '}' {
				depth--
				if depth == 0 {
					return stream[start+len(key)-1 : i+1], nil
				}
			}
		}
		i++
	}
	return nil, fmt.Errorf("initialModels 数组边界不完整")
}
