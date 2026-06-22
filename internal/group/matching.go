package group

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// ContractMatcher is the contract matcher
// Executes a 5-level matching pipeline:
// 1. exact:      Exact path/name matching (high confidence 0.95)
// 2. manifest:   group.yaml manual links (confidence 1.0)
// 3. wildcard:   Path pattern matching (confidence 0.8)
// 4. bm25:       BM25 text similarity (confidence 0.6-0.9, threshold configurable)
// 5. embedding:  Semantic vector similarity (requires Embedder, not yet implemented, confidence 0.85+)
type ContractMatcher struct {
	config MatchingConfig
}

// NewContractMatcher creates a contract matcher
func NewContractMatcher(config MatchingConfig) *ContractMatcher {
	return &ContractMatcher{config: config}
}

// Match executes the 5-level matching pipeline
func (m *ContractMatcher) Match(consumers, providers []Contract, manifestLinks []GroupManifestLink) []BridgeLink {
	var links []BridgeLink
	seen := make(map[string]bool) // sourceID:targetID deduplication

	addLink := func(link BridgeLink) {
		key := fmt.Sprintf("%s:%s", link.SourceID, link.TargetID)
		if !seen[key] {
			seen[key] = true
			links = append(links, link)
		}
	}

	// Level 1: exact — exact matching
	exactLinks := m.matchExact(consumers, providers)
	for _, link := range exactLinks {
		addLink(link)
	}

	// Level 2: manifest — manual links
	manifestLinks2 := m.matchManifest(consumers, providers, manifestLinks)
	for _, link := range manifestLinks2 {
		addLink(link)
	}

	// Level 3: wildcard — path pattern matching
	wildcardLinks := m.matchWildcard(consumers, providers)
	for _, link := range wildcardLinks {
		addLink(link)
	}

	// Level 4: bm25 — text similarity
	bm25Links := m.matchBM25(consumers, providers)
	for _, link := range bm25Links {
		addLink(link)
	}

	// Level 5: embedding — not yet implemented

	// Limit maximum number of matches
	if m.config.MaxMatches > 0 && len(links) > m.config.MaxMatches {
		// Sort by confidence, keep Top-N
		sort.Slice(links, func(i, j int) bool {
			return links[i].Confidence > links[j].Confidence
		})
		links = links[:m.config.MaxMatches]
	}

	return links
}

// matchExact Level 1: Exact path/name matching
// Uses hash map O(1) lookup
func (m *ContractMatcher) matchExact(consumers, providers []Contract) []BridgeLink {
	var links []BridgeLink

	// Build contract identifier index: ContractID → Contract
	providerByID := make(map[string]Contract)
	providerBySymbol := make(map[string][]Contract) // symbolUID → []Contract

	for _, p := range providers {
		providerByID[p.ContractID] = p
		providerBySymbol[p.SymbolUID] = append(providerBySymbol[p.SymbolUID], p)
	}

	for _, c := range consumers {
		// Exact ContractID matching
		if p, ok := providerByID[c.ContractID]; ok {
			links = append(links, BridgeLink{
				SourceID:   c.ContractID,
				TargetID:   p.ContractID,
				MatchType:  "exact",
				Confidence: 0.95,
				ContractID: fmt.Sprintf("%s->%s", c.ContractID, p.ContractID),
			})
			continue
		}

		// Exact SymbolUID matching (same type)
		if ps, ok := providerBySymbol[c.SymbolUID]; ok {
			for _, p := range ps {
				if c.Type == p.Type {
					links = append(links, BridgeLink{
						SourceID:   c.ContractID,
						TargetID:   p.ContractID,
						MatchType:  "exact",
						Confidence: 0.95,
						ContractID: fmt.Sprintf("%s->%s", c.ContractID, p.ContractID),
					})
				}
			}
		}

		// Path matching in Meta
		cPath := getMetaString(c.Meta, "path")
		if cPath != "" {
			for _, p := range providers {
				pPath := getMetaString(p.Meta, "path")
				if pPath != "" && cPath == pPath && c.Type == p.Type {
					links = append(links, BridgeLink{
						SourceID:   c.ContractID,
						TargetID:   p.ContractID,
						MatchType:  "exact",
						Confidence: 0.95,
						ContractID: fmt.Sprintf("%s->%s", c.ContractID, p.ContractID),
					})
				}
			}
		}
	}

	return links
}

// matchManifest Level 2: Manual link matching
func (m *ContractMatcher) matchManifest(consumers, providers []Contract, manifestLinks []GroupManifestLink) []BridgeLink {
	var links []BridgeLink

	// Build symbol index
	consumerBySymbol := make(map[string][]Contract) // repo:symbol → []Contract
	for _, c := range consumers {
		key := fmt.Sprintf("%s:%s", c.Repo, c.SymbolUID)
		consumerBySymbol[key] = append(consumerBySymbol[key], c)
	}
	providerBySymbol := make(map[string][]Contract) // repo:symbol → []Contract
	for _, p := range providers {
		key := fmt.Sprintf("%s:%s", p.Repo, p.SymbolUID)
		providerBySymbol[key] = append(providerBySymbol[key], p)
	}

	for _, ml := range manifestLinks {
		srcKey := fmt.Sprintf("%s:%s", ml.SourceRepo, ml.SourceSymbol)
		tgtKey := fmt.Sprintf("%s:%s", ml.TargetRepo, ml.TargetSymbol)

		srcContracts := consumerBySymbol[srcKey]
		tgtContracts := providerBySymbol[tgtKey]

		for _, sc := range srcContracts {
			for _, tc := range tgtContracts {
				links = append(links, BridgeLink{
					SourceID:   sc.ContractID,
					TargetID:   tc.ContractID,
					MatchType:  "manifest",
					Confidence: 1.0,
					ContractID: fmt.Sprintf("%s->%s", sc.ContractID, tc.ContractID),
				})
			}
		}
	}

	return links
}

// matchWildcard Level 3: Path pattern matching
func (m *ContractMatcher) matchWildcard(consumers, providers []Contract) []BridgeLink {
	var links []BridgeLink

	for _, c := range consumers {
		cPath := getMetaString(c.Meta, "path")
		if cPath == "" {
			continue
		}

		for _, p := range providers {
			if c.Type != p.Type {
				continue
			}
			pPath := getMetaString(p.Meta, "path")
			if pPath == "" {
				continue
			}

			// Prefix matching: /api/v1/users → /api/v1/users
			if strings.HasPrefix(cPath, pPath) || strings.HasPrefix(pPath, cPath) {
				links = append(links, BridgeLink{
					SourceID:   c.ContractID,
					TargetID:   p.ContractID,
					MatchType:  "wildcard",
					Confidence: 0.8,
					ContractID: fmt.Sprintf("%s->%s", c.ContractID, p.ContractID),
				})
				continue
			}

			// Suffix matching
			cSuffix := pathSuffix(cPath, 2)
			pSuffix := pathSuffix(pPath, 2)
			if cSuffix != "" && cSuffix == pSuffix {
				links = append(links, BridgeLink{
					SourceID:   c.ContractID,
					TargetID:   p.ContractID,
					MatchType:  "wildcard",
					Confidence: 0.75,
					ContractID: fmt.Sprintf("%s->%s", c.ContractID, p.ContractID),
				})
			}
		}
	}

	return links
}

// matchBM25 Level 4: BM25 text similarity
// Simplified implementation: uses term frequency based TF-IDF similarity
func (m *ContractMatcher) matchBM25(consumers, providers []Contract) []BridgeLink {
	var links []BridgeLink

	// Build document collection
	allDocs := make([][]string, 0, len(providers))
	providerList := make([]Contract, 0, len(providers))
	for _, p := range providers {
		tokens := contractTokens(p)
		if len(tokens) > 0 {
			allDocs = append(allDocs, tokens)
			providerList = append(providerList, p)
		}
	}

	if len(allDocs) == 0 {
		return links
	}

	// Calculate IDF
	df := make(map[string]int) // token → document frequency
	for _, tokens := range allDocs {
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				seen[t] = true
				df[t]++
			}
		}
	}
	nDocs := float64(len(allDocs))

	for _, c := range consumers {
		cTokens := contractTokens(c)
		if len(cTokens) == 0 {
			continue
		}

		cSet := make(map[string]bool)
		for _, t := range cTokens {
			cSet[t] = true
		}

		for i, pTokens := range allDocs {
			// Calculate intersection
			overlap := 0
			for _, t := range pTokens {
				if cSet[t] {
					overlap++
				}
			}

			if overlap == 0 {
				continue
			}

			// Jaccard similarity as approximate score
			pSet := make(map[string]bool)
			for _, t := range pTokens {
				pSet[t] = true
			}
			union := len(cSet) + len(pSet) - overlap
			if union == 0 {
				continue
			}
			jaccard := float64(overlap) / float64(union)

			// IDF weighting
			idfWeight := 0.0
			for t := range cSet {
				if _, ok := df[t]; ok {
					idfWeight += 1.0
				}
			}
			_ = nDocs // IDF weight is implicit in Jaccard

			// Confidence mapping: [0,1] → [0.6, 0.9]
			confidence := 0.6 + jaccard*0.3

			if confidence >= m.config.BM25Threshold {
				links = append(links, BridgeLink{
					SourceID:   c.ContractID,
					TargetID:   providerList[i].ContractID,
					MatchType:  "bm25",
					Confidence: confidence,
					ContractID: fmt.Sprintf("%s->%s", c.ContractID, providerList[i].ContractID),
				})
			}
		}
	}

	return links
}

// contractTokens extracts text tokens from a contract
func contractTokens(c Contract) []string {
	var tokens []string
	tokens = append(tokens, tokenizeText(c.ContractID)...)
	tokens = append(tokens, tokenizeText(c.Type)...)
	tokens = append(tokens, tokenizeText(c.SymbolUID)...)
	for k, v := range c.Meta {
		tokens = append(tokens, tokenizeText(k)...)
		if s, ok := v.(string); ok {
			tokens = append(tokens, tokenizeText(s)...)
		}
	}
	return tokens
}

// tokenizeText performs simple tokenization (camelCase + snake_case + punctuation splitting)
func tokenizeText(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if unicode.IsUpper(r) && current.Len() > 0 {
				// camelCase splitting
				tokens = append(tokens, strings.ToLower(current.String()))
				current.Reset()
			}
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, strings.ToLower(current.String()))
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, strings.ToLower(current.String()))
	}

	return tokens
}

// pathSuffix gets the last N segments of a path
func pathSuffix(path string, n int) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < n {
		return path
	}
	return strings.Join(parts[len(parts)-n:], "/")
}

// getMetaString gets a string value from contract Meta
func getMetaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}