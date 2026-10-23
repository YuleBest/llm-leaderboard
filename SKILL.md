---
name: llm-leaderboard
description: "一个用于查询大语言模型（LLM）能力数据的 CLI，基于 Artificial Analysis 权威数据，支持 JSON 结构化输出。"
---

# llm-leaderboard

## 输出演示

```
~ > llml
Artificial Analysis 模型排行榜 · 前 50 / 共 272 条 · 按智能指数降序 · 数据抓取于 10-24 04:10

 #  模型                                          上下文窗口  开发商       智能指数↓  每任务($)  中位数(tok/s)  首块延迟(s)  端到端(s)
 1  Claude Opus 5.5 (Max, Default Fallback)       1M          Anthropic         57.6      5.982          96.14        588.7      593.9
 2  Claude Sonnet 5.5 (Max, Default Fallback)     1M          Anthropic         56.0      5.461            132        373.4      377.2
 3  Claude Opus 5.5 (Xhigh, Default Fallback)     1M          Anthropic         56.0      3.459          82.19        126.3      132.4
 4  Muse Wonder Full (Max)                        1M          Meta              55.8          -              -            -          -
 5  Kimi K3.1 Pro                                 1M          Kimi              55.1      2.115          77.09        3.085      35.51
 6  Claude Opus 5.5 (High, Default Fallback)      1M          Anthropic         53.6      1.823          76.76         21.5      28.01
 7  Claude Fable 5.1 (Max, Default Fallback)      1M          Anthropic         53.4       7.63           68.4        164.5      171.8
 8  Claude Fable 5.1 (Xhigh, Default Fallback)    1M          Anthropic         53.2      5.978          64.74        54.39      62.12
 9  GPT-6 Astra (Max)                             1M          OpenAI            52.7      3.258          56.48          247      255.8
10  Gemini 4 Argon (High)                         1M          Google            52.6       1.99          81.73        53.92      60.04
```

```
~ > llml --json -n 5
[
  {
    "name": "Claude Opus 5.5 (Max, Default Fallback)",
    "modelCreatorName": "Anthropic",
    "contextWindowTokens": 1000000,
    "intelligenceIndex": 57.6223698102963,
    "intelligenceIndexIsEstimated": false,
    "intelligenceIndexCostPerTask": 5.982012019521066,
    "medianOutputTokensPerSecond": 96.1390422750912,
    "medianTimeToFirstTokenSeconds": 588.677757639,
    "medianEndToEndResponseTimeSeconds": 593.8785583560417
  },
  {
    "name": "Claude Sonnet 5.5 (Max, Default Fallback)",
    "modelCreatorName": "Anthropic",
    "contextWindowTokens": 1000000,
    "intelligenceIndex": 56.0001286580794,
    "intelligenceIndexIsEstimated": false,
    "intelligenceIndexCostPerTask": 5.4608525948098325,
    "medianOutputTokensPerSecond": 132.020524509377,
    "medianTimeToFirstTokenSeconds": 373.430792541,
    "medianEndToEndResponseTimeSeconds": 377.2180824480666
  },
  {
    "name": "Claude Opus 5.5 (Xhigh, Default Fallback)",
    "modelCreatorName": "Anthropic",
    "contextWindowTokens": 1000000,
    "intelligenceIndex": 55.9873505840139,
    "intelligenceIndexIsEstimated": false,
    "intelligenceIndexCostPerTask": 3.459110175822289,
    "medianOutputTokensPerSecond": 82.1853521212302,
    "medianTimeToFirstTokenSeconds": 126.281606047,
    "medianEndToEndResponseTimeSeconds": 132.36541523069076
  },
  {
    "name": "Muse Wonder Full (Max)",
    "modelCreatorName": "Meta",
    "contextWindowTokens": 1000000,
    "intelligenceIndex": 55.8021199855616,
    "intelligenceIndexIsEstimated": false,
    "intelligenceIndexCostPerTask": null,
    "medianOutputTokensPerSecond": null,
    "medianTimeToFirstTokenSeconds": null,
    "medianEndToEndResponseTimeSeconds": null
  },
  {
    "name": "Kimi K3.1 Pro",
    "modelCreatorName": "Kimi",
    "contextWindowTokens": 1000000,
    "intelligenceIndex": 55.0869210959395,
    "intelligenceIndexIsEstimated": false,
    "intelligenceIndexCostPerTask": 2.1145052918767275,
    "medianOutputTokensPerSecond": 77.0917244477023,
    "medianTimeToFirstTokenSeconds": 3.08462318100001,
    "medianEndToEndResponseTimeSeconds": 35.51352547771744
  }
]
```

---

## 功能

- 默认输出：模型、上下文窗口、开发商、智能指数、每任务成本、输出速度、首块延迟、端到端响应
- `--full` 打印全部列
- 关键词筛选
- `-s <键> [asc|desc]` 按任意数值列排序
- `--json` 输出 JSON（配合 `--full` 输出全部字段）
- 数据本地缓存 6 小时

## 用法

```sh
llml                         # 智能指数降序，前 50 条（--full 时默认 25 条）
llml -n 0                    # 全部
llml -n 10 claude deepseek   # 前 10 条，名称含 claude 或 deepseek
llml --full -n 5             # 全部列，两行一组
llml -s speed desc           # 按输出速度降序
llml -s cost asc             # 每任务成本升
llml --json -n 3 gemini      # JSON 输出，配合 jq 使用
llml --no-cache              # 忽略缓存强制抓取并刷新
```

排序键支持完整字段名与别名（大小写不敏感）：

| 字段 | 别名 |
| --- | --- |
| intelligenceIndex（智能指数） | `index` |
| contextWindowTokens（上下文窗口） | `ctx` |
| intelligenceIndexCostPerTask（每任务成本） | `cost` |
| medianOutputTokensPerSecond（输出速度） | `speed` |
| medianTimeToFirstTokenSeconds（首块延迟） | `ttft` / `latency` |
| medianEndToEndResponseTimeSeconds（端到端响应） | `e2e` / `endtoend` |
| 每百万 token 输入 / 输出价格 | `pricein` / `priceout` |
| 基准分数 | `hle` / `gpqa` / `mmmupro` |
