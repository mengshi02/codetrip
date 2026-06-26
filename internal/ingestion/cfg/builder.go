package cfg

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/mengshi02/codetrip/internal/graph"
)

// CFGBuilder is the control flow graph builder interface
type CFGBuilder interface {
	Build(funcNode *graph.Node, sourceCode string) (*FunctionCFG, error)
}

// DefaultCFGBuilder is the default CFG builder
// Builds CFG based on source code text analysis (simplified version), does not depend on gotreesitter
type DefaultCFGBuilder struct{}

// NewDefaultCFGBuilder creates a default CFG builder
func NewDefaultCFGBuilder() *DefaultCFGBuilder {
	return &DefaultCFGBuilder{}
}

// builderContext is the builder temporary context, uses sync.Pool for reuse
type builderContext struct {
	lines      []string
	blocks     []BasicBlock
	edges      []CFGEdge
	bindings   []BindingEntry
	sites      []SiteRecord
	statements []StatementFacts
	funcID     string
	funcName   string
}

// pendingEdge represents a pending edge
type pendingEdge struct {
	from      string
	to        string
	edgeType  string
	condition string
}

var ctxPool = sync.Pool{
	New: func() any {
		return &builderContext{
			blocks:     make([]BasicBlock, 0, 32),
			edges:      make([]CFGEdge, 0, 64),
			bindings:   make([]BindingEntry, 0, 16),
			sites:      make([]SiteRecord, 0, 16),
			statements: make([]StatementFacts, 0, 64),
		}
	},
}

func getBuilderContext() *builderContext {
	ctx := ctxPool.Get().(*builderContext)
	return ctx
}

func putBuilderContext(ctx *builderContext) {
	// Reset state
	ctx.lines = ctx.lines[:0]
	ctx.blocks = ctx.blocks[:0]
	ctx.edges = ctx.edges[:0]
	ctx.bindings = ctx.bindings[:0]
	ctx.sites = ctx.sites[:0]
	ctx.statements = ctx.statements[:0]
	ctx.funcID = ""
	ctx.funcName = ""
	ctxPool.Put(ctx)
}

// Pre-compiled regular expressions
var (
	reIf           = regexp.MustCompile(`^\s*if\s*[\(]`)
	reElseIf       = regexp.MustCompile(`^\s*else\s+if\s*[\(]`)
	reElse         = regexp.MustCompile(`^\s*else\s*[\{]?`)
	reFor          = regexp.MustCompile(`^\s*for\s*[\(]`)
	reWhile        = regexp.MustCompile(`^\s*while\s*[\(]`)
	// reDo reserved for future do-while support
	reSwitch       = regexp.MustCompile(`^\s*switch\s*[\(]`)
	reCase         = regexp.MustCompile(`^\s*case\s+`)
	reDefault      = regexp.MustCompile(`^\s*default\s*:`)
	reReturn       = regexp.MustCompile(`^\s*return\b`)
	reBreak        = regexp.MustCompile(`^\s*break\b`)
	reContinue     = regexp.MustCompile(`^\s*continue\b`)
	reThrow        = regexp.MustCompile(`^\s*throw\b`)
	reTry          = regexp.MustCompile(`^\s*try\s*[\{]?`)
	reCatch        = regexp.MustCompile(`^\s*catch\s*[\(]`)
	reFinally      = regexp.MustCompile(`^\s*finally\s*[\{]?`)
	// reFuncDecl reserved for future AST-based analysis
	reVarDecl      = regexp.MustCompile(`^\s*(const|let|var|int|float|double|string|bool|byte|rune|error|err)\s+`)
	reCallSite     = regexp.MustCompile(`(\w+(?:\.\w+)*)\s*\(`)
	reAssignment   = regexp.MustCompile(`(\w+)\s*(?::?=|\+\+|--|\+=|-=|\*=|/=)`)
	reMemberRead   = regexp.MustCompile(`(\w+)\.(\w+)(?!\s*\()`)
	// reBlockEnd reserved for future block-end detection
	reFuncParam    = regexp.MustCompile(`function\s*\(([^)]*)\)`)
	reGoFuncParam  = regexp.MustCompile(`func\s*\w*\s*\(([^)]*)\)`)
	reCondition    = regexp.MustCompile(`(?:if|while|for)\s*\(([^)]+)\)`)
	reOpenBrace    = regexp.MustCompile(`\{`)
	reCloseBrace   = regexp.MustCompile(`\}`)
)

// Build builds the CFG for a function
func (b *DefaultCFGBuilder) Build(funcNode *graph.Node, sourceCode string) (*FunctionCFG, error) {
	if funcNode == nil {
		return nil, fmt.Errorf("funcNode is nil")
	}
	if sourceCode == "" {
		return nil, fmt.Errorf("sourceCode is empty")
	}

	ctx := getBuilderContext()
	defer putBuilderContext(ctx)

	ctx.funcID = funcNode.ID
	ctx.funcName = funcNode.Name

	lines := strings.Split(sourceCode, "\n")
	ctx.lines = lines

	// Create entry and exit blocks
	entryID := fmt.Sprintf("%s:bb:entry", ctx.funcID)
	exitID := fmt.Sprintf("%s:bb:exit", ctx.funcID)

	entryBlock := BasicBlock{
		ID:        entryID,
		Label:     "entry",
		StartLine: 1,
		EndLine:   1,
	}
	exitBlock := BasicBlock{
		ID:    exitID,
		Label: "exit",
	}
	ctx.blocks = append(ctx.blocks, entryBlock, exitBlock)

	// Extract function parameters
	ctx.extractParams(sourceCode)

	// Analyze source code line by line, build basic blocks and edges
	currentBlockID := entryID
	currentStartLine := 1
	currentNodeIDs := []string{}
	currentStmtIDs := []string{}
	braceDepth := 0
	pendingEdges := make([]pendingEdge, 0, 64)

	for lineNum := 1; lineNum <= len(lines); lineNum++ {
		line := lines[lineNum-1]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || trimmed == "{" || trimmed == "}" {
			// Calculate brace depth
			opens := len(reOpenBrace.FindAllString(line, -1))
			closes := len(reCloseBrace.FindAllString(line, -1))
			braceDepth += opens - closes

			if trimmed == "}" && braceDepth <= 1 {
				// Block ended, create current basic block
				if len(currentNodeIDs) > 0 || currentStartLine < lineNum {
					blockID := fmt.Sprintf("%s:bb:%d", ctx.funcID, currentStartLine)
					block := BasicBlock{
						ID:           blockID,
						Label:        "normal",
						StartLine:    currentStartLine,
						EndLine:      lineNum - 1,
						NodeIDs:      currentNodeIDs,
						StatementIDs: currentStmtIDs,
					}
					ctx.blocks = append(ctx.blocks, block)
					pendingEdges = append(pendingEdges, pendingEdge{
						from: currentBlockID, to: blockID, edgeType: "normal",
					})
					currentBlockID = blockID
				}
				currentNodeIDs = nil
				currentStmtIDs = nil
				currentStartLine = lineNum + 1
			}
			continue
		}

		// Extract variable bindings
		ctx.extractBindings(trimmed, lineNum)

		// Extract call sites
		ctx.extractSites(trimmed, lineNum)

		// Extract statement facts
		ctx.extractStatementFacts(trimmed, lineNum)

		// Handle control flow statements
		switch {
		case reReturn.MatchString(trimmed):
			// return statement: end current block, connect to exit
			blockID := fmt.Sprintf("%s:bb:%d", ctx.funcID, lineNum)
			block := BasicBlock{
				ID:           blockID,
				Label:        "normal",
				StartLine:    lineNum,
				EndLine:      lineNum,
				NodeIDs:      append(currentNodeIDs, fmt.Sprintf("return:%d", lineNum)),
				StatementIDs: currentStmtIDs,
			}
			ctx.blocks = append(ctx.blocks, block)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: blockID, edgeType: "normal",
			})
			pendingEdges = append(pendingEdges, pendingEdge{
				from: blockID, to: exitID, edgeType: "normal",
			})
			currentBlockID = exitID // subsequent lines are not connected
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reIf.MatchString(trimmed):
			cond := ctx.extractCondition(trimmed)
			// Create branch blocks
			trueBlockID := fmt.Sprintf("%s:bb:%d:true", ctx.funcID, lineNum)
			falseBlockID := fmt.Sprintf("%s:bb:%d:false", ctx.funcID, lineNum)

			headerBlock := BasicBlock{
				ID:           fmt.Sprintf("%s:bb:%d", ctx.funcID, lineNum),
				Label:        "branch",
				StartLine:    lineNum,
				EndLine:      lineNum,
				NodeIDs:      append(currentNodeIDs, fmt.Sprintf("if:%d", lineNum)),
				StatementIDs: currentStmtIDs,
			}
			ctx.blocks = append(ctx.blocks, headerBlock)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: headerBlock.ID, edgeType: "normal",
			})
			pendingEdges = append(pendingEdges, pendingEdge{
				from: headerBlock.ID, to: trueBlockID, edgeType: "true", condition: cond,
			})
			pendingEdges = append(pendingEdges, pendingEdge{
				from: headerBlock.ID, to: falseBlockID, edgeType: "false", condition: cond,
			})

			currentBlockID = trueBlockID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reElseIf.MatchString(trimmed):
			cond := ctx.extractCondition(trimmed)
			trueBlockID := fmt.Sprintf("%s:bb:%d:true", ctx.funcID, lineNum)
			falseBlockID := fmt.Sprintf("%s:bb:%d:false", ctx.funcID, lineNum)

			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: trueBlockID, edgeType: "true", condition: cond,
			})
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: falseBlockID, edgeType: "false", condition: cond,
			})

			currentBlockID = trueBlockID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reElse.MatchString(trimmed):
			currentBlockID = fmt.Sprintf("%s:bb:%d:else", ctx.funcID, lineNum)
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reFor.MatchString(trimmed), reWhile.MatchString(trimmed):
			cond := ctx.extractCondition(trimmed)
			loopType := "for"
			if reWhile.MatchString(trimmed) {
				loopType = "while"
			}
			headerID := fmt.Sprintf("%s:bb:%d:loop", ctx.funcID, lineNum)
			bodyID := fmt.Sprintf("%s:bb:%d:body", ctx.funcID, lineNum)
			loopExitID := fmt.Sprintf("%s:bb:%d:loopexit", ctx.funcID, lineNum)

			headerBlock := BasicBlock{
				ID:           headerID,
				Label:        "loop",
				StartLine:    lineNum,
				EndLine:      lineNum,
				NodeIDs:      append(currentNodeIDs, fmt.Sprintf("%s:%d", loopType, lineNum)),
				StatementIDs: currentStmtIDs,
			}
			ctx.blocks = append(ctx.blocks, headerBlock)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: headerBlock.ID, edgeType: "normal",
			})
			pendingEdges = append(pendingEdges, pendingEdge{
				from: headerID, to: bodyID, edgeType: "true", condition: cond,
			})
			pendingEdges = append(pendingEdges, pendingEdge{
				from: headerID, to: loopExitID, edgeType: "false", condition: cond,
			})
			// Loop back edge
			pendingEdges = append(pendingEdges, pendingEdge{
				from: bodyID, to: headerID, edgeType: "normal",
			})

			currentBlockID = bodyID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reSwitch.MatchString(trimmed):
			switchID := fmt.Sprintf("%s:bb:%d:switch", ctx.funcID, lineNum)
			headerBlock := BasicBlock{
				ID:           switchID,
				Label:        "branch",
				StartLine:    lineNum,
				EndLine:      lineNum,
				NodeIDs:      append(currentNodeIDs, fmt.Sprintf("switch:%d", lineNum)),
				StatementIDs: currentStmtIDs,
			}
			ctx.blocks = append(ctx.blocks, headerBlock)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: switchID, edgeType: "normal",
			})
			currentBlockID = switchID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reCase.MatchString(trimmed), reDefault.MatchString(trimmed):
			caseLabel := "case"
			if reDefault.MatchString(trimmed) {
				caseLabel = "default"
			}
			caseID := fmt.Sprintf("%s:bb:%d:%s", ctx.funcID, lineNum, caseLabel)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: caseID, edgeType: "normal",
			})
			currentBlockID = caseID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum

		case reBreak.MatchString(trimmed):
			blockID := fmt.Sprintf("%s:bb:%d", ctx.funcID, lineNum)
			block := BasicBlock{
				ID:           blockID,
				Label:        "normal",
				StartLine:    lineNum,
				EndLine:      lineNum,
				NodeIDs:      append(currentNodeIDs, fmt.Sprintf("break:%d", lineNum)),
				StatementIDs: currentStmtIDs,
			}
			ctx.blocks = append(ctx.blocks, block)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: blockID, edgeType: "normal",
			})
			// break connects to loop exit or switch exit
			pendingEdges = append(pendingEdges, pendingEdge{
				from: blockID, to: exitID, edgeType: "normal",
			})
			currentBlockID = blockID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reContinue.MatchString(trimmed):
			blockID := fmt.Sprintf("%s:bb:%d", ctx.funcID, lineNum)
			block := BasicBlock{
				ID:           blockID,
				Label:        "normal",
				StartLine:    lineNum,
				EndLine:      lineNum,
				NodeIDs:      append(currentNodeIDs, fmt.Sprintf("continue:%d", lineNum)),
				StatementIDs: currentStmtIDs,
			}
			ctx.blocks = append(ctx.blocks, block)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: blockID, edgeType: "normal",
			})
			// continue connects back to loop header
			pendingEdges = append(pendingEdges, pendingEdge{
				from: blockID, to: exitID, edgeType: "normal",
			})
			currentBlockID = blockID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reThrow.MatchString(trimmed):
			blockID := fmt.Sprintf("%s:bb:%d", ctx.funcID, lineNum)
			block := BasicBlock{
				ID:           blockID,
				Label:        "normal",
				StartLine:    lineNum,
				EndLine:      lineNum,
				NodeIDs:      append(currentNodeIDs, fmt.Sprintf("throw:%d", lineNum)),
				StatementIDs: currentStmtIDs,
			}
			ctx.blocks = append(ctx.blocks, block)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: blockID, edgeType: "normal",
			})
			pendingEdges = append(pendingEdges, pendingEdge{
				from: blockID, to: exitID, edgeType: "exception",
			})
			currentBlockID = blockID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reTry.MatchString(trimmed):
			tryID := fmt.Sprintf("%s:bb:%d:try", ctx.funcID, lineNum)
			tryBlock := BasicBlock{
				ID:           tryID,
				Label:        "normal",
				StartLine:    lineNum,
				EndLine:      lineNum,
				NodeIDs:      append(currentNodeIDs, fmt.Sprintf("try:%d", lineNum)),
				StatementIDs: currentStmtIDs,
			}
			ctx.blocks = append(ctx.blocks, tryBlock)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: tryID, edgeType: "normal",
			})
			currentBlockID = tryID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum + 1

		case reCatch.MatchString(trimmed):
			catchID := fmt.Sprintf("%s:bb:%d:catch", ctx.funcID, lineNum)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: catchID, edgeType: "exception",
			})
			currentBlockID = catchID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum

		case reFinally.MatchString(trimmed):
			finallyID := fmt.Sprintf("%s:bb:%d:finally", ctx.funcID, lineNum)
			pendingEdges = append(pendingEdges, pendingEdge{
				from: currentBlockID, to: finallyID, edgeType: "normal",
			})
			currentBlockID = finallyID
			currentNodeIDs = nil
			currentStmtIDs = nil
			currentStartLine = lineNum

		default:
			// Normal statement, add to current block
			currentNodeIDs = append(currentNodeIDs, fmt.Sprintf("stmt:%d", lineNum))
			currentStmtIDs = append(currentStmtIDs, fmt.Sprintf("stmt:%d", lineNum))
		}
	}

	// If there are unclosed blocks, create them
	if len(currentNodeIDs) > 0 && currentBlockID != exitID {
		blockID := fmt.Sprintf("%s:bb:%d", ctx.funcID, currentStartLine)
		block := BasicBlock{
			ID:           blockID,
			Label:        "normal",
			StartLine:    currentStartLine,
			EndLine:      len(lines),
			NodeIDs:      currentNodeIDs,
			StatementIDs: currentStmtIDs,
		}
		ctx.blocks = append(ctx.blocks, block)
		pendingEdges = append(pendingEdges, pendingEdge{
			from: currentBlockID, to: blockID, edgeType: "normal",
		})
		// Last block connects to exit
		pendingEdges = append(pendingEdges, pendingEdge{
			from: blockID, to: exitID, edgeType: "normal",
		})
	} else if currentBlockID != exitID && currentBlockID != entryID {
		// Ensure the last block connects to exit
		pendingEdges = append(pendingEdges, pendingEdge{
			from: currentBlockID, to: exitID, edgeType: "normal",
		})
	}

	// Build edges
	edges := make([]CFGEdge, 0, len(pendingEdges))
	for _, pe := range pendingEdges {
		edges = append(edges, CFGEdge{
			From:      pe.from,
			To:        pe.to,
			EdgeType:  pe.edgeType,
			Condition: pe.condition,
		})
	}

	// Set exit block line numbers
	if len(lines) > 0 {
		ctx.blocks[1].StartLine = len(lines)
		ctx.blocks[1].EndLine = len(lines)
	}

	return &FunctionCFG{
		FuncID:     ctx.funcID,
		FuncName:   ctx.funcName,
		Bindings:   ctx.bindings,
		Blocks:     ctx.blocks,
		Edges:      edges,
		Sites:      ctx.sites,
		Statements: ctx.statements,
	}, nil
}

// extractParams extracts function parameters
func (ctx *builderContext) extractParams(sourceCode string) {
	var params []string

	// Try TypeScript/JavaScript function parameters
	if matches := reFuncParam.FindStringSubmatch(sourceCode); len(matches) > 1 {
		params = splitParams(matches[1])
	}
	// Try Go function parameters
	if len(params) == 0 {
		if matches := reGoFuncParam.FindStringSubmatch(sourceCode); len(matches) > 1 {
			params = splitParams(matches[1])
		}
	}

	for i, p := range params {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Extract variable name (remove type information)
		name := extractVarName(p)
		if name != "" {
			ctx.bindings = append(ctx.bindings, BindingEntry{
				Name:    name,
				NodeID:  fmt.Sprintf("%s:param:%d", ctx.funcID, i),
				Line:    1,
				IsParam: true,
			})
		}
	}
}

// extractBindings extracts variable bindings from a line of code
func (ctx *builderContext) extractBindings(line string, lineNum int) {
	if matches := reVarDecl.FindStringSubmatch(line); len(matches) > 0 {
		// Extract variable name
		name := extractVarName(strings.TrimPrefix(line, matches[1]+" "))
		name = strings.TrimSpace(name)
		if idx := strings.Index(name, " "); idx > 0 {
			name = name[:idx]
		}
		if idx := strings.Index(name, "="); idx > 0 {
			name = name[:idx]
		}
		name = strings.TrimSpace(name)
		if name != "" {
			ctx.bindings = append(ctx.bindings, BindingEntry{
				Name:    name,
				NodeID:  fmt.Sprintf("%s:var:%d", ctx.funcID, lineNum),
				Line:    lineNum,
				IsParam: false,
			})
		}
	}

	// Check assignment statements
	if matches := reAssignment.FindStringSubmatch(line); len(matches) > 1 {
		name := matches[1]
		if name != "" && !isKeyword(name) {
			// Check if already bound
			found := false
			for _, b := range ctx.bindings {
				if b.Name == name && b.Line == lineNum {
					found = true
					break
				}
			}
			if !found {
				ctx.bindings = append(ctx.bindings, BindingEntry{
					Name:    name,
					NodeID:  fmt.Sprintf("%s:assign:%d", ctx.funcID, lineNum),
					Line:    lineNum,
					IsParam: false,
				})
			}
		}
	}
}

// extractSites extracts call sites
func (ctx *builderContext) extractSites(line string, lineNum int) {
	// Extract function calls
	callMatches := reCallSite.FindAllStringSubmatch(line, -1)
	for _, m := range callMatches {
		if len(m) > 1 {
			symbol := m[1]
			if !isKeyword(symbol) {
				ctx.sites = append(ctx.sites, SiteRecord{
					SiteType: "call",
					Symbol:   symbol,
					Line:     lineNum,
					NodeID:   fmt.Sprintf("%s:call:%d:%s", ctx.funcID, lineNum, symbol),
				})
			}
		}
	}

	// Extract member reads
	memberMatches := reMemberRead.FindAllStringSubmatch(line, -1)
	for _, m := range memberMatches {
		if len(m) > 2 {
			ctx.sites = append(ctx.sites, SiteRecord{
				SiteType: "member-read",
				Symbol:   m[1] + "." + m[2],
				Line:     lineNum,
				NodeID:   fmt.Sprintf("%s:member:%d:%s.%s", ctx.funcID, lineNum, m[1], m[2]),
			})
		}
	}

	// Detect new keyword (constructor call)
	if strings.Contains(line, "new ") {
		newMatches := regexp.MustCompile(`new\s+(\w+)`).FindAllStringSubmatch(line, -1)
		for _, m := range newMatches {
			if len(m) > 1 {
				ctx.sites = append(ctx.sites, SiteRecord{
					SiteType: "construct",
					Symbol:   m[1],
					Line:     lineNum,
					NodeID:   fmt.Sprintf("%s:construct:%d:%s", ctx.funcID, lineNum, m[1]),
				})
			}
		}
	}
}

// extractStatementFacts extracts statement facts
func (ctx *builderContext) extractStatementFacts(line string, lineNum int) {
	stmtID := fmt.Sprintf("%s:stmt:%d", ctx.funcID, lineNum)
	facts := StatementFacts{
		StatementID: stmtID,
		Line:        lineNum,
	}

	// Extract definitions (left side of assignment)
	if matches := reAssignment.FindStringSubmatch(line); len(matches) > 1 {
		facts.Defines = append(facts.Defines, matches[1])
	}

	// Extract uses (variable references)
	uses := regexp.MustCompile(`\b([a-zA-Z_]\w*)\b`).FindAllStringSubmatch(line, -1)
	seenUses := make(map[string]bool)
	for _, m := range uses {
		if len(m) > 1 && !isKeyword(m[1]) && m[1] != ctx.funcName {
			if !seenUses[m[1]] {
				facts.Uses = append(facts.Uses, m[1])
				seenUses[m[1]] = true
			}
		}
	}

	if len(facts.Defines) > 0 || len(facts.Uses) > 0 {
		ctx.statements = append(ctx.statements, facts)
	}
}

// extractCondition extracts condition expression from conditional statement
func (ctx *builderContext) extractCondition(line string) string {
	matches := reCondition.FindStringSubmatch(line)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// splitParams splits parameter list
func splitParams(paramStr string) []string {
	var params []string
	depth := 0
	current := strings.Builder{}

	for _, ch := range paramStr {
		switch ch {
		case '(', '[', '<':
			depth++
			current.WriteRune(ch)
		case ')', ']', '>':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				params = append(params, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		params = append(params, current.String())
	}

	return params
}

// extractVarName extracts variable name from parameter/variable declaration
func extractVarName(decl string) string {
	decl = strings.TrimSpace(decl)
	// Go style: name type or name1, name2 type
	// TS/JS style: name or name: type
	parts := strings.Fields(decl)
	if len(parts) == 0 {
		return ""
	}
	first := parts[0]
	// Handle name: type
	if idx := strings.Index(first, ":"); idx > 0 {
		first = first[:idx]
	}
	// Handle name =
	if idx := strings.Index(first, "="); idx > 0 {
		first = first[:idx]
	}
	// Handle comma-separated multiple variables
	if idx := strings.Index(first, ","); idx > 0 {
		first = first[:idx]
	}
	// Handle destructuring
	if strings.HasPrefix(first, "{") || strings.HasPrefix(first, "[") || strings.HasPrefix(first, "...") {
		return ""
	}
	return strings.TrimSpace(first)
}

// isKeyword checks if a word is a keyword
func isKeyword(word string) bool {
	keywords := map[string]bool{
		// Go
		"func": true, "if": true, "else": true, "for": true, "while": true,
		"return": true, "break": true, "continue": true, "switch": true,
		"case": true, "default": true, "var": true, "const": true, "let": true,
		"type": true, "struct": true, "interface": true, "package": true,
		"import": true, "go": true, "defer": true, "range": true,
		"select": true, "chan": true, "map": true, "true": true, "false": true,
		"nil": true, "err": true,
		// TypeScript/JavaScript
		"function": true, "async": true, "await": true, "class": true,
		"new": true, "this": true, "super": true, "extends": true,
		"implements": true, "throw": true, "try": true, "catch": true,
		"finally": true, "typeof": true, "instanceof": true,
		"null": true, "undefined": true, "void": true, "delete": true,
		"in": true, "of": true, "yield": true, "from": true,
		"export": true, "require": true, "module": true,
		// General
		"int": true, "float": true, "double": true, "string": true,
		"bool": true, "byte": true, "rune": true, "error": true,
	}
	return keywords[word]
}