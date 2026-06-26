package cpp

// C++ Package Index — re-exports for the C++ language provider.
//
// This file serves as the package-level entry point, re-exporting
// the key public symbols for the C++ provider.
// Ported from TS languages/cpp/index.ts.
// NOTE: Go doesn't need index files like TS; this exists for structural parity.

// Public API summary (for documentation purposes):
//   CppProvider()           — LanguageProvider factory
//   CppScopeResolver        — ScopeResolver instance
//   CppMergeBindings()      — binding merge logic
//   EmitCppScopeCaptures()  — tree-sitter capture emission
//   ScanCppHeaderFiles()    — header discovery for import resolution