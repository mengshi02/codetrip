# Codetrip Ingestion 100% Port Plan

> Source: `/Users/mengshi3/ts/GitNexus/gitnexus/src/core/ingestion` (474 TS files total)
> Target: `internal/ingestion/` (81 Go files existing)
> Languages: C, C++, C#, Go, Java, JavaScript, Python, Rust, TypeScript (9 core)
> Scope: 346 TS files relevant to 9 core languages → ~265 new Go files needed

---

## Architecture Overview

```
Pipeline DAG:
  scan → structure → [markdown, cobol] → parse → [routes, tools, orm]
    → crossFile → scopeResolution → pruneLocalSymbols
    → mro → communities → processes

Data Flow:
  Filesystem → ParsedFile[] → ScopeResolutionIndexes → ReferenceIndex → KnowledgeGraph

Two Provider Contracts:
  LanguageProvider (parse-side, ~40 fields): languages/<lang>.ts
  ScopeResolver   (emit-side,  8+ fields):  languages/<lang>/scope-resolver.ts
```

---

## Phase 0: Type Foundation (gitnexus-shared → shared/)

Port all shared types that ingestion depends on. Without these, nothing compiles.

### 0A. Scope Resolution Core Types
| # | Source (gitnexus-shared/src) | Target Go File | Status |
|---|---|---|---|
| 1 | scope-resolution/types.ts | shared/types.go | DONE |
| 2 | scope-resolution/scope-id.ts | shared/scope_id.go | DONE |
| 3 | scope-resolution/symbol-definition.ts | shared/types.go | DONE (embedded in shared/types.go) |
| 4 | scope-resolution/parsed-file.ts | shared/parsed_file.go | DONE |
| 5 | scope-resolution/reference-site.ts | shared/types.go | DONE (embedded in shared/types.go) |
| 6 | scope-resolution/def-index.ts | shared/def_index.go | DONE |
| 7 | scope-resolution/scope-tree.ts | shared/scope_tree.go | DONE |
| 8 | scope-resolution/language-classification.ts | shared/language_classification.go | DONE |
| 9 | scope-resolution/origin-priority.ts | shared/origin_priority.go | DONE |
| 10 | scope-resolution/evidence-weights.ts | shared/evidence_weights.go | DONE |
| 11 | scope-resolution/position-index.ts | shared/position_index.go | DONE |

### 0B. Registry Types (used by resolve-references)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 12 | scope-resolution/registries/context.ts | shared/registries/context.go | DONE |
| 13 | scope-resolution/registries/evidence.ts | shared/registries/context.go | DONE (embedded in context.go) |
| 14 | scope-resolution/registries/lookup-core.ts | shared/registries/lookup_core.go | DONE |
| 15 | scope-resolution/registries/lookup-qualified.ts | shared/registries/lookup_qualified.go | DONE |
| 16 | scope-resolution/registries/tie-breaks.ts | shared/registries/tie_breaks.go | DONE |
| 17 | scope-resolution/registries/class-registry.ts | shared/registries/class_registry.go | DONE |
| 18 | scope-resolution/registries/field-registry.ts | shared/registries/field_registry.go | DONE |
| 19 | scope-resolution/registries/method-registry.ts | shared/registries/method_registry.go | DONE |
| 20 | scope-resolution/registries/macro-registry.ts | shared/registries/macro_registry.go | DONE |

### 0C. Finalize & Index Types
| # | Source | Target Go File | Status |
|---|---|---|---|
| 21 | scope-resolution/finalize-algorithm.ts | shared/finalize_algorithm.go | DONE (Finalize+hooks+Tarjan SCC, ~500 lines) |
| 22 | scope-resolution/method-dispatch-index.ts | shared/method_dispatch_index.go | DONE |
| 23 | scope-resolution/module-scope-index.ts | shared/module_scope_index.go | DONE |
| 24 | scope-resolution/qualified-name-index.ts | shared/qualified_name_index.go | DONE |
| 25 | scope-resolution/resolve-type-ref.ts | shared/resolve_type_ref.go | DONE |

### 0D. Graph & Language Types
| # | Source | Target Go File | Status |
|---|---|---|---|
| 26 | graph/types.ts | shared/graph_types.go | DONE (Evidence, KnowledgeGraph interface with graph.Node/Edge; unified — core.KnowledgeGraph removed) |
| 27 | languages.ts | shared/types.go + core/supported_language.go | DONE (split across shared + core with type alias) |
| 28 | mro-strategy.ts | core/supported_language.go | DONE (MroStrategy in core) |
| 29 | language-detection.ts | shared/language_detection.go | DONE (9 core languages, ext→lang init map, syntax highlight) |
| 30 | pipeline.ts | pipeline/options.go | DONE (moved to pipeline package) |

### 0E. Model Layer (additional, not in original plan)
| # | Source | Target Go File | Status |
|---|---|---|---|
| — | model/type-registry.ts | model/type_registry.go | DONE (TypeRegistry + MutableTypeRegistry + typeRegistryImpl) |
| — | model/scope-resolution-indexes.ts | model/scope_resolution_indexes.go | DONE (5 fields updated to pointer types) |
| — | typeextractors/shared.ts | typeextractors/shared.go | DONE (stub, ExtractSimpleTypeName) |
| — | type_extractors/shared.ts | type_extractors/extract_simple_type_name.go | DONE (partial implementation) |

---

## Phase 1: Pipeline Skeleton (pipeline/)

The pipeline runner + all phase definitions. This is the spine of the system.

### 1A. Pipeline Infrastructure
| # | Source | Target Go File | Status |
|---|---|---|---|
| 31 | pipeline-phases/types.ts | pipeline/types.go | DONE (PipelinePhase interface + PipelineContext(shared.KnowledgeGraph) + PhaseResult + GetPhaseOutput/GetPhaseOutputTyped) |
| 32 | pipeline-phases/runner.ts | pipeline/runner.go | DONE (Kahn's topologicalSort + RunPipeline sequential execution + findCyclePath DFS) |
| 33 | pipeline-phases/registry.ts | pipeline/registry.go | DONE (PhaseRegistry + Register fluent chain + Build with EnabledWhen predicates) |
| 34 | pipeline-phases/index.ts | pipeline/pipeline.go | DONE (merged — barrel exports not needed in Go) |
| 35 | pipeline.ts | pipeline/pipeline.go | DONE (BuildPhaseList + RunPipelineFromRepo(shared.KnowledgeGraph) + PipelineResult) |

### 1B. Pipeline Phases (in dependency order)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 36 | pipeline-phases/scan.ts | pipeline/pipeline.go (scanPhaseImpl) | STUB — execute returns empty ScanOutput |
| 37 | pipeline-phases/structure.ts | pipeline/pipeline.go (structurePhaseImpl) | STUB — execute returns passthrough StructureOutput |
| 38 | pipeline-phases/markdown.ts | pipeline/pipeline.go (markdownPhaseImpl) | STUB |
| 39 | pipeline-phases/parse.ts | pipeline/pipeline.go (parsePhaseImpl) | STUB — execute returns passthrough ParseOutput |
| 40 | pipeline-phases/parse-impl.ts | pipeline/parse_impl.go | DONE (skeleton: ScannedFile+BuildChunks+RunChunkedParse stubs) |
| 41 | pipeline-phases/routes.ts | pipeline/pipeline.go (routesPhaseImpl) | STUB |
| 42 | pipeline-phases/tools.ts | pipeline/pipeline.go (toolsPhaseImpl) | STUB |
| 43 | pipeline-phases/orm.ts | pipeline/pipeline.go (ormPhaseImpl) | STUB |
| 44 | pipeline-phases/cross-file.ts | pipeline/pipeline.go (crossFilePhaseImpl) | STUB |
| 45 | pipeline-phases/prune-local-symbols.ts | pipeline/pipeline.go (pruneLocalSymbolsPhaseImpl) | STUB |
| 46 | pipeline-phases/mro.ts | pipeline/pipeline.go (mroPhaseImpl) | STUB |
| 47 | pipeline-phases/communities.ts | pipeline/pipeline.go (communitiesPhaseImpl) | STUB |
| 48 | pipeline-phases/processes.ts | pipeline/pipeline.go (processesPhaseImpl) | STUB |

> **Note**: Cobol phase removed (codetrip targets 9 languages only — no COBOL support).
> Phase DAG: scan→structure→markdown→parse→[routes,tools,orm]→crossFile→scopeResolution→pruneLocalSymbols→mro→communities→processes
> **KnowledgeGraph unified**: core.KnowledgeGraph removed — all code now uses shared.KnowledgeGraph. ProcessStructure emits *graph.Node/*graph.Edge.

---

## Phase 2: Core Infrastructure Files

Standalone TS files at ingestion root that provide key capabilities.

| # | Source | Target Go File | Status |
|---|---|---|---|
| 49 | binding-accumulator.ts | core/binding_accumulator.go | DONE (BindingEntry+BindingAccumulator+EnrichExportedTypeMap) |
| 50 | call-processor.ts | core/call_processor.go | DONE (skeleton: CallSite+ProcessCalls+AddCallEdges) |
| 51 | cluster-enricher.ts | core/cluster_enricher.go | DONE (skeleton: ClusterInfo+EnrichClusters+AssignClusterLabels) |
| 52 | community-processor.ts | core/community_processor.go | DONE (skeleton: CommunityProcessorResult+DetectCommunities) |
| 53 | csharp-namespace-gate.ts | core/csharp_namespace_gate.go | DONE (CSharpNamespaceInfo + gate + helper) |
| 54 | emit-references.ts | core/emit_references.go | DONE (skeleton: EmitReferencesOptions+EmitReferences) |
| 55 | entry-point-scoring.ts | core/entry_point_scoring.go | DONE (CalculateEntryPointScore + IsTestFile + IsUtilityFile) |
| 56 | filesystem-walker.ts | core/filesystem_walker.go | DONE (WalkRepositoryPaths + ReadFileContents + WalkRepository) |
| 57 | finalize-orchestrator.ts | core/finalize_orchestrator.go | DONE (skeleton hooks) |
| 58 | framework-detection.ts | core/framework_detection.go | DONE (path-based, 7 frameworks) |
| 59 | import-target-adapter.ts | core/import_target_adapter.go | DONE (ImportKind+CommonAdapter) |
| 60 | language-config.ts | core/language_config.go | DONE (tsconfig+go.mod+csproj) |
| 61 | language-provider.ts | core/language_provider.go | DONE (skeleton: LanguageProvider interface+Registry+ScopeInfo) |
| 62 | local-symbol-pruner.ts | core/local_symbol_pruner.go | DONE (skeleton: PruneLocalSymbolsOptions+PruneLocalSymbols) |
| 63 | markdown-processor.ts | core/markdown_processor.go | DONE (skeleton: MarkdownSection+MarkdownLink+ProcessMarkdownFile) |
| 64 | mro-processor.ts | core/mro_processor.go | DONE (skeleton: MROEntry+ComputeMRO+C3Merge) |
| 65 | parsing-processor.ts | core/parsing_processor.go | DONE (skeleton: ParsingProcessorResult+ImportEdge+RunParsingProcessor) |
| 66 | process-processor.ts | core/process_processor.go | DONE (skeleton: ProcessDefinition+DetectProcesses) |
| 67 | resolve-references.ts | core/resolve_references.go | DONE (skeleton: ResolvedReference+ResolveReferences) |
| 68 | scope-extractor-bridge.ts | core/scope_extractor_bridge.go | DONE (skeleton: ScopeExtractorBridgeResult+RunScopeExtractorBridge) |
| 69 | scope-extractor.ts | core/scope_extractor.go | DONE (skeleton: ScopeExtractionConfig+ExtractScopesFromTree+GenerateScopeID+BuildScopeTree) |
| 70 | tree-sitter-queries.ts | core/tree_sitter_queries.go | DONE (skeleton: TSQuerySet+QueryResult+LoadQueriesForLanguage) |
| 71 | ts-js-hoc-utils.ts | core/ts_js_hoc_utils.go | DONE (skeleton: HOCPattern+builtin patterns+DetectHOC+UnwrapHOC) |
| 72 | type-env.ts | core/type_env.go | DONE (TypeEntry+TypeEnv+Lookup+LookupExact+Merge) |

---

## Phase 3: Scope Resolution Engine (scope_resolution/)

The most complex module — the core analysis engine.

### 3A. Contract
| # | Source | Target Go File | Status |
|---|---|---|---|
| 73 | scope-resolution/contract/scope-resolver.ts | scope_resolution/scope_resolver.go | DONE (full ScopeResolver interface + 8 method signatures) |

### 3B. Pipeline
| # | Source | Target Go File | Status |
|---|---|---|---|
| 74 | scope-resolution/pipeline/phase.ts | scope_resolution/phase.go | DONE (RunScopeResolutionPhase + RunScopeResolutionPhaseInput + ScopeResolutionOutput, ~400 lines) |
| 75 | scope-resolution/pipeline/run.ts | scope_resolution/run.go | DONE (RunScopeResolution + FinalizeScopeModel + PreEmitInheritanceEdges + EmitDetectedInterfaceImplementations + BuildPopulatedMethodDispatch + ValidateOwnershipParity, ~600 lines) |
| 76 | scope-resolution/pipeline/registry.ts | scope_resolution/registry.go | DONE (ScopeResolverRegistry + Register/Get/All/Languages) |
| 77 | scope-resolution/pipeline/reconcile-ownership.ts | scope_resolution/reconcile_ownership.go | DONE (ReconcileOwnership + callableSignatureMatches, ~190 lines) |
| 78 | scope-resolution/pipeline/validate-bindings-immutability.ts | scope_resolution/validate_bindings.go | DONE (ValidateBindingsImmutability + snapshotBindings, ~230 lines) |

### 3C. Graph Bridge
| # | Source | Target Go File | Status |
|---|---|---|---|
| 79 | scope-resolution/graph-bridge/edges.ts | scope_resolution/edges.go | DONE (MapReferenceKindToEdgeType + TryEmitEdge + TryEmitEdgeWithExplicitTargetId + EmitInheritanceEdge + EmitCallEdge + EmitAccessEdge) |
| 80 | scope-resolution/graph-bridge/ids.ts | scope_resolution/ids.go | DONE (ResolveDefGraphID + ResolveCallerGraphID + SimpleQualifiedName + QualifiedKeyWithFile, ~350 lines) |
| 81 | scope-resolution/graph-bridge/imports-to-edges.ts | scope_resolution/imports_to_edges.go | DONE (ImportsToEdges + EmitImportEdge) |
| 82 | scope-resolution/graph-bridge/method-dispatch.ts | scope_resolution/method_dispatch.go | DONE (DispatchMethodCall + FindOwnedMember + MroFor, ~130 lines) |
| 83 | scope-resolution/graph-bridge/node-lookup.ts | scope_resolution/node_lookup.go | DONE (GraphNodeLookup + BuildGraphNodeLookup + LookupBySimple/Qualified/ID/FilePath) |
| 84 | scope-resolution/graph-bridge/references-to-edges.ts | scope_resolution/references_to_edges.go | DONE (EmitReferencesViaLookup + EmitPreInheritanceEdges) |

### 3D. Passes
| # | Source | Target Go File | Status |
|---|---|---|---|
| 85 | scope-resolution/passes/compound-receiver.ts | scope_resolution/compound_receiver.go | DONE (ResolveCompoundReceiverClass + EmitCompoundReceiverCalls + ResolveCompoundReceiver + splitChainAtTopLevel + resolveBareIdent + resolveCallExpr + resolveMixedChain, ~750 lines) |
| 86 | scope-resolution/passes/free-call-fallback.ts | scope_resolution/free_call_fallback.go | DONE (EmitFreeCallFallback + LookupFreeCall + narrowByArity + pickUniqueGlobalCallable + pickConstructorOrClass, ~800 lines) |
| 87 | scope-resolution/passes/imported-return-types.ts | scope_resolution/imported_return_types.go | DONE (PropagateImportedReturnTypes + FollowChainPostFinalize, ~130 lines) |
| 88 | scope-resolution/passes/mro.ts | scope_resolution/mro_pass.go | DONE (BuildMroPass — delegates to provider.BuildMro) |
| 89 | scope-resolution/passes/overload-narrowing.ts | scope_resolution/overload_narrowing.go | DONE (NarrowOverloadCandidates + rankByConversion + rankByTemplatePartialOrdering + IsOverloadAmbiguousAfterNormalization, ~540 lines) |
| 90 | scope-resolution/passes/receiver-bound-calls.ts | scope_resolution/receiver_bound_calls.go | DONE (EmitReceiverBoundCalls + EmitReceiverBoundCallsFull + emitReceiverBoundCallForSite + emitInterfaceDispatchFor + ResolveReceiverType, ~930 lines) |

### 3E. Scope & Workspace
| # | Source | Target Go File | Status |
|---|---|---|---|
| 91 | scope-resolution/scope/namespace-targets.ts | scope_resolution/namespace_targets.go | DONE (BuildNamespaceTargets + ResolveNamespaceTarget + BuildAllNamespaceTargets) |
| 92 | scope-resolution/scope/walkers.ts | scope_resolution/walkers.go | DONE (LookupBindingsAt + CollectNamespaceFqnBindings + WalkScopeChain + FindClassBindingInScope + FindReceiverTypeBinding + FindCallableBindingInScope + PopulateClassOwnedMembers + TagNamespacePrefixes, ~700 lines) |
| 93 | scope-resolution/workspace-index.ts | scope_resolution/workspace_index.go | DONE (WorkspaceResolutionIndex + BuildWorkspaceResolutionIndex + BuildWorkspaceIndex, ~130 lines) |
| 94 | scope-resolution/resolution-outcome.ts | scope_resolution/resolution_outcome.go | DONE (ResolutionOutcome struct) |

---

## Phase 4: Specialized Engines

### 4A. CFG (Control Flow Graph) — 9 files (moved to ingestion/cfg/)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 95 | cfg/types.ts | ingestion/cfg/types.go | DONE (full impl) |
| 96 | cfg/cfg-builder.ts | ingestion/cfg/builder.go | DONE (full impl: 765 lines) |
| 97 | cfg/collect.ts | ingestion/cfg/collect.go | STUB |
| 98 | cfg/control-flow-context.ts | ingestion/cfg/control_flow_context.go | STUB |
| 99 | cfg/emit.ts | ingestion/cfg/emit.go | DONE (full impl) |
| 100 | cfg/reaching-defs.ts | ingestion/cfg/reaching_defs.go | DONE (full impl) |
| 101 | cfg/traversal-result.ts | ingestion/cfg/traversal_result.go | STUB |
| 102 | cfg/visitors/typescript.ts | ingestion/cfg/typescript_visitor.go | STUB |
| 103 | cfg/visitors/typescript-harvest.ts | ingestion/cfg/typescript_harvest.go | STUB |

### 4B. Taint Analysis — 8 files (moved to ingestion/taint/, independent of cfg)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 104 | taint/emit.ts | ingestion/taint/emit.go | STUB |
| 105 | taint/match.ts | ingestion/taint/match.go | STUB (SiteMatch struct) |
| 106 | taint/path-codec.ts | ingestion/taint/path_codec.go | STUB (HopInfo struct with NodeID) |
| 107 | taint/propagate.ts | ingestion/taint/propagate.go | MISSING |
| 108 | taint/site-safety.ts | ingestion/taint/site_safety.go | MISSING |
| 109 | taint/source-sink-config.ts | ingestion/taint/source_sink_config.go | MISSING |
| 110 | taint/source-sink-registry.ts | ingestion/taint/source_sink_registry.go | MISSING |
| 111 | taint/typescript-model.ts | ingestion/taint/typescript_model.go | MISSING |

> **Note**: analyzer.go (413 lines), models.go (352 lines), types.go (68 lines) are pre-existing full implementations
> that were moved from internal/cfg/taint/ to internal/ingestion/taint/. They correspond to source-sink-registry,
> typescript-model, and types respectively. The collection dependency was replaced by local types (HopInfo, TaintFinding).

### 4C. Route Extractors — 8 files
| # | Source | Target Go File | Status |
|---|---|---|---|
| 112 | route-extractors/expo.ts | route_extractors/expo.go | MISSING |
| 113 | route-extractors/fastapi-router-bindings.ts | route_extractors/fastapi_router_bindings.go | MISSING |
| 114 | route-extractors/laravel.ts | route_extractors/laravel.go | MISSING |
| 115 | route-extractors/middleware.ts | route_extractors/middleware.go | MISSING |
| 116 | route-extractors/nextjs.ts | route_extractors/nextjs.go | MISSING |
| 117 | route-extractors/response-shapes.ts | route_extractors/response_shapes.go | MISSING |
| 118 | route-extractors/spring-shared.ts | route_extractors/spring_shared.go | MISSING |
| 119 | route-extractors/spring.ts | route_extractors/spring.go | MISSING |

### 4D. Workers — 6 files
| # | Source | Target Go File | Status |
|---|---|---|---|
| 120 | workers/worker-pool.ts | workers/worker_pool.go | MISSING |
| 121 | workers/parse-worker.ts | workers/parse_worker.go | MISSING |
| 122 | workers/result-merge.ts | workers/result_merge.go | MISSING |
| 123 | workers/post-result.ts | workers/post_result.go | MISSING |
| 124 | workers/quarantine.ts | workers/quarantine.go | MISSING |
| 125 | workers/clone-safety.ts | workers/clone_safety.go | MISSING |

---

## Phase 5: Extractors (Complete Missing Parts)

### 5A. Variable Extractors
| # | Source | Target Go File | Status |
|---|---|---|---|
| 126 | variable-extractors/generic.ts | variable_extractors/generic.go | DONE |
| 127 | variable-extractors/configs/c-cpp.ts | variable_extractors/configs/c_cpp.go | DONE |
| 128 | variable-extractors/configs/csharp.ts | variable_extractors/configs/csharp.go | DONE |
| 129 | variable-extractors/configs/go.ts | variable_extractors/configs/go.go | DONE |
| 130 | variable-extractors/configs/jvm.ts | variable_extractors/configs/jvm.go | DONE |
| 131 | variable-extractors/configs/python.ts | variable_extractors/configs/python.go | DONE |
| 132 | variable-extractors/configs/rust.ts | variable_extractors/configs/rust.go | DONE |
| 133 | variable-extractors/configs/typescript-javascript.ts | variable_extractors/configs/typescript_javascript.go | DONE |

### 5B. Class Extractors (missing configs)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 134 | class-extractors/configs/c-cpp.ts | class_extractors/configs/c_cpp.go | DONE |
| 135 | class-extractors/configs/csharp.ts | class_extractors/configs/csharp.go | DONE |
| 136 | class-extractors/configs/go.ts | class_extractors/configs/go.go | DONE |
| 137 | class-extractors/configs/jvm.ts | class_extractors/configs/jvm.go | DONE |
| 138 | class-extractors/configs/python.ts | class_extractors/configs/python.go | DONE |
| 139 | class-extractors/configs/rust.ts | class_extractors/configs/rust.go | DONE |
| 140 | class-extractors/configs/typescript-javascript.ts | class_extractors/configs/typescript_javascript.go | DONE |

### 5C. Type Extractors (configs)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 141 | type-extractors/types.ts | type_extractors/types.go | DONE |
| 142 | type-extractors/shared.ts | type_extractors/shared.go | DONE |
| 143 | type-extractors/c-cpp.ts | type_extractors/configs/c_cpp.go | DONE |
| 144 | type-extractors/csharp.ts | type_extractors/configs/csharp.go | DONE |
| 145 | type-extractors/go.ts | type_extractors/configs/go.go | DONE |
| 146 | type-extractors/jvm.ts | type_extractors/configs/jvm.go | DONE |
| 147 | type-extractors/python.ts | type_extractors/configs/python.go | DONE |
| 148 | type-extractors/rust.ts | type_extractors/configs/rust.go | DONE |
| 149 | type-extractors/typescript.ts | type_extractors/configs/typescript_javascript.go | DONE |

> Cleanup: removed legacy `typeextractors` (no underscore) package; all callers migrated to `type_extractors` with `ExtractSimpleTypeNameFromNode`.

### 5D. Field Extractors (missing configs)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 150 | field-extractors/configs/c-cpp.ts | field_extractors/configs/c_cpp.go | DONE |
| 151 | field-extractors/configs/csharp.ts | field_extractors/configs/csharp.go | DONE |
| 152 | field-extractors/configs/go.ts | field_extractors/configs/go.go | DONE |
| 153 | field-extractors/configs/jvm.ts | field_extractors/configs/jvm.go | DONE |
| 154 | field-extractors/configs/python.ts | field_extractors/configs/python.go | DONE |
| 155 | field-extractors/configs/rust.ts | field_extractors/configs/rust.go | DONE |
| 156 | field-extractors/configs/typescript-javascript.ts | field_extractors/configs/typescript_javascript.go | DONE |
| 157 | field-extractors/typescript.ts | field_extractors/typescript.go | DONE |

### 5E. Import Resolvers (missing language-specific resolvers + configs)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 158 | import-resolvers/standard.ts | import_resolvers/standard.go | DONE |
| 159 | import-resolvers/go.ts | import_resolvers/go.go | DONE |
| 160 | import-resolvers/jvm.ts | import_resolvers/jvm.go | DONE |
| 161 | import-resolvers/csharp.ts | import_resolvers/csharp.go | DONE |
| 162 | import-resolvers/python.ts | import_resolvers/python.go | DONE |
| 163 | import-resolvers/rust.ts | import_resolvers/rust.go | DONE |
| 164 | import-resolvers/configs/c-cpp.ts | import_resolvers/configs/c_cpp.go | DONE |
| 165 | import-resolvers/configs/csharp.ts | import_resolvers/configs/csharp.go | DONE |
| 166 | import-resolvers/configs/go.ts | import_resolvers/configs/go.go | DONE |
| 167 | import-resolvers/configs/jvm.ts | import_resolvers/configs/jvm.go | DONE |
| 168 | import-resolvers/configs/python.ts | import_resolvers/configs/python.go | DONE |
| 169 | import-resolvers/configs/rust.ts | import_resolvers/configs/rust.go | DONE |
| 170 | import-resolvers/configs/typescript-javascript.ts | import_resolvers/configs/typescript_javascript.go | DONE |

---

## Phase 6: Language Providers (9 languages)

### 6A. Go Language (20 source files → ~18 new Go files)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 171 | languages/go.ts | languages/go/provider.go | DONE |
| 172 | languages/go/arity-metadata.ts | languages/go/arity_metadata.go | DONE |
| 173 | languages/go/arity.ts | languages/go/arity.go | DONE |
| 174 | languages/go/cache-stats.ts | languages/go/cache_stats.go | DONE |
| 175 | languages/go/captures.ts | languages/go/captures.go | DONE |
| 176 | languages/go/expand-wildcards.ts | languages/go/expand_wildcards.go | DONE |
| 177 | languages/go/import-decomposer.ts | languages/go/import_decomposer.go | DONE |
| 178 | languages/go/import-target.ts | languages/go/import_target.go | DONE |
| 179 | languages/go/interface-impls.ts | languages/go/interface_impls.go | DONE |
| 180 | languages/go/interpret.ts | languages/go/interpret.go | DONE |
| 181 | languages/go/merge-bindings.ts | languages/go/merge_bindings.go | DONE |
| 182 | languages/go/method-owners.ts | languages/go/method_owners.go | DONE |
| 183 | languages/go/namespace-mirror.ts | languages/go/namespace_mirror.go | DONE |
| 184 | languages/go/package-siblings.ts | languages/go/package_siblings.go | DONE |
| 185 | languages/go/query.ts | languages/go/query.go | DONE |
| 186 | languages/go/range-binding.ts | languages/go/range_binding.go | DONE |
| 187 | languages/go/receiver-binding.ts | languages/go/receiver_binding.go | DONE |
| 188 | languages/go/scope-resolver.ts | languages/go/scope_resolver.go | DONE |
| 189 | languages/go/simple-hooks.ts | languages/go/simple_hooks.go | DONE |
| 190 | languages/go/type-binding.ts | languages/go/type_binding.go | DONE |

### 6B. Python Language (15 source files → ~14 new Go files)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 191 | languages/python.ts | languages/python/provider.go | DONE |
| 192 | languages/python/arity-metadata.ts | languages/python/arity_metadata.go | DONE |
| 193 | languages/python/arity.ts | languages/python/arity.go | DONE |
| 194 | languages/python/cache-stats.ts | languages/python/cache_stats.go | DONE |
| 195 | languages/python/captures.ts | languages/python/captures.go | DONE |
| 196 | languages/python/depends-references.ts | languages/python/depends_references.go | DONE |
| 197 | languages/python/import-decomposer.ts | languages/python/import_decomposer.go | DONE |
| 198 | languages/python/import-target.ts | languages/python/import_target.go | DONE |
| 199 | languages/python/interpret.ts | languages/python/interpret.go | DONE |
| 200 | languages/python/merge-bindings.ts | languages/python/merge_bindings.go | DONE |
| 201 | languages/python/query.ts | languages/python/query.go | DONE |
| 202 | languages/python/receiver-binding.ts | languages/python/receiver_binding.go | DONE |
| 203 | languages/python/scope-resolver.ts | languages/python/scope_resolver.go | DONE |
| 204 | languages/python/simple-hooks.ts | languages/python/simple_hooks.go | DONE |
| 205 | languages/python/namespace-siblings.ts | languages/python/namespace_siblings.go | DONE |

### 6C. TypeScript Language (14 source files → ~14 new Go files)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 206 | languages/typescript.ts | languages/typescript/provider.go | DONE |
| 207 | languages/typescript/arity-metadata.ts | languages/typescript/arity_metadata.go | DONE |
| 208 | languages/typescript/arity.ts | languages/typescript/arity.go | DONE |
| 209 | languages/typescript/array-callback.ts | languages/typescript/array_callback.go | DONE |
| 210 | languages/typescript/cache-stats.ts | languages/typescript/cache_stats.go | DONE |
| 211 | languages/typescript/captures.ts | languages/typescript/captures.go | DONE |
| 212 | languages/typescript/import-decomposer.ts | languages/typescript/import_decomposer.go | DONE |
| 213 | languages/typescript/import-target.ts | languages/typescript/import_target.go | DONE |
| 214 | languages/typescript/interpret.ts | languages/typescript/interpret.go | DONE |
| 215 | languages/typescript/merge-bindings.ts | languages/typescript/merge_bindings.go | DONE |
| 216 | languages/typescript/query.ts | languages/typescript/query.go | DONE |
| 217 | languages/typescript/receiver-binding.ts | languages/typescript/receiver_binding.go | DONE |
| 218 | languages/typescript/scope-resolver.ts | languages/typescript/scope_resolver.go | DONE |
| 219 | languages/typescript/simple-hooks.ts | languages/typescript/simple_hooks.go | DONE |

### 6D. JavaScript Language (9 source files → ~9 new Go files)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 220 | languages/javascript/arity.ts | languages/javascript/arity.go | DONE |
| 221 | languages/javascript/captures.ts | languages/javascript/captures.go | DONE |
| 222 | languages/javascript/import-target.ts | languages/javascript/import_target.go | DONE |
| 223 | languages/javascript/interpret.ts | languages/javascript/interpret.go | DONE |
| 224 | languages/javascript/merge-bindings.ts | languages/javascript/merge_bindings.go | DONE |
| 225 | languages/javascript/query.ts | languages/javascript/query.go | DONE |
| 226 | languages/javascript/scope-resolver.ts | languages/javascript/scope_resolver.go | DONE |
| 227 | languages/javascript/simple-hooks.ts | languages/javascript/simple_hooks.go | DONE |
| 228 | (shares provider with typescript) | languages/javascript/provider.go | DONE |

### 6E. C# Language (17 source files → ~17 new Go files)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 229 | languages/csharp.ts | languages/csharp/provider.go | DONE |
| 230 | languages/csharp/accessor-unwrap.ts | languages/csharp/accessor_unwrap.go | DONE |
| 231 | languages/csharp/arity-metadata.ts | languages/csharp/arity_metadata.go | DONE |
| 232 | languages/csharp/arity.ts | languages/csharp/arity.go | DONE |
| 233 | languages/csharp/cache-stats.ts | languages/csharp/cache_stats.go | DONE |
| 234 | languages/csharp/captures.ts | languages/csharp/captures.go | DONE |
| 235 | languages/csharp/import-decomposer.ts | languages/csharp/import_decomposer.go | DONE |
| 236 | languages/csharp/import-target.ts | languages/csharp/import_target.go | DONE |
| 237 | languages/csharp/interpret.ts | languages/csharp/interpret.go | DONE |
| 238 | languages/csharp/merge-bindings.ts | languages/csharp/merge_bindings.go | DONE |
| 239 | languages/csharp/namespace-siblings.ts | languages/csharp/namespace_siblings.go | DONE |
| 240 | languages/csharp/qualified-type-names.ts | languages/csharp/qualified_type_names.go | DONE |
| 241 | languages/csharp/query.ts | languages/csharp/query.go | DONE |
| 242 | languages/csharp/receiver-binding.ts | languages/csharp/receiver_binding.go | DONE |
| 243 | languages/csharp/resolution-config.ts | languages/csharp/resolution_config.go | DONE |
| 244 | languages/csharp/scope-resolver.ts | languages/csharp/scope_resolver.go | DONE |
| 245 | languages/csharp/simple-hooks.ts | languages/csharp/simple_hooks.go | DONE |

### 6F. Java Language (14 source files → ~14 new Go files)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 246 | languages/java.ts | languages/java/provider.go | DONE |
| 247 | languages/java/arity-metadata.ts | languages/java/arity_metadata.go | DONE |
| 248 | languages/java/arity.ts | languages/java/arity.go | DONE |
| 249 | languages/java/cache-stats.ts | languages/java/cache_stats.go | DONE |
| 250 | languages/java/captures.ts | languages/java/captures.go | DONE |
| 251 | languages/java/import-decomposer.ts | languages/java/import_decomposer.go | DONE |
| 252 | languages/java/import-target.ts | languages/java/import_target.go | DONE |
| 253 | languages/java/interpret.ts | languages/java/interpret.go | DONE |
| 254 | languages/java/merge-bindings.ts | languages/java/merge_bindings.go | DONE |
| 255 | languages/java/package-siblings.ts | languages/java/package_siblings.go | DONE |
| 256 | languages/java/query.ts | languages/java/query.go | DONE |
| 257 | languages/java/receiver-binding.ts | languages/java/receiver_binding.go | DONE |
| 258 | languages/java/scope-resolver.ts | languages/java/scope_resolver.go | DONE |
| 259 | languages/java/simple-hooks.ts | languages/java/simple_hooks.go | DONE |

### 6G. Rust Language (14 source files → ~14 new Go files)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 260 | languages/rust.ts | languages/rust/provider.go | DONE |
| 261 | languages/rust/arity-metadata.ts | languages/rust/arity_metadata.go | DONE |
| 262 | languages/rust/arity.ts | languages/rust/arity.go | DONE |
| 263 | languages/rust/cache-stats.ts | languages/rust/cache_stats.go | DONE |
| 264 | languages/rust/captures.ts | languages/rust/captures.go | DONE |
| 265 | languages/rust/import-decomposer.ts | languages/rust/import_decomposer.go | DONE |
| 266 | languages/rust/import-target.ts | languages/rust/import_target.go | DONE |
| 267 | languages/rust/interpret.ts | languages/rust/interpret.go | DONE |
| 268 | languages/rust/merge-bindings.ts | languages/rust/merge_bindings.go | DONE |
| 269 | languages/rust/method-owners.ts | languages/rust/method_owners.go | DONE |
| 270 | languages/rust/query.ts | languages/rust/query.go | DONE |
| 271 | languages/rust/range-binding.ts | languages/rust/range_binding.go | DONE |
| 272 | languages/rust/receiver-binding.ts | languages/rust/receiver_binding.go | DONE |
| 273 | languages/rust/scope-resolver.ts | languages/rust/scope_resolver.go | DONE |
| 274 | languages/rust/simple-hooks.ts | languages/rust/simple_hooks.go | DONE |

### 6H. C++ Language (24 source files → ~17 new Go files, 7 already ported)
| # | Source | Target Go File | Status |
|---|---|---|---|
| 275 | languages/cpp/adl.ts | languages/cpp/adl.go | DONE |
| 276 | languages/cpp/arity-metadata.ts | languages/cpp/arity_metadata.go | DONE |
| 277 | languages/cpp/arity.ts | — | EXISTS |
| 278 | languages/cpp/capture-side-channel.ts | languages/cpp/capture_side_channel.go | DONE |
| 279 | languages/cpp/captures.ts | languages/cpp/captures.go | DONE |
| 280 | languages/cpp/constraint-extractor.ts | languages/cpp/constraint_extractor.go | DONE |
| 281 | languages/cpp/constraint-filter.ts | languages/cpp/constraint_filter.go | DONE |
| 282 | languages/cpp/conversion-rank.ts | languages/cpp/conversion_rank.go | DONE |
| 283 | languages/cpp/file-local-linkage.ts | languages/cpp/file_local_linkage.go | DONE |
| 284 | languages/cpp/header-scan.ts | languages/cpp/header_scan.go | DONE |
| 285 | languages/cpp/import-decomposer.ts | — | EXISTS |
| 286 | languages/cpp/index.ts | languages/cpp/index.go | DONE |
| 287 | languages/cpp/inline-namespaces.ts | languages/cpp/inline_namespaces.go | DONE |
| 288 | languages/cpp/interpret.ts | — | EXISTS |
| 289 | languages/cpp/member-lookup.ts | languages/cpp/member_lookup.go | DONE |
| 290 | languages/cpp/merge-bindings.ts | — | EXISTS |
| 291 | languages/cpp/query.ts | languages/cpp/query.go | DONE |
| 292 | languages/cpp/range-bindings.ts | languages/cpp/range_bindings.go | DONE |
| 293 | languages/cpp/scope-resolver.ts | languages/cpp/scope_resolver.go | DONE |
| 294 | languages/cpp/simple-hooks.ts | — | EXISTS |
| 295 | languages/cpp/two-phase-lookup.ts | languages/cpp/two_phase_lookup.go | DONE |
| 296 | languages/cpp/type-classifier.ts | — | EXISTS |
| 297 | languages/cpp/user-defined-conversions.ts | languages/cpp/user_defined_conversions.go | DONE |

### 6I. C Language (14 source files → 0 new Go files, all 14 already ported + provider/scope_resolver rewritten)
All 14 original files exist in `languages/c/`. Provider.go and scope_resolver.go rewritten to implement full ScopeResolver interface. DONE.

### 6J. Language Registry
| # | Source | Target Go File | Status |
|---|---|---|---|
| 298 | languages/index.ts | languages/registry.go | DONE (ProviderRegistry + ResolverRegistry + GetProvider/GetResolver) |
| 299 | languages/c-cpp.ts | languages/c_cpp_provider.go | TODO (C/C++ shared header resolution) |

---

## Phase 7: Missing Utils
| # | Source | Target Go File | Status |
|---|---|---|---|
| 300 | utils/event-loop.ts | utils/event_loop.go | MISSING (may skip — Go has goroutines) |

---

## Execution Order & Dependencies

```
Phase 0 (shared types)    ← ALL other phases depend on this
  ↓
Phase 1 (pipeline skeleton) ← needs Phase 0 types
  ↓
Phase 2 (core infra)       ← needs Phase 0 + Phase 1
  ↓
Phase 3 (scope-resolution) ← needs Phase 0 + 2 + model/
  ↓
Phase 4 (engines: cfg/taint/route/workers) ← needs Phase 2 + 3
  ↓
Phase 5 (extractors completion) ← needs Phase 0 + 2
  ↓
Phase 6 (language providers) ← needs ALL above
  ↓
Phase 7 (cleanup)          ← finalize, test, verify
```

Within each phase, files should be ported in dependency order (leaves first).

## Statistics

| Category | Source Files | Already Ported | Done This Sprint | Remaining |
|---|---|---|---|---|
| Phase 0: Shared Types | 30 | 5 | 20 | 5 |
| Phase 1: Pipeline | 14 | 0 | 5+12 stubs | 1 (parse_impl) |
| Phase 2: Core Infra | 24 | 2 | 0 | 22 |
| Phase 3: Scope Resolution | 22 | 0 | 22 | 0 |
| Phase 4: Specialized Engines | 31 | 0 | 0 | 31 |
| Phase 5: Extractors | 45 | 9 | 2 | 34 |
| Phase 6: Language Providers | 149 | 20 | 0 | 129 |
| Phase 7: Utils | 1 | 0 | 0 | 0-1 |
| **TOTAL** | **346** | **81** | **~51** | **~222** |

### Completed Files (this sprint)

**shared/ (13 files):**
- shared/types.go — ScopeID, SymbolDefinition, Resolution, RawSignals, BindingRef, TypeRef, Callsite, NodeLabel, all core scope-resolution types
- shared/scope_id.go — ScopeID encoding/decoding (file/module/namespace/block/function/class/method)
- shared/parsed_file.go — ParsedFile struct
- shared/def_index.go — DefIndex (name→[]DefID multi-index, scope→[]DefID)
- shared/scope_tree.go — ScopeTree (parent/children maps, Insert/Lookup/Parent/Walk)
- shared/language_classification.go — SupportedLanguage enum (9 langs), IsProductionLanguage, ClassifyLanguage
- shared/origin_priority.go — Origin enum + RawSignals + signal composition
- shared/evidence_weights.go — EvidenceWeights, ComposeEvidence, ConfidenceFromEvidence
- shared/position_index.go — PositionIndex (byte-offset ↔ ScopeID mapping)
- shared/resolve_type_ref.go — ResolveTypeRef, ResolveTypeRefFunc, TypeRefResolution
- shared/qualified_name_index.go — QualifiedNameIndex (qualifiedName→[]DefID)
- shared/module_scope_index.go — ModuleScopeIndex (modulePath→ScopeID)
- shared/method_dispatch_index.go — MethodDispatchIndex (receiver+name→[]DefID)

**shared/registries/ (8 files):**
- shared/registries/context.go — RegistryContext (holds ScopeTree, DefIndex, QualifiedNameIndex, etc.)
- shared/registries/lookup_core.go — 7-step lookupCore algorithm (300 lines, full implementation)
- shared/registries/lookup_qualified.go — LookupQualified fast-path (qualified name direct lookup)
- shared/registries/tie_breaks.go — TieBreakKey, CompareByConfidenceWithTiebreaks
- shared/registries/class_registry.go — ClassRegistry (CLASS_KINDS, no MRO)
- shared/registries/method_registry.go — MethodRegistry (METHOD_KINDS, receiver binding, callsite)
- shared/registries/field_registry.go — FieldRegistry (FIELD_KINDS, receiver binding)
- shared/registries/macro_registry.go — MacroRegistry (MACRO_KINDS, no MRO)

**model/ (2 files modified):**
- model/type_registry.go — TypeRegistry + MutableTypeRegistry interfaces + typeRegistryImpl (4 map indexes)
- model/scope_resolution_indexes.go — Updated 5 fields to pointer types

**typeextractors/ (2 files):**
- typeextractors/shared.go — stub (ExtractSimpleTypeName, TODO Phase 5)
- type_extractors/extract_simple_type_name.go — partial implementation

**core/ (1 file modified):**
- core/supported_language.go — SupportedLanguage = shared.SupportedLanguage type alias, LangXxx const aliases

**pipeline/ (5 files):**
- pipeline/types.go — PipelinePhase interface, PipelineContext, PhaseResult, GetPhaseOutput, GetPhaseOutputTyped
- pipeline/options.go — PipelineOptions (SkipGraphPhases, PDG flags, KeepLocalValueSymbols, etc.)
- pipeline/registry.go — PhaseRegistry (Register fluent chain + Build with EnabledWhen predicates)
- pipeline/runner.go — topologicalSort (Kahn's algo) + findCyclePath (DFS) + RunPipeline
- pipeline/pipeline.go — 12 phase stubs + BuildPhaseList + RunPipelineFromRepo + PipelineResult + all output types

**scope_resolution/ (22 files — all DONE, full implementation):**
- scope_resolution/scope_resolver.go — ScopeResolver interface (8 methods: ResolveLocalSymbol + BuildDefIndex + BuildScopeTree + BuildReferenceIndex + BuildMro + ResolveImportTarget + ExpandsWildcardTo + MergeBindings)
- scope_resolution/phase.go — RunScopeResolutionPhase + RunScopeResolutionPhaseInput + ScopeResolutionOutput (~400 lines)
- scope_resolution/run.go — RunScopeResolution + FinalizeScopeModel + PreEmitInheritanceEdges + EmitDetectedInterfaceImplementations + BuildPopulatedMethodDispatch + ValidateOwnershipParity (~600 lines)
- scope_resolution/registry.go — ScopeResolverRegistry + Register/Get/All/Languages
- scope_resolution/reconcile_ownership.go — ReconcileOwnership + callableSignatureMatches (~190 lines)
- scope_resolution/validate_bindings.go — ValidateBindingsImmutability + snapshotBindings (~230 lines)
- scope_resolution/edges.go — MapReferenceKindToEdgeType + TryEmitEdge + TryEmitEdgeWithExplicitTargetId + EmitInheritanceEdge + EmitCallEdge + EmitAccessEdge
- scope_resolution/ids.go — ResolveDefGraphID + ResolveCallerGraphID + SimpleQualifiedName + QualifiedKeyWithFile (~350 lines)
- scope_resolution/imports_to_edges.go — ImportsToEdges + EmitImportEdge
- scope_resolution/method_dispatch.go — DispatchMethodCall + FindOwnedMember + MroFor (~130 lines)
- scope_resolution/node_lookup.go — GraphNodeLookup + BuildGraphNodeLookup + LookupBySimple/Qualified/ID/FilePath
- scope_resolution/references_to_edges.go — EmitReferencesViaLookup + EmitPreInheritanceEdges
- scope_resolution/compound_receiver.go — ResolveCompoundReceiverClass + EmitCompoundReceiverCalls + ResolveCompoundReceiver + splitChainAtTopLevel + resolveBareIdent + resolveCallExpr + resolveMixedChain (~750 lines)
- scope_resolution/free_call_fallback.go — EmitFreeCallFallback + LookupFreeCall + narrowByArity + pickUniqueGlobalCallable + pickConstructorOrClass (~800 lines)
- scope_resolution/imported_return_types.go — PropagateImportedReturnTypes + FollowChainPostFinalize (~130 lines)
- scope_resolution/mro_pass.go — BuildMroPass (delegates to provider.BuildMro)
- scope_resolution/overload_narrowing.go — NarrowOverloadCandidates + rankByConversion + rankByTemplatePartialOrdering + IsOverloadAmbiguousAfterNormalization (~540 lines)
- scope_resolution/receiver_bound_calls.go — EmitReceiverBoundCalls + EmitReceiverBoundCallsFull + emitReceiverBoundCallForSite + emitInterfaceDispatchFor + ResolveReceiverType (~930 lines)
- scope_resolution/namespace_targets.go — BuildNamespaceTargets + ResolveNamespaceTarget + BuildAllNamespaceTargets
- scope_resolution/walkers.go — LookupBindingsAt + CollectNamespaceFqnBindings + WalkScopeChain + FindClassBindingInScope + FindReceiverTypeBinding + FindCallableBindingInScope + PopulateClassOwnedMembers + TagNamespacePrefixes (~700 lines)
- scope_resolution/workspace_index.go — WorkspaceResolutionIndex + BuildWorkspaceResolutionIndex + BuildWorkspaceIndex (~130 lines)
- scope_resolution/resolution_outcome.go — ResolutionOutcome struct

**core/ structure_processor rewrite + KnowledgeGraph merge:**
- core/structure_processor.go — Rewritten to use shared.KnowledgeGraph (was core.KnowledgeGraph); core.Node/Relationship/GenerateID removed; ProcessStructure now emits *graph.Node/*graph.Edge
- pipeline/types.go — PipelineContext.Graph changed from core.KnowledgeGraph → shared.KnowledgeGraph
- pipeline/pipeline.go — PipelineResult.Graph + RunPipelineFromRepo param changed to shared.KnowledgeGraph
- shared/graph_types.go — Removed comment referencing core.KnowledgeGraph (now sole KnowledgeGraph interface)

## Key Adaptation Decisions (TS → Go)

1. **Worker Pool**: TS uses Web Workers with MessageChannel; Go uses goroutines + channels
2. **Async/Await**: TS `async fn` → Go sync functions (pipeline phases run sequentially)
3. **Tree-sitter**: TS uses wasm-binding; Go uses `gotreesitter` pure Go binding (NO CGo)
4. **Enums**: TS `SupportedLanguages` enum → Go `string` type with const block
5. **ReadonlyMap/Set**: TS → Go `map[K]V` with encapsulation (unexported map + exported methods)
6. **Union Types**: TS string unions → Go string constants with validation
7. **Optional fields**: TS `T?` → Go `*T` (pointer nil = undefined)
8. **ReadonlyArray**: TS → Go `[]T` (passed as copy or with interface guard)
9. **package.json/tsconfig**: TS project config parsing → Go native `encoding/json`
10. **Event loop**: TS `setImmediate`/`nextTick` → Not needed in Go (goroutine scheduler)
11. **Type alias for dedup**: core.SupportedLanguage = shared.SupportedLanguage (type alias, not re-definition)
12. **Interface+Impl pattern**: TypeRegistry/MethodRegistry/FieldRegistry all use Interface + MutableInterface + impl struct + Create factory
13. **PipelinePhase as Go interface**: Name()/Deps()/Execute() methods instead of TS object literal
14. **Phase stubs embedded**: 12 phase impl structs in single pipeline.go file (not separate files like TS)
15. **Cobol phase removed**: codetrip only supports 9 core languages, no COBOL
16. **Phase DAG simplified**: scan→structure→markdown→parse (no cobol branch)
17. **Lookup returns empty slice not nil**: All registry lookup methods return `[]*SymbolDefinition{}` on miss (never nil) for caller convenience
18. **PipelineContext.Graph**: shared.KnowledgeGraph interface passed in by caller (pipeline doesn't create it); formerly core.KnowledgeGraph, unified in sprint 2
19. **Sequential execution only**: RunPipeline executes phases sequentially in topological order (no parallel phase support needed — linear DAG)
20. **gotreesitter API**: node.Text(source) not node.Utf8Text; no CGo dependency
21. **KnowledgeGraph unified**: core.KnowledgeGraph (2-method subset) removed — ProcessStructure and PipelineContext now use shared.KnowledgeGraph exclusively; core.Node/core.Relationship/core.GenerateID deleted; ProcessStructure emits *graph.Node/*graph.Edge directly