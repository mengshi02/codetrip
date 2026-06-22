# codetrip 用户使用手册

## 概述

codetrip 是一个嵌入式混合图增强代码智能引擎（Hybrid Graph-Augmented Code Intelligence Engine）。它将代码仓库解析为知识图谱，并提供 Cypher 查询、影响分析、混合搜索、重命名、污点追踪等丰富工具，帮助开发者深入理解代码结构和依赖关系。

**核心特性：**
- 嵌入式部署，零外部依赖（基于 Pebble KV 存储）
- 14 阶段 DAG 管线自动索引（Scan → Structure → Markdown → COBOL → Parse → Routes → Tools → ORM → CrossFile → ScopeResolution → PruneLocal → MRO → Communities → Index）
- 支持多语言扩展（Go / TypeScript / JavaScript / Python / Java / C++ / C / C# / Rust / Markdown 等）
- Cypher 查询语言，默认 30s 超时保护
- BM25 + 双模态语义向量（Description + Code）混合搜索，RRF 融合
- int8 标量量化 + 两阶段搜索（粗排 int8 + 精排 float32）
- 增量索引：SHA1 哈希驱动的变更检测
- 跨仓库分组：契约检测 + 桥接链接
- 数据一致性验证与 Pebble Checkpoint 备份
- MCP 集成：通过 `codetrip mcp` 暴露 18 个工具供 AI 编码助手调用
- 可扩展的 Tool / Phase / Embedder / ContractExtractor 插件体系

---

## 快速开始

### 安装

```bash
go get github.com/mengshi02/codetrip
```

### 5 分钟上手

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/mengshi02/codetrip"
)

func main() {
    // 1. 打开引擎
    trip, err := codetrip.Open("./my-project.codetrip")
    if err != nil {
        log.Fatal(err)
    }
    defer trip.Close()

    ctx := context.Background()

    // 2. 索引仓库
    result, err := trip.IndexRepo(ctx, "/path/to/my-project",
        codetrip.WithRepoName("my-project"),
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("索引完成: %d 文件, %d 节点, %d 边\n", result.Files, result.Nodes, result.Edges)

    // 3. 执行 Cypher 查询
    cypherResult, err := trip.Cypher(ctx,
        "MATCH (n:Function) RETURN n.name LIMIT 5",
        codetrip.Param{Key: "repo", Value: "my-project"},
    )
    if err != nil {
        log.Fatal(err)
    }
    for _, row := range cypherResult.Rows {
        fmt.Println(row)
    }

    // 4. 影响分析
    impact, err := trip.Impact(ctx, &codetrip.ImpactRequest{
        Target:    "handleRequest",
        Direction: "downstream",
        MaxDepth:  3,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("风险等级: %s, 受影响符号数: %d\n", impact.Risk, len(impact.ByDepth))

    // 5. 生成向量嵌入（用于语义搜索）
    embedResult, err := trip.EmbedRepo(ctx, "my-project",
        codetrip.WithEmbedEndpoint("http://localhost:11434/v1/embeddings"),
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("嵌入完成: %d 节点, %d 描述分块, %d 代码分块\n",
        embedResult.NodesEmbedded, embedResult.DescChunks, embedResult.CodeChunks)
}
```

---

## 引擎生命周期

### Open — 打开引擎

```go
func Open(dir string, opts ...Option) (*Trip, error)
```

打开或创建一个 codetrip 引擎实例。`dir` 为数据存储目录，首次使用会自动创建。

**配置选项：**

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithCacheSize(size int64)` | 存储引擎缓存大小 | 256 MB |
| `WithPhase(phase pipeline.Phase)` | 注册自定义 Pipeline Phase | 无 |
| `WithNodeCacheSize(size int)` | 节点缓存条目数 | 10000 |
| `WithTraversalLimit(limit int)` | 图遍历节点上限 | 100000 |
| `WithScalePreset(preset string)` | 规模预设：`small`/`medium`/`large` | 无 |
| `WithQuantization(enable bool)` | 启用 int8 向量量化 | false |
| `WithTwoStageSearch(enable bool)` | 启用两阶段搜索（int8 粗排 + float32 精排） | false |
| `WithBM25ChunkSize(size int)` | BM25 索引分块大小 | 65536 |
| `WithCypherTimeout(d time.Duration)` | Cypher 查询超时 | 30s |
| `WithAutoMigrate(enable bool)` | 自动 Schema 版本迁移 | false |

**示例：**

```go
trip, err := codetrip.Open("./data",
    codetrip.WithCacheSize(512 << 20),     // 512MB 缓存
    codetrip.WithScalePreset("large"),      // 大规模预设
    codetrip.WithQuantization(true),        // 启用 int8 量化
    codetrip.WithTwoStageSearch(true),      // 两阶段搜索
    codetrip.WithCypherTimeout(60*time.Second), // Cypher 超时 60s
)
```

### Close — 关闭引擎

```go
func (trip *Trip) Close() error
```

关闭引擎，释放所有 BM25 索引和 Pebble 存储资源。务必在程序退出前调用（通常配合 `defer`）。

### Ping — 健康检查

```go
func (trip *Trip) Ping() error
```

检查引擎是否正常工作。返回 `nil` 表示健康。

### Backup — 数据备份

```go
func (trip *Trip) Backup(backupDir string) error
```

基于 Pebble Checkpoint 创建引擎快照备份到指定目录。

**示例：**

```go
err := trip.Backup("/backups/codetrip-20240101")
```

### GraphStore — 获取图存储

```go
func (trip *Trip) GraphStore(repo string) *graph.GraphStore
```

获取指定仓库的底层图存储实例，用于高级图操作。

---

## 索引管理

### IndexRepo — 索引仓库

```go
func (trip *Trip) IndexRepo(ctx context.Context, repoPath string, opts ...IndexOption) (*IndexResult, error)
```

将本地仓库解析为知识图谱。内部运行 14 阶段 DAG 管线：Scan → Structure → Markdown → COBOL → Parse → Routes → Tools → ORM → CrossFile → ScopeResolution → PruneLocal → MRO → Communities → Index。

> **注意：** 向量嵌入已从索引管线中独立为 `EmbedRepo` API，索引时不再内嵌嵌入阶段。

**索引选项：**

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithRepoName(name)` | 仓库名称（默认取路径末尾目录名） | 路径 base 名 |
| `WithMaxWorkers(n)` | 最大并行工作数 | 0 (GOMAXPROCS) |
| `WithByteBudget(budget)` | 每个 chunk 的字节预算 | 20 MB |
| `WithCFG(enable)` | 启用控制流图 (CFG) 构建 | false |
| `WithPDG(enable)` | 启用程序依赖图 (PDG) 构建 | false |
| `WithIndexTimeout(d)` | 索引超时时间 | 30 min |

**返回值 `IndexResult`：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Repo` | string | 仓库名称 |
| `Files` | int | 扫描文件数 |
| `Nodes` | int | 创建节点数 |
| `Edges` | int | 创建边数 |
| `Duration` | float64 | 耗时（秒） |

**示例：**

```go
result, err := trip.IndexRepo(ctx, "/path/to/repo",
    codetrip.WithRepoName("my-service"),
    codetrip.WithCFG(true),
    codetrip.WithMaxWorkers(4),
)
```

### ReIndex — 增量重索引

```go
func (trip *Trip) ReIndex(ctx context.Context, repoPath string, opts ...IndexOption) (*ReIndexResult, error)
```

检测仓库变更（新增/修改/删除文件），增量更新代码图谱、BM25 和向量索引。仓库必须已通过 `IndexRepo` 索引。

**`ReIndexResult` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Added` | int | 新增文件数 |
| `Modified` | int | 修改文件数 |
| `Deleted` | int | 删除文件数 |
| `Unchanged` | int | 未变更文件数 |

**示例：**

```go
result, err := trip.ReIndex(ctx, "/path/to/my-project",
    codetrip.WithRepoName("my-project"),
)
fmt.Printf("新增: %d, 修改: %d, 删除: %d\n", result.Added, result.Modified, result.Deleted)
```

### EmbedRepo — 向量嵌入

```go
func (trip *Trip) EmbedRepo(ctx context.Context, repo string, opts ...EmbedOption) (*EmbedResult, error)
```

为已索引仓库生成双模态向量嵌入（Description + Code），用于语义混合搜索。需要先执行 `IndexRepo`。

**双模态嵌入：**
- **Description 模态**：符号签名 + 关系摘要
- **Code 模态**：源码片段分块

**嵌入选项：**

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithEmbedEndpoint(url)` | 嵌入服务 HTTP 端点（必填） | 无 |
| `WithEmbedModel(name)` | 模型名称 | 自动检测 |
| `WithEmbedAPIKey(key)` | API 密钥 | 无 |
| `WithEmbedDimensions(d)` | 向量维度 | 自动检测 |
| `WithEmbedBatchSize(n)` | 批量嵌入请求大小 | 16 |
| `WithEmbedIncremental(enable)` | 跳过内容哈希未变的节点 | false |
| `WithEmbedQuantInt8(enable)` | 启用 int8 向量量化 | false |
| `WithEmbedTwoStageSearch(enable)` | 启用两阶段搜索 | false |
| `WithEmbedTimeout(d)` | 嵌入超时 | 30 min |

**`EmbedResult` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `NodesEmbedded` | int | 嵌入节点数 |
| `DescChunks` | int | Description 分块数 |
| `CodeChunks` | int | Code 分块数 |
| `Skipped` | int | 跳过（未变更）数 |
| `Errors` | int | 错误数 |
| `Duration` | float64 | 耗时（秒） |

**示例：**

```go
result, err := trip.EmbedRepo(ctx, "my-project",
    codetrip.WithEmbedEndpoint("http://localhost:11434/v1/embeddings"),
    codetrip.WithEmbedIncremental(true),
    codetrip.WithEmbedQuantInt8(true),
    codetrip.WithEmbedTwoStageSearch(true),
)
```

### Verify — 数据一致性验证

```go
func (trip *Trip) Verify(ctx context.Context) ([]VerifyIssue, error)
```

检查引擎一致性，验证邻接表、类型/名称/文件索引、嵌入哈希的完整性。

**`VerifyIssue` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Type` | string | 问题类型 |
| `Repo` | string | 所属仓库 |
| `Detail` | string | 问题描述 |

**示例：**

```go
issues, err := trip.Verify(ctx)
for _, issue := range issues {
    fmt.Printf("[%s] %s: %s\n", issue.Type, issue.Repo, issue.Detail)
}
```

### DropIndex — 删除索引

```go
func (trip *Trip) DropIndex(repoName string) error
```

删除指定仓库的所有索引数据，包括节点、边、邻接表、类型索引、名称索引、文件索引、全文索引、嵌入向量、作用域数据。

### ListRepos — 列出仓库

```go
func (trip *Trip) ListRepos() ([]RepoInfo, error)
```

返回当前已索引的所有仓库列表。

### Stats — 索引统计

```go
func (trip *Trip) Stats(repo string) (*IndexStats, error)
```

返回指定仓库的索引统计信息：

| 字段 | 说明 |
|------|------|
| `NameCount` | 名称索引条目数 |
| `LabelCount` | 标签索引条目数 |
| `FileCount` | 文件索引条目数 |
| `UIDCount` | UID 索引条目数 |

### RepoStatus — 仓库状态

```go
func (trip *Trip) RepoStatus(repoName string) (*RepoStatusInfo, error)
```

返回仓库状态概览：

| 字段 | 说明 |
|------|------|
| `Name` | 仓库名称 |
| `NodeCount` | 节点数 |
| `EdgeCount` | 边数 |

---

## 图查询

### Cypher — 执行 Cypher 查询

```go
func (trip *Trip) Cypher(ctx context.Context, query string, params ...Param) (*CypherResult, error)
```

使用 Cypher 查询语言查询知识图谱。通过 `Param{Key: "repo", Value: "repo-name"}` 指定目标仓库。内置 Volcano 迭代器模型执行引擎，默认 30s 超时保护（可通过 `WithCypherTimeout` 调整）。

**返回值 `CypherResult`：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Columns` | []string | 列名列表 |
| `Rows` | []map[string]any | 结果行 |
| `Stats` | map[string]int | 查询统计（如 `rows` 行数） |

**示例：**

```go
// 查找所有函数节点
result, _ := trip.Cypher(ctx,
    "MATCH (n:Function) RETURN n.name, n.filePath LIMIT 10",
    codetrip.Param{Key: "repo", Value: "my-project"},
)

// 查找调用关系
result, _ = trip.Cypher(ctx,
    "MATCH (a)-[:CALLS]->(b) WHERE a.name = $name RETURN b.name",
    codetrip.Param{Key: "repo", Value: "my-project"},
    codetrip.Param{Key: "name", Value: "handleRequest"},
)
```

### Query — 图查询（简写）

```go
func (trip *Trip) Query(ctx context.Context, stmt string, params ...Param) (*QueryResult, error)
```

与 `Cypher` 功能一致，返回 `QueryResult`（不含 Stats 字段）。

---

## 搜索

### Search — 代码搜索

```go
func (trip *Trip) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error)
```

支持两种搜索模式：

| 模式 | `Semantic` 值 | 说明 |
|------|--------|------|
| BM25 文本搜索 | `false` | 基于关键词的全文检索 |
| 混合搜索 | `true` | BM25 + 双模态语义向量 RRF 融合排序 |

**BM25 Fallback：** 当 `Semantic=true` 但仓库未执行 `EmbedRepo` 时，自动降级为 BM25 搜索，`SearchResult.Fallback` 返回 `"bm25"`。

**`SearchRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Query` | string | 搜索查询词 |
| `Limit` | int | 返回数量上限（默认 20） |
| `Repo` | string | 目标仓库（空则使用默认） |
| `Semantic` | bool | 是否启用语义搜索 |

**`SearchResult` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Results` | []SearchItem | 搜索结果列表 |
| `Fallback` | string | 降级标识：`"bm25"` 表示语义搜索降级为 BM25 |

**`SearchItem` 字段：**

| 字段 | 说明 |
|------|------|
| `NodeID` | 图节点 ID |
| `Name` | 符号名称 |
| `Kind` | 符号类型 |
| `FilePath` | 文件路径 |
| `Score` | 相关性分数 |
| `StartLine` | 起始行号 |
| `EndLine` | 结束行号 |

**示例：**

```go
// BM25 搜索
result, _ := trip.Search(ctx, &codetrip.SearchRequest{
    Query: "authenticate",
    Limit: 10,
    Repo:  "my-project",
})

// 语义混合搜索
result, _ = trip.Search(ctx, &codetrip.SearchRequest{
    Query:    "user login validation",
    Semantic: true,
    Limit:    10,
    Repo:     "my-project",
})

// 检查是否降级
if result.Fallback == "bm25" {
    fmt.Println("语义搜索不可用，已降级为 BM25 搜索")
}
```

---

## 分析工具

### Impact — 影响分析

```go
func (trip *Trip) Impact(ctx context.Context, req *ImpactRequest) (*ImpactResult, error)
```

评估符号变更对下游/上游的影响范围和风险等级。基于 BFS 图遍历，按深度分组展示受影响符号。

**`ImpactRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Target` | string | 目标符号名称 |
| `TargetUID` | string | 目标符号 UID（优先于名称） |
| `Direction` | string | `"downstream"` 或 `"upstream"` |
| `FilePath` | string | 文件路径（辅助定位） |
| `Kind` | string | 符号类型过滤 |
| `MaxDepth` | int | 遍历深度上限（默认 3，最大 32） |
| `RelationTypes` | []string | 仅追踪指定边类型 |
| `MinConfidence` | float64 | 最低置信度过滤 |
| `IncludeTests` | bool | 是否包含测试代码 |
| `SummaryOnly` | bool | 仅返回摘要 |
| `Limit` | int | 结果数量限制 |
| `Offset` | int | 结果偏移量 |
| `TimeoutMs` | int | 超时时间（毫秒） |

**`ImpactResult` 字段：**

| 字段 | 说明 |
|------|------|
| `Risk` | 风险等级：`LOW` / `MEDIUM` / `HIGH` / `CRITICAL` |
| `AffectedProcesses` | 受影响的进程列表 |
| `AffectedModules` | 受影响的模块列表 |
| `ByDepth` | 按深度分组的受影响符号 |
| `ByDepthCounts` | 各深度受影响符号数量 |

**风险等级规则：** 受影响符号数 ≥10 → CRITICAL，≥6 → HIGH，≥3 → MEDIUM，其他 → LOW。

**示例：**

```go
result, _ := trip.Impact(ctx, &codetrip.ImpactRequest{
    Target:        "handleRequest",
    Direction:     "downstream",
    MaxDepth:      3,
    MinConfidence: 0.8,
})
fmt.Printf("风险: %s\n", result.Risk)
for _, dg := range result.ByDepth {
    fmt.Printf("  深度 %d: %d 个符号\n", dg.Depth, len(dg.Symbols))
}
```

### Context — 360度符号视图

```go
func (trip *Trip) Context(ctx context.Context, req *ContextRequest) (*ContextResult, error)
```

获取符号的完整上下文视图：包含定义信息、所有入边（谁引用了我）和出边（我引用了谁）引用，以及同名消歧候选。

**`ContextRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 符号名称 |
| `UID` | string | 符号 UID（优先于名称） |
| `FilePath` | string | 文件路径（消歧用） |
| `Kind` | string | 符号类型过滤 |
| `IncludeContent` | bool | 是否包含源码内容 |

**`ContextResult` 字段：**

| 字段 | 说明 |
|------|------|
| `Symbol` | 符号基本信息（NodeID, Name, Kind, FilePath） |
| `Incoming` | 入边引用分组（按边类型分组） |
| `Outgoing` | 出边引用分组（按边类型分组） |
| `Processes` | 所属进程列表 |
| `Disambiguation` | 同名消歧候选列表 |

**示例：**

```go
result, _ := trip.Context(ctx, &codetrip.ContextRequest{
    Name:     "User",
    FilePath: "pkg/models/user.go",
})
fmt.Printf("符号: %s (%s) @ %s\n", result.Symbol.Name, result.Symbol.Kind, result.Symbol.FilePath)
for _, group := range result.Incoming {
    fmt.Printf("  入边 [%s]: %d 个引用\n", group.Type, len(group.Refs))
}
```

### DetectChanges — 变更检测

```go
func (trip *Trip) DetectChanges(ctx context.Context, req *DetectChangesRequest) (*DetectChangesResult, error)
```

基于 SHA1 内容哈希的增量索引，检测文件变更并评估受影响流程和风险。

**`DetectChangesRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Scope` | string | 检测范围（目录或文件路径） |
| `BaseRef` | string | 基准引用（预留） |
| `Repo` | string | 仓库名称 |

**`DetectChangesResult` 字段：**

| 字段 | 说明 |
|------|------|
| `ChangedSymbols` | 变更符号列表 |
| `AffectedProcesses` | 受影响进程列表 |
| `RiskSummary` | 风险摘要（Level, TotalChanges, HighRisk） |

**示例：**

```go
result, _ := trip.DetectChanges(ctx, &codetrip.DetectChangesRequest{
    Repo:  "my-project",
    Scope: "src/handlers/",
})
fmt.Printf("风险: %s, 变更数: %d\n", result.RiskSummary.Level, result.RiskSummary.TotalChanges)
```

### Rename — 多文件协调重命名

```go
func (trip *Trip) Rename(ctx context.Context, req *RenameRequest) (*RenameResult, error)
```

跨文件定位符号的所有引用点，返回重命名编辑建议。使用两层策略：
1. **高置信度**：基于图边（CALLS / ACCESSES / IMPORTS）定位的精确引用
2. **低置信度**：基于 BM25 文本搜索的模糊匹配

**`RenameRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `SymbolName` | string | 原始符号名称 |
| `SymbolUID` | string | 符号 UID（优先于名称） |
| `NewName` | string | 新名称 |
| `FilePath` | string | 文件路径（辅助定位） |
| `DryRun` | bool | 试运行模式 |

**`RenameResult` 包含 `RenameEdit` 列表：**

| 字段 | 说明 |
|------|------|
| `FilePath` | 文件路径 |
| `OldText` | 原始文本 |
| `NewText` | 替换文本 |
| `Confidence` | 置信度：`high` 或 `low` |

**示例：**

```go
result, _ := trip.Rename(ctx, &codetrip.RenameRequest{
    SymbolName: "oldFunc",
    NewName:    "newFunc",
    DryRun:     true,
})
for _, edit := range result.Edits {
    fmt.Printf("[%s] %s: %s → %s\n", edit.Confidence, edit.FilePath, edit.OldText, edit.NewText)
}
```

### RouteMap — API 路由映射

```go
func (trip *Trip) RouteMap(ctx context.Context, req *RouteMapRequest) (*RouteMapResult, error)
```

查询 API 路由节点及其 Handler、消费者、中间件。

**`RouteMapRequest`：** `Route string`, `Repo string`

**`RouteInfo` 字段：**

| 字段 | 说明 |
|------|------|
| `Path` | 路由路径 |
| `Method` | HTTP 方法 |
| `HandlerID` | 处理函数节点 ID |
| `Middleware` | 中间件列表 |
| `Consumers` | 消费者节点 ID 列表 |

### ToolMap — MCP/RPC 工具映射

```go
func (trip *Trip) ToolMap(ctx context.Context, req *ToolMapRequest) (*ToolMapResult, error)
```

查询 MCP/RPC 工具定义节点及其 Handler。

**`ToolMapRequest`：** `Tool string`, `Repo string`

**`ToolInfo` 字段：**

| 字段 | 说明 |
|------|------|
| `Name` | 工具名称 |
| `Description` | 工具描述 |
| `HandlerID` | 处理函数节点 ID |

### ShapeCheck — 响应形状检查

```go
func (trip *Trip) ShapeCheck(ctx context.Context, req *ShapeCheckRequest) (*ShapeCheckResult, error)
```

检测 Route 生产者的响应字段与消费者期望字段之间的不匹配。

**`ShapeCheckRequest`：** `Route string`, `Repo string`

**`ShapeMismatch` 字段：**

| 字段 | 说明 |
|------|------|
| `Route` | 路由名称 |
| `Field` | 不匹配的字段 |
| `Producer` | 生产者的字段定义 |
| `Consumer` | 消费者期望的字段定义 |

### ApiImpact — API 影响分析

```go
func (trip *Trip) ApiImpact(ctx context.Context, req *ApiImpactRequest) (*ApiImpactResult, error)
```

组合 RouteMap + Impact + ShapeCheck 的综合 API 变更影响分析。

**`ApiImpactRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Route` | string | 目标路由名称 |
| `Repo` | string | 仓库名称 |

**`ApiImpactResult` 字段：**

| 字段 | 说明 |
|------|------|
| `Risk` | 风险等级 |
| `Consumers` | 消费者信息列表 |
| `Mismatches` | 形状不匹配列表 |
| `Middleware` | 涉及的中间件 |
| `Processes` | 受影响进程列表 |

**风险等级规则（当 Impact 未提供风险时）：** 消费者 ≥10 → CRITICAL，≥5 → HIGH，≥2 → MEDIUM，其他 → LOW。

### Check — 结构检查

```go
func (trip *Trip) Check(ctx context.Context, req *CheckRequest) (*CheckResult, error)
```

执行结构性检查，如循环依赖检测。

**`CheckRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Repo` | string | 仓库名称 |
| `Cycles` | bool | 是否检测循环依赖 |

**示例：**

```go
result, _ := trip.Check(ctx, &codetrip.CheckRequest{
    Repo:   "my-project",
    Cycles: true,
})
fmt.Printf("检测到 %d 个循环依赖\n", len(result.Cycles))
```

### Explain — 污点追踪

```go
func (trip *Trip) Explain(ctx context.Context, req *ExplainRequest) (*ExplainResult, error)
```

追踪数据流中的污点传播路径。查找目标符号上的 TAINTED 入边，追踪完整跳板路径（包括 SANITIZES 消毒节点和 TAINT_PATH 传播节点）。

**`ExplainRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `Target` | string | 目标符号名称 |
| `Repo` | string | 仓库名称 |
| `Limit` | int | 最大发现数（默认 100） |

**`ExplainResult` 字段：**

| 字段 | 说明 |
|------|------|
| `Findings` | 污点发现列表 |
| `TotalFindings` | 总发现数 |
| `Truncated` | 是否因 Limit 截断 |

**`TaintFinding` 字段：**

| 字段 | 说明 |
|------|------|
| `Category` | 污点类别 |
| `SourceLine` | 源头行号 |
| `SinkLine` | 汇点行号 |
| `HopPath` | 跳板路径（NodeID + Line） |

---

## 跨仓库分组

### GroupList — 列出分组

```go
func (trip *Trip) GroupList() ([]GroupInfo, error)
```

列出所有跨仓库分组信息。

**`GroupInfo` 字段：**

| 字段 | 说明 |
|------|------|
| `Name` | 分组名称 |
| `Description` | 分组描述 |
| `Repos` | 包含的仓库名称列表 |

### GroupSync — 同步分组

```go
func (trip *Trip) GroupSync(ctx context.Context, req *GroupSyncRequest) (*GroupSyncResult, error)
```

同步跨仓库分组，自动检测跨仓库契约（Contract）和桥接链接（Bridge Link）。

**`GroupSyncRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `GroupName` | string | 分组名称 |
| `RepoPaths` | map[string]string | 仓库路径映射 (repoPath → repoName) |

**`GroupSyncResult` 字段：**

| 字段 | 说明 |
|------|------|
| `Group` | 分组名称 |
| `Contracts` | 检测到的契约数 |
| `BridgeLinks` | 检测到的桥接链接数 |
| `Duration` | 耗时（秒） |

**示例：**

```go
result, _ := trip.GroupSync(ctx, &codetrip.GroupSyncRequest{
    GroupName: "microservices",
    RepoPaths: map[string]string{
        "/path/to/user-service":    "user-service",
        "/path/to/order-service":   "order-service",
        "/path/to/payment-service": "payment-service",
    },
})
fmt.Printf("契约: %d, 桥接: %d\n", result.Contracts, result.BridgeLinks)
```

### GroupImpact — 跨仓库影响分析

```go
func (trip *Trip) GroupImpact(ctx context.Context, req *GroupImpactRequest) (*GroupImpactResult, error)
```

跨仓库影响分析，追踪跨仓库引用（名称匹配 + 契约链接）。

**`GroupImpactRequest` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `GroupName` | string | 分组名称 |
| `Target` | string | 目标符号名称 |
| `Direction` | string | 分析方向 |

**`GroupImpactResult` 字段：**

| 字段 | 说明 |
|------|------|
| `Risk` | 风险等级 |
| `LocalImpact` | 本仓库影响分析结果 |
| `CrossRepoRefs` | 跨仓库引用列表 |

**`CrossRepoRef` 字段：**

| 字段 | 说明 |
|------|------|
| `SourceRepo` | 源仓库 |
| `SourceSymbol` | 源符号 |
| `TargetRepo` | 目标仓库 |
| `TargetSymbol` | 目标符号 |
| `MatchType` | 匹配类型：`name` 或 `contract` |
| `Confidence` | 置信度 |

---

## 扩展注册

codetrip 提供丰富的插件扩展点，通过 `Register*` 系列方法注册。

### RegisterLanguageProvider — 注册语言解析器

```go
func (trip *Trip) RegisterLanguageProvider(lang graph.Label, provider LanguageProvider)
```

为指定语言注册解析器，控制代码捕获、调用提取、类提取、字段提取、导入解析等行为。

**`LanguageProvider` 接口：**

| 方法 | 说明 |
|------|------|
| `Language()` | 返回语言标签 |
| `TreeSitterLanguage()` | 返回 Tree-sitter 语言指针 |
| `QuerySet()` | 返回查询集（作用域/声明/导入/类型绑定/引用） |
| `InterpretScope(captures)` | 解释作用域捕获结果 |
| `InterpretDeclaration(captures)` | 解释声明捕获结果 |
| `InterpretImport(captures)` | 解释导入捕获结果 |
| `InterpretTypeBinding(captures)` | 解释类型绑定捕获结果 |
| `InterpretReference(captures)` | 解释引用捕获结果 |
| `Captures()` | **Legacy** 返回捕获配置 |
| `CallExtractConfig()` | **Legacy** 返回调用提取配置 |
| `ClassExtractConfig()` | **Legacy** 返回类提取配置 |
| `FieldExtractConfig()` | **Legacy** 返回字段提取配置 |
| `ImportResolveConfig()` | **Legacy** 返回导入解析配置 |
| `Interpret(captures)` | **Legacy** 解释捕获结果 |
| `ImportSemantics()` | 导入语义策略 |

> **注意：** `QuerySet()` + `InterpretXxx()` 系列方法是新接口，推荐使用。`Captures()` / `CallExtractConfig()` 等为 Legacy 兼容接口。

**内置语言提供者：** Go, TypeScript, JavaScript, Python, Java, C++, C, C#, Rust, Markdown

### RegisterScopeResolver — 注册作用域解析器

```go
func (trip *Trip) RegisterScopeResolver(lang graph.Label, resolver ScopeResolver)
```

为指定语言注册作用域解析器。`ScopeResolver` 已拆分为组合接口：

| 子接口 | 说明 |
|--------|------|
| `CoreResolver` | 核心作用域解析（MRO 构建、作用域树维护） |
| `BindingResolver` | 绑定解析（导入目标解析、绑定合并） |
| `HookResolver` | 钩子解析（调用兼容性判断、方法重写） |
| `EmitResolver` | 发射解析（符号发射、作用域边界处理） |

### RegisterPhase — 注册自定义 Pipeline Phase

```go
func (trip *Trip) RegisterPhase(phase pipeline.Phase)
```

在索引管线的内置阶段之后追加自定义处理阶段。

### RegisterTool — 注册自定义分析工具

```go
func (trip *Trip) RegisterTool(name string, tool Tool)
```

注册自定义分析工具，支持两种接口：

**基础 `Tool` 接口：**

```go
type Tool interface {
    Name() string
    Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error)
}
```

**泛型 `GenericTool[T, R]` 接口（推荐）：**

```go
type GenericTool[T any, R any] interface {
    Name() string
    Run(ctx context.Context, trip *Trip, req T) (R, error)
}
```

同时提供 `ToolAdapter` 适配器，可将 `GenericTool` 适配为基础 `Tool` 接口。

**示例：**

```go
type myTool struct{}

func (t *myTool) Name() string { return "my_tool" }
func (t *myTool) Run(ctx context.Context, trip *codetrip.Trip, req interface{}) (interface{}, error) {
    // 自定义分析逻辑
    return map[string]any{"status": "ok"}, nil
}

trip.RegisterTool("my_tool", &myTool{})
```

### RegisterEmbedder — 注册向量嵌入模型

```go
func (trip *Trip) RegisterEmbedder(embedder Embedder)
```

注册向量嵌入模型，用于语义搜索。默认使用 `NoopEmbedder`（不生成嵌入）。

**`Embedder` 接口：**

| 方法 | 说明 |
|------|------|
| `Dimensions()` | 嵌入维度 |
| `Embed(texts)` | 批量文本嵌入 |
| `EmbedBatch(nodes, config)` | 批量节点嵌入 |

### RegisterContractExtractor — 注册契约提取器

```go
func (trip *Trip) RegisterContractExtractor(contractType ContractType, extractor ContractExtractor)
```

注册指定类型的契约提取器，用于跨仓库分组时检测 API 契约。

**支持的契约类型：**

| 类型 | 常量 |
|------|------|
| HTTP | `ContractHTTP` |
| gRPC | `ContractGRPC` |
| Thrift | `ContractThrift` |
| Topic | `ContractTopic` |
| Lib | `ContractLib` |
| Custom | `ContractCustom` |
| Include | `ContractInclude` |

---

## 命令行工具

codetrip 提供命令行工具 `codetrip`，支持以下命令操作。所有命令共享全局标志 `--trip-dir`，用于指定引擎数据目录（默认为 `~/.codetrip`）。

---

### index — 索引仓库

扫描并索引代码仓库，构建代码图谱和搜索索引。

**语法：** `codetrip index <repo-path>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--name` | string | 路径末尾目录名 | 仓库名称 |
| `--workers` | int | 0 (GOMAXPROCS) | 最大并行工作数 |
| `--byte-budget` | int64 | 20 MB | 每个 chunk 的字节预算 |
| `--cfg` | bool | false | 启用 CFG 构建 |
| `--pdg` | bool | false | 启用 PDG 构建 |

**示例：**

```bash
# 基本索引
codetrip index /path/to/my-project

# 指定仓库名称，启用 CFG
codetrip index /path/to/my-project --name my-service --cfg

# 限制并行数
codetrip index /path/to/my-project --workers 4 --byte-budget 41943040

# 使用自定义数据目录
codetrip index /path/to/my-project --trip-dir /custom/trip
```

---

### reindex — 增量重索引

检测仓库变更，增量更新代码图谱和搜索索引。仓库必须已通过 `codetrip index` 索引。

**语法：** `codetrip reindex <repo-path>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--name` | string | 路径末尾目录名 | 仓库名称 |

**示例：**

```bash
# 增量重索引
codetrip reindex /path/to/my-project

# 指定仓库名称
codetrip reindex /path/to/my-project --name my-service
```

---

### embed — 生成向量嵌入

为已索引仓库生成双模态向量嵌入，启用语义混合搜索。需要先执行 `codetrip index`。

**语法：** `codetrip embed`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--endpoint` | string | **必填** | 嵌入服务端点（如 `http://localhost:11434/v1/embeddings`） |
| `--model` | string | 自动检测 | 模型名称 |
| `--api-key` | string | 无 | API 密钥 |
| `--dimensions` | int | 自动检测 | 向量维度 |
| `--batch-size` | int | 16 | 批量嵌入大小 |
| `--incremental` | bool | false | 跳过未变更节点 |
| `--quant-int8` | bool | false | 启用 int8 量化 |
| `--two-stage` | bool | false | 启用两阶段搜索 |
| `--timeout` | duration | 30m | 嵌入超时 |

**示例：**

```bash
# 生成嵌入
codetrip embed --repo my-project --endpoint http://localhost:11434/v1/embeddings

# 增量嵌入 + int8 量化 + 两阶段搜索
codetrip embed --repo my-project --endpoint http://localhost:11434/v1/embeddings \
  --incremental --quant-int8 --two-stage
```

---

### query — 执行 Cypher 查询

对知识图谱执行 Cypher 查询语言查询。

**语法：** `codetrip query <cypher-query>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |

**示例：**

```bash
# 查找所有函数节点
codetrip query "MATCH (n:Function) RETURN n.name LIMIT 10" --repo my-project

# 查找调用关系
codetrip query "MATCH (a)-[:CALLS]->(b) WHERE a.name = 'handleRequest' RETURN b.name" --repo my-project
```

---

### search — 代码搜索

使用 BM25 或混合（语义）搜索查找符号。

**语法：** `codetrip search <query>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--limit` | int | 20 | 最大返回结果数 |
| `--semantic` | bool | false | 启用语义混合搜索 |

**示例：**

```bash
# BM25 文本搜索
codetrip search "authenticate" --repo my-project

# 语义混合搜索（需先执行 embed）
codetrip search "user login validation" --repo my-project --semantic --limit 5
```

---

### impact — 影响分析

评估符号变更对下游/上游的影响范围和风险等级。

**语法：** `codetrip impact <target>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--direction` | string | downstream | 遍历方向：`downstream` 或 `upstream` |
| `--max-depth` | int | 3 | 最大遍历深度 |

**示例：**

```bash
# 下游影响分析
codetrip impact handleRequest --repo my-project

# 上游影响分析
codetrip impact User --repo my-project --direction upstream --max-depth 5
```

---

### context — 360度符号视图

获取符号的完整上下文视图：定义信息、入边/出边引用、同名消歧候选。

**语法：** `codetrip context <symbol-name>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--file` | string | "" | 文件路径（用于消歧） |

**示例：**

```bash
codetrip context handleRequest --repo my-project
codetrip context User --repo my-project --file pkg/models/user.go
```

---

### check — 结构检查

执行结构性检查，如循环依赖检测。

**语法：** `codetrip check`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--cycles` | bool | true | 检测循环依赖 |

**示例：**

```bash
codetrip check --repo my-project
```

---

### explain — 污点追踪

追踪数据流中的污点传播路径，标记源头到汇点的完整路径。

**语法：** `codetrip explain <target>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--limit` | int | 100 | 最大发现数量 |

**示例：**

```bash
codetrip explain handleRequest --repo my-project
codetrip explain handleRequest --repo my-project --limit 20
```

---

### rename — 多文件协调重命名

跨文件定位符号的所有引用点，返回重命名编辑建议。

**语法：** `codetrip rename <symbol-name>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--new-name` | string | **必填** | 新符号名称 |
| `--dry-run` | bool | true | 试运行模式（不修改文件） |

**示例：**

```bash
# 试运行模式查看重命名影响范围
codetrip rename oldFunc --repo my-project --new-name newFunc

# 正式执行重命名
codetrip rename oldFunc --repo my-project --new-name newFunc --dry-run=false
```

---

### detect-changes — 变更检测

基于增量索引检测文件变更和受影响符号。

**语法：** `codetrip detect-changes <scope>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |

**示例：**

```bash
codetrip detect-changes /path/to/my-project --repo my-project
```

---

### route-map — API 路由映射

列出 API 路由及其 Handler 和消费者。

**语法：** `codetrip route-map`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--route` | string | "" | 按路由名称过滤 |

**示例：**

```bash
codetrip route-map --repo my-project
codetrip route-map --repo my-project --route /api/users
```

---

### tool-map — MCP/RPC 工具映射

列出 MCP/RPC 工具定义及其 Handler。

**语法：** `codetrip tool-map`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--tool` | string | "" | 按工具名称过滤 |

**示例：**

```bash
codetrip tool-map --repo my-project
```

---

### shape-check — 响应形状检查

检测 Route 生产者的响应字段与消费者期望字段之间的不匹配。

**语法：** `codetrip shape-check`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--route` | string | "" | 按路由名称过滤 |

**示例：**

```bash
codetrip shape-check --repo my-project
```

---

### api-impact — API 综合影响分析

组合 RouteMap + Impact + ShapeCheck 的综合 API 变更影响分析。

**语法：** `codetrip api-impact`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |
| `--route` | string | "" | 分析特定 API 路由 |

**示例：**

```bash
codetrip api-impact --repo my-project
codetrip api-impact --repo my-project --route /api/users
```

---

### group — 跨仓库分组管理

跨仓库分组的列表、同步和影响分析。包含三个子命令。

#### group list — 列出跨仓库分组

**语法：** `codetrip group list`

**示例：**

```bash
codetrip group list
```

#### group sync — 同步跨仓库分组

**语法：** `codetrip group sync <group-name>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string[] | 无 | 仓库路径（可重复，格式：`path=name`） |

**示例：**

```bash
codetrip group sync microservices \
  --repo /path/to/user-service=user-service \
  --repo /path/to/order-service=order-service
```

#### group impact — 跨仓库影响分析

**语法：** `codetrip group impact <group-name> <target>`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--direction` | string | downstream | 分析方向 |

**示例：**

```bash
codetrip group impact microservices handleRequest
codetrip group impact microservices User --direction upstream
```

---

### info — 查看索引统计

显示索引的统计信息（名称索引、标签索引、文件索引、UID 索引条目数）。

**语法：** `codetrip info`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |

**示例：**

```bash
codetrip info --repo my-project
```

---

### version — 查看版本

**语法：** `codetrip version`

**示例：**

```bash
codetrip version
```

---

### list-repos — 列出已索引仓库

**语法：** `codetrip list-repos`

**示例：**

```bash
codetrip list-repos
```

---

### repo-status — 仓库状态概览

**语法：** `codetrip repo-status`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |

**示例：**

```bash
codetrip repo-status --repo my-project
```

---

### drop — 删除仓库索引

删除指定仓库的所有索引数据。

**语法：** `codetrip drop`

**标志：**

| 标志 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--repo` | string | **必填** | 仓库名称 |

**示例：**

```bash
codetrip drop --repo old-project
```

---

### mcp — 启动 MCP 服务器

启动 Model Context Protocol (MCP) 服务器，通过 stdio 暴露 codetrip 的代码分析能力，供 AI 编码助手（如 Claude Desktop、Cursor 等）调用。

**语法：** `codetrip mcp`

**示例：**

```bash
# 通过 stdio 启动（适用于 Claude Desktop、Cursor 等）
codetrip mcp

# 指定数据目录
codetrip mcp --trip-dir /custom/trip
```

**MCP 工具列表（18 个）：**

| 工具名 | 说明 |
|--------|------|
| `index_repo` | 索引代码仓库 |
| `reindex` | 增量重索引 |
| `embed_repo` | 生成双模态向量嵌入 |
| `cypher_query` | 执行 Cypher 查询 |
| `search_symbols` | 搜索代码符号 |
| `impact_analysis` | 影响分析 |
| `symbol_context` | 360度符号视图 |
| `detect_changes` | 变更检测 |
| `structural_check` | 结构检查 |
| `explain_taint` | 污点追踪 |
| `route_map` | API 路由映射 |
| `tool_map` | MCP/RPC 工具映射 |
| `shape_check` | 响应形状检查 |
| `api_impact` | API 综合影响分析 |
| `rename_symbol` | 多文件协调重命名 |
| `index_stats` | 索引统计 |
| `list_repos` | 列出已索引仓库 |
| `repo_status` | 仓库状态概览 |
| `drop_index` | 删除仓库索引 |
| `group_list` | 跨仓库分组列表 |
| `group_sync` | 跨仓库分组同步 |
| `group_impact` | 跨仓库影响分析 |

---

## 内置工具索引

| 工具名 | API 方法 | 说明 |
|--------|---------|------|
| `check` | `Check()` | 结构检查（循环依赖等） |
| `list_repos` | `ListRepos()` | 列出已索引仓库 |
| `repo_status` | `RepoStatus()` | 仓库状态概览 |
| `impact` | `Impact()` | 影响分析 |
| `context` | `Context()` | 360度符号视图 |
| `detect_changes` | `DetectChanges()` | 变更检测 |
| `rename` | `Rename()` | 多文件协调重命名 |
| `route_map` | `RouteMap()` | API 路由映射 |
| `tool_map` | `ToolMap()` | MCP/RPC 工具映射 |
| `shape_check` | `ShapeCheck()` | 响应形状检查 |
| `api_impact` | `ApiImpact()` | API 综合影响分析 |
| `explain` | `Explain()` | 污点追踪解释 |
| `search` | `Search()` | 代码搜索 |
| `group_list` | `GroupList()` | 跨仓库分组列表 |
| `group_sync` | `GroupSync()` | 跨仓库分组同步 |
| `group_impact` | `GroupImpact()` | 跨仓库影响分析 |
| `embed_repo` | `EmbedRepo()` | 双模态向量嵌入 |

---

## 典型工作流

### 工作流 1：单仓库代码理解

```
Open → IndexRepo → Cypher/Query 探索结构 → Context 查看符号详情 → Search 查找相关代码
```

### 工作流 2：变更影响评估

```
Open → IndexRepo → DetectChanges 识别变更 → Impact 分析影响 → Check 检查循环依赖
```

### 工作流 3：安全审计

```
Open → IndexRepo(WithCFG+PDG) → Explain 追踪污点路径 → ShapeCheck 检查接口一致性
```

### 工作流 4：微服务架构分析

```
Open → IndexRepo(多个仓库) → GroupSync 建立跨仓库分组 → GroupImpact 跨仓库影响分析 → ApiImpact API 变更评估
```

### 工作流 5：大规模重构

```
Open → IndexRepo → Rename(试运行) 评估影响范围 → Rename(正式) 执行重命名
```

### 工作流 6：语义搜索启用

```
Open → IndexRepo → EmbedRepo(生成嵌入) → Search(Semantic=true) 语义混合搜索
```

### 工作流 7：增量更新与数据验证

```
Open → ReIndex(增量重索引) → EmbedRepo(WithEmbedIncremental) 增量嵌入 → Verify 数据一致性验证
```