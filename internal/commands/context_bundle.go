package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tyemirov/ctx/internal/config"
	"github.com/tyemirov/ctx/internal/tokenizer"
	"github.com/tyemirov/ctx/internal/types"
	"github.com/tyemirov/ctx/internal/utils"
)

const (
	contextBundleVersion        = 1
	defaultContextBundleTokens  = 24000
	contextBundleContractRunes  = 12000
	contextBundleFileRunes      = 16000
	contextBundleMaxFileBytes   = 512 * 1024
	contextBundleGeneratedBy    = "ctx bundle"
	contextBundleRoleContract   = "contract"
	contextBundleRoleIssueDoc   = "issue-document"
	contextBundleRolePlan       = "plan"
	contextBundleRoleImpl       = "implementation"
	contextBundleRoleTest       = "test"
	contextBundleRoleDocs       = "docs"
	contextBundleRoleRuntime    = "runtime"
	contextBundleRoleConfig     = "config"
	contextBundleRoleAsset      = "asset"
	contextBundleReasonContract = "binding repository contract"
)

var contextBundleWordPattern = regexp.MustCompile(`[A-Za-z0-9_]+`)

var contextBundleStopWords = map[string]struct{}{
	"about": {}, "after": {}, "agent": {}, "agents": {}, "also": {}, "and": {}, "any": {}, "are": {},
	"can": {}, "could": {}, "for": {}, "from": {}, "goal": {}, "have": {}, "how": {}, "into": {},
	"its": {}, "new": {}, "not": {}, "now": {}, "our": {}, "out": {}, "put": {}, "properly": {},
	"should": {}, "stage": {}, "that": {}, "the": {}, "then": {}, "there": {}, "this": {}, "tool": {},
	"use": {}, "want": {}, "when": {}, "with": {}, "work": {}, "would": {},
}

var contextBundleDefaultExclusions = []string{
	".git/",
	".cache/",
	"node_modules/",
	"vendor/",
	"dist/",
	"build/",
	"coverage/",
	"tmp/",
	"*.png",
	"*.jpg",
	"*.jpeg",
	"*.gif",
	"*.webp",
	"*.pdf",
	"*.zip",
	"*.tar",
	"*.gz",
}

// ContextBundleOptions configures goal-oriented bundle extraction.
type ContextBundleOptions struct {
	Request      types.ContextBundleRequest
	TokenCounter tokenizer.Counter
	TokenModel   string
}

type contextBundleCandidate struct {
	path      string
	role      string
	reason    string
	score     int
	content   string
	lineStart int
	lineEnd   int
	tokens    int
	hash      string
}

// BuildContextBundle assembles deterministic repository context for a concrete execution goal.
func BuildContextBundle(ctx context.Context, options ContextBundleOptions) (*types.ContextBundleOutput, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	request := normalizeContextBundleRequest(options.Request)
	if strings.TrimSpace(request.RepositoryRoot) == "" {
		return nil, fmt.Errorf("repositoryRoot is required")
	}
	if strings.TrimSpace(request.Goal.Title) == "" && strings.TrimSpace(request.Goal.ID) == "" {
		return nil, fmt.Errorf("goal.title or goal.id is required")
	}
	if options.TokenCounter == nil {
		return nil, fmt.Errorf("token counter is required")
	}
	repositoryRoot, rootErr := filepath.Abs(request.RepositoryRoot)
	if rootErr != nil {
		return nil, fmt.Errorf("resolve repository root: %w", rootErr)
	}
	rootInfo, statErr := os.Stat(repositoryRoot)
	if statErr != nil {
		return nil, fmt.Errorf("stat repository root: %w", statErr)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("repositoryRoot must be a directory")
	}

	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultContextBundleTokens
	}
	tokenModel := strings.TrimSpace(options.TokenModel)
	if tokenModel == "" {
		tokenModel = strings.TrimSpace(request.Model)
	}
	terms := deriveContextBundleTerms(request)
	ignorePatterns, _, ignoreErr := config.LoadRecursiveIgnorePatterns(
		repositoryRoot,
		append(append([]string{}, contextBundleDefaultExclusions...), request.ExcludePaths...),
		true,
		true,
		false,
	)
	if ignoreErr != nil {
		return nil, fmt.Errorf("load ignore patterns: %w", ignoreErr)
	}

	var contracts []contextBundleCandidate
	var files []contextBundleCandidate
	var exclusions []types.ContextBundleExclusion
	walkErr := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		relativePath := utils.RelativePathOrSelf(path, repositoryRoot)
		if relativePath == "." {
			return nil
		}
		relativePath = filepath.ToSlash(relativePath)
		if utils.ShouldIgnoreByPath(relativePath, ignorePatterns) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Size() > contextBundleMaxFileBytes {
			exclusions = appendContextBundleExclusion(exclusions, types.ContextBundleExclusion{
				Path:   relativePath,
				Role:   classifyContextBundleRole(relativePath, request),
				Reason: "file exceeds context bundle size limit",
			})
			return nil
		}
		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if utils.IsBinary(contentBytes) || !utf8.Valid(contentBytes) {
			exclusions = appendContextBundleExclusion(exclusions, types.ContextBundleExclusion{
				Path:   relativePath,
				Role:   classifyContextBundleRole(relativePath, request),
				Reason: "binary file omitted",
			})
			return nil
		}
		role := classifyContextBundleRole(relativePath, request)
		content := string(contentBytes)
		score, reason := scoreContextBundleFile(relativePath, content, role, terms, request.IncludePaths)
		if role == contextBundleRoleContract || role == contextBundleRoleIssueDoc || role == contextBundleRolePlan {
			item, itemErr := buildContextBundleCandidate(relativePath, role, contextBundleReason(relativePath, role, reason), 1000+score, content, contextBundleContractRunes, options.TokenCounter)
			if itemErr != nil {
				return itemErr
			}
			contracts = append(contracts, item)
			return nil
		}
		if score <= 0 {
			exclusions = appendContextBundleExclusion(exclusions, types.ContextBundleExclusion{
				Path:   relativePath,
				Role:   role,
				Reason: "no goal term match",
				Score:  score,
			})
			return nil
		}
		item, itemErr := buildContextBundleCandidate(relativePath, role, reason, score, content, contextBundleFileRunes, options.TokenCounter)
		if itemErr != nil {
			return itemErr
		}
		files = append(files, item)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sortContextBundleCandidates(contracts)
	sortContextBundleCandidates(files)

	var usedTokens int
	selectedContracts := make([]types.ContextBundleItem, 0, len(contracts))
	for _, candidate := range contracts {
		if candidate.tokens > maxTokens && len(selectedContracts) > 0 {
			exclusions = appendContextBundleExclusion(exclusions, contextBundleBudgetExclusion(candidate))
			continue
		}
		selectedContracts = append(selectedContracts, candidate.toItem())
		usedTokens += candidate.tokens
	}

	selectedFiles := make([]types.ContextBundleItem, 0, len(files))
	for _, candidate := range files {
		if usedTokens+candidate.tokens > maxTokens {
			exclusions = appendContextBundleExclusion(exclusions, contextBundleBudgetExclusion(candidate))
			continue
		}
		selectedFiles = append(selectedFiles, candidate.toItem())
		usedTokens += candidate.tokens
	}

	symbols := extractContextBundleSelectedSymbols(repositoryRoot, selectedFiles)
	repositoryName := filepath.Base(repositoryRoot)
	output := &types.ContextBundleOutput{
		Version: contextBundleVersion,
		Repository: types.ContextBundleRepository{
			Root: repositoryRoot,
			Name: repositoryName,
		},
		Goal: request.Goal,
		Budget: types.ContextBundleBudget{
			MaxTokens:  maxTokens,
			UsedTokens: usedTokens,
			Model:      tokenModel,
		},
		Terms:       terms,
		Contracts:   selectedContracts,
		Files:       selectedFiles,
		Symbols:     symbols,
		Exclusions:  exclusions,
		GeneratedBy: contextBundleGeneratedBy,
	}
	return output, nil
}

func normalizeContextBundleRequest(request types.ContextBundleRequest) types.ContextBundleRequest {
	request.RepositoryRoot = strings.TrimSpace(request.RepositoryRoot)
	request.IssueDocumentPath = filepath.ToSlash(strings.TrimSpace(request.IssueDocumentPath))
	request.PlanDocumentPath = filepath.ToSlash(strings.TrimSpace(request.PlanDocumentPath))
	request.ValidationTarget = strings.TrimSpace(request.ValidationTarget)
	request.Model = strings.TrimSpace(request.Model)
	request.Goal.ID = strings.TrimSpace(request.Goal.ID)
	request.Goal.Kind = strings.TrimSpace(request.Goal.Kind)
	request.Goal.Title = strings.TrimSpace(request.Goal.Title)
	request.Goal.Body = strings.TrimSpace(request.Goal.Body)
	request.Goal.Category = strings.TrimSpace(request.Goal.Category)
	request.IncludePaths = normalizeContextBundlePatterns(request.IncludePaths)
	request.ExcludePaths = normalizeContextBundlePatterns(request.ExcludePaths)
	return request
}

func normalizeContextBundlePatterns(patterns []string) []string {
	var normalized []string
	for _, pattern := range patterns {
		trimmed := filepath.ToSlash(strings.TrimSpace(pattern))
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func deriveContextBundleTerms(request types.ContextBundleRequest) []string {
	termCounts := map[string]int{}
	source := strings.Join([]string{
		request.Goal.ID,
		request.Goal.Kind,
		request.Goal.Category,
		request.Goal.Title,
		request.Goal.Body,
		request.ValidationTarget,
	}, " ")
	for _, match := range contextBundleWordPattern.FindAllString(source, -1) {
		term := strings.ToLower(match)
		term = strings.TrimFunc(term, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if len(term) < 3 {
			continue
		}
		if _, stopped := contextBundleStopWords[term]; stopped {
			continue
		}
		termCounts[term]++
	}
	terms := make([]string, 0, len(termCounts))
	for term := range termCounts {
		terms = append(terms, term)
	}
	sort.Slice(terms, func(leftIndex, rightIndex int) bool {
		left := terms[leftIndex]
		right := terms[rightIndex]
		if termCounts[left] == termCounts[right] {
			return left < right
		}
		return termCounts[left] > termCounts[right]
	})
	if len(terms) > 24 {
		terms = terms[:24]
	}
	return terms
}

func classifyContextBundleRole(relativePath string, request types.ContextBundleRequest) string {
	normalized := filepath.ToSlash(relativePath)
	base := filepath.Base(normalized)
	if normalized == request.IssueDocumentPath || normalized == ".mprlab/ISSUES.md" {
		return contextBundleRoleIssueDoc
	}
	if normalized == request.PlanDocumentPath || normalized == ".mprlab/PLAN.md" || normalized == ".mprlab/PLANNING.md" {
		return contextBundleRolePlan
	}
	switch {
	case base == "AGENTS.md",
		base == "ARCHITECTURE.md",
		base == "README.md",
		base == "PRD.md",
		normalized == ".mprlab/POLICY.md",
		strings.HasPrefix(normalized, ".mprlab/AGENTS."):
		return contextBundleRoleContract
	case isContextBundleTestPath(normalized):
		return contextBundleRoleTest
	case isContextBundleRuntimePath(normalized):
		return contextBundleRoleRuntime
	case strings.HasSuffix(normalized, ".md") || strings.HasPrefix(normalized, "docs/"):
		return contextBundleRoleDocs
	case isContextBundleConfigPath(normalized):
		return contextBundleRoleConfig
	case isContextBundleSourcePath(normalized):
		return contextBundleRoleImpl
	default:
		return contextBundleRoleAsset
	}
}

func isContextBundleTestPath(relativePath string) bool {
	base := filepath.Base(relativePath)
	return strings.Contains(relativePath, "/test/") ||
		strings.Contains(relativePath, "/tests/") ||
		strings.Contains(relativePath, "__tests__/") ||
		strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".spec.js") ||
		strings.HasSuffix(base, "_test.py")
}

func isContextBundleRuntimePath(relativePath string) bool {
	base := filepath.Base(relativePath)
	switch base {
	case "Makefile", "Dockerfile", "docker-compose.yml", "docker-compose.yaml", "go.mod", "go.sum", "package.json", "package-lock.json", "pyproject.toml", "requirements.txt":
		return true
	default:
		return strings.HasPrefix(relativePath, ".github/workflows/")
	}
}

func isContextBundleConfigPath(relativePath string) bool {
	extension := strings.ToLower(filepath.Ext(relativePath))
	switch extension {
	case ".json", ".toml", ".yaml", ".yml", ".ini", ".env":
		return true
	default:
		return false
	}
}

func isContextBundleSourcePath(relativePath string) bool {
	extension := strings.ToLower(filepath.Ext(relativePath))
	switch extension {
	case ".go", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx", ".py", ".css", ".html", ".sh":
		return true
	default:
		return false
	}
}

func scoreContextBundleFile(relativePath string, content string, role string, terms []string, includePatterns []string) (int, string) {
	lowerPath := strings.ToLower(relativePath)
	lowerContent := strings.ToLower(content)
	score := 0
	pathMatches := 0
	contentMatches := 0
	for _, term := range terms {
		if strings.Contains(lowerPath, term) {
			pathMatches++
			score += 30
		}
		count := strings.Count(lowerContent, term)
		if count > 0 {
			contentMatches += count
			if count > 8 {
				count = 8
			}
			score += count * 4
		}
	}
	switch role {
	case contextBundleRoleImpl:
		score += 12
	case contextBundleRoleTest:
		score += 10
	case contextBundleRoleRuntime:
		score += 6
	case contextBundleRoleDocs:
		score -= 8
	case contextBundleRoleConfig:
		score += 2
	}
	if matchesContextBundlePattern(relativePath, includePatterns) {
		score += 80
	}
	reasonParts := []string{}
	if pathMatches > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d goal term path matches", pathMatches))
	}
	if contentMatches > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d goal term content matches", contentMatches))
	}
	if len(reasonParts) == 0 {
		reasonParts = append(reasonParts, "role relevance")
	}
	return score, strings.Join(reasonParts, "; ")
}

func matchesContextBundlePattern(relativePath string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		if pattern == relativePath {
			return true
		}
		if matched, matchErr := filepath.Match(pattern, relativePath); matchErr == nil && matched {
			return true
		}
		if strings.HasSuffix(pattern, "/") && strings.HasPrefix(relativePath, pattern) {
			return true
		}
	}
	return false
}

func contextBundleReason(relativePath string, role string, scoredReason string) string {
	switch role {
	case contextBundleRoleContract:
		return contextBundleReasonContract
	case contextBundleRoleIssueDoc:
		return "active issue document"
	case contextBundleRolePlan:
		return "current planning document"
	default:
		if scoredReason != "" {
			return scoredReason
		}
		return "selected for context bundle"
	}
}

func buildContextBundleCandidate(relativePath string, role string, reason string, score int, content string, maxRunes int, counter tokenizer.Counter) (contextBundleCandidate, error) {
	excerpt, lineStart, lineEnd := contextBundleExcerpt(content, maxRunes)
	tokens, countErr := counter.CountString(excerpt)
	if countErr != nil {
		return contextBundleCandidate{}, fmt.Errorf("count tokens for %s: %w", relativePath, countErr)
	}
	hash := sha256.Sum256([]byte(content))
	return contextBundleCandidate{
		path:      relativePath,
		role:      role,
		reason:    reason,
		score:     score,
		content:   excerpt,
		lineStart: lineStart,
		lineEnd:   lineEnd,
		tokens:    tokens,
		hash:      hex.EncodeToString(hash[:]),
	}, nil
}

func contextBundleExcerpt(content string, maxRunes int) (string, int, int) {
	lineStart := 1
	runes := []rune(content)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return content, lineStart, countContextBundleLines(content)
	}
	excerpt := string(runes[:maxRunes])
	return excerpt, lineStart, countContextBundleLines(excerpt)
}

func countContextBundleLines(content string) int {
	if content == "" {
		return 1
	}
	lines := strings.Count(content, "\n") + 1
	if strings.HasSuffix(content, "\n") && lines > 1 {
		lines--
	}
	return lines
}

func sortContextBundleCandidates(candidates []contextBundleCandidate) {
	sort.SliceStable(candidates, func(leftIndex, rightIndex int) bool {
		left := candidates[leftIndex]
		right := candidates[rightIndex]
		if left.score == right.score {
			return left.path < right.path
		}
		return left.score > right.score
	})
}

func (candidate contextBundleCandidate) toItem() types.ContextBundleItem {
	return types.ContextBundleItem{
		Path:      candidate.path,
		Role:      candidate.role,
		Reason:    candidate.reason,
		Score:     candidate.score,
		Tokens:    candidate.tokens,
		SHA256:    candidate.hash,
		LineStart: candidate.lineStart,
		LineEnd:   candidate.lineEnd,
		Content:   candidate.content,
	}
}

func contextBundleBudgetExclusion(candidate contextBundleCandidate) types.ContextBundleExclusion {
	return types.ContextBundleExclusion{
		Path:   candidate.path,
		Role:   candidate.role,
		Reason: "token budget exceeded",
		Score:  candidate.score,
	}
}

func appendContextBundleExclusion(exclusions []types.ContextBundleExclusion, exclusion types.ContextBundleExclusion) []types.ContextBundleExclusion {
	const limit = 64
	if len(exclusions) >= limit {
		return exclusions
	}
	return append(exclusions, exclusion)
}

func extractContextBundleSelectedSymbols(root string, files []types.ContextBundleItem) []types.ContextBundleSymbol {
	var symbols []types.ContextBundleSymbol
	for _, file := range files {
		if file.Role != contextBundleRoleImpl && file.Role != contextBundleRoleTest {
			continue
		}
		contentBytes, readErr := os.ReadFile(filepath.Join(root, file.Path))
		if readErr != nil {
			continue
		}
		symbols = append(symbols, extractContextSymbols(root, file.Path, contentBytes)...)
	}
	sort.SliceStable(symbols, func(leftIndex, rightIndex int) bool {
		left := symbols[leftIndex]
		right := symbols[rightIndex]
		if left.Path == right.Path {
			if left.LineStart == right.LineStart {
				return left.QualifiedName < right.QualifiedName
			}
			return left.LineStart < right.LineStart
		}
		return left.Path < right.Path
	})
	return symbols
}
