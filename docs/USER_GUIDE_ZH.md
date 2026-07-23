# Codetrip 用户手册

Codetrip 是混合图增强代码智能引擎（Hybrid Graph-Augmented Code Intelligence Engine），正式支持 Go LIB、CLI 和 MCP 三种集成方式。三者共享相同的仓库快照和 `Engine` 公共能力。

## 数据与更新模型

默认数据目录是 `~/.codetrip`，可以使用全局参数 `--dir` 修改。

```text
repos/<id>/
  manifest.json  逻辑仓库元数据
  graph/db/      权威图数据、邻接索引和活动指针
  index/         带版本的符号索引
  content/       带版本的源码索引
  vectors/       可选语义数据
trash/           中断删除的清理目录
```

每个逻辑仓库拥有独立数据库，并且只指向一个不可变物理快照。仓库数据库按需打开，访问一个项目不会锁住其他项目。`index --replace` 会完整构建新图和全部派生索引，成功后才发布活动指针，并回收旧快照；不会局部修改当前活动快照。旧版共享数据库布局中的数据需要重新索引。

## CLI

业务命令统一使用单个单词：

| 命令 | 用途 |
|---|---|
| `index` | 解析并持久化仓库 |
| `delete` | 删除仓库及其全部持久化数据 |
| `search` | 搜索符号及元数据 |
| `source` | 搜索代码、工程配置或文档内容 |
| `embed` | 构建仓库语义数据 |
| `hybrid` | 融合符号和语义检索 |
| `context` | 解释符号及其直接语义关系 |
| `impact` | 分析修改可能影响的符号 |
| `check` | 检查图完整性与仓库结构 |
| `diff` | 将 Git 变更映射为符号和受影响代码 |
| `rename` | 生成符号重命名计划但不修改源码 |
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
| `--export` | 输出确定性的解析检查 CSV |
| `--export-strict` | 解析 CSV 生成失败时让索引任务失败 |

删除逻辑仓库及其所有图、源码、符号和向量快照：

```bash
codetrip delete project
```

LIB 提供相同的 `Engine.DeleteRepo` 方法。MCP 不暴露这种破坏性仓库管理能力。

### 搜索

```bash
codetrip search "ParseConfig" --repo project --limit 20
codetrip source 'lang:go file:config ParseConfig' --repo project --context 2
codetrip source '部署架构' --repo project --scope docs
codetrip source 'NewHTTPServer' --repo project --scope all
```

`search` 面向符号和元数据；`source` 面向仓库文本，支持文本、正则、文件和语言过滤。`--scope` 默认为 `code`：`code` 包含编程语言及工程配置，`docs` 只搜索文档，`all` 同时搜索两者。二进制文件、依赖目录、构建产物、缓存和不支持的文本始终排除。

Linux 和 macOS 使用原生高吞吐源码检索后端。Windows 使用便携全文检索后端召回文件，再执行精确行匹配；公共 API 和查询能力保持一致，但在大型仓库上的源码搜索速度会更慢。Windows 与 Linux/macOS 的源码索引格式不同，跨平台迁移数据目录后需要重新构建仓库快照；升级到带 scope 的源码索引格式后，已有仓库也需要重新执行索引。

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
codetrip context NODE_ID --repo project
codetrip context NODE_ID --repo project --relations CALLS,IMPLEMENTS
codetrip impact NODE_ID --repo project --depth 3 --limit 100
codetrip check --repo project
codetrip check --repo project --checks confidence --confidence 0.7
codetrip traverse NODE_ID --repo project --direction both --depth 3
codetrip traverse NODE_ID --repo project --relations CALLS,IMPORTS
codetrip path SOURCE_ID TARGET_ID --repo project
```

`context` 返回目标符号、原始代码目录仍可访问时的源码片段，以及直接类型化关系；默认排除结构图噪音。`impact` 沿反向语义依赖查找受影响的可行动符号，并返回深度、到达关系和置信度；默认深度为 3，也可用 `--relations` 收窄分析范围。

`check` 默认执行 `integrity` 和 `cycles`。完整性检查报告缺失的边端点和无效自依赖；环检查将继承环标记为错误，将导入环标记为警告。可选的 `confidence` 检查报告低于 `--confidence` 的语义关系；它面向专项评审，默认不会作为正确性告警开启。

### 变更分析

```bash
# 比较 HEAD 与已跟踪的工作区修改。
codetrip diff --repo project

# 比较两个提交。
codetrip diff HEAD~1 --target HEAD --repo project

# 只映射修改符号，不扩展反向影响。
codetrip diff HEAD~1 --target HEAD --repo project --no-impact
```

`diff` 解析无上下文 Git hunk，将修改行映射到已持久化的可行动符号，并聚合 `impact` 结果，同时记录每个影响节点对应的修改原因。它使用仓库 manifest 中保存的源码目录；即使索引目录只是更大 Git worktree 的子目录，也只分析该目录。未跟踪文件在加入 Git 并重新索引前不会进入结果。

### 重命名计划

```bash
codetrip rename NODE_ID LoadConfig --repo project
```

`rename` 只分析，绝不会修改文件。它会验证目标标识符，将同文件冲突报告为错误、仓库范围同名符号报告为警告，沿入向类型化关系识别语义引用，并从代码索引中搜索精确标识符出现位置。声明和图关系支持的引用行会标为高置信度修改点；注释、字符串、反射访问等文本候选会明确标记 `requiresReview`。

规范方向为 `out`、`in` 和 `both`。同时接受适合 Agent 表达的别名：`forward`、`down`、`downstream`、`call` 映射为 `out`；`reverse`、`backward`、`up`、`upstream` 映射为 `in`；`any`、`all`、`bidirectional` 映射为 `both`。方向决定边的遍历方向，`--relations CALLS` 用于限定关系类型；引擎会执行可配置的访问节点上限。

### CSV

解析检查 CSV 可供维护者在本地进行语言精调：

```bash
codetrip index /src/project --repo project \
  --export ./local-review/project --export-strict
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
    codetrip.WithCSVExport("./local-review/project"),
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

symbolContext, err := engine.Context(ctx, &codetrip.ContextRequest{
    Repo: "project", NodeID: nodeID,
})

impact, err := engine.Impact(ctx, &codetrip.ImpactRequest{
    Repo: "project", NodeID: nodeID, MaxDepth: 3, Limit: 100,
})

checks, err := engine.Check(ctx, &codetrip.CheckRequest{
    Repo: "project",
})

changes, err := engine.Diff(ctx, &codetrip.DiffRequest{
    Repo: "project", BaseRef: "HEAD~1", TargetRef: "HEAD",
    MaxDepth: 3, Limit: 100,
})

rename, err := engine.Rename(ctx, &codetrip.RenameRequest{
    Repo: "project", NodeID: nodeID, NewName: "LoadConfig",
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
codetrip mcp --dir ~/.codetrip
```

stdio 服务提供 `list`、`search`、`source`、`context`、`impact`、`check`、`diff`、`rename`、`traverse` 和 `path`，与对应的 CLI 命令同名。MCP 位于 CLI 适配层，只调用 `Engine` 公共方法。

MCP 进程只在单次工具请求期间打开引擎，请求结束后立即释放，因此空闲 MCP server 不会长期占用数据库锁，也不会阻止 CLI 建立索引。MCP 请求会串行执行；如果 CLI 索引正在占用数据库，此时的 MCP 请求可能返回临时存储繁忙错误，稍后重试即可。

## 本地语言精调

生产环境只有一条分析链路。维护者可以使用 `--export` 输出的确定性 CSV，结合本地样本、金标期望和评审工具进行精调。这些精调资产不属于公开源码仓库，CLI、MCP 和 Go LIB 用户也不需要安装它们。

```bash
codetrip index /src/project --repo project \
  --export /path/to/local-review/project --export-strict
```
