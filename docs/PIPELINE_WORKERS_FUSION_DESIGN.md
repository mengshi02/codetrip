# Pipeline + Workers 融合设计方案

> 核心原则：**利用 Go goroutine+channel 高性能并发模型，不照搬 TS Node.js Worker Threads 基础设施**

## 1. 问题分析

### 1.1 TS Workers 的真正价值

TS `workers/` 目录 6 个文件的核心价值拆分：

| 文件 | 体积 | 价值分类 | Go 中是否需要 |
|------|------|----------|--------------|
| `parse-worker.ts` | 103KB | **核心业务逻辑**：AST 解析 + 6 种提取器 + scope 提取 + 结果组装 | **需要适配** |
| `worker-pool.ts` | 87KB | **Node.js 基础设施**：Worker Threads 管理 + sub-batch + transferList + quarantine + circuit breaker + slot respawn | **不需要** |
| `clone-safety.ts` | 24KB | **V8 限制**：structured clone 不支持 function/symbol | **不需要** |
| `post-result.ts` | 5KB | **通信边界**：postMessage + clone-safety 重试 | **不需要** |
| `result-merge.ts` | 3KB | **简单合并**：appendAll 数组合并 | **简化实现** |
| `quarantine.ts` | 2KB | **故障隔离**：Set<string> 封装 | **用 recover 替代** |

**结论**：只需要保留 parse-worker.ts 的业务逻辑，其余 5 个文件的 Node.js 特有机制用 Go 原生并发替代。

### 1.2 Go 的天然优势

| TS 问题 | Go 解决方案 |
|---------|------------|
| Worker Threads 重量级（每个 30MB+） | goroutine 2KB 栈，百万级并发 |
| postMessage 序列化边界 | goroutine 共享内存，无需序列化 |
| transferList 零拷贝 ArrayBuffer | 指针传递，天然零拷贝 |
| structured clone 不支持 function/symbol | 无此限制 |
| quarantine 故障隔离 | `recover` + 错误 channel |
| circuit breaker 防雪崩 | `semaphore.Weighted` 控制并发度 |
| slot respawn 重启 | goroutine 极轻量，无需"重启"概念 |

## 2. 架构设计

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    Pipeline (14 phases)                       │
│                                                               │
│  scan → structure → markdown → parse ────────────────────── │
│    → [routes, tools, orm] → crossFile → scopeResolution      │
│    → pruneLocalSymbols → mro → communities → processes       │
│                                                               │
│                    parse phase 内部                            │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  RunChunkedParseAndResolve()                             │ │
│  │                                                           │ │
│  │  BuildChunks() ──→ chunk1, chunk2, ... chunkN            │ │
│  │                                                           │ │
│  │  ┌─────────────────────────────────────────────────┐     │ │
│  │  │         GoroutinePool (并发调度层)                │     │ │
│  │  │                                                   │     │ │
│  │  │  goroutine1 ── ParseFileSet(chunk1_files)         │     │ │
│  │  │  goroutine2 ── ParseFileSet(chunk2_files)         │     │ │
│  │  │  goroutine3 ── ParseFileSet(chunk3_files)         │     │ │
│  │  │  ...                                              │     │ │
│  │  │                                                   │     │ │
│  │  │  每个 goroutine 内部：                             │     │ │
│  │  │  1. ReadFileContents (共享内存)                    │     │ │
│  │  │  2. TreeSitter Parse (per-file)                    │     │ │
│  │  │  3. Extract (6 extractors + scope)                 │     │ │
│  │  │  4. Send ParseFileSetResult → resultCh             │     │ │
│  │  │  (panic → recover → send error → errCh)            │     │ │
│  │  └─────────────────────────────────────────────────┘     │ │
│  │                                                           │ │
│  │  主 goroutine (收集层)：                                  │ │
│  │  for result := range resultCh {                           │ │
│  │    MergeResult(aggregate, result)                         │ │
│  │  }                                                        │ │
│  │                                                           │ │
│  │  → ParseImplResult (exportedTypeMap + 所有提取结果)      │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 文件结构

每个 phase 一个文件（对齐 TS pipeline-phases/ 结构），从现有 pipeline.go 拆分：

```
internal/ingestion/
├── pipeline/
│   ├── types.go              (已有 — PipelinePhase/PipelineContext)
│   ├── runner.go             (已有 — Kahn 拓扑排序)
│   ├── registry.go           (已有 — enabledWhen 门控)
│   ├── options.go            (已有 — PDG/SkipGraphPhases 等)
│   │
│   ├── scan.go               (从 pipeline.go 拆出 — scan phase)
│   ├── structure.go          (从 pipeline.go 拆出 — structure phase)
│   ├── markdown.go           (从 pipeline.go 拆出 — markdown phase)
│   ├── parse.go              (从 pipeline.go 拆出 — parse phase 入口)
│   ├── parse_impl.go         (已有，重构 — 对接 GoroutinePool)
│   ├── routes.go             (从 pipeline.go 拆出 — routes phase)
│   ├── tools.go              (从 pipeline.go 拆出 — tools phase)
│   ├── orm.go                (从 pipeline.go 拆出 — orm phase)
│   ├── cross_file.go         (从 pipeline.go 拆出 — crossFile phase)
│   ├── scope_resolution.go   (从 pipeline.go 拆出 — scopeResolution phase)
│   ├── prune_local_symbols.go(从 pipeline.go 拆出 — pruneLocalSymbols phase)
│   ├── mro.go                (从 pipeline.go 拆出 — mro phase)
│   ├── communities.go        (从 pipeline.go 拆出 — communities phase)
│   ├── processes.go          (从 pipeline.go 拆出 — processes phase)
│   │
│   └── pipeline.go           (精简 — 只保留 BuildPhaseList + RunPipelineFromRepo)
│
├── workers/
│   ├── types.go              (新增 — ParseFileSetResult 等核心类型)
│   ├── goroutine_pool.go     (新增 — Go 原生 goroutine 并发池)
│   ├── parse_engine.go       (新增 — 并发解析引擎核心业务逻辑)
│   ├── result_merge.go       (新增 — 结果合并，替代 TS result-merge.ts)
│   └── quarantine.go         (新增 — Go recover + 错误追踪)
│
│   ❌ 不需要的文件：
│   ❌ worker_pool.go         (TS Node.js Worker Threads — 不适用)
│   ❌ post_result.go         (TS postMessage + clone-safety — 不适用)
│   ❌ clone_safety.go        (TS V8 structured clone — 不适用)
```

与 TS pipeline-phases/ 的对应关系：

| TS 文件 | Go 文件 | 说明 |
|---------|---------|------|
| `types.ts` | `types.go` | 已有 |
| `runner.ts` | `runner.go` | 已有 |
| `registry.ts` | `registry.go` | 已有 |
| `scan.ts` | `scan.go` | 从 pipeline.go 拆出 |
| `structure.ts` | `structure.go` | 从 pipeline.go 拆出 |
| `markdown.ts` | `markdown.go` | 从 pipeline.go 拆出 |
| `parse.ts` | `parse.go` | phase 入口，调 parse_impl |
| `parse-impl.ts` | `parse_impl.go` | 已有，重构 |
| `routes.ts` | `routes.go` | 从 pipeline.go 拆出 |
| `tools.ts` | `tools.go` | 从 pipeline.go 拆出 |
| `orm.ts` | `orm.go` | 从 pipeline.go 拆出 |
| `cross-file.ts` | `cross_file.go` | 从 pipeline.go 拆出 |
| (TS 内部实现) | `scope_resolution.go` | Go 新增独立文件 |
| `prune-local-symbols.ts` | `prune_local_symbols.go` | 从 pipeline.go 拆出 |
| `mro.ts` | `mro.go` | 从 pipeline.go 拆出 |
| `communities.ts` | `communities.go` | 从 pipeline.go 拆出 |
| `processes.ts` | `processes.go` | 从 pipeline.go 拆出 |
| `index.ts` | `pipeline.go` | 已有（精简后） |
| `cobol.ts` | ❌ 不需要 | codetrip 不支持 COBOL |

## 3. 核心类型设计

### 3.1 workers/types.go — 结果类型

```go
// ParseFileSetResult 是单个 chunk 解析的结果。
// 对应 TS ParseWorkerResult，但去掉了 Worker Threads 相关字段。
type ParseFileSetResult struct {
    // ── Graph 节点/边 ──
    Nodes         []shared.NodeRecord
    Relationships []shared.RelationshipRecord
    Symbols       []shared.SymbolRecord

    // ── 提取结果 ──
    Calls           []shared.CallRecord
    Assignments     []shared.AssignmentRecord
    Routes          []ExtractedRoute          // route_extractors 产出
    FetchCalls      []ExtractedFetchCall
    FetchWrapperDefs []FetchWrapperDef
    DecoratorRoutes []ExtractedDecoratorRoute
    RouterIncludes  []ExtractedRouterInclude
    RouterImports   []ExtractedRouterImport
    RouterModuleAliases []ExtractedRouterModuleAlias
    ToolDefs        []ExtractedToolDef
    ORMQueries      []ExtractedORMQuery

    // ── Binding ──
    ConstructorBindings []shared.BindingEntry
    FileScopeBindings   []FileScopeBinding

    // ── Metadata ──
    ParsedFiles    []shared.ParsedFile
    SkippedPaths   map[string]string  // path → reason
    FileCount      int
    ChunkIndex     int
}

// FileScopeBinding 对应 TS parse-worker.ts 的 fileScopeBindings
type FileScopeBinding struct {
    FilePath string
    Bindings []shared.BindingEntry
}
```

### 3.2 workers/goroutine_pool.go — goroutine 并发池

```go
// GoroutinePool 管理 goroutine 并发解析。
// 替代 TS WorkerPool，但利用 Go 原生优势：
//   - goroutine 极轻量（2KB 栈），无需 slot/respawn 机制
//   - channel 原生通信，无需 postMessage/transferList
//   - recover 处理 panic，无需 quarantine/circuit breaker
//   - 共享内存，无需 structured clone
type GoroutinePool struct {
    maxWorkers    int                      // 最大并发 goroutine 数
    semaphore     *semaphore.Weighted      // 并发度控制
    resultCh      chan ParseFileSetResult  // 结果通道
    errCh         chan PoolError           // 错误通道
    wg            sync.WaitGroup           // 等待所有 goroutine 完成
    quarantine    *QuarantineTracker       // 故障文件追踪
    ctx           context.Context          // 取消信号
    cancel        context.CancelFunc       // 取消函数
}

// PoolError 是 goroutine panic 或错误的结果。
type PoolError struct {
    FilePath  string    // 出错的文件路径
    Err       error     // 原始错误
    Recovered bool      // 是否通过 recover 恢复
}

// PoolStats 是池状态统计（对应 TS WorkerPoolStats）。
type PoolStats struct {
    MaxWorkers    int
    ActiveWorkers int
    Quarantined   int
    Errors        int
    Completed     int
}
```

### 3.3 workers/quarantine.go — 故障隔离

```go
// QuarantineTracker 追踪因 panic/错误而跳过的文件路径。
// 替代 TS quarantine.ts（Set<string> 封装）。
// 在 Go 中，goroutine panic 通过 recover 捕获，
// 出错文件被标记为 quarantined，后续 chunk 不再尝试解析。
type QuarantineTracker struct {
    mu      sync.RWMutex
    paths   map[string]string  // path → reason
    maxRetry int               // 最大重试次数（默认 0 — Go 中不需要 respawn）
}
```

## 4. 核心流程设计

### 4.1 GoroutinePool.Dispatch — 并发调度

```go
// Dispatch 将文件集并发分发给 goroutine 解析。
// 替代 TS WorkerPool.dispatch：
//   - TS: postMessage + transferList → worker 线程 → postMessage 回结果
//   - Go: goroutine 直接共享内存 → channel 发回结果
//
// chunk 的文件被按 subBatchSize 分成多个 sub-batch，
// 每个 sub-batch 由一个 goroutine 处理。
// 并发度由 semaphore 控制，保证不超过 maxWorkers。
func (p *GoroutinePool) Dispatch(
    files []FileEntry,           // chunk 内的文件列表
    repoPath string,             // 仓库路径
    onProgress func(int, string), // 进度回调
) ([]ParseFileSetResult, []PoolError) {

    // 将 chunk 文件分为 sub-batch
    subBatches := splitIntoSubBatches(files, p.subBatchSize)

    var results []ParseFileSetResult
    var errors []PoolError

    // 发起所有 sub-batch goroutine
    for i, batch := range subBatches {
        p.wg.Add(1)
        go func(idx int, batch []FileEntry) {
            // ── panic recovery（替代 TS quarantine + circuit breaker）──
            defer func() {
                if r := recover(); r != nil {
                    // panic 被捕获 → 标记 quarantined → 发送 PoolError
                    for _, f := range batch {
                        p.quarantine.Add(f.Path, fmt.Sprintf("panic: %v", r))
                    }
                    p.errCh <- PoolError{
                        FilePath:  batch[0].Path,
                        Err:       fmt.Errorf("panic: %v", r),
                        Recovered: true,
                    }
                }
                p.wg.Done()
                p.semaphore.Release(1)
            }()

            p.semaphore.Acquire(p.ctx, 1)  // 控制并发度

            // ── 核心业务逻辑（替代 TS parse-worker.ts）──
            result := ParseFileSet(batch, repoPath)

            // ── 通过 channel 发回结果（替代 TS postMessage）──
            p.resultCh <- result
        }(i, batch)
    }

    // 主 goroutine 收集结果
    go func() {
        p.wg.Wait()
        close(p.resultCh)
        close(p.errCh)
    }()

    for result := range p.resultCh {
        results = append(results, result)
    }
    for err := range p.errCh {
        errors = append(errors, err)
    }

    return results, errors
}
```

### 4.2 ParseFileSet — 核心业务逻辑

```go
// ParseFileSet 解析一组文件，产出 ParseFileSetResult。
// 对应 TS parse-worker.ts 的核心逻辑（103KB），但去掉 Worker Threads 包装。
//
// 每个 goroutine 内部流程：
//   1. ReadFileContents (共享内存，无需序列化)
//   2. TreeSitter Parse per file
//   3. Run 6 extractors (call/class/field/method/variable/type)
//   4. Run scope extractor
//   5. Run route extractor (route_extractors 包)
//   6. Assemble ParseFileSetResult
func ParseFileSet(files []FileEntry, repoPath string) ParseFileSetResult {
    result := NewEmptyParseFileSetResult()

    for _, file := range files {
        // 1. 获取语言配置
        lang := shared.GetLanguageFromFilename(file.Path)
        if lang == "" {
            result.SkippedPaths[file.Path] = "unknown language"
            continue
        }

        // 2. Tree-sitter 解析
        tree, root, err := ParseWithTreeSitter(file.Content, lang)
        if err != nil {
            result.SkippedPaths[file.Path] = err.Error()
            continue
        }

        // 3. 运行 6 种提取器（复用已有 Go 代码）
        //    call_extractors/generic.go → calls
        //    class_extractors/generic.go → classes → symbols, nodes
        //    field_extractors/generic.go → fields → symbols, nodes
        //    method_extractors → methods → symbols, nodes
        //    variable_extractors → variables → symbols, nodes
        //    type_extractors → types → symbols, nodes

        // 4. 运行 scope extractor
        //    core/scope_extractor.go → fileScopeBindings

        // 5. 运行 route extractor（复用 route_extractors 包）
        //    taint + route_extractors → routes, decoratorRoutes, etc.

        // 6. 运行 fetch/tool/ORM extractor
        //    → fetchCalls, toolDefs, ormQueries

        // 7. 组装 ParsedFile
        parsedFile := shared.ParsedFile{Path: file.Path, ...}
        result.ParsedFiles = append(result.ParsedFiles, parsedFile)
        result.FileCount++
    }

    return result
}
```

### 4.3 RunChunkedParseAndResolve — 重构后的主循环

```go
// RunChunkedParseAndResolve 重构为使用 GoroutinePool。
//
// TS 的主循环流程：
//   for each chunk:
//     1. Prefetch file contents (readFileContents)
//     2. Check parse cache (cache hit → replay, miss → dispatch)
//     3. WorkerPool.dispatch(chunk) → ParseWorkerResult[]
//     4. MergeChunkResults → graph aggregation
//     5. ApplyChunkResults → run-level accumulators
//
// Go 的简化流程：
//   for each chunk:
//     1. Read file contents (共享内存)
//     2. GoroutinePool.Dispatch(chunk) → ParseFileSetResult[]
//     3. MergeResults(aggregate, chunkResults) → 逐字段 append
//     4. (parse cache 可后续集成，当前版本暂不实现)
func RunChunkedParseAndResolve(
    graph *shared.KnowledgeGraph,
    scannedFiles []ScannedFile,
    allPaths []string,
    totalFiles int,
    repoPath string,
    budget int64,
    options *PipelineOptions,
) (*ParseImplResult, error) {
    // 1. 过滤 parseable 文件
    parseable := filterParseableFiles(scannedFiles)

    // 2. Build byte-budget chunks
    chunks := BuildChunks(parseable, budget)

    // 3. 创建 GoroutinePool
    pool := NewGoroutinePool(
        resolveConcurrency(options),  // 默认: runtime.NumCPU()
        context.Background(),
    )
    defer pool.Shutdown()

    // 4. 初始化聚合结果
    aggregate := NewEmptyParseImplResult(allPaths, totalFiles)

    // 5. 逐 chunk 并发解析
    for chunkIdx, chunkPaths := range chunks {
        // 读文件内容
        files := ReadFileContents(repoPath, chunkPaths)

        // 过滤掉 quarantined 文件
        files = pool.Quarantine().FilterOut(files)

        // 并发分发
        chunkResults, chunkErrors := pool.Dispatch(files, repoPath, onProgress)

        // 合并结果
        for _, cr := range chunkResults {
            MergeResults(&aggregate, &cr)
        }

        // 记录错误
        for _, ce := range chunkErrors {
            log.Printf("parse error in %s: %v", ce.FilePath, ce.Err)
        }
    }

    return &aggregate, nil
}
```

## 5. 与 TS 的对比

### 5.1 流程对比

```
TS (Node.js Worker Threads):
  主线程: 构建 chunk → postMessage(transferList) → 等待 → postMessage 返回 → structured clone → 合并
  Worker:  接收 → 反序列化 → AST 解析 → 提取 → structured clone 安全检查 → postMessage 返回

Go (goroutine + channel):
  主 goroutine: 构建 chunk → goroutine 直接共享内存 → 等待 channel → 直接合并
  子 goroutine:  直接读内存 → AST 解析 → 提取 → channel 发结果 → (panic → recover → errCh)
```

### 5.2 不移植的 TS 机制及 Go 替代

| TS 机制 | 不移植原因 | Go 替代 |
|---------|----------|--------|
| Worker Threads (worker-pool.ts 87KB) | Node.js 专用进程隔离 | goroutine (2KB 栈，百万级) |
| sub-batch 分发 | Worker Threads 内部调度 | semaphore.Weighted + channel |
| transferList 零拷贝 | ArrayBuffer 传输机制 | 指针传递（天然零拷贝） |
| structured clone (clone-safety.ts 24KB) | V8 序列化限制 | goroutine 共享内存无此限制 |
| postMessage (post-result.ts 5KB) | Worker↔主线程通信 | channel 传结构体指针 |
| quarantine Set<string> | 故障隔离 | recover + QuarantineTracker map |
| circuit breaker | 防雪崩 | semaphore 控制并发度 + context 取消 |
| slot respawn | 重启崩溃的 Worker | goroutine 不需要"重启" |
| merge pipelining | Worker↔主线程重叠 | goroutine 天然并发，无重叠需求 |

## 6. 并发度策略

### 6.1 默认并发度

```go
func resolveConcurrency(options *PipelineOptions) int {
    if options != nil && options.MaxConcurrent > 0 {
        return options.MaxConcurrent
    }
    // 默认: CPU 核数，但不超过文件 chunk 数
    return runtime.NumCPU()
}
```

### 6.2 sub-batch 大小

```go
// 每个 goroutine 处理一个 sub-batch（一组文件）。
// sub-batch 大小由文件数和字节预算决定：
//   - 太小 → goroutine 调度开销
//   - 太大 → 单个 goroutine 阻塞太久
// 默认: 256KB 或至少 1 个文件
const DefaultSubBatchBytes = 256 * 1024
```

## 7. 实现优先级

### Phase 1 — 基础设施（当前优先）

1. **workers/types.go** — 定义 ParseFileSetResult、PoolError 等核心类型
2. **workers/goroutine_pool.go** — 实现 GoroutinePool（Dispatch/Shutdown/Stats）
3. **workers/quarantine.go** — 实现 QuarantineTracker（Add/Has/FilterOut/Snapshot）
4. **workers/result_merge.go** — 实现 MergeResults（逐字段 append）

### Phase 2 — 核心业务逻辑

5. **workers/parse_engine.go** — 实现 ParseFileSet（AST 解析 + 提取器调用）
6. **重构 pipeline/parse_impl.go** — 对接 GoroutinePool

### Phase 3 — Phase 实际逻辑

7. **pipeline/scan_impl.go** — scan phase（walkRepositoryPaths）
8. **pipeline/structure_impl.go** — structure phase
9. **pipeline/markdown_impl.go** — markdown phase

### Phase 4 — 编译验证

10. 全局编译验证 + 单元测试

## 8. 关键设计决策

1. **goroutine 而非 Worker Threads**：Go 的 goroutine 极轻量，不需要 TS 那样的进程隔离和 slot 管理
2. **channel 而非 postMessage**：Go channel 原生支持类型化通信，无需序列化/反序列化
3. **recover 而非 quarantine/circuit breaker**：Go 的 panic/recover 机制天然替代 TS 的复杂故障隔离
4. **共享内存而非 structured clone**：goroutine 共享地址空间，无 V8 的序列化限制
5. **semaphore 控制并发度**：替代 TS 的 pool size + sub-batch + slot 管理
6. **parse cache 暂不实现**：首版先跑通核心流程，cache 作为后续优化
7. **merge pipelining 不需要**：goroutine 天然并发，主 goroutine 收集结果即可

## 9. 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| tree-sitter parser 并发 | gotreesitter 纯 Go 实现，天然 goroutine 安全 |
| goroutine 数量过多 | semaphore.Weighted 限制并发度 |
| panic 导致 chunk 数据丢失 | recover + QuarantineTracker + 错误日志 |
| 结果合并竞态 | 主 goroutine 顺序收集，无竞态 |