// Entry Point Scoring — calculates entry point scores for process detection.
//
// Mirrors TS entry-point-scoring.ts, adapted for codetrip:
//   - Uses Go regexp instead of JS RegExp
//   - Language-specific patterns will be added per LanguageProvider
//   - Framework detection is delegated to framework_detection.go
//
// Score = baseScore × exportMultiplier × nameMultiplier × frameworkMultiplier
// Higher scores indicate better entry point candidates.

package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ─── Types ──────────────────────────────────────────────────

// EntryPointScoreResult holds the score and reasoning for an entry point candidate.
type EntryPointScoreResult struct {
	Score   float64
	Reasons []string
}

// FrameworkHint holds framework detection info for entry point scoring.
// Populated by framework_detection.go's DetectFrameworkFromPath.
type FrameworkHint struct {
	EntryPointMultiplier float64
	Reason               string
}

// ─── Universal entry point naming patterns ──────────────────

var universalEntryPointPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(main|init|bootstrap|start|run|setup|configure)$`),
	regexp.MustCompile(`^handle[A-Z]`),   // handleLogin, handleSubmit
	regexp.MustCompile(`^on[A-Z]`),       // onClick, onSubmit
	regexp.MustCompile(`Handler$`),       // RequestHandler
	regexp.MustCompile(`Controller$`),    // UserController
	regexp.MustCompile(`^process[A-Z]`),  // processPayment
	regexp.MustCompile(`^execute[A-Z]`),  // executeQuery
	regexp.MustCompile(`^perform[A-Z]`),  // performAction
	regexp.MustCompile(`^dispatch[A-Z]`), // dispatchEvent
	regexp.MustCompile(`^trigger[A-Z]`),  // triggerAction
	regexp.MustCompile(`^fire[A-Z]`),     // fireEvent
	regexp.MustCompile(`^emit[A-Z]`),     // emitEvent
}

// ─── Utility patterns (functions that should be penalized) ───

var utilityPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(get|set|is|has|can|should|will|did)[A-Z]`), // Accessors/predicates
	regexp.MustCompile(`^_`), // Private by convention
	regexp.MustCompile(`(?i)^(format|parse|validate|convert|transform)`), // Transformation utilities
	regexp.MustCompile(`(?i)^(log|debug|error|warn|info)$`),              // Logging
	regexp.MustCompile(`^(to|from)[A-Z]`),                                // Conversions
	regexp.MustCompile(`(?i)^(encode|decode)`),                           // Encoding utilities
	regexp.MustCompile(`(?i)^(serialize|deserialize)`),                   // Serialization
	regexp.MustCompile(`(?i)^(clone|copy|deep)`),                         // Cloning utilities
	regexp.MustCompile(`(?i)^(merge|extend|assign)`),                     // Object utilities
	regexp.MustCompile(`(?i)^(filter|map|reduce|sort|find)`),             // Collection utilities
	regexp.MustCompile(`Helper$`),
	regexp.MustCompile(`Util$`),
	regexp.MustCompile(`Utils$`),
	regexp.MustCompile(`(?i)^utils?$`),
	regexp.MustCompile(`(?i)^helpers?$`),
}

// ─── Per-language merged patterns ────────────────────────────

// mergedEntryPointPatterns will be populated by LanguageProviders
// once they are implemented in Phase 6. For now, all languages use
// only the universal patterns.
// TODO(Phase 6): Merge provider-specific entryPointPatterns into this map.
var mergedEntryPointPatterns = map[shared.SupportedLanguage][]*regexp.Regexp{
	shared.SupportedLanguageTypeScript: universalEntryPointPatterns,
	shared.SupportedLanguageJavaScript: universalEntryPointPatterns,
	shared.SupportedLanguagePython:     universalEntryPointPatterns,
	shared.SupportedLanguageJava:       universalEntryPointPatterns,
	shared.SupportedLanguageC:          universalEntryPointPatterns,
	shared.SupportedLanguageCpp:        universalEntryPointPatterns,
	shared.SupportedLanguageCSharp:     universalEntryPointPatterns,
	shared.SupportedLanguageGo:         universalEntryPointPatterns,
	shared.SupportedLanguageRust:       universalEntryPointPatterns,
}

// ─── Main scoring function ──────────────────────────────────

// CalculateEntryPointScore calculates an entry point score for a function/method.
//
// Higher scores indicate better entry point candidates.
// Score = baseScore × exportMultiplier × nameMultiplier × frameworkMultiplier
//
// Parameters:
//   - name: function/method name
//   - language: programming language
//   - isExported: whether the function is exported/public
//   - callerCount: number of functions that call this function
//   - calleeCount: number of functions this function calls
//   - filePath: optional file path for framework detection
func CalculateEntryPointScore(
	name string,
	language shared.SupportedLanguage,
	isExported bool,
	callerCount int,
	calleeCount int,
	filePath string,
) EntryPointScoreResult {
	var reasons []string

	// Must have outgoing calls to be an entry point
	if calleeCount == 0 {
		return EntryPointScoreResult{Score: 0, Reasons: []string{"no-outgoing-calls"}}
	}

	// Base score: call ratio (high ratio = calls many, called by few = likely entry point)
	baseScore := float64(calleeCount) / float64(callerCount+1)
	reasons = append(reasons, fmt.Sprintf("base:%.2f", baseScore))

	// Export bonus: exported/public functions are more likely entry points
	exportMultiplier := 1.0
	if isExported {
		exportMultiplier = 2.0
		reasons = append(reasons, "exported")
	}

	// Name pattern scoring
	nameMultiplier := 1.0

	// Check negative patterns first (utilities get penalized)
	if matchesAnyPattern(name, utilityPatterns) {
		nameMultiplier = 0.3
		reasons = append(reasons, "utility-pattern")
	} else {
		// Check positive patterns
		patterns := mergedEntryPointPatterns[language]
		if matchesAnyPattern(name, patterns) {
			nameMultiplier = 1.5
			reasons = append(reasons, "entry-pattern")
		}
	}

	// Framework detection bonus
	frameworkMultiplier := 1.0
	if filePath != "" {
		hint := DetectFrameworkFromPath(filePath)
		if hint != nil {
			frameworkMultiplier = hint.EntryPointMultiplier
			reasons = append(reasons, "framework:"+hint.Reason)
		}
	}

	// Calculate final score
	finalScore := baseScore * exportMultiplier * nameMultiplier * frameworkMultiplier

	return EntryPointScoreResult{
		Score:   finalScore,
		Reasons: reasons,
	}
}

// ─── Helper functions ────────────────────────────────────────

// matchesAnyPattern checks whether a name matches any of the given patterns.
func matchesAnyPattern(name string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(name) {
			return true
		}
	}
	return false
}

// IsTestFile checks whether a file path is a test file (should be excluded
// from entry points). Covers common test file patterns across all supported
// languages.
func IsTestFile(filePath string) bool {
	p := strings.ToLower(strings.ReplaceAll(filePath, "\\", "/"))

	return strings.Contains(p, ".test.") ||
		strings.Contains(p, ".spec.") ||
		strings.Contains(p, "__tests__/") ||
		strings.Contains(p, "__mocks__/") ||
		strings.Contains(p, "/test/") ||
		strings.Contains(p, "/tests/") ||
		strings.Contains(p, "/testing/") ||
		strings.HasSuffix(p, "_test.py") ||
		strings.Contains(p, "/test_") ||
		strings.HasSuffix(p, "_test.go") ||
		strings.Contains(p, "/src/test/") ||
		strings.Contains(p, ".tests/") ||
		strings.Contains(p, ".test/") ||
		strings.HasSuffix(p, "tests.cs") ||
		strings.HasSuffix(p, "test.cs") ||
		strings.Contains(p, "/test/fixtures/")
}

// IsUtilityFile checks whether a file path is likely a utility/helper file.
// These might still have entry points but should be lower priority.
func IsUtilityFile(filePath string) bool {
	p := strings.ToLower(strings.ReplaceAll(filePath, "\\", "/"))

	return strings.Contains(p, "/utils/") ||
		strings.Contains(p, "/util/") ||
		strings.Contains(p, "/helpers/") ||
		strings.Contains(p, "/helper/") ||
		strings.Contains(p, "/common/") ||
		strings.Contains(p, "/shared/") ||
		strings.Contains(p, "/lib/") ||
		strings.HasSuffix(p, "/utils.ts") ||
		strings.HasSuffix(p, "/utils.js") ||
		strings.HasSuffix(p, "/helpers.ts") ||
		strings.HasSuffix(p, "/helpers.js") ||
		strings.HasSuffix(p, "_utils.py") ||
		strings.HasSuffix(p, "_helpers.py")
}
