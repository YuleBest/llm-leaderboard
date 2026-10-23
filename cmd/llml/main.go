// llml 抓取 Artificial Analysis 模型排行榜（https://artificialanalysis.ai/zh/leaderboards/models）
// 并将主表格数据以对齐表格（默认）或 JSON（--json）打印。
//
// 用法：llml [-n N] [--json] [--no-cache] [关键词...]（关键词对模型名做大小写不敏感的包含匹配，多个为「或」关系）
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"llml/internal/leaderboard"
	"llml/internal/render"
)

// flagVisited 报告某个 flag 是否被用户显式设置过。
func flagVisited(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// applyTrailingFlags 应用混在位置参数中的 flag（如 llml "x" --json：
// flag.Parse 遇到首个非 flag 参数即停止，后面的 flag 会落到 flag.Args() 里），
// 返回剩下的关键词。
func applyTrailingFlags(args []string) []string {
	var keywords []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			keywords = append(keywords, a)
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
		val, hasVal := "true", false
		if eq := strings.Index(name, "="); eq >= 0 {
			val, name, hasVal = name[eq+1:], name[:eq], true
		}
		f := flag.Lookup(name)
		if f == nil {
			fmt.Fprintf(os.Stderr, "未定义的 flag: %s\n\n", a)
			flag.Usage()
			os.Exit(2)
		}
		if _, isBool := f.Value.(interface{ IsBoolFlag() bool }); !isBool && !hasVal {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "flag %s 缺少参数值\n", a)
				os.Exit(2)
			}
			i++
			val = args[i]
		}
		if err := flag.Set(name, val); err != nil {
			fmt.Fprintf(os.Stderr, "无效的 flag 值: %s %s\n", a, val)
			os.Exit(2)
		}
	}
	return keywords
}

func main() {
	n := flag.Int("n", 0, "显示前 N 条（默认 50，--full 时 25；0 为全部）")
	asJSON := flag.Bool("json", false, "以 JSON 格式输出（非交互式输出时默认启用）")
	var tui bool
	flag.BoolVar(&tui, "tui", false, "以表格输出（非交互式输出时默认 JSON，此选项强制表格）")
	noCache := flag.Bool("no-cache", false, "忽略缓存强制抓取，并刷新缓存（缓存有效期 6 小时）")
	var full bool
	flag.BoolVar(&full, "full", false, "打印全部列（列多一行放不下，每个模型占两行）")
	flag.BoolVar(&full, "f", false, "同 --full")
	var sortArg string
	flag.StringVar(&sortArg, "s", "", "按数值列排序，后可跟 asc|desc，如 -s speed desc（默认 intelligenceIndex 倒序）")
	flag.StringVar(&sortArg, "sort", "", "同 -s")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: llml [-n N] [--json|--tui] [--full|-f] [--no-cache] [-s <键> [asc|desc]] [关键词...]\n\n"+
			"抓取 Artificial Analysis 模型排行榜并打印。stdout 不是终端（管道/重定向）时默认输出\n"+
			"JSON，--tui 可强制表格；默认前 50 条，--full 时 25，-n 0 为全部。数据缓存 6 小时。\n"+
			"排序键支持别名（如 speed、ttft、e2e、cost、ctx、index），完整键名可用 --json 查看。\n"+
			"关键词对 slug/模型名/短名/开发商做大小写不敏感的包含匹配（多个关键词为「或」关系）。\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	keywords := applyTrailingFlags(flag.Args())

	// 非交互式（stdout 被管道/重定向）默认 JSON，便于脚本处理；-tui 可强制表格
	if !flagVisited("json") && !flagVisited("tui") {
		*asJSON = !term.IsTerminal(int(os.Stdout.Fd()))
	}

	// -s 的方向词（asc/desc）允许写成独立参数跟在键后面
	sortKey, asc := "intelligenceIndex", false
	if sortArg != "" {
		sortKey = sortArg
		if len(keywords) > 0 && (keywords[0] == "asc" || keywords[0] == "desc") {
			asc = keywords[0] == "asc"
			keywords = keywords[1:]
		}
		canon, ok := leaderboard.CanonicalSortKey(sortKey)
		if !ok {
			fmt.Fprintf(os.Stderr, "不支持的排序键: %s（可选: %s）\n", sortKey, strings.Join(leaderboard.SortableKeys(), ", "))
			os.Exit(2)
		}
		sortKey = canon
	}
	if *n == 0 && !flagVisited("n") { // 未显式指定 -n 时按模式取默认值
		if full {
			*n = 25
		} else {
			*n = 50
		}
	}

	models, fetchedAt, err := leaderboard.Load(*noCache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "抓取失败: %v\n", err)
		os.Exit(1)
	}
	total := len(models)
	models = leaderboard.Filter(models, keywords)
	matched := len(models)
	if sortArg != "" {
		if err := leaderboard.Sort(models, sortKey, asc); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
	}
	if *n > 0 && *n < len(models) {
		models = models[:*n]
	}

	if *asJSON {
		render.PrintJSON(models, full)
		return
	}

	var title string
	switch {
	case matched < total && len(models) < matched:
		title = fmt.Sprintf("前 %d / %d 条匹配 · 共 %d 条", len(models), matched, total)
	case matched < total:
		title = fmt.Sprintf("%d 条匹配 · 共 %d 条", matched, total)
	case len(models) < total:
		title = fmt.Sprintf("前 %d / 共 %d 条", len(models), total)
	default:
		title = fmt.Sprintf("共 %d 条", total)
	}
	sortName := render.KeyTitles[sortKey]
	if sortName == "" {
		sortName = sortKey
	}
	direction := "降序"
	if asc {
		direction = "升序"
	}
	fmt.Printf("Artificial Analysis 模型排行榜 · %s · 按%s%s · 数据抓取于 %s\n\n",
		title, sortName, direction, fetchedAt.Format("01-02 15:04"))
	if matched == 0 {
		fmt.Println("没有匹配的模型")
		return
	}
	if full {
		render.PrintFull(models, sortKey, asc)
	} else {
		render.PrintTable(models, sortKey, asc)
	}
}
