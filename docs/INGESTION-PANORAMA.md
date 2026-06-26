# Ingestion 全景关系图

> **语言范围**: 本文档按 **9 种生效语言** 标注：JavaScript / TypeScript / Python / Java / Go / C / C++ / C# / Rust。
> 代码中另有 7 种降级语言（PHP / Ruby / Kotlin / Swift / Dart / Vue / Cobol），其 grammar/provider/resolver 存在但不在 9 语言生效范围内。
> 图中以 ✅ 标注生效、⚠ 标注降级。LadybugDB 32 种 NODE_TABLES schema 不受语言范围影响，仅实际写入行数减少。

## 1. 顶层 Pipeline 编排

```mermaid
graph TB
    subgraph Pipeline["Pipeline 编排层"]
        P[pipeline.ts<br/>PipelineOptions: skipGraphPhases<br/>pdg / parseCache / workerPoolSize] --> PP[pipeline-phases/]
        PP --> R[runner.ts<br/>Kahn 拓扑排序 + 环检测]
        PP --> REG[registry.ts<br/>enabledWhen 谓词门控]
    end

    subgraph Phases["Pipeline Phases 依赖拓扑"]
        direction TB
        PH1["1 scan"] --> PH2["2 structure"]
        PH2 --> PH3["3 markdown"]
        PH2 --> PH4["4 cobol ⚠"]
        PH3 --> PH5["5 parse"]
        PH4 --> PH5
        PH5 --> PH6["6 routes"]
        PH5 --> PH7["7 tools"]
        PH5 --> PH8["8 orm"]
        PH6 --> PH9["9 crossFile"]
        PH7 --> PH9
        PH8 --> PH9
        PH9 --> PH10["10 scopeResolution"]
        PH10 --> PH11["11 pruneLocalSymbols"]
        PH11 --> PH12["12 mro"]
        PH12 --> PH13["13 communities"]
        PH13 --> PH14["14 processes"]
    end

    P --> Phases
```

## 2. Worker 并行解析架构

```mermaid
graph TB
    subgraph Main["主线程"]
        FW[filesystem-walker.ts<br/>递归扫描 + .gitignore 过滤] --> WP[worker-pool.ts<br/>分片调度]
    end

    subgraph Pool["Worker 池"]
        WP --> |Uint8Array 零拷贝| W1["Worker 1<br/>parse-worker.ts"]
        WP --> |Uint8Array 零拷贝| W2["Worker 2"]
        WP --> |Uint8Array 零拷贝| WN["Worker N"]
    end

    subgraph Merge["结果合并"]
        W1 --> RM[result-merge.ts<br/>appendAll 合并]
        W2 --> RM
        WN --> RM
        RM --> Q[quarantine.ts<br/>故障隔离]
    end

    WP -.-> |TextEncoder 编码<br/>ArrayBuffer transferList| W1
```

## 3. Parse Worker 内部流程

```mermaid
graph TB
    subgraph Grammars["语法加载 (9 种生效 + 7 种降级)"]
        G1[javascript / typescript / tsx]
        G2[python / java / go]
        G3[c-sharp / cpp / c / rust]
        G4_off["⚠ 降级: php / ruby<br/>grammar 在但代码逻辑不生效"]
        G5_off["⚠ 降级: swift / dart / kotlin<br/>vendored grammars，条件加载"]
        G6_vue["⚠ Vue: 复用 TS grammar<br/>无独立 grammar"]
        G7_cobol["⚠ Cobol: 无 grammar<br/>regex 提取"]
    end

    subgraph Parse["AST 解析"]
        GL[语法选择<br/>getLanguageFromFilename] --> SP[safe-parse<br/>try/catch 包裹]
        SP --> |Tree| TQ[tree-sitter-queries<br/>Capture 匹配]
    end

    subgraph Legacy["传统提取路径 (6 种提取器)"]
        TQ --> CE[call-extractors]
        TQ --> CLE[class-extractors]
        TQ --> FE[field-extractors]
        TQ --> ME[method-extractors]
        TQ --> VE[variable-extractors]
        TQ --> TE[type-extractors]
    end

    subgraph ScopePath["Scope 提取路径"]
        TQ --> SEC[provider.emitScopeCaptures<br/>语言自定义 Capture]
    end

    subgraph Other["其他提取"]
        TQ --> RE[route-extractors<br/>Next.js / Spring / FastAPI<br/>⚠ Laravel(PHP)降级]
        TQ --> VE2[vue-sfc-extractor<br/>⚠ Vue降级，不生效]
        TQ --> CUP[cpp-ue-preprocessor<br/>UE 反射宏预处理]
    end

    subgraph Assemble["结果组装"]
        CE --> RS[ParseWorkerResult]
        CLE --> RS
        FE --> RS
        ME --> RS
        VE --> RS
        TE --> RS
        SEC --> RS
        RE --> RS
        RS --> PCS[postResultCloneSafe<br/>结构化克隆安全序列化]
    end

    Grammars --> Parse
```

## 4. Scope Extractor 五遍流水线

```mermaid
graph TB
    INPUT["CaptureMatch 数组<br/>来自 provider.emitScopeCaptures"]

    subgraph Pass1["Pass 1: Build Scope Tree"]
        P1A["遍历 @scope.* captures"]
        P1B["provider.resolveScopeKind<br/>默认: capture 名后缀"]
        P1C["按词法范围包含推导父节点"]
        P1D["buildScopeTree 验证不变量<br/>非-module 有 parent<br/>parent 包含 child<br/>兄弟不重叠"]
        P1A --> P1B --> P1C --> P1D
    end

    subgraph Pass2["Pass 2: Attach Declarations"]
        P2A["遍历 @declaration.* captures"]
        P2B["构建 SymbolDefinition"]
        P2C["provider.bindingScopeFor<br/>默认: 最内层包含 scope"]
        P2D["附加到 Scope.ownedDefs<br/>+ 本地 BindingRef"]
        P2A --> P2B --> P2C --> P2D
    end

    subgraph Pass3["Pass 3: Collect Imports"]
        P3A["遍历 @import.* captures"]
        P3B["provider.interpretImport<br/>返回 ParsedImport"]
        P3C["附加到 ParsedFile.parsedImports<br/>非 Scope (finalize 重建)"]
        P3A --> P3B --> P3C
    end

    subgraph Pass4["Pass 4: Collect Type Bindings"]
        P4A["遍历 @type-binding.* captures"]
        P4B["provider.interpretTypeBinding<br/>返回 TypeRef"]
        P4C["附加到 Scope.typeBindings<br/>或 provider.bindingScopeFor 覆盖"]
        P4A --> P4B --> P4C
    end

    subgraph Pass5["Pass 5: Collect Reference Sites"]
        P5A["遍历 @reference.* captures"]
        P5B["provider.classifyCallForm<br/>默认: capture sub-tag 或 free"]
        P5C["生成 ReferenceSite<br/>kind: call/read/write/inherits/type-ref/import-use"]
        P5A --> P5B --> P5C
    end

    INPUT --> Pass1 --> Pass2 --> Pass3 --> Pass4 --> Pass5
    Pass5 --> OUTPUT["ParsedFile<br/>scopeTree + localDefs<br/>parsedImports + referenceSites"]
```

## 5. Finalize Orchestrator

```mermaid
graph TB
    INPUT["ParsedFile 数组<br/>来自 ScopeExtractor"]

    subgraph Finalize["finalizeScopeModel"]
        FI["调用 gitnexus-shared finalize()<br/>合并跨文件 imports/wildcards<br/>到 Scope.bindings"]
        FI --> |"FinalizeFile 数组"| IDX[构建 4 个索引]
    end

    subgraph Indexes["4 个工作区索引"]
        DI["buildDefIndex<br/>defId → SymbolDefinition"]
        ST["buildScopeTree<br/>ScopeId → Scope"]
        MSI["buildModuleScopeIndex<br/>filePath → Module Scope"]
        QNI["buildQualifiedNameIndex<br/>qualifiedName → defId"]
        MDI["buildMethodDispatchIndex<br/>ownerId + methodName → Method defs"]
    end

    subgraph Output["ScopeResolutionIndexes"]
        SRI["scopes: ScopeTree<br/>defs: DefIndex<br/>bindings: 跨文件绑定<br/>sccs: Tarjan SCC<br/>methodDispatch: 方法分派<br/>qualifiedNames: 限定名<br/>moduleScopes: 模块作用域"]
    end

    INPUT --> Finalize
    IDX --> DI
    IDX --> ST
    IDX --> MSI
    IDX --> QNI
    IDX --> MDI
    DI --> SRI
    ST --> SRI
    MSI --> SRI
    QNI --> SRI
    MDI --> SRI
```

## 6. Scope Resolution 完整流水线

```mermaid
graph TB
    INPUT["scannedFiles<br/>按语言过滤"]

    subgraph Registry["ScopeResolver 注册表 (9 生效 + 7 降级)"]
        REG["SCOPE_RESOLVERS Map<br/>✅ Python / CSharp / TypeScript<br/>✅ Go / Java / C / C++ / Rust<br/>✅ JavaScript<br/>⚠ 降级: PHP / Ruby / Kotlin<br/>⚠ 降级: Swift / Dart<br/>⚠ 降级: Cobol / Vue"]
    end

    subgraph Phase1["Phase 1: Extract"]
        EX["extractParsedFile<br/>scope-extractor-bridge.ts<br/>provider.emitScopeCaptures<br/>→ ScopeExtractor.extract"]
        EX --> PF["ParsedFile"]
    end

    subgraph Phase2["Phase 2: Finalize"]
        FO["finalizeScopeModel<br/>finalize() + 4 索引构建"]
        PF --> FO
        FO --> IDX["ScopeResolutionIndexes"]
    end

    subgraph Phase2b["Phase 2b: 类型传播"]
        IRT["propagateImportedReturnTypes<br/>SCC 逆拓扑序<br/>跨文件返回类型镜像<br/>chain-follow 深度 8"]
        IDX --> IRT
    end

    subgraph Phase3["Phase 3: Resolve"]
        RR["resolveReferenceSites<br/>Registry.lookup<br/>Method/Class/Field Registry"]
        IRT --> RR
        RR --> REFI["ReferenceIndex"]
    end

    subgraph Phase4["Phase 4: Emit"]
        RBC["emitReceiverBoundCalls<br/>7-case 分发器<br/>FIRST — I1 不变量"]
        FCF["emitFreeCallFallback<br/>导入可见的 free call<br/>THEN — I2 不变量"]
        RVL["emitReferencesViaLookup<br/>共享 resolver 遗留<br/>LAST — 使用 handledSites"]
        EIE["emitImportEdges<br/>IMPORTS 边发射"]
        REFI --> RBC
        REFI --> FCF
        REFI --> RVL
        REFI --> EIE
    end

    subgraph PDG["PDG 窗口 (--pdg opt-in)"]
        CFG["emitFileCfgs<br/>BasicBlock + CFG 边"]
        RD["emitFileReachingDefs<br/>REACHING_DEF 边"]
        TAI["emitFileTaint<br/>TAINTED + SANITIZES 边"]
        RBC --> CFG --> RD --> TAI
    end

    INPUT --> Registry --> Phase1 --> Phase2 --> Phase2b --> Phase3 --> Phase4
    Phase4 --> OUTPUT["KnowledgeGraph<br/>CALLS / ACCESSES / INHERITS<br/>USES / IMPORTS edges"]
```

## 7. Receiver-Bound Calls 7-Case 分发器

```mermaid
graph TB
    INPUT["ReferenceSite<br/>receiver + callee"]

    subgraph Cases["7-Case 分发 (顺序敏感, 首个匹配胜出)"]
        C0["Case 0: super<br/>provider.isSuperReceiver<br/>→ MRO walk 跳过 self"]
        C1["Case 1: compound<br/>receiver 含 . 或 (<br/>→ compound-receiver.ts"]
        C2["Case 2: namespace<br/>receiver 在 namespaceTargets<br/>→ exported def"]
        C3["Case 3: class-name<br/>receiver 解析为 class-like<br/>→ MRO walk<br/>静态调用 ACCESSES"]
        C4["Case 3b: dotted typeBinding<br/>typeRef.rawName 含 dot<br/>但非 namespace 前缀<br/>→ compound resolver"]
        C5["Case 4: simple typeBinding<br/>typeRef.rawName 无 dot<br/>→ MRO walk + findOwnedMember"]
        C6["Case 5: value-receiver<br/>Const/Variable 的 ownerId<br/>在 model.methods 中<br/>对象字面量服务"]
    end

    C0 --> |不匹配| C1
    C1 --> |不匹配| C2
    C2 --> |不匹配| C3
    C3 --> |不匹配| C4
    C4 --> |不匹配| C5
    C5 --> |不匹配| C6

    C0 --> EMIT["tryEmitEdge<br/>CALLS / ACCESSES"]
    C1 --> EMIT
    C2 --> EMIT
    C3 --> EMIT
    C4 --> EMIT
    C5 --> EMIT
    C6 --> EMIT

    INPUT --> Cases
```

## 8. Overload Narrowing 决策树

```mermaid
graph TB
    INPUT["同名人选列表<br/>argCount + argTypes"]

    S1["Step 1: arity 过滤<br/>精确必需参数匹配<br/>优先于可变参数"]
    S1 --> |空集| S1B["有未知边界候选?<br/>是 → 回退全列表<br/>否 → 信任过滤 (空)"]
    S1 --> |非空| S2

    S2["Step 2: 精确类型过滤<br/>argTypes 逐槽相等<br/>空串 = unknown = 匹配"]
    S2 --> |非空类型化结果| WIN["返回"]
    S2 --> |空| S3

    S3["Step 3: Conversion Rank<br/>ISO C++ over.ics.rank<br/>F1 优于 F2: 每参不差 + 至少一优<br/>返回非支配候选"]
    S3 --> |有唯一最优| WIN
    S3 --> |多候选| S4

    S4["Step 4: Constraint Filter<br/>SFINAE / requires<br/>compatible/incompatible/unknown<br/>unknown 保留 (单调性)"]
    S4 --> S5

    S5["Step 5: Template Partial Order<br/>T* 优于 T<br/>const T& 优于 T<br/>更特化优先"]
    S5 --> WIN

    INPUT --> S1
```

## 9. Compound Receiver 解析

```mermaid
graph TB
    INPUT["复合接收者表达式<br/>user.address.save()<br/>svc.get_user().save()"]

    subgraph Shapes["三种形态"]
        S1["bare identifier<br/>name → typeBinding 链"]
        S2["dotted chain<br/>obj.field.field<br/>逐字段走 class-scope typeBindings"]
        S3["call chain<br/>expr.method()<br/>递归入 expr → 找方法返回类型"]
    end

    subgraph Options["语言特定选项"]
        O1["fieldFallback<br/>默认 true (Python)<br/>静态语言应关闭"]
        O2["unwrapCollectionAccessor<br/>Dictionary K,V → V"]
        O3["hoistTypeBindingsToModule<br/>C# 方法返回类型<br/>挂载到 Module scope"]
    end

    INPUT --> Shapes
    S1 --> RESOLVE["resolveCompoundReceiverClass<br/>最大深度 8"]
    S2 --> RESOLVE
    S3 --> RESOLVE
    Options --> RESOLVE
    RESOLVE --> OUTPUT["SymbolDefinition<br/>接收者类的 class def"]
```

## 10. Semantic Model 分层架构

```mermaid
graph TB
    subgraph Leaf["叶子层"]
        ST["symbol-table.ts<br/>fileIndex + callableByName<br/>双索引纯数据结构"]
    end

    subgraph Registry["注册表层"]
        TR["type-registry.ts<br/>ownerId → Type defs"]
        MR["method-registry.ts<br/>ownerId → Method defs<br/>lookupAllByOwner<br/>lookupMethodByName"]
        FR["field-registry.ts<br/>ownerId → Field defs"]
    end

    subgraph Dispatch["分发表层"]
        RT["registration-table.ts<br/>NodeLabel → hook 分发<br/>Function-with-ownerId<br/>路由为 Method"]
    end

    subgraph Orchestrator["编排器层"]
        SM["semantic-model.ts<br/>fan-out add:<br/>1. rawSymbols.add<br/>2. 预分发规范化<br/>3. 分发表 hook → registry"]
    end

    subgraph Consumer["消费者"]
        CP["call-processor.ts"]
        MRO["mro-processor.ts"]
        SR["scope-resolution<br/>passes"]
        ER["emit-references.ts"]
    end

    ST --> TR
    ST --> MR
    ST --> FR
    TR --> RT
    MR --> RT
    FR --> RT
    RT --> SM

    SM --> CP
    SM --> MRO
    SM --> SR
    SM --> ER

    subgraph WRI["WorkspaceResolutionIndex<br/>scope-resolution 专用"]
        CS["classScopeByDefId<br/>defId → Scope"]
        CI["classScopeIdToDefId<br/>ScopeId → defId"]
        MS["moduleScopeByFile<br/>filePath → Scope"]
        EC["exportedCallableByName<br/>simpleName → SymbolDef"]
    end
```

## 11. TypeEnv 三层推断

```mermaid
graph TB
    INPUT["AST + provider<br/>单文件"]

    subgraph Tier0["Tier 0: 注解类型"]
        T0["从类型注解提取<br/>参数类型 / 返回类型<br/>extractSimpleTypeName<br/>stripNullable"]
        T0 --> ENV["TypeEnv<br/>(scope, varName) → typeName"]
    end

    subgraph Tier1["Tier 1: 构造函数推断"]
        T1["new ClassName()<br/>从构造函数调用推断<br/>变量类型 = 类名"]
        T1 --> ENV
    end

    subgraph Tier2["Tier 2: 赋值链传播"]
        T2["const b = a<br/>a 已有 Tier 0/1 类型<br/>b 继承 a 的类型<br/>单遍源码顺序传播"]
        T2 --> ENV
    end

    INPUT --> Tier0 --> Tier1 --> Tier2
    ENV --> LOOKUP["lookup(varName, callNode)<br/>self/this/super AST 解析<br/>作用域感知类型查找"]
```

## 12. 提取器 Generic + Config 模式

```mermaid
graph TB
    subgraph Pattern["通用模式"]
        GEN["generic.ts<br/>工厂函数<br/>createXxxExtractor(config)"]
        CFG["语言 Config<br/>声明式配置"]
        RUN["运行时 Extractor<br/>extract(node, nameNode)"]
        GEN --> |"config 驱动"| RUN
        CFG --> GEN
    end

    subgraph CallExt["call-extractors/"]
        CG["generic.ts<br/>createCallExtractor"]
        CC["9 生效语言 configs<br/>✅ c-cpp / csharp / go<br/>✅ jvm(Java) / python<br/>✅ rust / typescript-javascript<br/>⚠ 降级: dart / php / ruby / swift"]
        CG --> |"Path 1: 语言特异"| L1["config.extractLanguageCallSite<br/>Java :: 方法引用等"]
        CG --> |"Path 2: 通用"| L2["inferCallForm<br/>extractReceiverName<br/>extractMixedChain"]
    end

    subgraph MethodExt["method-extractors/"]
        MG["generic.ts<br/>createMethodExtractor"]
        MC["9 生效语言 configs<br/>✅ c-cpp / csharp / go<br/>✅ jvm(Java) / python<br/>✅ rust / typescript-javascript<br/>⚠ 降级: dart / php / ruby / swift"]
    end

    subgraph FieldExt["field-extractors/"]
        FG["generic.ts<br/>typescript.ts 特殊"]
        FC["9 生效语言 configs + helpers<br/>✅ c-cpp / csharp / go / jvm<br/>✅ python / rust / typescript-javascript<br/>⚠ 降级: dart / php / ruby / swift"]
    end

    subgraph TypeExt["type-extractors/"]
        TSH["shared.ts<br/>extractSimpleTypeName<br/>stripNullable<br/>extractReturnTypeName"]
        TIM["9 生效语言实现<br/>✅ c-cpp / csharp / go / jvm<br/>✅ python / rust / typescript<br/>⚠ 降级: dart / php / ruby / swift"]
    end
```

## 13. Import Resolver Factory 策略链

```mermaid
graph TB
    subgraph Factory["resolver-factory.ts"]
        CIF["createImportResolver<br/>(config: ImportResolutionConfig)"]
    end

    subgraph Chain["策略链 (声明顺序, 首个非 null 胜出)"]
        S1["Strategy 1<br/>如: standard.ts<br/>路径规范化"]
        S2["Strategy 2<br/>如: go module<br/>go.mod 解析"]
        S3["Strategy N<br/>如: jvm classpath<br/>composer / csproj"]
        S1 --> |null| S2
        S2 --> |null| S3
        S3 --> |null| NULL["返回 null<br/>未解析"]
        S1 --> |非 null| WIN["返回结果<br/>files 数组"]
        S2 --> |非 null| WIN
        S3 --> |非 null| WIN
    end

    subgraph Configs["9 生效语言 Configs"]
        IC["✅ csharp / go / jvm(Java)<br/>✅ python / rust / typescript<br/>✅ c-cpp<br/>⚠ 降级: php / ruby / swift / dart<br/>⚠ 降级: kotlin(复用jvm)"]
    end

    subgraph Utils["共享工具"]
        IU["import-resolvers/utils.ts<br/>路径规范化<br/>扩展名补全"]
        IS["import-resolvers/standard.ts<br/>标准相对路径解析"]
    end

    CIF --> Chain
    Configs --> CIF
    CIF --> Chain
    Configs --> CIF
    IU --> IS
```

## 14. CFG Builder + Taint 子系统

```mermaid
graph TB
    subgraph CFG_Sub["CFG 子系统"]
        subgraph Builder["CfgBuilder (语言无关累加器)"]
            CB["cfg-builder.ts<br/>entry/exit block<br/>newBlock + connect<br/>边去重 + finish"]
        end

        subgraph Visitor["CfgVisitor (语言驱动)"]
            VT["visitors/typescript.ts<br/>AST → blocks + edges"]
            VTH["visitors/typescript-harvest.ts<br/>def/use 事实收集"]
        end

        subgraph Context["ControlFlowContext"]
            CFC["control-flow-context.ts<br/>break/continue/return<br/>目标栈管理"]
        end

        Visitor --> Builder
        Context --> Visitor
    end

    subgraph RD_Sub["Reaching Definitions"]
        RD["reaching-defs.ts<br/>computeReachingDefs<br/>maxFacts 上限<br/>truncated 标记"]
    end

    subgraph Taint_Sub["Taint 分析 (opt-in --pdg)"]
        subgraph Pipeline["每函数流水线 (顺序敏感)"]
            T0["1. hasTaintSafeSites<br/>损坏站点 → SKIP"]
            T1["2. matchFunctionSites<br/>语言 source/sink 匹配"]
            T2["3. ZERO-MATCH 快路径<br/>无 source+sink → 跳过 RD"]
            T3["4. computeReachingDefs<br/>taint maxFacts<br/>与 RD 同公式"]
            T4["5. computeTaintFlows<br/>传播 + dedup"]
            T5["6. emit TAINTED + SANITIZES"]
            T0 --> T1 --> T2 --> T3 --> T4 --> T5
        end

        subgraph Config["Source/Sink 配置"]
            TSC["source-sink-config.ts<br/>语言特定规范"]
            TSR["source-sink-registry.ts<br/>注册 + 查询"]
            TTM["typescript-model.ts<br/>内置模型"]
        end

        subgraph Prop["传播引擎"]
            TP["propagate.ts<br/>污点传播"]
            TM["match.ts<br/>站点匹配"]
            TPC["path-codec.ts<br/>hop 编码/解码"]
            TSS["site-safety.ts<br/>安全性判定"]
        end

        Config --> Pipeline
        Prop --> T4
    end

    Builder --> RD
    RD --> T3
```

## 15. MRO Processor 流程

```mermaid
graph TB
    INPUT["KnowledgeGraph<br/>EXTENDS / IMPLEMENTS / HAS_METHOD 边"]

    subgraph Build["邻接表构建"]
        BA["buildAdjacency<br/>childId → parentIds<br/>parentId → childIds<br/>classId → methodIds"]
    end

    subgraph Linearize["MRO 线性化"]
        subgraph Rules["语言特定规则"]
            R_CPP["C++: 左优先<br/>声明顺序最先"]
            R_CS["C#/Java: 类优先于接口<br/>多接口同名 → 模糊"]
            R_PY["Python: C3 线性化<br/>c3Linearize()"]
            R_RS["Rust: 无自动解析<br/>qualified syntax"]
            R_GO["Go: 接口隐式实现<br/>struct→interface"]
            R_DEF["Default: 单继承<br/>首个定义胜出"]
        end
    end

    subgraph Detect["冲突检测"]
        CD["方法名碰撞<br/>跨父类同名方法<br/>→ MethodAmbiguity"]
    end

    subgraph Emit["边发射"]
        OE["METHOD_OVERRIDES<br/>Class → winning Method<br/>reason: 解析策略"]
        IE["METHOD_IMPLEMENTS<br/>Interface → implementing Method"]
    end

    INPUT --> Build --> Linearize --> Detect --> Emit
    Rules --> Linearize
```

## 16. Community + Process Detection

```mermaid
graph TB
    subgraph Community["社区检测 (community-processor.ts)"]
        subgraph Algorithm["Leiden 算法"]
            GRAPH["构建 graphology 图<br/>CALLS 关系作为边"]
            LEIDEN["Leiden 详细模式<br/>seeded PRNG (mulberry32)<br/>LEIDEN_SEED = 0xc0de<br/>确定性输出"]
            GRAPH --> LEIDEN
        end

        subgraph Enrich["语义增强"]
            CE["cluster-enricher.ts<br/>LLM 语义命名<br/>社区 → 功能描述"]
            LEIDEN --> CE
        end
    end

    subgraph Process["入口点检测 (process-processor.ts)"]
        subgraph Score["评分系统"]
            EPS["entry-point-scoring.ts<br/>语言特定 entryPointPatterns<br/>AST framework patterns"]
            FW["framework-detection.ts<br/>运行时框架识别"]
            EPS --> SCORE["入口点评分<br/>multiplier × reason"]
        end

        subgraph Track["前向追踪"]
            BFS["BFS 前向追踪<br/>CALLS 边遍历<br/>可达节点集"]
            SCORE --> BFS
        end
    end

    Community --> OUTPUT["KnowledgeGraph<br/>COMMUNITY 节点<br/>PROCESS 节点"]
    Process --> OUTPUT
```

## 17. 语言提供者注册表 + ScopeResolver

```mermaid
graph TB
    subgraph LP["LanguageProvider (解析侧契约)"]
        LPI["language-provider.ts<br/>~40 字段<br/>emitScopeCaptures<br/>resolveScopeKind<br/>interpretImport<br/>interpretTypeBinding<br/>classifyCallForm<br/>bindingScopeFor"]
    end

    subgraph SR["ScopeResolver (发射侧契约)"]
        SRI["scope-resolver.ts<br/>8 必需字段<br/>language / languageProvider<br/>importEdgeReason<br/>resolveImportTarget<br/>mergeBindings<br/>arityCompatibility<br/>buildMro<br/>populateOwners<br/>isSuperReceiver"]
        OPT["可选开关<br/>propagatesReturnTypes<br/>fieldFallbackOnMethodLookup<br/>unwrapCollectionAccessor<br/>collapseMemberCalls<br/>populateNamespaceSiblings<br/>hoistTypeBindingsToModule"]
        SRI --> OPT
    end

    subgraph Providers["9 生效语言双契约实例"]
        LP_PY["python: python.ts<br/>+ scope-resolver.ts"]
        LP_CS["csharp: csharp.ts<br/>+ scope-resolver.ts"]
        LP_TS["typescript: typescript.ts<br/>+ scope-resolver.ts"]
        LP_GO["go: go.ts<br/>+ scope-resolver.ts"]
        LP_JA["java: java.ts<br/>+ scope-resolver.ts"]
        LP_CPP["cpp: cpp.ts<br/>+ scope-resolver.ts"]
        LP_C["c: c.ts<br/>+ scope-resolver.ts"]
        LP_RS["rust: rust.ts<br/>+ scope-resolver.ts"]
        LP_JS["javascript: javascript.ts<br/>+ scope-resolver.ts"]
    end

    subgraph ProvidersDegraded["⚠ 7 种降级语言"]
        LP_KT["kotlin: kotlin.ts<br/>+ scope-resolver.ts"]
        LP_PH["php: php.ts<br/>+ scope-resolver.ts"]
        LP_RB["ruby: ruby.ts<br/>+ scope-resolver.ts"]
        LP_CO["cobol: cobol.ts<br/>+ scope-resolver.ts"]
        LP_SW["swift: swift.ts<br/>+ scope-resolver.ts"]
        LP_DA["dart: dart.ts<br/>+ scope-resolver.ts"]
        LP_VU["vue: vue.ts<br/>+ scope-resolver.ts"]
    end

    LPI --> LP_PY
    SRI --> LP_PY
    LPI --> LP_CS
    SRI --> LP_CS
    LPI --> LP_TS
    SRI --> LP_TS
```

## 18. 核心模块关系总览

```mermaid
graph TB
    subgraph Core["核心抽象"]
        LP["language-provider.ts"]
        LC["language-config.ts<br/>TsconfigPaths<br/>GoModuleConfig<br/>ComposerConfig"]
        TE_ENV["type-env.ts<br/>TypeEnv 三层推断"]
        BA["binding-accumulator.ts"]
        FW["filesystem-walker.ts"]
        FD["framework-detection.ts"]
        TSQ["tree-sitter-queries.ts"]
    end

    subgraph Types["类型定义"]
        CT["call-types.ts"]
        CLT["class-types.ts"]
        FT["field-types.ts"]
        MT["method-types.ts"]
        VT["variable-types.ts"]
        TET["type-extractors/types.ts"]
        IRT["import-resolvers/types.ts"]
    end

    subgraph ScopeRes["Scope Resolution"]
        SE["scope-extractor.ts"]
        SEB["scope-extractor-bridge.ts"]
        FO["finalize-orchestrator.ts"]
        RR["resolve-references.ts"]
        SRP["scope-resolution/pipeline"]
    end

    subgraph Model["语义模型"]
        SM["semantic-model.ts"]
        ST["symbol-table.ts"]
        SRI["scope-resolution-indexes.ts"]
    end

    subgraph Emit["图发射"]
        ER["emit-references.ts"]
        ED["export-detection.ts"]
        CR["call-routing.ts"]
    end

    subgraph Workers["Worker 并行"]
        PW["worker-pool.ts"]
        WK["parse-worker.ts"]
        RM["result-merge.ts"]
        QS["quarantine.ts"]
    end

    subgraph Post["后处理器"]
        MRO["mro-processor.ts"]
        CMP["community-processor.ts"]
        CEP["cluster-enricher.ts"]
        EPP["process-processor.ts"]
    end

    subgraph Special["特殊处理器"]
        SP1["⚠ cobol-processor.ts<br/>regex 处理 (9语言下不生效)"]
        SP2["markdown-processor.ts<br/>regex 处理"]
        SP3["⚠ vue-sfc-extractor.ts<br/>script 块 (9语言下不生效)"]
        SP4["cpp-ue-preprocessor.ts<br/>UE 反射宏"]
        SP5["csharp-namespace-gate.ts<br/>BCL using 回退"]
    end

    LP --> Types
    LP --> IRT
    LP --> ED
    LP --> CR
    SE --> LP
    SEB --> SE
    FO --> SRP
    RR --> Model
    SM --> ST
    SM --> SRI
    ER --> SM
    TE_ENV --> BA
    TE_ENV --> TET
    WK --> LP
    PW --> WK
    PW --> RM
    MRO --> SM
    CMP --> ER
    CEP --> CMP
    EPP --> FD
    LC --> IRT
```

## 19. 数据流总览

```mermaid
graph LR
    FS["文件系统"] --> |scan| FW["filesystem-walker"]
    FW --> |文件列表| SP["structure-processor<br/>Folder/File 节点"]
    FW --> |文件列表| PP["parse-phase"]
    PP --> |Worker 并行| WK["parse-worker"]
    WK --> |CaptureMatches| SE["scope-extractor<br/>五遍流水线"]
    WK --> |nodes+edges| SM["semantic-model<br/>SymbolTable+Registries"]
    SE --> |ParsedFiles| FO["finalize-orchestrator<br/>ScopeResolutionIndexes"]
    FO --> |Indexes| SR["scope-resolution<br/>4 个 emit passes"]
    SR --> |CALLS/ACCESSES/INHERITS| KG["KnowledgeGraph"]
    KG --> |graph| MRO_P["mro-processor<br/>METHOD_OVERRIDES"]
    KG --> |graph| CMP["community-processor<br/>Leiden 社区"]
    CMP --> |COMMUNITY| CEP["cluster-enricher<br/>LLM 语义命名"]
    KG --> |graph| EPP["process-processor<br/>BFS 前向追踪"]
```

## 20. 存储层架构 — .gitnexus 目录结构

```mermaid
graph TB
    REPO["仓库根目录"] --> GN[".gitnexus/"]
    GN --> META["meta.json<br/>RepoMeta 持久化<br/>lastCommit / fileHashes<br/>incrementalInProgress<br/>pdg / schemaVersion"]
    GN --> LB["lbug/<br/>LadybugDB 数据库文件<br/>+ .wal / .lock sidecar"]
    GN --> PC["parse-cache/<br/>chunk 级内容寻址缓存<br/>SCHEMA_BUMP=6 + pkg版本<br/>sha256 chunk key"]
    GN --> PFS["parsedfile-store/<br/>运行期 ParsedFile JSON 分片<br/>每次解析前清除"]
    GN --> PFC["parsedfile-cache/<br/>持久化内容寻址 ParsedFile<br/>与 parse-cache 同版本<br/>热缓存命中时 byte-copy 恢复"]
    GN --> SIS["scope-index-store/<br/>磁盘 Scope 分片<br/>GITNEXUS_DISK_SCOPE_INDEX=1 启用<br/>LRU 骨架 + 按需加载"]
    GN --> CSV["csv/<br/>临时 CSV 中间文件<br/>loadGraphToLbug 后清除"]
    GN --> BR["branches/<br/>多分支索引<br/>branchSlug-safe-hash<br/>每分支独立 lbug/ + meta.json"]
    GN --> GI[".gitignore<br/>自动写入 * 忽略"]
```

## 21. LadybugDB 数据模型 — 32 种节点表 + 1 关系表

```mermaid
graph TB
    subgraph CoreNodes["核心代码节点 (schema.ts)"]
        F["File<br/>id / name / filePath / content"]
        FD["Folder<br/>id / name / filePath"]
        FN["Function<br/>id / name / filePath / startLine / endLine<br/>isExported / content / description"]
        CL["Class<br/>同 Function 字段"]
        IF["Interface<br/>同 Function 字段"]
        MT["Method<br/>同 Function + parameterCount / returnType"]
        CE["CodeElement<br/>同 Function 字段 (通用回退)"]
    end

    subgraph MultiLang["多语言节点 (19 种，9语言实际生效)"]
        ST["Struct / Enum / Macro / Typedef<br/>id / name / filePath / startLine / endLine<br/>content / description"]
        NS["Namespace / Trait / Impl / TypeAlias<br/>同上"]
        CO["Const / Static / Variable<br/>同上"]
        PR["Property<br/>同上 + declaredType"]
        RC["Record / Delegate / Annotation<br/>Constructor / Template / Module<br/>同上"]
    end

    subgraph SpecialNodes["特殊节点"]
        CM["Community<br/>id / label / heuristicLabel / keywords<br/>description / enrichedBy / cohesion / symbolCount"]
        PC["Process<br/>id / label / heuristicLabel / processType<br/>stepCount / communities / entryPointId / terminalId"]
        SEC["Section<br/>id / name / filePath / startLine / endLine<br/>level / content / description"]
        RT["Route<br/>id / name / filePath<br/>responseKeys / errorKeys / middleware"]
        TL["Tool<br/>id / name / filePath / description"]
        BB["BasicBlock<br/>id / filePath / startLine / endLine / text"]
    end

    subgraph Embedding["向量嵌入"]
        EM["CodeEmbedding<br/>id / nodeId / chunkIndex / startLine / endLine<br/>embedding FLOAT 384d / contentHash"]
        VI["HNSW 向量索引<br/>code_embedding_idx<br/>cosine 相似度"]
        EM --> VI
    end

    subgraph RelTable["CodeRelation (单一关系表)"]
        RT_CORE["核心: CONTAINS / DEFINES / IMPORTS / CALLS<br/>EXTENDS / IMPLEMENTS / HAS_METHOD<br/>HAS_PROPERTY / ACCESSES"]
        RT_MRO["MRO: METHOD_OVERRIDES / OVERRIDES<br/>METHOD_IMPLEMENTS"]
        RT_GRAPH["图级: MEMBER_OF / STEP_IN_PROCESS"]
        RT_API["API: HANDLES_ROUTE / FETCHES<br/>HANDLES_TOOL / ENTRY_POINT_OF"]
        RT_MISC["WRAPS / QUERIES"]
        RT_PDG["PDG: CFG / REACHING_DEF<br/>TAINTED / SANITIZES / TAINT_PATH"]
        RT_ATTR["通用属性: type / confidence<br/>reason / step"]
    end

    CoreNodes --> RelTable
    MultiLang --> RelTable
    SpecialNodes --> RelTable
```

## 22. KnowledgeGraph → LadybugDB 写入流水线 — 汇聚后一次性写入

```mermaid
graph TB
    subgraph Ingestion["14 Phase 汇聚到同一个 KnowledgeGraph"]
        P1["Phase 1-5: scan/structure/md/⚠cobol(parse)/parse<br/>⚠ cobol 9语言下不生效<br/>File/Folder/Section 节点 + 19种符号节点"]
        P6["Phase 6-8: routes/tools/orm<br/>Route/Tool 节点 + 特殊边"]
        P9["Phase 9: crossFile<br/>IMPORTS 边"]
        P10["Phase 10: scopeResolution<br/>CALLS/ACCESSES/INHERITS/EXTENDS<br/>CFG/REACHING_DEF/TAINTED/SANITIZES 边"]
        P11["Phase 11: pruneLocalSymbols<br/>修剪私有符号"]
        P12["Phase 12: mro<br/>METHOD_OVERRIDES/METHOD_IMPLEMENTS 边"]
        P13["Phase 13: communities<br/>Community 节点 + MEMBER_OF 边"]
        P14["Phase 14: processes<br/>Process 节点 + STEP_IN_PROCESS/ENTRY_POINT_OF 边"]
        P1 --> KG["共享 KnowledgeGraph<br/>pipeline.ts 第 242 行<br/>createKnowledgeGraph()"]
        P6 --> KG
        P9 --> KG
        P10 --> KG
        P11 --> KG
        P12 --> KG
        P13 --> KG
        P14 --> KG
    end

    KG --> |"14 Phase 全部完成后<br/>run-analyze.ts 单次调用"| LOAD["loadGraphToLbug<br/>graph → DB 一次性批量写入"]

    subgraph WritePipeline["写入流水线 (顺序执行)"]
        LOAD --> CSV1["Step 1: streamAllCSVsToDisk<br/>单遍遍历 KnowledgeGraph<br/>按 node.label 路由到 32 个 CSV writer<br/>FileContentCache LRU 懒读源码"]
        CSV1 --> CSV2["Step 2: 合并 rel.csv 输出<br/>所有边 → 单一 rel.csv 文件"]
        CSV2 --> SPLIT["Step 3: splitRelCsvByLabelPair<br/>按 FROM→TO 标签对拆分<br/>rel_Function_Function.csv<br/>rel_Function_Method.csv 等"]
        SPLIT --> COPY_N["Step 4: 批量 COPY 节点<br/>逐表顺序执行<br/>LadybugDB 一次只允许一个写事务"]
        COPY_N --> COPY_R["Step 5: 批量 COPY 关系<br/>逐 FROM-TO 对顺序执行<br/>COPY CodeRelation FROM csv<br/>from/to 参数指定标签对"]
        COPY_R --> FTS["Step 6: CREATE_FTS_INDEX<br/>Function.name / Class.name 等"]
        FTS --> EMB["Step 7: 批量 INSERT CodeEmbedding<br/>节点数门控 / 维度守卫<br/>cachedEmbeddings 先恢复"]
        EMB --> VI["Step 8: CREATE_VECTOR_INDEX<br/>HNSW cosine"]
    end

    subgraph IncrementalPath["增量写入路径 (同一 loadGraphToLbug)"]
        HASH["file-hash.ts: SHA-256 diff<br/>→ changed / added / deleted"]
        BFS["importer BFS: MAX_DEPTH=4<br/>queryImporters 读 DB IMPORTS 边<br/>+ shadow-candidates 种子"]
        EFFECTIVE["computeEffectiveWriteSet<br/>1-hop 边界跨越扩展"]
        DEL["deleteNodesForFile: DETACH DELETE<br/>逐表 WHERE filePath<br/>级联删 CodeEmbedding"]
        DEL2["deleteAllCommunitiesAndProcesses<br/>Community/Process 全删重写"]
        SUB["extractChangedSubgraph<br/>只取 writable-set 节点 + 边"]
        HASH --> BFS --> EFFECTIVE --> DEL --> DEL2 --> SUB --> LOAD
    end
```

```mermaid
graph TB
    KG["KnowledgeGraph<br/>ingestion 产出"] --> CSV_GEN["csv-generator.ts<br/>streamAllCSVsToDisk"]

    subgraph CSV_Phase["Phase 1: 流式 CSV 生成"]
        CSV_GEN --> NW["单遍遍历节点<br/>按 label 路由到 32 个 CSV writer"]
        CSV_GEN --> RW["关系 CSV 合并输出<br/>rel.csv 单文件"]
        CSV_GEN --> CC["FileContentCache<br/>LRU 3000 文件<br/>懒读磁盘源码内容"]
        CSV_GEN --> BUF["BufferedCSVWriter<br/>FLUSH_EVERY=500 行<br/>RFC 4180 转义"]
    end

    CSV_Phase --> SPLIT["splitRelCsvByLabelPair<br/>按 FROM-TO 标签对<br/>拆分 rel.csv 为 per-pair CSV"]

    subgraph COPY_Phase["Phase 2: 批量 COPY 导入"]
        SPLIT --> NC["节点 COPY<br/>逐表顺序<br/>COPY Table FROM csv<br/>IGNORE_ERRORS 兜底"]
        SPLIT --> RC["关系 COPY<br/>逐 FROM-TO 对<br/>COPY CodeRelation FROM csv<br/>from/to 标签参数"]
    end

    COPY_Phase --> FTS["Phase 3: FTS 索引<br/>CREATE_FTS_INDEX<br/>File.name / Function.name 等"]
    COPY_Phase --> EMB["Phase 4: Embeddings<br/>skipForCap 节点数门控<br/>snowflake-arctic-embed-xs<br/>批量 INSERT CodeEmbedding"]
    COPY_Phase --> VEC["HNSW 向量索引<br/>CREATE_VECTOR_INDEX<br/>cosine 相似度"]

    subgraph Incremental["增量写入路径"]
        HASH["file-hash.ts<br/>SHA-256 逐文件<br/>diffFileHashes → changed/added/deleted"]
        BFS["importer BFS<br/>MAX_DEPTH=4<br/>queryImporters 读 DB IMPORTS 边"]
        DEL["deleteNodesForFile<br/>DETACH DELETE per table<br/>级联删 CodeEmbedding"]
        SUB["extractChangedSubgraph<br/>只提取 writable-set 节点+边<br/>computeEffectiveWriteSet 1-hop 扩展"]
        HASH --> BFS --> DEL --> SUB
        SUB --> CSV_GEN
    end
```

## 23. 磁盘存储辅助系统

```mermaid
graph TB
    subgraph ParseCache["解析缓存 (parse-cache.ts)"]
        PC_KEY["缓存键: sha256(sorted filePath:contentHash)"]
        PC_VER["版本: SCHEMA_BUMP + pkg.version"]
        PC_HIT["热命中: Worker 不运行<br/>durable store byte-copy 恢复"]
        PC_MISS["冷缺失: Worker 解析<br/>写入 parse-cache + durable"]
    end

    subgraph ParsedFileStore["ParsedFile 磁盘存储"]
        PFS_RUN["运行期: parsedfile-store/<br/>shardId.json per chunk<br/>scope-resolution 消费后清除"]
        PFS_DUR["持久化: parsedfile-cache/<br/>内容寻址 chunk-hash key<br/>与 parse-cache 生命周期同步<br/>pruned by usedKeys"]
        PFS_INT["字符串内联<br/>makeInterningReviver<br/>def-object 内联<br/>Linux kernel 15GB→7.6GB"]
        PFS_RUN --> PFS_INT
        PFS_DUR --> PFS_INT
    end

    subgraph ScopeIndexStore["Scope 磁盘索引 (opt-in)"]
        SIS_ENV["GITNEXUS_DISK_SCOPE_INDEX=1 启用"]
        SIS_SHARD["per-file JSON 分片<br/>s1.json / s2.json / ..."]
        SIS_SKEL["内存骨架: ScopeSkeletonEntry<br/>shard / parent / childIds<br/>不含 bindings/typeBindings"]
        SIS_LRU["DiskBackedScopeTree<br/>LRU 缓存解码分片<br/>getScope 按需加载<br/>getChildren 骨架直接返回"]
        SIS_ENV --> SIS_SHARD --> SIS_SKEL --> SIS_LRU
    end

    subgraph BranchIndex["多分支索引"]
        BI_SLUG["branchSlug<br/>sanitizeRepoName + sha256 prefix"]
        BI_FLAT["首个分支 → 扁平 .gitnexus/<br/>meta.json 标记 branch 字段"]
        BI_SUB["后续分支 → branches/slug/<br/>独立 lbug/ + meta.json"]
        BI_SHARED["共享: parse-cache / parsedfile-cache<br/>cacheKeys 联合保留"]
        BI_SLUG --> BI_FLAT
        BI_SLUG --> BI_SUB
        BI_FLAT --> BI_SHARED
        BI_SUB --> BI_SHARED
    end
```

## 24. RepoMeta 元数据模型 + 增量安全机制

```mermaid
graph TB
    subgraph Meta["RepoMeta (meta.json)"]
        M_CORE["repoPath / lastCommit / indexedAt<br/>remoteUrl (sibling-clone 指纹)"]
        M_STATS["stats: files / nodes / edges<br/>communities / processes / embeddings"]
        M_SCHEMA["schemaVersion: 增量不变量版本<br/>不匹配 → 强制全量"]
        M_HASH["fileHashes: Record path→SHA256<br/>增量 diff 基准"]
        M_DIRTY["incrementalInProgress<br/>startedAt / toWriteCount<br/>写前标记 / 成功清除"]
        M_PDG["pdg: maxFunctionLines<br/>maxEdgesPerFunction<br/>maxReachingDefEdgesPerFunction<br/>maxTaintFindings / maxTaintHops<br/>taintModelVersion<br/>模式不匹配 → 强制全量"]
        M_BRANCH["branch: 当前分支名<br/>cacheKeys: 存活 chunk keys"]
    end

    subgraph Safety["增量安全机制"]
        S_DIRTY["崩溃恢复: incrementalInProgress 脏标记<br/>写前 set → 成功覆盖清除<br/>残留 → 下次运行强制全量"]
        S_ATOMIC["原子写入: tmp 文件 + rename<br/>POSIX/Windows 均原子"]
        S_PDG_MODE["PDG 模式守卫: pdg 字段比较<br/>on→off / off→on / cap 变化<br/>强制全量重建"]
        S_SCHEMA_VER["Schema 版本守卫: schemaVersion<br/>INCREMENTAL_SCHEMA_VERSION=1<br/>不匹配 → 强制全量"]
    end

    Meta --> Safety
```

## 25. 14 Phase 细化流程与输出数据模型

```mermaid
graph TB
    subgraph P1["Phase 1: scan"]
        S_WALK["walkRepositoryPaths<br/>.gitignore + ignorePatterns<br/>过滤隐藏/二进制/node_modules"] --> S_OUT["ScannedFile[]<br/>path / language / size"]
    end

    subgraph P2["Phase 2: structure"]
        ST_IN["ScannedFile[]"] --> ST_PROC["processStructure<br/>按路径段创建层次"]
        ST_PROC --> ST_N["节点: File, Folder"]
        ST_PROC --> ST_E["边: CONTAINS<br/>Folder→File/Folder"]
    end

    subgraph P3["Phase 3: markdown"]
        MD_IN["ScannedFile[]<br/>(仅.md)"] --> MD_PROC["processMarkdown<br/>AST解析→heading/code块"]
        MD_PROC --> MD_N["节点: Section"]
        MD_PROC --> MD_E1["边: CONTAINS<br/>File→Section"]
        MD_PROC --> MD_E2["边: IMPORTS<br/>Section→File(跨链)"]
    end

    subgraph P4["Phase 4: cobol (⚠ 9语言下不生效)"]
        CB_IN["ScannedFile[]<br/>(.cbl/.cob/.cpy/.jcl等)"] --> CB_PRE["cobol-preprocessor<br/>PROGRAM-ID→Module<br/>Section→Namespace<br/>Paragraph→Function<br/>DataItem→Property"]
        CB_IN --> CB_JCL["jcl-processor<br/>Job/Step/Dataset→CodeElement<br/>PROC→Module"]
        CB_PRE --> CB_N1["节点: Module, Namespace<br/>Function, Property"]
        CB_JCL --> CB_N2["节点: CodeElement<br/>(Job/Step/Dataset)<br/>Module(PROC)"]
        CB_PRE --> CB_E1["边: CONTAINS, CALLS<br/>ACCESSES, IMPORTS"]
        CB_JCL --> CB_E2["边: CONTAINS, CALLS<br/>IMPORTS"]
        CB_NOTE["⚠ COBOL为experimental语言<br/>9种生效语言下无.cob/.jcl输入<br/>P4不产出任何节点/边"]
    end

    subgraph P5["Phase 5: parse"]
        PA_IN["ScannedFile[]"] --> PA_POOL["Worker Pool<br/>runChunkedParseAndResolve"]
        PA_POOL --> PA_MERGE["mergeChunkResults<br/>nodes/relationships/symbols"]
        PA_MERGE --> PA_N["19种符号节点(9语言生效)<br/>Function/Method/Constructor<br/>Class/Interface/Enum/Struct<br/>Record/Trait/TypeAlias<br/>Const/Variable/Property/Field<br/>Namespace/Macro<br/>Typedef/Union/Template"]
        PA_MERGE --> PA_E1["边: DEFINES<br/>File→顶层符号"]
        PA_MERGE --> PA_E2["边: HAS_METHOD<br/>Class→Method"]
        PA_MERGE --> PA_E3["边: HAS_PROPERTY<br/>Class→Property"]
    end

    P1 --> P2 --> P3 --> P5
    P2 --> P4 --> P5

    subgraph P6["Phase 6: routes"]
        RT_IN["graph + symbolTable"] --> RT_PROC["RouteExtractor<br/>framework detectors"]
        RT_PROC --> RT_N["节点: Route"]
        RT_PROC --> RT_E1["边: HANDLES_ROUTE<br/>Handler→Route"]
        RT_PROC --> RT_E2["边: FETCHES<br/>Caller→Route"]
    end

    subgraph P7["Phase 7: tools"]
        TL_IN["graph + symbolTable"] --> TL_PROC["ToolExtractor<br/>MCP/CLI tool defs"]
        TL_PROC --> TL_N["节点: Tool"]
        TL_PROC --> TL_E["边: HANDLES_TOOL<br/>Handler→Tool"]
    end

    subgraph P8["Phase 8: orm"]
        OR_IN["graph + symbolTable"] --> OR_PROC["ORM extractor<br/>Prisma/Supabase"]
        OR_PROC --> OR_N["节点: CodeElement<br/>(ORM model)"]
        OR_PROC --> OR_E["边: QUERIES<br/>File→CodeElement"]
    end

    P5 --> P6 & P7 & P8

    subgraph P9["Phase 9: crossFile"]
        CF_IN["graph"] --> CF_PROC["dispose<br/>BindingAccumulator"]
        CF_PROC --> CF_OUT["不写入graph<br/>(legacy DAG已移除)"]
    end

    P6 & P7 & P8 --> P9

    subgraph P10["Phase 10: scopeResolution"]
        SR_EX["1-extract<br/>获取ParsedFile"] --> SR_FI["2-finalize<br/>构建索引+预发射继承边"]
        SR_FI --> SR_RS["3-resolve<br/>resolveReferenceSites"]
        SR_RS --> SR_EM["4-emit<br/>发射所有跨引用边"]
        SR_FI --> SR_E1["边: EXTENDS<br/>IMPLEMENTS"]
        SR_EM --> SR_E2["边: CALLS<br/>ACCESSES<br/>IMPORTS<br/>USES"]
        SR_EM --> SR_E3["边(opt-in PDG):<br/>CFG<br/>REACHING_DEF<br/>TAINTED<br/>SANITIZES"]
        SR_EM --> SR_NB["节点(opt-in PDG):<br/>BasicBlock"]
    end

    P9 --> P10

    subgraph P11["Phase 11: pruneLocalSymbols"]
        PL_IN["graph"] --> PL_PROC["pruneLocalSymbols<br/>删除block-local无语义出边节点"]
        PL_PROC --> PL_DEL["删除: Const/Variable/Static<br/>scope=block 且无语义出边"]
        PL_PROC --> PL_KEEP["保留: 有CALLS/ACCESSES等<br/>语义出边的block-local节点"]
    end

    subgraph P12["Phase 12: mro"]
        MRO_IN["graph"] --> MRO_PROC["computeMRO<br/>C3线性化+接口实现匹配"]
        MRO_PROC --> MRO_E1["边: METHOD_OVERRIDES<br/>Class→Method"]
        MRO_PROC --> MRO_E2["边: METHOD_IMPLEMENTS<br/>ConcreteMethod→InterfaceMethod"]
    end

    subgraph P13["Phase 13: communities"]
        CM_IN["graph"] --> CM_PROC["Leiden算法<br/>社区检测"]
        CM_PROC --> CM_N["节点: Community<br/>name/heuristicLabel<br/>cohesion/symbolCount"]
        CM_PROC --> CM_E["边: MEMBER_OF<br/>Symbol→Community"]
    end

    subgraph P14["Phase 14: processes"]
        PR_IN["graph + routes + tools"] --> PR_PROC["追踪检测<br/>BFS/DFS路径追踪"]
        PR_PROC --> PR_N["节点: Process<br/>name/heuristicLabel/processType<br/>stepCount/communities<br/>entryPointId/terminalId"]
        PR_PROC --> PR_E1["边: STEP_IN_PROCESS<br/>Symbol→Process(step序号)"]
        PR_PROC --> PR_E2["边: ENTRY_POINT_OF<br/>Route/Tool→Process"]
    end

    P10 --> P11 --> P12 --> P13 --> P14
```

### Phase 输出数据模型汇总表

| Phase | 名称 | 产出节点标签 | 产出边类型 | 关键属性 |
|-------|------|-------------|-----------|---------|
| 1 | scan | _(无，输出ScannedFile[])_ | _(无)_ | path, language, size |
| 2 | structure | File, Folder | CONTAINS | filePath, startLine, endLine |
| 3 | markdown | Section | CONTAINS, IMPORTS | name, headingLevel, slug |
| 4 | cobol | ⚠ 9语言下不生效(experimental) | ⚠ 9语言下不生效 | COBOL/JCL文件不存在于9语言项目 |
| 5 | parse | Function, Method, Constructor, Class, Interface, Enum, Struct, Record, Trait, TypeAlias, Const, Variable, Property, Field, Namespace, Macro, Typedef, Union, Template (19种生效) | DEFINES, HAS_METHOD, HAS_PROPERTY | ⚠ Impl/Delegate/Annotation/Module/Static仅降级语言产出 |
| 6 | routes | Route | HANDLES_ROUTE, FETCHES | name, filePath, httpMethod, path |
| 7 | tools | Tool | HANDLES_TOOL | name, description |
| 8 | orm | CodeElement(ORM model) | QUERIES | name, filePath, modelName |
| 9 | crossFile | _(无)_ | _(无，仅dispose)_ | — |
| 10 | scopeResolution | BasicBlock(PDG opt-in) | EXTENDS, IMPLEMENTS, CALLS, ACCESSES, IMPORTS, USES, CFG, REACHING_DEF, TAINTED, SANITIZES(PDG opt-in) | confidence, reason, step |
| 11 | pruneLocalSymbols | _(删除节点)_ | _(删除关联边)_ | 删除scope=block且无语义出边的Const/Variable/Static |
| 12 | mro | _(无)_ | METHOD_OVERRIDES, METHOD_IMPLEMENTS | confidence, reason |
| 13 | communities | Community | MEMBER_OF | name, heuristicLabel, cohesion, symbolCount |
| 14 | processes | Process | STEP_IN_PROCESS, ENTRY_POINT_OF | name, heuristicLabel, processType, stepCount, communities, entryPointId, terminalId |

**统计**: 19种生效节点标签(9语言) / 5种仅降级语言(Impl,Delegate,Annotation,Module,Static) + 12种(PDG) = 32种DB表不变 | 17种活跃边类型 + 4种(PDG) = 21种REL_TYPES

## 26. scopeResolution 内部 4 子阶段细化流程

```mermaid
graph TB
    subgraph SR["Phase 10: scopeResolution"]
        direction TB
        subgraph EX["Sub-phase 1: extract"]
            EX1["复用Worker产出的ParsedFile<br/>或主线程re-parse"] --> EX2["每文件: AST + 符号表<br/>+ 引用位点 + 作用域树"]
        end

        subgraph FI["Sub-phase 2: finalize"]
            FI1["构建ScopeResolutionIndexes<br/>fileIndex + symbolIndex + refIndex"]
            FI1 --> FI2["preEmitInheritanceEdges<br/>→ EXTENDS边"]
            FI1 --> FI3["emitDetectedInterfaceImplementations<br/>→ IMPLEMENTS边"]
            FI2 & FI3 --> FI4["buildMro<br/>C3线性化预留"]
            FI4 --> FI5["buildWorkspaceResolutionIndex<br/>全局符号+导入合并"]
            FI5 --> FI6["propagateImportedReturnTypes<br/>类型传播优化"]
        end

        subgraph RS["Sub-phase 3: resolve"]
            RS1["resolveReferenceSites<br/>Registry.lookup逐引用解析"] --> RS2["ReferenceIndex<br/>resolvedRefSite[] + unresolved[]"]
        end

        subgraph EM["Sub-phase 4: emit"]
            EM1["emitReceiverBoundCalls<br/>obj.method() → CALLS"]
            EM1 --> EM2["emitFreeCallFallback<br/>裸函数调用 → CALLS"]
            EM2 --> EM3["emitReferencesViaLookup<br/>read/write→ACCESSES<br/>inherits→EXTENDS<br/>type-reference→USES<br/>macro→USES"]
            EM3 --> EM4["emitImportEdges<br/>→ IMPORTS边"]
            EM4 --> EM5["emitPostResolutionEdges<br/>语言特定后处理"]
            EM5 --> EM6["opt-in: emitFileCfgs<br/>→ BasicBlock节点 + CFG边"]
            EM6 --> EM7["opt-in: emitFileReachingDefs<br/>→ REACHING_DEF边"]
            EM7 --> EM8["opt-in: emitFileTaint<br/>→ TAINTED + SANITIZES边"]
        end

        EX --> FI --> RS --> EM
    end
```

### scopeResolution 边发射顺序与映射规则

| 发射函数 | 引用类型 → 边类型 | 方向 | 说明 |
|---------|------------------|------|------|
| emitReceiverBoundCalls | call(receiver-bound) → CALLS | Caller→Callee | obj.method() 有明确接收者 |
| emitFreeCallFallback | call(free) → CALLS | Caller→Callee | 裸函数调用，无接收者 |
| emitReferencesViaLookup | read/write → ACCESSES | Reader→Target | 属性读写 |
| emitReferencesViaLookup | inherits → EXTENDS | Child→Parent | 类继承 |
| emitReferencesViaLookup | type-reference → USES | User→Used | 类型引用 |
| emitReferencesViaLookup | macro → USES | User→Macro | 宏展开引用 |
| emitImportEdges | import → IMPORTS | Importer→Importee | 跨文件导入 |
| emitPostResolutionEdges | 语言特定 | 因语言而异 | ⚠ Swift降级 / ✅ Rust等后处理 |
| emitFileCfgs(PDG) | — → CFG | Block→Block | 控制流图 |
| emitFileReachingDefs(PDG) | — → REACHING_DEF | Def→Use | 到达定义 |
| emitFileTaint(PDG) | — → TAINTED/SANITIZES | Source→Sink | 污点传播 |

## 27. 节点标签 → 产出 Phase 溯源矩阵

```mermaid
graph LR
    subgraph Structural["结构层节点"]
        FILE["File"] -.->|P2| P2L["structure"]
        FOLDER["Folder"] -.->|P2| P2L
        SECTION["Section"] -.->|P3| P3L["markdown"]
        CODEELEMENT1["CodeElement<br/>(JCL/ORM)"] -.->|P4+P8| P4_8L["cobol + orm<br/>⚠ P4 9语言下不生效"]
    end

    subgraph SharedLabels["共享标签节点(⚠ P4 9语言下不生效，仅P5产出)"]
        SHARED_NOTE["⚠ COBOL为experimental<br/>9语言下P4不运行<br/>以下标签仅由P5 tree-sitter产出"]
        MOD1["Module"] -.->|P5 only| SHARED_P["parse"]
        NS1["Namespace"] -.->|P5| SHARED_P
        FUNC1["Function"] -.->|P5| SHARED_P
        PROP1["Property"] -.->|P5| SHARED_P
    end

    subgraph Symbols["P5 parse 独占节点标签 (9语言生效)"]
        METH["Method"]
        CTOR["Constructor"]
        CLS["Class"]
        IFACE["Interface"]
        ENUM["Enum"]
        STRCT["Struct"]
        RCRD["Record"]
        TRAIT["Trait"]
        TALIAS["TypeAlias"]
        CONST["Const"]
        VAR["Variable"]
        MACRO["Macro"]
        TYPEDEF["Typedef"]
        UNION["Union"]
        TMPL["Template"]
        FIELD["Field"]
    end

    subgraph SymbolsDegraded["⚠ 仅降级语言产出的标签"]
        IMPL["Impl → Kotlin"]
        DELEG["Delegate → 无源产出"]
        ANNOT["Annotation → 无源产出"]
        STATIC["Static → method属性"]
    end

    METH & CTOR & CLS & IFACE & ENUM & STRCT & RCRD & TRAIT & TALIAS & CONST & VAR & MACRO & TYPEDEF & UNION & TMPL & FIELD -.->|P5| P5L["parse"]

    subgraph Domain["领域层节点"]
        ROUTE["Route"] -.->|P6| P6L["routes"]
        TOOL["Tool"] -.->|P7| P7L["tools"]
    end

    subgraph Analysis["分析层节点"]
        BB["BasicBlock<br/>(PDG opt-in)"] -.->|P10| P10L["scopeResolution"]
        COMM["Community"] -.->|P13| P13L["communities"]
        PROC["Process"] -.->|P14| P14L["processes"]
    end
```

### 节点标签 → Phase 溯源表

| 节点标签 | 产出Phase | LadybugDB表名 | 可被Phase删除 |
|---------|----------|--------------|-------------|
| File | P2 structure | File | — |
| Folder | P2 structure | Folder | — |
| Section | P3 markdown | Section | — |
| Module | P5 parse (⚠ P4 cobol 9语言下不生效) | Module | — |
| Namespace | P5 parse | Namespace | — |
| Function | P5 parse | Function | — |
| Property | P5 parse | Property | — |
| CodeElement | P8 orm (⚠ P4 cobol 9语言下不生效) | CodeElement | — |
| Method | P5 parse | Method | — |
| Constructor | P5 parse | Constructor | — |
| Class | P5 parse | Class | — |
| Interface | P5 parse | Interface | — |
| Enum | P5 parse | Enum | — |
| Struct | P5 parse | Struct | — |
| Record | P5 parse | Record | — |
| Trait | P5 parse | Trait | — |
| Impl | P5 parse (⚠ 仅Kotlin降级) | Impl | — |
| TypeAlias | P5 parse | TypeAlias | — |
| Const | P5 parse | Const | P11(无语义出边) |
| Static | P5 parse (⚠ 仅降级语言产出) | Static | P11(无语义出边) |
| Variable | P5 parse | Variable | P11(无语义出边) |
| Macro | P5 parse | Macro | — |
| Typedef | P5 parse | Typedef | — |
| Union | P5 parse | Union | — |
| Template | P5 parse | Template | — |
| Delegate | P5 parse (⚠ 仅降级语言产出) | Delegate | — |
| Annotation | P5 parse (⚠ 仅降级语言产出) | Annotation | — |
| Route | P6 routes | Route | — |
| Tool | P7 tools | Tool | — |
| BasicBlock | P10 scopeResolution(PDG) | BasicBlock | — |
| Community | P13 communities | Community | — |
| Process | P14 processes | Process | — |

## 28. 边类型 → 产出 Phase 溯源矩阵

```mermaid
graph LR
    subgraph Structure_E["结构层边"]
        CONTAINS["CONTAINS"] -.->|P2,P3<br/>⚠P4 9语言下不生效| P_E1["structure<br/>markdown"]
        DEFINES["DEFINES"] -.->|P5| P_E2["parse"]
        HAS_METHOD["HAS_METHOD"] -.->|P5| P_E2
        HAS_PROPERTY["HAS_PROPERTY"] -.->|P5| P_E2
    end

    subgraph Domain_E["领域层边"]
        HANDLES_ROUTE["HANDLES_ROUTE"] -.->|P6| P_E3["routes"]
        FETCHES["FETCHES"] -.->|P6| P_E3
        HANDLES_TOOL["HANDLES_TOOL"] -.->|P7| P_E4["tools"]
        QUERIES["QUERIES"] -.->|P8| P_E5["orm"]
    end

    subgraph Reference_E["引用层边(P10 scopeResolution)"]
        CALLS["CALLS"]
        ACCESSES["ACCESSES"]
        EXTENDS["EXTENDS"]
        IMPLEMENTS["IMPLEMENTS"]
        USES["USES"]
        IMPORTS["IMPORTS"]
    end

    CALLS & ACCESSES & EXTENDS & IMPLEMENTS & USES & IMPORTS -.->|P10| P_E6["scopeResolution"]

    subgraph MRO_E["MRO层边"]
        METHOD_OVERRIDES["METHOD_OVERRIDES"] -.->|P12| P_E7["mro"]
        METHOD_IMPLEMENTS["METHOD_IMPLEMENTS"] -.->|P12| P_E7
    end

    subgraph Analysis_E["分析层边"]
        MEMBER_OF["MEMBER_OF"] -.->|P13| P_E8["communities"]
        STEP_IN_PROCESS["STEP_IN_PROCESS"] -.->|P14| P_E9["processes"]
        ENTRY_POINT_OF["ENTRY_POINT_OF"] -.->|P14| P_E9
    end

    subgraph PDG_E["PDG层边(opt-in)"]
        CFG["CFG"]
        REACHING_DEF["REACHING_DEF"]
        TAINTED["TAINTED"]
        SANITIZES["SANITIZES"]
    end

    CFG & REACHING_DEF & TAINTED & SANITIZES -.->|P10| P_E6
```

### 边类型 → Phase + FROM→TO 溯源表

| 边类型 | 产出Phase | FROM标签 | TO标签 | 语义 |
|-------|----------|---------|--------|------|
| CONTAINS | P2/P3/P4 | Folder/File/CodeElement | File/Folder/Section/CodeElement | 层次包含 |
| DEFINES | P5 | File | 符号节点 | 文件定义符号 |
| HAS_METHOD | P5 | Class | Method | 类拥有方法 |
| HAS_PROPERTY | P5 | Class | Property | 类拥有属性 |
| HANDLES_ROUTE | P6 | Function/Method | Route | 函数处理路由 |
| FETCHES | P6 | Function/Method | Route | 函数请求路由 |
| HANDLES_TOOL | P7 | Function/Method | Tool | 函数处理工具 |
| QUERIES | P8 | File | CodeElement(ORM) | 文件查询ORM模型 |
| EXTENDS | P10 | Class | Class | 类继承 |
| IMPLEMENTS | P10 | Class | Interface | 类实现接口 |
| CALLS | P10 (⚠ P4 cobol 9语言下不生效) | Method/Function | Method/Function | 函数调用 |
| ACCESSES | P10 (⚠ P4 cobol 9语言下不生效) | Method/Function | Property/Variable | 属性读写 |
| IMPORTS | P3/P10 (⚠ P4 cobol 9语言下不生效) | File/Section | File/Module | 跨文件导入 |
| USES | P10 | Method/Function | TypeAlias/Macro/Typedef | 类型/宏引用 |
| METHOD_OVERRIDES | P12 | Class | Method | 方法覆盖 |
| METHOD_IMPLEMENTS | P12 | Method(Concrete) | Method(Interface) | 接口方法实现 |
| MEMBER_OF | P13 | Symbol | Community | 符号属于社区 |
| STEP_IN_PROCESS | P14 | Symbol | Process | 符号是流程步骤 |
| ENTRY_POINT_OF | P14 | Route/Tool | Process | 路由/工具是流程入口 |
| CFG | P10(PDG) | BasicBlock | BasicBlock | 控制流 |
| REACHING_DEF | P10(PDG) | Node | Node | 到达定义 |
| TAINTED | P10(PDG) | Node | Node | 污点传播 |
| SANITIZES | P10(PDG) | Node | Node | 污点净化 |

