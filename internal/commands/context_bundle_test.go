package commands_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyemirov/ctx/internal/commands"
	"github.com/tyemirov/ctx/internal/types"
)

const contextBundleTestModel = "stub-model"

func TestBuildContextBundleEnforcesCumulativeContractBudget(testingHandle *testing.T) {
	testCases := []struct {
		testName                  string
		maxTokens                 int
		expectedSelectedContracts int
	}{
		{
			testName:                  "selects only contracts within cumulative budget",
			maxTokens:                 70,
			expectedSelectedContracts: 1,
		},
		{
			testName:                  "admits additional contracts while budget remains",
			maxTokens:                 90,
			expectedSelectedContracts: 2,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testingHandle.Run(testCase.testName, func(subtestHandle *testing.T) {
			repositoryRoot := subtestHandle.TempDir()
			contractContent := strings.Repeat("x", 40)
			writeContextBundleTestFile(subtestHandle, repositoryRoot, "AGENTS.md", contractContent)
			writeContextBundleTestFile(subtestHandle, repositoryRoot, ".mprlab/POLICY.md", contractContent)
			writeContextBundleTestFile(subtestHandle, repositoryRoot, ".mprlab/ISSUES.md", contractContent)

			bundle := buildContextBundleForTest(subtestHandle, types.ContextBundleRequest{
				RepositoryRoot: repositoryRoot,
				MaxTokens:      testCase.maxTokens,
				Goal: types.ContextBundleGoal{
					ID:    "B001",
					Title: "Keep contract budget bounded",
				},
			})

			if bundle.Budget.UsedTokens > bundle.Budget.MaxTokens {
				subtestHandle.Fatalf("used tokens exceeded max tokens: got %d, max %d", bundle.Budget.UsedTokens, bundle.Budget.MaxTokens)
			}
			if len(bundle.Contracts) != testCase.expectedSelectedContracts {
				subtestHandle.Fatalf("expected %d selected contracts, got %d", testCase.expectedSelectedContracts, len(bundle.Contracts))
			}
			if !contextBundleHasExclusion(bundle.Exclusions, "token budget exceeded") {
				subtestHandle.Fatalf("expected token budget exclusion, got %+v", bundle.Exclusions)
			}
		})
	}
}

func TestBuildContextBundleOmitsSecretEnvironmentFiles(testingHandle *testing.T) {
	testCases := []struct {
		testName   string
		secretPath string
	}{
		{
			testName:   "dot env local",
			secretPath: ".env.local",
		},
		{
			testName:   "env production suffix",
			secretPath: "server.env.production",
		},
		{
			testName:   "secret key directory",
			secretPath: "secrets/provider.key",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		testingHandle.Run(testCase.testName, func(subtestHandle *testing.T) {
			repositoryRoot := subtestHandle.TempDir()
			secretMarker := "super-secret-api-key-context"
			writeContextBundleTestFile(subtestHandle, repositoryRoot, "AGENTS.md", "contract\n")
			writeContextBundleTestFile(subtestHandle, repositoryRoot, testCase.secretPath, secretMarker)
			writeContextBundleTestFile(subtestHandle, repositoryRoot, "src/config.js", "export const apiKeyConfig = 'api key config context';\n")

			bundle := buildContextBundleForTest(subtestHandle, types.ContextBundleRequest{
				RepositoryRoot: repositoryRoot,
				MaxTokens:      1000,
				IncludePaths:   []string{testCase.secretPath},
				Goal: types.ContextBundleGoal{
					ID:    "B002",
					Title: "Add api key config context",
				},
			})

			if contextBundleHasItem(bundle.Files, testCase.secretPath) || contextBundleHasItem(bundle.Contracts, testCase.secretPath) {
				subtestHandle.Fatalf("secret path %s was included in bundle", testCase.secretPath)
			}
			if contextBundleContainsContent(bundle.Files, secretMarker) || contextBundleContainsContent(bundle.Contracts, secretMarker) {
				subtestHandle.Fatalf("secret content marker leaked into bundle")
			}
			if !contextBundleHasItem(bundle.Files, "src/config.js") {
				subtestHandle.Fatalf("expected non-secret config file to remain eligible, got %+v", bundle.Files)
			}
		})
	}
}

func buildContextBundleForTest(testingHandle *testing.T, request types.ContextBundleRequest) types.ContextBundleOutput {
	testingHandle.Helper()

	bundle, buildError := commands.BuildContextBundle(context.Background(), commands.ContextBundleOptions{
		Request:      request,
		TokenCounter: stubCounter{},
		TokenModel:   contextBundleTestModel,
	})
	if buildError != nil {
		testingHandle.Fatalf("build context bundle: %v", buildError)
	}
	return *bundle
}

func writeContextBundleTestFile(testingHandle *testing.T, repositoryRoot string, relativePath string, content string) {
	testingHandle.Helper()

	filePath := filepath.Join(repositoryRoot, relativePath)
	if mkdirError := os.MkdirAll(filepath.Dir(filePath), 0o755); mkdirError != nil {
		testingHandle.Fatalf("create parent directory for %s: %v", relativePath, mkdirError)
	}
	if writeError := os.WriteFile(filePath, []byte(content), 0o600); writeError != nil {
		testingHandle.Fatalf("write %s: %v", relativePath, writeError)
	}
}

func contextBundleHasItem(items []types.ContextBundleItem, path string) bool {
	for _, item := range items {
		if item.Path == path {
			return true
		}
	}
	return false
}

func contextBundleContainsContent(items []types.ContextBundleItem, content string) bool {
	for _, item := range items {
		if strings.Contains(item.Content, content) {
			return true
		}
	}
	return false
}

func contextBundleHasExclusion(exclusions []types.ContextBundleExclusion, reason string) bool {
	for _, exclusion := range exclusions {
		if exclusion.Reason == reason {
			return true
		}
	}
	return false
}
