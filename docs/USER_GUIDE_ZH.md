# Codetrip 用户手册

Codetrip 是混合图代码智能引擎，正式支持 Go LIB、CLI 和 MCP 三种集成方式。三者共享相同的仓库快照和 `Trip` 公共能力。

## 数据与更新模型

默认数据目录是 `~/.codetrip`，可以使用全局参数 `--trip-dir` 修改。

```text
db/       权威图数据、邻接索引、元数据和活动指针
index/    带版本的符号索引
content/  带版本的源码索引
vectors/  可选语义数据
```

每个逻辑仓库只指向一个不可变物理快照。`index --replace` 会完整构建新图和全部派生索引，成功后才发布活动指针，并回收旧快照；不会局部修改当前活动快照。

## CLI

业务命令统一使用单个单词：

| 命令 | 用途 |
|---|---|
| `index` | 解析并持久化仓库 |
| `search` | 搜索符号及元数据 |
| `source` | 搜索文件名和源码内容 |
| `embed` | 构建仓库语义数据 |
| `hybrid` | 融合符号和语义检索 |
| `traverse` | 从节点执行有界 BFS |
| `path` | 查找最短有向路径 |
| `export` | 将持久化图导出为 CSV |
| `list` | 列出活动逻辑仓库 |
| `mcp` | 启动 stdio MCP 服务 |
| `version` | 输出构建版本 |

### 索引与完整替换

```bash
codetrip index /src/project --repo project
codetrip index /src/project --repo project --replace
```

| 参数 | 含义 |
|---|---|
| `--repo` | 逻辑仓库名，默认使用源码目录名 |
| `--replace` | 原子发布完整替换快照 |
| `--export` | 输出解析验证 CSV |
| `--export-strict` | 验证 CSV 失败时让索引任务失败 |

### 搜索

```bash
codetrip search "ParseConfig" --repo project --limit 20
codetrip source 'lang:go file:config ParseConfig' --repo project --context 2
```

`search` 面向符号和元数据；`source` 面向文件名及源码内容，支持文本、正则、文件和语言过滤。

Linux 和 macOS 使用原生高吞吐源码检索后端。Windows 使用便携全文检索后端召回文件，再执行精确行匹配；公共 API 和查询能力保持一致，但在大型仓库上的源码搜索速度会更慢。Windows 与 Linux/macOS 的源码索引格式不同，跨平台迁移数据目录后需要重新构建仓库快照。

### 语义与混合检索

```bash
codetrip embed --repo project \
  --endpoint http://localhost:11434/v1/embeddings \
  --model nomic-embed-text --dimensions 768

codetrip hybrid "configuration loading" --repo project \
  --endpoint http://localhost:11434/v1/embeddings \
  --model nomic-embed-text --dimensions 768
```

`embed` 可使用 `--quantize-int8` 构建紧凑语义数据。API Key 可通过 `--api-key` 或 `CODETRIP_EMBEDDING_API_KEY` 提供。

### 图遍历

```bash
codetrip traverse NODE_ID --repo project --direction both --depth 3
codetrip traverse NODE_ID --repo project --relations CALLS,IMPORTS
codetrip path SOURCE_ID TARGET_ID --repo project
```

方向支持 `out`、`in` 和 `both`，引擎会执行可配置的访问节点上限。

### CSV

解析验证 CSV 用于语言精调：

```bash
codetrip index /src/project --repo project \
  --export ./validation-output/project --export-strict
```

完整 CSV 反映活动快照中实际持久化的数据：

```bash
codetrip export --repo project --output ./exports/project
```

输出包括 `nodes.csv`、`edges.csv` 和带行数及 SHA-256 的 `manifest.json`。

## Go LIB

### 打开与配置

```go
engine, err := codetrip.Open("./.codetrip",
    codetrip.WithCacheSize(512<<20),
    codetrip.WithMaxConcurrentIndex(2),
    codetrip.WithNodeCacheSize(500_000),
    codetrip.WithTraversalLimit(100_000),
    codetrip.WithScalePreset(codetrip.ScaleMedium),
)
if err != nil { /* 处理错误 */ }
defer engine.Close()
```

### 索引和仓库信息

```go
result, err := engine.IndexRepo(ctx, "/src/project",
    codetrip.WithRepoName("project"),
    codetrip.WithReplaceExisting(true),
    codetrip.WithIndexTimeout(30*time.Minute),
    codetrip.WithCSVExport("./validation-output/project"),
    codetrip.WithCSVExportStrict(true),
)

repositories, err := engine.ListRepos()
metrics := engine.GetMetrics()
```

### 搜索和图查询

```go
symbols, err := engine.Search(ctx, &codetrip.SearchRequest{
    Repo: "project", Query: "ParseConfig", Limit: 20,
})

source, err := engine.SearchSource(ctx, &codetrip.SourceSearchRequest{
    Repo: "project", Query: "lang:go ParseConfig", Limit: 20, ContextLines: 2,
})

nodes, err := engine.Traverse(ctx, &codetrip.TraverseRequest{
    Repo: "project", StartNodeID: nodeID,
    Direction: codetrip.TraverseAny, MaxDepth: 3,
    RelationTypes: []string{"CALLS"},
})

path, err := engine.ShortestPath(ctx, &codetrip.PathRequest{
    Repo: "project", SourceNodeID: sourceID, TargetNodeID: targetID,
})
```

### 语义 API

可以实现 `codetrip.Embedder`，也可以使用内置 HTTP 实现：

```go
embedder := codetrip.NewHTTPEmbedder(endpoint, model, apiKey, 768)

embedded, err := engine.EmbedRepo(ctx, "project", embedder, &codetrip.EmbedOptions{
    QuantizeInt8: true,
})

if err := engine.AttachEmbedder("project", embedder); err != nil { /* 处理错误 */ }
hybrid, err := engine.HybridSearch(ctx, &codetrip.HybridSearchRequest{
    Repo: "project", Query: "configuration loading", Limit: 20,
})
```

### 导出持久化图

```go
manifest, err := engine.ExportFullCSV("project", "./exports/project")
```

Go LIB 只暴露领域请求和结果类型，不公开内部存储及索引实现。

## MCP

```bash
codetrip mcp --trip-dir ~/.codetrip
```

stdio 服务提供 `list_repositories`、`search_symbols`、`search_source`、`traverse_graph` 和 `shortest_path`。MCP 位于 CLI 适配层，只调用 LIB 公共方法。

## 验证

生产环境只有一条分析链路，不区分分析模式。独立期望和样本统一位于 `validation/`。

```bash
go build -o ./codetrip ./cmd/codetrip
python3 validation/tools/check_quality.py
python3 -m unittest discover -s validation/tools -p 'test_*.py'
```
