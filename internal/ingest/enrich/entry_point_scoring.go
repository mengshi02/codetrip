package enrich

// Entry Point Scoring.
//
// Calculates entry point scores for process detection based on:
// 1. Call ratio (callees / (callers + 1))
// 2. Export status (exported functions get higher priority)
// 3. Name patterns (handle*, on*, *Controller, etc.)
// 4. Framework detection (path-based detection for Next.js, Express, Django, etc.)

import (
	"fmt"
	"regexp"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// Name patterns — entry point patterns by language
// ─────────────────────────────────────────────────────────────────────────────

// universalPatterns apply to all languages.
var universalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(main|init|bootstrap|start|run|setup|configure)$`),
	regexp.MustCompile(`^handle[A-Z]`),
	regexp.MustCompile(`^on[A-Z]`),
	regexp.MustCompile(`Handler$`),
	regexp.MustCompile(`Controller$`),
	regexp.MustCompile(`^process[A-Z]`),
	regexp.MustCompile(`^execute[A-Z]`),
	regexp.MustCompile(`^perform[A-Z]`),
	regexp.MustCompile(`^dispatch[A-Z]`),
	regexp.MustCompile(`^trigger[A-Z]`),
	regexp.MustCompile(`^fire[A-Z]`),
	regexp.MustCompile(`^emit[A-Z]`),
}

// languagePatterns maps language ID → additional entry-point regexps.
var languagePatterns = map[string][]*regexp.Regexp{
	"javascript": {
		regexp.MustCompile(`^use[A-Z]`), // React hooks
	},
	"typescript": {
		regexp.MustCompile(`^use[A-Z]`),
	},
	"tsx": {
		regexp.MustCompile(`^use[A-Z]`),
	},
	"python": {
		regexp.MustCompile(`(?i)^app$`),
		regexp.MustCompile(`(?i)^(get|post|put|delete|patch)_`),
		regexp.MustCompile(`(?i)^api_`),
		regexp.MustCompile(`(?i)^view_`),
	},
	"java": {
		regexp.MustCompile(`^do[A-Z]`),
		regexp.MustCompile(`^create[A-Z]`),
		regexp.MustCompile(`^build[A-Z]`),
		regexp.MustCompile(`Service$`),
	},
	"csharp": {
		regexp.MustCompile(`^(Get|Post|Put|Delete|Patch)`),
		regexp.MustCompile(`Action$`),
		regexp.MustCompile(`^On[A-Z]`),
		regexp.MustCompile(`Async$`),
		regexp.MustCompile(`^Configure$`),
		regexp.MustCompile(`^ConfigureServices$`),
		regexp.MustCompile(`^Handle$`),
		regexp.MustCompile(`^Execute$`),
		regexp.MustCompile(`^Invoke$`),
		regexp.MustCompile(`^Map[A-Z]`),
		regexp.MustCompile(`Service$`),
		regexp.MustCompile(`^Seed`),
	},
	"go": {
		regexp.MustCompile(`Handler$`),
		regexp.MustCompile(`^Serve`),
		regexp.MustCompile(`^New[A-Z]`),
		regexp.MustCompile(`^Make[A-Z]`),
	},
	"rust": {
		regexp.MustCompile(`(?i)^(get|post|put|delete)_handler$`),
		regexp.MustCompile(`^handle_`),
		regexp.MustCompile(`(?i)^new$`),
		regexp.MustCompile(`(?i)^run$`),
		regexp.MustCompile(`^spawn`),
	},
	"c": {
		regexp.MustCompile(`^main$`),
		regexp.MustCompile(`^init_`),
		regexp.MustCompile(`_init$`),
		regexp.MustCompile(`^start_`),
		regexp.MustCompile(`_start$`),
		regexp.MustCompile(`^run_`),
		regexp.MustCompile(`_run$`),
		regexp.MustCompile(`^stop_`),
		regexp.MustCompile(`_stop$`),
		regexp.MustCompile(`^open_`),
		regexp.MustCompile(`_open$`),
		regexp.MustCompile(`^close_`),
		regexp.MustCompile(`_close$`),
		regexp.MustCompile(`^create_`),
		regexp.MustCompile(`_create$`),
		regexp.MustCompile(`^destroy_`),
		regexp.MustCompile(`_destroy$`),
		regexp.MustCompile(`^handle_`),
		regexp.MustCompile(`_handler$`),
		regexp.MustCompile(`_callback$`),
		regexp.MustCompile(`^cmd_`),
		regexp.MustCompile(`^server_`),
		regexp.MustCompile(`^client_`),
		regexp.MustCompile(`^session_`),
		regexp.MustCompile(`^window_`),
		regexp.MustCompile(`^key_`),
		regexp.MustCompile(`^input_`),
		regexp.MustCompile(`^output_`),
		regexp.MustCompile(`^notify_`),
		regexp.MustCompile(`^control_`),
	},
	"cpp": {
		regexp.MustCompile(`^main$`),
		regexp.MustCompile(`^init_`),
		regexp.MustCompile(`_init$`),
		regexp.MustCompile(`^Create[A-Z]`),
		regexp.MustCompile(`^create_`),
		regexp.MustCompile(`^Run$`),
		regexp.MustCompile(`(?i)^run$`),
		regexp.MustCompile(`^Start$`),
		regexp.MustCompile(`(?i)^start$`),
		regexp.MustCompile(`^handle_`),
		regexp.MustCompile(`_handler$`),
		regexp.MustCompile(`_callback$`),
		regexp.MustCompile(`^OnEvent`),
		regexp.MustCompile(`^on_`),
		regexp.MustCompile(`::Run$`),
		regexp.MustCompile(`::Start$`),
		regexp.MustCompile(`::Init$`),
		regexp.MustCompile(`::Execute$`),
	},
	"swift": {
		regexp.MustCompile(`^viewDidLoad$`),
		regexp.MustCompile(`^viewWillAppear$`),
		regexp.MustCompile(`^viewDidAppear$`),
		regexp.MustCompile(`^viewWillDisappear$`),
		regexp.MustCompile(`^viewDidDisappear$`),
		regexp.MustCompile(`^application\(`),
		regexp.MustCompile(`^scene\(`),
		regexp.MustCompile(`^body$`),
		regexp.MustCompile(`Coordinator$`),
		regexp.MustCompile(`^sceneDidBecomeActive$`),
		regexp.MustCompile(`^sceneWillResignActive$`),
		regexp.MustCompile(`^didFinishLaunchingWithOptions$`),
		regexp.MustCompile(`ViewController$`),
		regexp.MustCompile(`^configure[A-Z]`),
		regexp.MustCompile(`^setup[A-Z]`),
		regexp.MustCompile(`^makeBody$`),
	},
	"php": {
		regexp.MustCompile(`Controller$`),
		regexp.MustCompile(`^handle$`),
		regexp.MustCompile(`^execute$`),
		regexp.MustCompile(`^boot$`),
		regexp.MustCompile(`^register$`),
		regexp.MustCompile(`^__invoke$`),
		regexp.MustCompile(`(?i)^(index|show|store|update|destroy|create|edit)$`),
		regexp.MustCompile(`(?i)^(get|post|put|delete|patch)[A-Z]`),
		regexp.MustCompile(`(?i)^run$`),
		regexp.MustCompile(`(?i)^fire$`),
		regexp.MustCompile(`(?i)^dispatch$`),
		regexp.MustCompile(`Service$`),
		regexp.MustCompile(`Repository$`),
		regexp.MustCompile(`^find$`),
		regexp.MustCompile(`^findAll$`),
		regexp.MustCompile(`^save$`),
		regexp.MustCompile(`^delete$`),
	},
}

// mergedPatterns caches universal + language-specific patterns per language.
var mergedPatterns map[string][]*regexp.Regexp

func init() {
	mergedPatterns = make(map[string][]*regexp.Regexp)
	for lang := range languagePatterns {
		pats := make([]*regexp.Regexp, len(universalPatterns), len(universalPatterns)+len(languagePatterns[lang]))
		copy(pats, universalPatterns)
		pats = append(pats, languagePatterns[lang]...)
		mergedPatterns[lang] = pats
	}
	// Languages not in languagePatterns get universal only.
}

// ─────────────────────────────────────────────────────────────────────────────
// Utility patterns — functions that should be penalized
// ─────────────────────────────────────────────────────────────────────────────

var utilityPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(get|set|is|has|can|should|will|did)[A-Z]`),
	regexp.MustCompile(`^_`),
	regexp.MustCompile(`(?i)^(format|parse|validate|convert|transform)`),
	regexp.MustCompile(`(?i)^(log|debug|error|warn|info)$`),
	regexp.MustCompile(`^(to|from)[A-Z]`),
	regexp.MustCompile(`(?i)^(encode|decode)`),
	regexp.MustCompile(`(?i)^(serialize|deserialize)`),
	regexp.MustCompile(`(?i)^(clone|copy|deep)`),
	regexp.MustCompile(`(?i)^(merge|extend|assign)`),
	regexp.MustCompile(`(?i)^(filter|map|reduce|sort|find)`),
	regexp.MustCompile(`Helper$`),
	regexp.MustCompile(`Util$`),
	regexp.MustCompile(`Utils$`),
	regexp.MustCompile(`(?i)^utils?$`),
	regexp.MustCompile(`(?i)^helpers?$`),
}

// ─────────────────────────────────────────────────────────────────────────────
// Public types
// ─────────────────────────────────────────────────────────────────────────────

// EntryPointScoreResult holds the score and reasons for an entry point candidate.
type EntryPointScoreResult struct {
	Score   float64
	Reasons []string
}

// ─────────────────────────────────────────────────────────────────────────────
// Main scoring function
// ─────────────────────────────────────────────────────────────────────────────

// CalculateEntryPointScore computes an entry point score for a function/method.
// Higher scores indicate better entry point candidates.
// Score = baseScore × exportMultiplier × nameMultiplier × frameworkMultiplier.
func CalculateEntryPointScore(
	name string,
	language string,
	isExported bool,
	callerCount int,
	calleeCount int,
	filePath string,
) EntryPointScoreResult {
	reasons := []string{}

	// Must have outgoing calls to be an entry point
	if calleeCount == 0 {
		return EntryPointScoreResult{Score: 0, Reasons: []string{"no-outgoing-calls"}}
	}

	// Base score: call ratio
	baseScore := float64(calleeCount) / float64(callerCount+1)
	reasons = append(reasons, fmt.Sprintf("base:%.2f", baseScore))

	// Export bonus
	exportMultiplier := 1.0
	if isExported {
		exportMultiplier = 2.0
		reasons = append(reasons, "exported")
	}

	// Name pattern scoring
	nameMultiplier := 1.0
	if matchesAnyPattern(name, utilityPatterns) {
		nameMultiplier = 0.3
		reasons = append(reasons, "utility-pattern")
	} else {
		pats := getPatternsForLanguage(language)
		if matchesAnyPattern(name, pats) {
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

	finalScore := baseScore * exportMultiplier * nameMultiplier * frameworkMultiplier
	return EntryPointScoreResult{Score: finalScore, Reasons: reasons}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test file detection
// ─────────────────────────────────────────────────────────────────────────────

// IsTestFile returns true if the file path is a test file (should be excluded from entry points).
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
		strings.HasSuffix(p, "tests.swift") ||
		strings.HasSuffix(p, "test.swift") ||
		strings.Contains(p, "uitests/") ||
		strings.HasSuffix(p, "tests.cs") ||
		strings.HasSuffix(p, "test.cs") ||
		strings.Contains(p, ".tests/") ||
		strings.Contains(p, ".test/") ||
		strings.Contains(p, ".integrationtests/") ||
		strings.Contains(p, ".unittests/") ||
		strings.Contains(p, "/testproject/") ||
		strings.HasSuffix(p, "test.php") ||
		strings.HasSuffix(p, "spec.php") ||
		strings.Contains(p, "/tests/feature/") ||
		strings.Contains(p, "/tests/unit/")
}

// IsUtilityFile returns true if the file path is likely a utility/helper file.
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

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func matchesAnyPattern(name string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(name) {
			return true
		}
	}
	return false
}

func getPatternsForLanguage(language string) []*regexp.Regexp {
	if pats, ok := mergedPatterns[language]; ok {
		return pats
	}
	return universalPatterns
}

// capitalize, sanitizeID, and pascalCaseFromPath are defined in
// community_processor.go and process_processor.go respectively (same package).
