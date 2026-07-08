// Package tests contains the integration‑level test‑suite for ctx.
package tests

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tyemirov/ctx/internal/commands"
	"github.com/tyemirov/ctx/internal/output"
	"github.com/tyemirov/ctx/internal/tokenizer"
	appTypes "github.com/tyemirov/ctx/internal/types"
	"github.com/tyemirov/ctx/internal/utils"
)

const (
	visibleFileName    = "visible.txt"
	hiddenFileName     = "hidden.txt"
	visibleFileContent = "visible"
	hiddenFileContent  = "secret"
	includeGitFlag     = "--git"
	versionFlag        = "--version"
	// documentationFlag enables inclusion of documentation.
	documentationFlag = "--doc"
	treeAlias         = "t"
	// contentAlias represents the shorthand for the content command.
	contentAlias          = "c"
	subDirectoryName      = "sub"
	gitignoreFileName     = ".gitignore"
	nodeModulesDirName    = "node_modules"
	dependencyFileName    = "dependency.js"
	nodeModulesPattern    = nodeModulesDirName + "/\n"
	dependencyFileContent = "dependency"
	// ignoredFileName names a file that should be excluded by gitignore.
	ignoredFileName = "ignored.txt"
	// ignoredFileContent supplies the content for ignoredFileName.
	ignoredFileContent = "ignore"
	// googleSheetsAddonDirectoryName names the root directory of the Google Sheets add-on fixture.
	googleSheetsAddonDirectoryName = "google-sheets-addon"
	// claspConfigurationFileName is the configuration file name for the Apps Script CLI.
	claspConfigurationFileName = ".clasp.json"
	// claspConfigurationPattern instructs git to ignore the clasp configuration file.
	claspConfigurationPattern = claspConfigurationFileName + "\n"
	// claspConfigurationContent is placeholder JSON content for the clasp configuration file.
	claspConfigurationContent = "{\"scriptId\":\"1\"}"
	// googleSheetsAddonGitignoreContent combines ignore rules for the add-on fixture.
	googleSheetsAddonGitignoreContent = nodeModulesPattern + claspConfigurationPattern
	commandDirectoryRelativePath      = "."
	integrationBinaryBaseName         = "ctx_integration_binary"
	contentDataFunction               = "github.com/tyemirov/ctx/internal/commands.GetContentData"
	streamContentFunction             = "github.com/tyemirov/ctx/internal/commands.StreamContent"
	runStreamCommandFunction          = "github.com/tyemirov/ctx/internal/cli.runStreamCommand"
	runToolFunction                   = "github.com/tyemirov/ctx/internal/cli.runTool"
	callChainAlias                    = "cc"
	formatFlag                        = "--format"
	depthFlag                         = "--depth"
	depthTwoValue                     = "2"
	depthThreeValue                   = "3"

	usageSnippet = "Usage:\n  ctx"
	// unknownDocumentationFlagErrorSnippet captures the error when documentation flag is unsupported.
	unknownDocumentationFlagErrorSnippet = "unknown flag: --doc"

	binaryFixtureFileName  = "fixture.png"
	expectedBinaryMimeType = "image/png"
	ignoreFileName         = ".ignore"
	// binarySectionHeader identifies the section that lists binary content patterns in an ignore file.
	binarySectionHeader                = "[binary]"
	unmatchedBinaryFixtureName         = "unmatched.bin"
	onePixelPNGBase64Content           = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAASsJTYQAAAAASUVORK5CYII="
	expectedTextMimeType               = "text/plain; charset=utf-8"
	mimeTypeIndicator                  = "Mime Type:"
	toolsDirectoryName                 = "tools"
	githubDirectoryName                = ".github"
	yamlRootFileName                   = "config.yml"
	yamlPattern                        = "*.yml"
	nestedDirectoryName                = "nested"
	nestedYamlFileName                 = "nested.yml"
	nestedTextFileName                 = "keep.txt"
	docKindModule                      = "module"
	docKindClass                       = "class"
	docKindMethod                      = "method"
	docKindFunction                    = "function"
	pythonFixtureFileName              = "documentation.py"
	pythonModuleDocstring              = "Python module overview."
	pythonClassDocstring               = "Represents a greeter component."
	pythonMethodDocstring              = "Delivers a greeting message."
	pythonFunctionDocstring            = "Builds a standalone greeting."
	pythonDocumentationFixture         = "\"\"\"" + pythonModuleDocstring + "\"\"\"\n\nclass Greeter:\n    \"\"\"" + pythonClassDocstring + "\"\"\"\n\n    def greet(self):\n        \"\"\"" + pythonMethodDocstring + "\"\"\"\n        return \"hello\"\n\n\nasync def build_greeting():\n    \"\"\"" + pythonFunctionDocstring + "\"\"\"\n    return \"hello\"\n"
	javaScriptFixtureFileName          = "documentation.js"
	javaScriptClassDocstring           = "Controls widget lifecycle."
	javaScriptFunctionDocstring        = "Creates a widget instance."
	javaScriptDocumentationFixture     = "/**\n * " + javaScriptClassDocstring + "\n */\nexport class Widget {\n}\n\n/**\n * " + javaScriptFunctionDocstring + "\n */\nexport const createWidget = () => {};\n"
	pythonCallchainModuleDirectoryName = "app"
	pythonCallchainServiceFileName     = "service.py"
	pythonCallchainConsumerFileName    = "consumer.py"
	pythonCallchainServiceContent      = `"""Service module used in call chain tests."""

def core():
    return "core"

def helper():
    return core()

class Greeter:
    def reply(self):
        return helper()

    def greet(self):
        return self.reply()

def entry():
    greeter = Greeter()
    return Greeter.greet(greeter)
`
	pythonCallchainConsumerContent = `from app import service


def run():
    return service.entry()
`
	pythonCallchainTargetFunction          = "app.service.Greeter.reply"
	javaScriptCallchainModuleDirectoryName = "app"
	javaScriptCallchainServiceFileName     = "service.js"
	javaScriptCallchainConsumerFileName    = "consumer.js"
	javaScriptCallchainServiceContent      = `export function core() {
    return 'core';
}

export function helper() {
    return core();
}

export function entry() {
    return helper();
}
`
	javaScriptCallchainConsumerContent = `import { entry } from './app/service.js';

export function run() {
    return entry();
}
`
	javaScriptCallchainTargetFunction = "app.service.helper"
)

func decodeJSONRoots(t *testing.T, data string) []appTypes.TreeOutputNode {
	t.Helper()
	var single appTypes.TreeOutputNode
	if err := json.Unmarshal([]byte(data), &single); err == nil && single.Path != "" {
		return []appTypes.TreeOutputNode{single}
	}
	var multi []appTypes.TreeOutputNode
	if err := json.Unmarshal([]byte(data), &multi); err == nil {
		return multi
	}
	t.Fatalf("invalid JSON: %s", data)
	return nil
}

func decodeXMLRoots(t *testing.T, data string) []appTypes.TreeOutputNode {
	t.Helper()
	var single appTypes.TreeOutputNode
	if err := xml.Unmarshal([]byte(data), &single); err == nil && single.Path != "" {
		return []appTypes.TreeOutputNode{single}
	}
	var wrapper struct {
		Nodes []appTypes.TreeOutputNode `xml:"node"`
		Items []appTypes.TreeOutputNode `xml:"item"`
	}
	if err := xml.Unmarshal([]byte(data), &wrapper); err == nil {
		if len(wrapper.Nodes) > 0 {
			return wrapper.Nodes
		}
		if len(wrapper.Items) > 0 {
			return wrapper.Items
		}
	}
	t.Fatalf("invalid XML: %s", data)
	return nil
}

func flattenFileNodes(nodes []appTypes.TreeOutputNode) []appTypes.TreeOutputNode {
	var files []appTypes.TreeOutputNode
	var walk func(node appTypes.TreeOutputNode)
	walk = func(node appTypes.TreeOutputNode) {
		if node.Type == appTypes.NodeTypeFile || node.Type == appTypes.NodeTypeBinary {
			files = append(files, node)
		}
		for _, child := range node.Children {
			if child != nil {
				walk(*child)
			}
		}
	}
	for _, node := range nodes {
		walk(node)
	}
	return files
}

func withFormat(arguments []string, format string) []string {
	formatted := append([]string{}, arguments...)
	return append(formatted, formatFlag, format)
}

func withJSONFormat(arguments []string) []string {
	return withFormat(arguments, appTypes.FormatJSON)
}

func TestContentJSONStreamingMatchesExpectedOutput(t *testing.T) {
	binary := buildBinary(t)
	workingDir := setupTestDirectory(t, map[string]string{
		"sample.txt": "sample content",
	})

	command := exec.Command(binary, []string{appTypes.CommandContent, "--format", appTypes.FormatJSON, workingDir}...)
	command.Dir = workingDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		t.Fatalf("content command failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var collected []appTypes.FileOutput
	streamOptions := commands.ContentStreamOptions{Root: workingDir}
	if err := commands.StreamContent(streamOptions, func(file appTypes.FileOutput) error {
		collected = append(collected, file)
		return nil
	}); err != nil {
		t.Fatalf("StreamContent failed: %v", err)
	}

	tree, err := commands.BuildContentTree(workingDir, collected, true, "")
	if err != nil {
		t.Fatalf("BuildContentTree failed: %v", err)
	}

	expected, err := output.RenderJSON([]interface{}{tree})
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}

	expectedWithNewline := expected + "\n"
	if stdout.String() != expectedWithNewline {
		t.Fatalf("unexpected output\nexpected: %s\nactual: %s", expectedWithNewline, stdout.String())
	}
}

func TestTreeMatchesContentWithoutContentAcrossFormats(t *testing.T) {
	binary := buildBinary(t)
	workingDir := setupTestDirectory(t, map[string]string{
		"main.go":          "package main\nfunc main() {}\n",
		"nested/readme.md": "# Nested\n",
	})

	formats := []string{appTypes.FormatToon, appTypes.FormatRaw, appTypes.FormatJSON, appTypes.FormatXML}
	for _, format := range formats {
		format := format
		t.Run(format, func(t *testing.T) {
			treeArgs := []string{appTypes.CommandTree, "--format", format, workingDir}
			contentArgs := []string{appTypes.CommandContent, "--format", format, "--content=false", workingDir}

			treeOutput := runCommand(t, binary, treeArgs, workingDir)
			contentOutput := runCommand(t, binary, contentArgs, workingDir)

			if treeOutput != contentOutput {
				t.Fatalf("expected identical output for tree and content --content=false\nTree:\n%s\nContent:\n%s", treeOutput, contentOutput)
			}
		})
	}
}

func TestBundleBuildsGoalOrientedContext(t *testing.T) {
	binary := buildBinary(t)
	workingDir := setupTestDirectory(t, map[string]string{
		"AGENTS.md":         "Read .mprlab/POLICY.md before changing code.\n",
		".mprlab/POLICY.md": "Forward-only validation through integration tests.\n",
		".mprlab/ISSUES.md": "- [ ] Add paste dropdown context for the new button.\n",
		".mprlab/PLAN.md":   "- [ ] Build a context bundle before execution.\n",
		"static/app/index.html": `<button id="new-issue">New</button>
<button id="paste-issue">Paste from clipboard</button>
`,
		"static/js/app.js": `export function renderPasteDropdown() {
  return "paste dropdown context builder for new issue";
}

export function preserveNewIssueAction() {
  return renderPasteDropdown();
}
`,
		"static/js/__tests__/app.test.js": `import { renderPasteDropdown } from "../app.js";

test("paste dropdown remains attached to the new issue button", () => {
  expect(renderPasteDropdown()).toContain("paste dropdown");
});
`,
		"docs/paste-dropdown-notes.md": "Paste dropdown notes are supporting documentation, not the primary implementation surface.\n",
		"CHANGELOG.md":                 "Historical paste dropdown release note.\n",
	})
	request := appTypes.ContextBundleRequest{
		RepositoryRoot:    workingDir,
		IssueDocumentPath: ".mprlab/ISSUES.md",
		PlanDocumentPath:  ".mprlab/PLAN.md",
		MaxTokens:         12000,
		Model:             "gpt-4o",
		Goal: appTypes.ContextBundleGoal{
			ID:       "B019",
			Kind:     "BugFix",
			Title:    "Add Paste as a dropdown option on the New button",
			Body:     "The execution agent needs focused frontend context for the paste dropdown and its rendered integration tests.",
			Category: "BugFixes",
		},
	}
	requestBytes, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		t.Fatalf("marshal request: %v", marshalErr)
	}
	requestPath := filepath.Join(workingDir, "context-request.json")
	if writeErr := os.WriteFile(requestPath, requestBytes, 0o644); writeErr != nil {
		t.Fatalf("write request: %v", writeErr)
	}

	jsonOutput := runCommand(t, binary, []string{appTypes.CommandBundle, "--request", requestPath, "--format", appTypes.FormatJSON}, workingDir)
	var bundle appTypes.ContextBundleOutput
	if decodeErr := json.Unmarshal([]byte(jsonOutput), &bundle); decodeErr != nil {
		t.Fatalf("decode bundle json: %v\n%s", decodeErr, jsonOutput)
	}
	if bundle.Version != 1 {
		t.Fatalf("expected bundle version 1, got %d", bundle.Version)
	}
	for _, expectedContract := range []string{"AGENTS.md", ".mprlab/POLICY.md", ".mprlab/ISSUES.md", ".mprlab/PLAN.md"} {
		if !bundleHasItem(bundle.Contracts, expectedContract) {
			t.Fatalf("expected contract %s in bundle contracts: %+v", expectedContract, bundle.Contracts)
		}
	}
	if !bundleHasItem(bundle.Files, "static/js/app.js") {
		t.Fatalf("expected implementation file in bundle files: %+v", bundle.Files)
	}
	if !bundleHasItem(bundle.Files, "static/js/__tests__/app.test.js") {
		t.Fatalf("expected integration test file in bundle files: %+v", bundle.Files)
	}
	if !bundleHasSymbol(bundle.Symbols, "static/js/app.js", "renderPasteDropdown") {
		t.Fatalf("expected renderPasteDropdown symbol in bundle symbols: %+v", bundle.Symbols)
	}

	toonOutput := runCommand(t, binary, []string{appTypes.CommandBundle, "--request", requestPath, "--format", appTypes.FormatToon}, workingDir)
	for _, snippet := range []string{"contextBundle:", "contracts[", "files[", "symbols["} {
		if !strings.Contains(toonOutput, snippet) {
			t.Fatalf("expected TOON output to contain %q\n%s", snippet, toonOutput)
		}
	}
}

func decodeJSONFiles(t *testing.T, data string) []appTypes.TreeOutputNode {
	return flattenFileNodes(decodeJSONRoots(t, data))
}

func bundleHasItem(items []appTypes.ContextBundleItem, path string) bool {
	for _, item := range items {
		if item.Path == path {
			return true
		}
	}
	return false
}

func bundleHasSymbol(symbols []appTypes.ContextBundleSymbol, path string, name string) bool {
	for _, symbol := range symbols {
		if symbol.Path == path && symbol.Name == name {
			return true
		}
	}
	return false
}

func decodeXMLFiles(t *testing.T, data string) []appTypes.TreeOutputNode {
	return flattenFileNodes(decodeXMLRoots(t, data))
}

func findNodeByName(nodes []appTypes.TreeOutputNode, name string) *appTypes.TreeOutputNode {
	for index := range nodes {
		if nodes[index].Name == name {
			return &nodes[index]
		}
		if child := findNodeByName(childrenToSlice(nodes[index].Children), name); child != nil {
			return child
		}
	}
	return nil
}

func childrenToSlice(children []*appTypes.TreeOutputNode) []appTypes.TreeOutputNode {
	result := make([]appTypes.TreeOutputNode, 0, len(children))
	for _, child := range children {
		if child != nil {
			result = append(result, *child)
		}
	}
	return result
}

// buildBinary compiles the ctx binary and returns its path.
func buildBinary(testingHandle *testing.T) string {
	testingHandle.Helper()

	temporaryDirectory := testingHandle.TempDir()
	binaryName := integrationBinaryBaseName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(temporaryDirectory, binaryName)

	moduleRootDirectory := getModuleRoot(testingHandle)
	commandDirectory := filepath.Join(moduleRootDirectory, commandDirectoryRelativePath)
	buildCommand := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCommand.Dir = commandDirectory

	combinedOutput, buildError := buildCommand.CombinedOutput()
	if buildError != nil {
		testingHandle.Fatalf("build failed in %s: %v\n%s", commandDirectory, buildError, string(combinedOutput))
	}

	return binaryPath
}

// runCommand executes the binary with arguments and returns stdout.
func runCommand(testingHandle *testing.T, binaryPath string, arguments []string, workingDirectory string) string {
	testingHandle.Helper()

	command := exec.Command(binaryPath, arguments...)
	command.Dir = workingDirectory

	var stdoutBuffer, stderrBuffer bytes.Buffer
	command.Stdout = &stdoutBuffer
	command.Stderr = &stderrBuffer

	runError := command.Run()

	stdout := stdoutBuffer.String()
	stderr := stderrBuffer.String()

	if runError != nil {
		if exitError, ok := runError.(*exec.ExitError); ok {
			testingHandle.Fatalf("command failed (%d): %v\nstdout:\n%s\nstderr:\n%s",
				exitError.ExitCode(), runError, stdout, stderr)
		}
		testingHandle.Fatalf("command failed: %v\nstdout:\n%s\nstderr:\n%s", runError, stdout, stderr)
	}

	if strings.Contains(stderr, "Warning:") {
		testingHandle.Logf("command produced warnings:\n%s", stderr)
	}

	return stdout
}

// runCommandExpectError runs the binary expecting a failure and returns combined output.
func runCommandExpectError(testingHandle *testing.T, binaryPath string, arguments []string, workingDirectory string) string {
	testingHandle.Helper()

	command := exec.Command(binaryPath, arguments...)
	command.Dir = workingDirectory

	var buffer bytes.Buffer
	command.Stdout = &buffer
	command.Stderr = &buffer

	runError := command.Run()
	output := buffer.String()

	if runError == nil {
		testingHandle.Fatalf("command succeeded unexpectedly\noutput:\n%s", output)
	}

	return output
}

// runCommandWithWarnings runs the binary and returns stdout while allowing warnings.
func runCommandWithWarnings(testingHandle *testing.T, binaryPath string, arguments []string, workingDirectory string) string {
	testingHandle.Helper()

	command := exec.Command(binaryPath, arguments...)
	command.Dir = workingDirectory

	var stdoutBuffer, stderrBuffer bytes.Buffer
	command.Stdout = &stdoutBuffer
	command.Stderr = &stderrBuffer

	runError := command.Run()

	stdout := stdoutBuffer.String()
	stderr := stderrBuffer.String()

	if runError != nil {
		var exitError *exec.ExitError
		if errors.As(runError, &exitError) {
			testingHandle.Fatalf("command failed when warnings were expected (%d): %v\nstderr:\n%s",
				exitError.ExitCode(), runError, stderr)
		}
		testingHandle.Fatalf("command failed when warnings were expected: %v\nstderr:\n%s", runError, stderr)
	}

	if !strings.Contains(stderr, "Warning:") {
		testingHandle.Fatalf("expected warnings on stderr\nstderr:\n%s", stderr)
	}

	return stdout
}

// setupTestDirectory creates a temporary directory populated with the provided layout.
func setupTestDirectory(testingHandle *testing.T, layout map[string]string) string {
	testingHandle.Helper()

	root := testingHandle.TempDir()

	for relativePath, content := range layout {
		absolutePath := filepath.Join(root, relativePath)

		if strings.HasSuffix(relativePath, "/") || content == "" {
			_ = os.MkdirAll(absolutePath, 0o755)
			continue
		}

		parent := filepath.Dir(absolutePath)
		_ = os.MkdirAll(parent, 0o755)

		if content == "<UNREADABLE>" {
			_ = os.WriteFile(absolutePath, []byte("unreadable placeholder"), 0o644)
			_ = os.Chmod(absolutePath, 0o000)
			continue
		}

		_ = os.WriteFile(absolutePath, []byte(content), 0o644)
	}

	return root
}

// setupGoogleSheetsAddonFixture creates a directory tree resembling a minimal Google Sheets add-on.
// The returned path points to the add-on directory within the temporary workspace.
func setupGoogleSheetsAddonFixture(testingHandle *testing.T) string {
	layout := map[string]string{
		filepath.Join(googleSheetsAddonDirectoryName, gitignoreFileName):                      googleSheetsAddonGitignoreContent,
		filepath.Join(googleSheetsAddonDirectoryName, nodeModulesDirName, dependencyFileName): dependencyFileContent,
		filepath.Join(googleSheetsAddonDirectoryName, claspConfigurationFileName):             claspConfigurationContent,
		filepath.Join(googleSheetsAddonDirectoryName, visibleFileName):                        visibleFileContent,
	}
	root := setupTestDirectory(testingHandle, layout)
	return filepath.Join(root, googleSheetsAddonDirectoryName)
}

// setupExclusionPatternFixture creates a directory tree for exclusion flag tests.
func setupExclusionPatternFixture(testingHandle *testing.T) string {
	layout := map[string]string{
		toolsDirectoryName + "/":  "",
		githubDirectoryName + "/": "",
		yamlRootFileName:          visibleFileContent,
		visibleFileName:           visibleFileContent,
		filepath.Join(nestedDirectoryName, nestedYamlFileName): visibleFileContent,
		filepath.Join(nestedDirectoryName, nestedTextFileName): visibleFileContent,
	}
	return setupTestDirectory(testingHandle, layout)
}

// getModuleRoot returns the repository root directory.
func getModuleRoot(testingHandle *testing.T) string {
	testingHandle.Helper()

	directory, err := os.Getwd()
	if err != nil {
		testingHandle.Fatalf("failed to determine working directory: %v", err)
	}

	for {
		goMod := filepath.Join(directory, "go.mod")
		if _, statErr := os.Stat(goMod); statErr == nil {
			return directory
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			testingHandle.Fatalf("could not locate go.mod from %s", directory)
		}
		directory = parent
	}
}

// TestCTX verifies the ctx CLI across diverse scenarios.
func TestCTX(testingHandle *testing.T) {
	binary := buildBinary(testingHandle)

	var explicitFilePath string
	var defaultTreeRoot string
	var defaultContentRoot string

	testCases := []struct {
		name            string
		arguments       []string
		prepare         func(*testing.T) string
		expectError     bool
		expectWarning   bool
		requiresHelpers bool
		validate        func(*testing.T, string)
	}{
		{
			name:      "NoArgumentsDisplaysHelp",
			arguments: nil,
			prepare:   func(testingHandle *testing.T) string { return setupTestDirectory(testingHandle, nil) },
			validate: func(testingHandle *testing.T, output string) {
				if !strings.Contains(output, usageSnippet) {
					testingHandle.Fatalf("expected help output containing %q\n%s", usageSnippet, output)
				}
			},
		},
		{
			name: "DocFlagCallChainRaw",
			arguments: []string{
				"callchain",
				streamContentFunction,
				documentationFlag,
				"--format",
				"raw",
			},
			prepare: func(t *testing.T) string { return getModuleRoot(t) },
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "package fmt") {
					t.Logf("callchain output:\n%s", output)
					t.Errorf("expected documentation output to include package fmt")
				}
			},
		},
		{
			name: "DefaultFormatTreeCommandToon",
			arguments: []string{
				appTypes.CommandTree,
				"fileA.txt",
				"directoryB",
			},
			prepare: func(t *testing.T) string {
				defaultTreeRoot = setupTestDirectory(t, map[string]string{
					"fileA.txt":             "A",
					"directoryB/":           "",
					"directoryB/itemB1.txt": "B1",
				})
				return defaultTreeRoot
			},
			validate: func(t *testing.T, output string) {
				trimmed := strings.TrimSpace(output)
				if !strings.HasPrefix(trimmed, "roots[2]:") {
					t.Fatalf("expected TOON array header, got:\n%s", output)
				}
				filePath := filepath.Join(defaultTreeRoot, "fileA.txt")
				if !strings.Contains(output, "- path: "+filePath) {
					t.Fatalf("expected file node path %s in TOON output\n%s", filePath, output)
				}
				if !strings.Contains(output, "mimeType: "+expectedTextMimeType) && !strings.Contains(output, `mimeType: "`+expectedTextMimeType+`"`) {
					t.Fatalf("expected mimeType %s in TOON output\n%s", expectedTextMimeType, output)
				}
				dirPath := filepath.Join(defaultTreeRoot, "directoryB")
				if !strings.Contains(output, "- path: "+dirPath) {
					t.Fatalf("expected directory node path %s in TOON output\n%s", dirPath, output)
				}
				if !strings.Contains(output, "children[1]:") {
					t.Fatalf("expected children block for directory node\n%s", output)
				}
			},
		},
		{
			name: "DocumentationFlagTreeUnsupported",
			arguments: []string{
				appTypes.CommandTree,
				documentationFlag,
			},
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, nil)
			},
			expectError: true,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, unknownDocumentationFlagErrorSnippet) {
					t.Errorf("expected error for unsupported documentation flag\n%s", output)
				}
			},
		},
		{
			name: "DefaultFormatContentCommandToon",
			arguments: []string{
				appTypes.CommandContent,
				"fileA.txt",
				"directoryB",
			},
			prepare: func(t *testing.T) string {
				defaultContentRoot = setupTestDirectory(t, map[string]string{
					"fileA.txt":             "Content A",
					"directoryB/":           "",
					"directoryB/itemB1.txt": "Content B1",
				})
				return defaultContentRoot
			},
			validate: func(t *testing.T, output string) {
				trimmed := strings.TrimSpace(output)
				if !strings.HasPrefix(trimmed, "roots[2]:") {
					t.Fatalf("expected TOON array header, got:\n%s", output)
				}
				filePath := filepath.Join(defaultContentRoot, "fileA.txt")
				if !strings.Contains(output, "- path: "+filePath) {
					t.Fatalf("expected file node path %s in TOON output\n%s", filePath, output)
				}
				if !strings.Contains(output, "content: \"Content A\"") {
					t.Fatalf("expected inline file content in TOON output\n%s", output)
				}
				itemPath := filepath.Join(defaultContentRoot, "directoryB", "itemB1.txt")
				if !strings.Contains(output, "- path: "+itemPath) {
					t.Fatalf("expected nested file node path %s in TOON output\n%s", itemPath, output)
				}
				if !strings.Contains(output, "mimeType: "+expectedTextMimeType) && !strings.Contains(output, `mimeType: "`+expectedTextMimeType+`"`) {
					t.Fatalf("expected mimeType %s in TOON output\n%s", expectedTextMimeType, output)
				}
			},
		},
		{
			name: "PythonDocumentationExtraction",
			arguments: []string{
				appTypes.CommandContent,
				documentationFlag,
				"--format",
				appTypes.FormatJSON,
				".",
			},
			prepare: func(testingHandle *testing.T) string {
				return setupTestDirectory(testingHandle, map[string]string{
					pythonFixtureFileName: pythonDocumentationFixture,
				})
			},
			validate: func(testingHandle *testing.T, output string) {
				files := decodeJSONFiles(testingHandle, output)
				if len(files) != 1 {
					testingHandle.Fatalf("expected one file node, got %d", len(files))
				}
				documentationByName := map[string]appTypes.DocumentationEntry{}
				for _, entry := range files[0].Documentation {
					documentationByName[entry.Name] = entry
				}
				expected := map[string]struct {
					text string
					kind string
				}{
					"documentation":                {text: pythonModuleDocstring, kind: docKindModule},
					"documentation.Greeter":        {text: pythonClassDocstring, kind: docKindClass},
					"documentation.Greeter.greet":  {text: pythonMethodDocstring, kind: docKindMethod},
					"documentation.build_greeting": {text: pythonFunctionDocstring, kind: docKindFunction},
				}
				for name, expectation := range expected {
					entry, ok := documentationByName[name]
					if !ok {
						testingHandle.Fatalf("missing documentation entry %s", name)
					}
					if entry.Doc != expectation.text {
						testingHandle.Fatalf("unexpected documentation for %s: %q", name, entry.Doc)
					}
					if entry.Kind != expectation.kind {
						testingHandle.Fatalf("unexpected kind for %s: %s", name, entry.Kind)
					}
				}
			},
		},
		{
			name: "JavaScriptDocumentationExtraction",
			arguments: []string{
				appTypes.CommandContent,
				documentationFlag,
				"--format",
				appTypes.FormatJSON,
				".",
			},
			prepare: func(testingHandle *testing.T) string {
				return setupTestDirectory(testingHandle, map[string]string{
					javaScriptFixtureFileName: javaScriptDocumentationFixture,
				})
			},
			validate: func(testingHandle *testing.T, output string) {
				files := decodeJSONFiles(testingHandle, output)
				if len(files) != 1 {
					testingHandle.Fatalf("expected one file node, got %d", len(files))
				}
				documentationByName := map[string]appTypes.DocumentationEntry{}
				for _, entry := range files[0].Documentation {
					documentationByName[entry.Name] = entry
				}
				expected := map[string]struct {
					text string
					kind string
				}{
					"documentation.Widget":       {text: javaScriptClassDocstring, kind: docKindClass},
					"documentation.createWidget": {text: javaScriptFunctionDocstring, kind: docKindFunction},
				}
				for name, expectation := range expected {
					entry, ok := documentationByName[name]
					if !ok {
						testingHandle.Fatalf("missing documentation entry %s", name)
					}
					if entry.Doc != expectation.text {
						testingHandle.Fatalf("unexpected documentation for %s: %q", name, entry.Doc)
					}
					if entry.Kind != expectation.kind {
						testingHandle.Fatalf("unexpected kind for %s: %s", name, entry.Kind)
					}
				}
			},
		},
		{
			name: "TreeXML",
			arguments: []string{
				appTypes.CommandTree,
				"fileA.txt",
				"--format",
				appTypes.FormatXML,
			},
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"fileA.txt": "A",
				})
			},
			validate: func(t *testing.T, output string) {
				roots := decodeXMLRoots(t, output)
				if len(roots) != 1 {
					t.Fatalf("expected one top-level node, got %d", len(roots))
				}
				if roots[0].MimeType != expectedTextMimeType {
					t.Fatalf("expected MIME type %s", expectedTextMimeType)
				}
				info, err := os.Stat(roots[0].Path)
				if err != nil {
					t.Fatalf("stat failed for %s: %v", roots[0].Path, err)
				}
				expectedSize := utils.FormatFileSize(info.Size())
				if roots[0].Size != expectedSize {
					t.Fatalf("expected size %s, got %s", expectedSize, roots[0].Size)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if roots[0].LastModified != expectedTimestamp {
					t.Fatalf("expected last modified %s, got %s", expectedTimestamp, roots[0].LastModified)
				}
			},
		},
		{
			name: "TreeTokensJSON",
			arguments: withJSONFormat([]string{
				appTypes.CommandTree,
				"--tokens",
				"--model",
				"gpt-4o",
				"--summary",
			}),
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"token.txt": "token counting sample",
				})
			},
			validate: func(t *testing.T, output string) {
				roots := decodeJSONRoots(t, output)
				if len(roots) != 1 {
					t.Fatalf("expected one root, got %d", len(roots))
				}
				root := roots[0]
				fileNode := findNodeByName(roots, "token.txt")
				if fileNode == nil {
					t.Fatalf("token file node not found")
				}
				contentBytes, err := os.ReadFile(fileNode.Path)
				if err != nil {
					t.Fatalf("failed to read token file: %v", err)
				}
				counter, resolvedModel, err := tokenizer.NewCounter(tokenizer.Config{Model: "gpt-4o"})
				if err != nil {
					t.Fatalf("NewCounter error: %v", err)
				}
				if resolvedModel != "gpt-4o" {
					t.Fatalf("expected resolved model gpt-4o, got %q", resolvedModel)
				}
				countResult, err := tokenizer.CountBytes(counter, contentBytes)
				if err != nil {
					t.Fatalf("CountBytes error: %v", err)
				}
				if !countResult.Counted {
					t.Fatalf("expected text content to be counted")
				}
				if fileNode.Tokens != countResult.Tokens {
					t.Fatalf("expected file tokens %d, got %d", countResult.Tokens, fileNode.Tokens)
				}
				if root.TotalTokens != countResult.Tokens {
					t.Fatalf("expected root tokens %d, got %d", countResult.Tokens, root.TotalTokens)
				}
				if fileNode.Model != "gpt-4o" {
					t.Fatalf("expected file model gpt-4o, got %q", fileNode.Model)
				}
				if root.Model != "gpt-4o" {
					t.Fatalf("expected root model gpt-4o, got %q", root.Model)
				}
				rootJSON, err := json.Marshal(root)
				if err != nil {
					t.Fatalf("marshal root: %v", err)
				}
				var jsonRoot map[string]interface{}
				if err := json.Unmarshal(rootJSON, &jsonRoot); err != nil {
					t.Fatalf("failed to decode marshaled root: %v", err)
				}
				childrenValue, ok := jsonRoot["children"].([]interface{})
				if !ok || len(childrenValue) == 0 {
					t.Fatalf("missing children in JSON output: %v", jsonRoot)
				}
				fileEntry, ok := childrenValue[0].(map[string]interface{})
				if !ok {
					t.Fatalf("unexpected child payload type: %T", childrenValue[0])
				}
				if _, hasTotalFiles := fileEntry["totalFiles"]; hasTotalFiles {
					t.Fatalf("file entry unexpectedly contains totalFiles: %v", fileEntry)
				}
				if _, hasTotalSize := fileEntry["totalSize"]; hasTotalSize {
					t.Fatalf("file entry unexpectedly contains totalSize: %v", fileEntry)
				}
				if modelValue, ok := fileEntry["model"].(string); !ok || modelValue != "gpt-4o" {
					t.Fatalf("expected child model gpt-4o, got %v", fileEntry["model"])
				}
				if rootModel, ok := jsonRoot["model"].(string); !ok || rootModel != "gpt-4o" {
					t.Fatalf("expected root model gpt-4o, got %v", jsonRoot["model"])
				}
			},
		},
		{
			name: "ContentTokensJSON",
			arguments: withJSONFormat([]string{
				appTypes.CommandContent,
				"--tokens",
				"--model",
				"gpt-4o",
				"--summary",
			}),
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"main.go": "package main\n// hello",
				})
			},
			validate: func(t *testing.T, output string) {
				roots := decodeJSONRoots(t, output)
				if len(roots) != 1 {
					t.Fatalf("expected one root, got %d", len(roots))
				}
				root := roots[0]
				fileNode := findNodeByName(roots, "main.go")
				if fileNode == nil {
					t.Fatalf("content file node not found")
				}
				if fileNode.Tokens <= 0 {
					t.Fatalf("expected positive token count, got %d", fileNode.Tokens)
				}
				if root.TotalTokens != fileNode.Tokens {
					t.Fatalf("expected root tokens %d, got %d", fileNode.Tokens, root.TotalTokens)
				}
				if fileNode.Model != "gpt-4o" {
					t.Fatalf("expected file model gpt-4o, got %q", fileNode.Model)
				}
				if root.Model != "gpt-4o" {
					t.Fatalf("expected root model gpt-4o, got %q", root.Model)
				}
				rootJSON, err := json.Marshal(root)
				if err != nil {
					t.Fatalf("marshal root: %v", err)
				}
				var jsonRoot map[string]interface{}
				if err := json.Unmarshal(rootJSON, &jsonRoot); err != nil {
					t.Fatalf("failed to decode marshaled root: %v", err)
				}
				childrenValue, ok := jsonRoot["children"].([]interface{})
				if !ok || len(childrenValue) == 0 {
					t.Fatalf("missing children in JSON output: %v", jsonRoot)
				}
				fileEntry, ok := childrenValue[0].(map[string]interface{})
				if !ok {
					t.Fatalf("unexpected child payload type: %T", childrenValue[0])
				}
				if _, hasTotalFiles := fileEntry["totalFiles"]; hasTotalFiles {
					t.Fatalf("file entry unexpectedly contains totalFiles: %v", fileEntry)
				}
				if _, hasTotalSize := fileEntry["totalSize"]; hasTotalSize {
					t.Fatalf("file entry unexpectedly contains totalSize: %v", fileEntry)
				}
				if modelValue, ok := fileEntry["model"].(string); !ok || modelValue != "gpt-4o" {
					t.Fatalf("expected child model gpt-4o, got %v", fileEntry["model"])
				}
				if rootModel, ok := jsonRoot["model"].(string); !ok || rootModel != "gpt-4o" {
					t.Fatalf("expected root model gpt-4o, got %v", jsonRoot["model"])
				}
			},
		},
		{
			name: "ContentTokensUVLlama",
			arguments: withJSONFormat([]string{
				appTypes.CommandContent,
				"--tokens",
				"--model",
				"llama-3.1-8b",
			}),
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"sample.txt": "llama helper integration",
				})
			},
			requiresHelpers: true,
			validate: func(t *testing.T, output string) {
				roots := decodeJSONRoots(t, output)
				if len(roots) == 0 {
					t.Fatalf("expected at least one root, got zero")
				}
				fileNode := findNodeByName(roots, "sample.txt")
				if fileNode == nil {
					t.Fatalf("llama sample file not found")
				}
				if fileNode.Tokens <= 0 {
					t.Fatalf("expected positive token count, got %d", fileNode.Tokens)
				}
				if fileNode.Model != "llama-3.1-8b" {
					t.Fatalf("expected file model llama-3.1-8b, got %q", fileNode.Model)
				}
			},
		},
		{
			name: "ContentXML",
			arguments: []string{
				appTypes.CommandContent,
				"fileA.txt",
				"--format",
				appTypes.FormatXML,
			},
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"fileA.txt": "Content A",
				})
			},
			validate: func(t *testing.T, output string) {
				roots := decodeXMLRoots(t, output)
				files := flattenFileNodes(roots)
				if len(files) != 1 {
					t.Fatalf("expected one item, got %d", len(files))
				}
				if files[0].MimeType != expectedTextMimeType {
					t.Fatalf("expected MIME type %s", expectedTextMimeType)
				}
				info, err := os.Stat(files[0].Path)
				if err != nil {
					t.Fatalf("stat failed for %s: %v", files[0].Path, err)
				}
				expectedSize := utils.FormatFileSize(info.Size())
				if files[0].Size != expectedSize {
					t.Fatalf("expected size %s, got %s", expectedSize, files[0].Size)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if files[0].LastModified != expectedTimestamp {
					t.Fatalf("expected last modified %s, got %s", expectedTimestamp, files[0].LastModified)
				}
			},
		},
		{
			name: "RawFormatExplicitFlag",
			arguments: []string{
				appTypes.CommandContent,
				"onlyfile.txt",
				"--format",
				appTypes.FormatRaw,
			},
			prepare: func(t *testing.T) string {
				dir := setupTestDirectory(t, map[string]string{
					"onlyfile.txt": "Explicit raw content",
				})
				explicitFilePath = filepath.Join(dir, "onlyfile.txt")
				return dir
			},
			validate: func(t *testing.T, output string) {
				if !(strings.Contains(output, "File: "+explicitFilePath) &&
					strings.Contains(output, "Explicit raw content") &&
					strings.Contains(output, "End of file: "+explicitFilePath)) {
					t.Fatalf("unexpected raw content output\n%s", output)
				}
				if strings.Contains(output, mimeTypeIndicator) {
					t.Fatalf("unexpected MIME type in raw output\n%s", output)
				}
			},
		},
		{
			name: "RawFormatExplicitTree",
			arguments: []string{
				appTypes.CommandTree,
				"onlyfile.txt",
				"--format",
				appTypes.FormatRaw,
			},
			prepare: func(t *testing.T) string {
				dir := setupTestDirectory(t, map[string]string{
					"onlyfile.txt": "Explicit raw content",
				})
				explicitFilePath = filepath.Join(dir, "onlyfile.txt")
				return dir
			},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "[File] "+explicitFilePath) {
					t.Fatalf("unexpected raw tree output\n%s", output)
				}
				if strings.Contains(output, mimeTypeIndicator) {
					t.Fatalf("unexpected MIME type in raw output\n%s", output)
				}
			},
		},
		{
			name: "TreeRawSummaryDefault",
			arguments: []string{
				appTypes.CommandTree,
				"--format",
				appTypes.FormatRaw,
			},
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"summary.txt": "hello",
				})
			},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "Summary: 1 file") {
					t.Fatalf("expected default summary in output\n%s", output)
				}
			},
		},
		{
			name: "TreeRawSummaryDisabled",
			arguments: []string{
				appTypes.CommandTree,
				"--format",
				appTypes.FormatRaw,
				"--summary=false",
			},
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"summary.txt": "hello",
				})
			},
			validate: func(t *testing.T, output string) {
				if strings.Contains(output, "Summary:") {
					t.Fatalf("unexpected summary when disabled\n%s", output)
				}
			},
		},
		{
			name: "ContentRawSummaryDefault",
			arguments: []string{
				appTypes.CommandContent,
				"--format",
				appTypes.FormatRaw,
			},
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"summary.txt": "hello",
				})
			},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "Summary:") {
					t.Fatalf("expected default summary in content output\n%s", output)
				}
			},
		},
		{
			name: "ContentRawSummaryDisabled",
			arguments: []string{
				appTypes.CommandContent,
				"--format",
				appTypes.FormatRaw,
				"--summary=false",
			},
			prepare: func(t *testing.T) string {
				return setupTestDirectory(t, map[string]string{
					"summary.txt": "hello",
				})
			},
			validate: func(t *testing.T, output string) {
				if strings.Contains(output, "Summary:") {
					t.Fatalf("unexpected summary when disabled for content\n%s", output)
				}
			},
		},
		{
			name: "CallChainRaw",
			arguments: []string{
				appTypes.CommandCallChain,
				streamContentFunction,
				"--format",
				appTypes.FormatRaw,
			},
			prepare: func(t *testing.T) string { return getModuleRoot(t) },
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "Target Function: "+streamContentFunction) {
					t.Fatalf("missing target function in output")
				}
			},
		},
		{
			name: "CallChainJSON",
			arguments: []string{
				appTypes.CommandCallChain,
				streamContentFunction,
				"--format",
				appTypes.FormatJSON,
			},
			prepare: func(t *testing.T) string { return getModuleRoot(t) },
			validate: func(t *testing.T, output string) {
				var list []appTypes.CallChainOutput
				if err := json.Unmarshal([]byte(output), &list); err != nil {
					t.Fatalf("invalid JSON: %v\n%s", err, output)
				}
				if len(list) == 0 {
					t.Fatalf("expected at least one element, got zero")
				}
				chain := list[0]
				if chain.TargetFunction != streamContentFunction {
					t.Fatalf("unexpected target function %q", chain.TargetFunction)
				}
			},
		},
		{
			name: "CallChainXML",
			arguments: []string{
				appTypes.CommandCallChain,
				streamContentFunction,
				"--format",
				appTypes.FormatXML,
			},
			prepare: func(t *testing.T) string { return getModuleRoot(t) },
			validate: func(t *testing.T, output string) {
				type callChainsWrapper struct {
					XMLName xml.Name `xml:"callchains"`
					Chains  []struct {
						TargetFunction string `xml:"targetFunction"`
					} `xml:"callchain"`
				}
				var wrapper callChainsWrapper
				if err := xml.Unmarshal([]byte(output), &wrapper); err != nil {
					t.Fatalf("invalid XML: %v\n%s", err, output)
				}
				if len(wrapper.Chains) == 0 {
					t.Fatalf("expected at least one element, got zero")
				}
				if wrapper.Chains[0].TargetFunction != streamContentFunction {
					t.Fatalf("unexpected target function %q", wrapper.Chains[0].TargetFunction)
				}
			},
		},
		{
			name: "CallChainAliasDefaultDepthReturnsOnlyDirectCallers",
			arguments: []string{
				callChainAlias,
				streamContentFunction,
				formatFlag,
				appTypes.FormatJSON,
			},
			prepare: func(t *testing.T) string { return getModuleRoot(t) },
			validate: func(t *testing.T, output string) {
				var callChains []appTypes.CallChainOutput
				if err := json.Unmarshal([]byte(output), &callChains); err != nil {
					t.Fatalf("invalid JSON: %v\n%s", err, output)
				}
				if len(callChains) != 1 {
					t.Logf("callchain json: %s", output)
					t.Fatalf("expected one call chain, got %d", len(callChains))
				}
				callers := callChains[0].Callers
				expected := map[string]struct{}{
					contentDataFunction: {},
				}
				if len(callers) != len(expected) {
					t.Logf("callchain callers: %+v", callers)
					t.Fatalf("expected %d direct callers, got %d", len(expected), len(callers))
				}
				for _, caller := range callers {
					if _, ok := expected[caller]; !ok {
						t.Fatalf("unexpected caller %s", caller)
					}
				}
			},
		},
		{
			name: "CallChainAliasDepthTwoIncludesSecondLevelCallers",
			arguments: []string{
				callChainAlias,
				streamContentFunction,
				depthFlag,
				depthTwoValue,
				formatFlag,
				appTypes.FormatJSON,
			},
			prepare: func(t *testing.T) string { return getModuleRoot(t) },
			validate: func(t *testing.T, output string) {
				var callChains []appTypes.CallChainOutput
				if err := json.Unmarshal([]byte(output), &callChains); err != nil {
					t.Fatalf("invalid JSON: %v\n%s", err, output)
				}
				if len(callChains) != 1 {
					t.Logf("callchain json depth two: %s", output)
					t.Fatalf("expected one call chain, got %d", len(callChains))
				}
				callers := callChains[0].Callers
				expected := map[string]struct{}{
					contentDataFunction: {},
				}
				if len(callers) != len(expected) {
					t.Logf("callchain callers depth two: %+v", callers)
					t.Fatalf("expected %d callers, got %d", len(expected), len(callers))
				}
				for _, caller := range callers {
					if _, ok := expected[caller]; !ok {
						t.Fatalf("unexpected caller %s", caller)
					}
				}
			},
		},
		{
			name: "CallChainPythonDepthThreeIncludesCrossModuleCallersAndCallees",
			arguments: []string{
				appTypes.CommandCallChain,
				pythonCallchainTargetFunction,
				depthFlag,
				depthThreeValue,
				formatFlag,
				appTypes.FormatJSON,
			},
			prepare: func(t *testing.T) string {
				layout := map[string]string{
					filepath.Join(pythonCallchainModuleDirectoryName, pythonCallchainServiceFileName): pythonCallchainServiceContent,
					pythonCallchainConsumerFileName: pythonCallchainConsumerContent,
				}
				return setupTestDirectory(t, layout)
			},
			validate: func(t *testing.T, output string) {
				var callChains []appTypes.CallChainOutput
				if err := json.Unmarshal([]byte(output), &callChains); err != nil {
					t.Fatalf("invalid JSON: %v\n%s", err, output)
				}
				if len(callChains) != 1 {
					t.Fatalf("expected one call chain entry, got %d", len(callChains))
				}
				chain := callChains[0]
				if chain.TargetFunction != pythonCallchainTargetFunction {
					t.Fatalf("unexpected target %s", chain.TargetFunction)
				}
				expectedCallers := map[string]struct{}{
					"app.service.Greeter.greet": {},
					"app.service.entry":         {},
					"consumer.run":              {},
				}
				if len(chain.Callers) != len(expectedCallers) {
					t.Fatalf("expected %d callers, got %d: %+v", len(expectedCallers), len(chain.Callers), chain.Callers)
				}
				for _, caller := range chain.Callers {
					if _, ok := expectedCallers[caller]; !ok {
						t.Fatalf("unexpected caller %s", caller)
					}
				}
				if chain.Callees == nil {
					t.Fatalf("expected callees in output")
				}
				expectedCallees := map[string]struct{}{
					"app.service.core":   {},
					"app.service.helper": {},
				}
				if len(*chain.Callees) != len(expectedCallees) {
					t.Fatalf("expected %d callees, got %d: %+v", len(expectedCallees), len(*chain.Callees), *chain.Callees)
				}
				for _, callee := range *chain.Callees {
					if _, ok := expectedCallees[callee]; !ok {
						t.Fatalf("unexpected callee %s", callee)
					}
				}
				if len(chain.Functions) != len(expectedCallers)+len(expectedCallees)+1 {
					t.Fatalf("expected %d functions, got %d", len(expectedCallers)+len(expectedCallees)+1, len(chain.Functions))
				}
			},
		},
		{
			name: "CallChainJavaScriptDepthTwoResolvesCrossFileCallers",
			arguments: []string{
				appTypes.CommandCallChain,
				javaScriptCallchainTargetFunction,
				depthFlag,
				depthTwoValue,
				formatFlag,
				appTypes.FormatJSON,
			},
			prepare: func(t *testing.T) string {
				layout := map[string]string{
					filepath.Join(javaScriptCallchainModuleDirectoryName, javaScriptCallchainServiceFileName): javaScriptCallchainServiceContent,
					javaScriptCallchainConsumerFileName: javaScriptCallchainConsumerContent,
				}
				return setupTestDirectory(t, layout)
			},
			validate: func(t *testing.T, output string) {
				var callChains []appTypes.CallChainOutput
				if err := json.Unmarshal([]byte(output), &callChains); err != nil {
					t.Fatalf("invalid JSON: %v\n%s", err, output)
				}
				if len(callChains) != 1 {
					t.Fatalf("expected one call chain entry, got %d", len(callChains))
				}
				chain := callChains[0]
				if chain.TargetFunction != javaScriptCallchainTargetFunction {
					t.Fatalf("unexpected target %s", chain.TargetFunction)
				}
				expectedCallers := map[string]struct{}{
					"app.service.entry": {},
					"consumer.run":      {},
				}
				if len(chain.Callers) != len(expectedCallers) {
					t.Fatalf("expected %d callers, got %d: %+v", len(expectedCallers), len(chain.Callers), chain.Callers)
				}
				for _, caller := range chain.Callers {
					if _, ok := expectedCallers[caller]; !ok {
						t.Fatalf("unexpected caller %s", caller)
					}
				}
				if chain.Callees == nil {
					t.Fatalf("expected callees in output")
				}
				expectedCallees := map[string]struct{}{
					"app.service.core": {},
				}
				if len(*chain.Callees) != len(expectedCallees) {
					t.Fatalf("expected %d callees, got %d: %+v", len(expectedCallees), len(*chain.Callees), *chain.Callees)
				}
				for _, callee := range *chain.Callees {
					if _, ok := expectedCallees[callee]; !ok {
						t.Fatalf("unexpected callee %s", callee)
					}
				}
				if len(chain.Functions) != len(expectedCallers)+len(expectedCallees)+1 {
					t.Fatalf("expected %d functions, got %d", len(expectedCallers)+len(expectedCallees)+1, len(chain.Functions))
				}
			},
		},
		{
			name:          "VersionFlag",
			arguments:     []string{versionFlag},
			prepare:       func(t *testing.T) string { return setupTestDirectory(t, nil) },
			expectError:   false,
			expectWarning: false,
			validate: func(t *testing.T, output string) {
				const prefix = "ctx version:"
				if !strings.HasPrefix(output, prefix) {
					t.Fatalf("version output should start with %q\n%s", prefix, output)
				}
			},
		},
		{
			name: "JsonFormatContentUnreadableFile",
			arguments: []string{
				appTypes.CommandContent,
				"readable.txt",
				"unreadable.txt",
				"--format",
				appTypes.FormatJSON,
			},
			prepare: func(t *testing.T) string {
				if runtime.GOOS == "windows" || os.Geteuid() == 0 {
					t.Skip("Skipping unreadable file test on this platform")
				}
				return setupTestDirectory(t, map[string]string{
					"readable.txt":   "OK",
					"unreadable.txt": "<UNREADABLE>",
				})
			},
			expectWarning: true,
			validate: func(t *testing.T, output string) {
				files := decodeJSONFiles(t, output)
				if len(files) != 1 {
					t.Fatalf("expected one readable item, got %d", len(files))
				}
				if files[0].MimeType != expectedTextMimeType {
					t.Fatalf("expected MIME type %s", expectedTextMimeType)
				}
				expectedSize := utils.FormatFileSize(int64(len("OK")))
				if files[0].Size != expectedSize {
					t.Fatalf("expected size %s, got %s", expectedSize, files[0].Size)
				}
				info, err := os.Stat(files[0].Path)
				if err != nil {
					t.Fatalf("stat failed for %s: %v", files[0].Path, err)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if files[0].LastModified != expectedTimestamp {
					t.Fatalf("expected last modified %s, got %s", expectedTimestamp, files[0].LastModified)
				}
			},
		},
		{
			name: "BinaryFileContentJSON",
			arguments: withJSONFormat([]string{
				appTypes.CommandContent,
				".",
			}),
			prepare: func(t *testing.T) string {
				binaryBytes, decodeError := base64.StdEncoding.DecodeString(onePixelPNGBase64Content)
				if decodeError != nil {
					t.Fatalf("failed to decode binary fixture: %v", decodeError)
				}
				return setupTestDirectory(t, map[string]string{
					binaryFixtureFileName: string(binaryBytes),
				})
			},
			validate: func(t *testing.T, output string) {
				files := decodeJSONFiles(t, output)
				if len(files) != 1 {
					t.Fatalf("expected one item, got %d", len(files))
				}
				fileOutput := files[0]
				if fileOutput.Type != appTypes.NodeTypeBinary {
					t.Fatalf("expected type %q, got %q", appTypes.NodeTypeBinary, fileOutput.Type)
				}
				if fileOutput.Content != "" {
					t.Fatalf("expected empty content for binary file, got %q", fileOutput.Content)
				}
				if fileOutput.MimeType != expectedBinaryMimeType {
					t.Fatalf("expected MIME type %q, got %q", expectedBinaryMimeType, fileOutput.MimeType)
				}
				binaryBytes, decodeError := base64.StdEncoding.DecodeString(onePixelPNGBase64Content)
				if decodeError != nil {
					t.Fatalf("failed to decode binary fixture during validation: %v", decodeError)
				}
				expectedSize := utils.FormatFileSize(int64(len(binaryBytes)))
				if fileOutput.Size != expectedSize {
					t.Fatalf("expected size %s, got %s", expectedSize, fileOutput.Size)
				}
				info, err := os.Stat(fileOutput.Path)
				if err != nil {
					t.Fatalf("stat failed for %s: %v", fileOutput.Path, err)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if fileOutput.LastModified != expectedTimestamp {
					t.Fatalf("expected last modified %s, got %s", expectedTimestamp, fileOutput.LastModified)
				}
			},
		},
		{
			name: "BinaryFileTreeJSON",
			arguments: withJSONFormat([]string{
				appTypes.CommandTree,
				".",
			}),
			prepare: func(t *testing.T) string {
				binaryBytes, decodeError := base64.StdEncoding.DecodeString(onePixelPNGBase64Content)
				if decodeError != nil {
					t.Fatalf("failed to decode binary fixture: %v", decodeError)
				}
				return setupTestDirectory(t, map[string]string{
					binaryFixtureFileName: string(binaryBytes),
				})
			},
			validate: func(t *testing.T, output string) {
				nodes := decodeJSONRoots(t, output)
				if len(nodes) != 1 {
					t.Fatalf("expected one top-level node, got %d", len(nodes))
				}
				if len(nodes[0].Children) != 1 {
					t.Fatalf("expected one child, got %d", len(nodes[0].Children))
				}
				child := nodes[0].Children[0]
				if child.Type != appTypes.NodeTypeBinary {
					t.Fatalf("expected type %q, got %q", appTypes.NodeTypeBinary, child.Type)
				}
				if child.MimeType != expectedBinaryMimeType {
					t.Fatalf("expected MIME type %q, got %q", expectedBinaryMimeType, child.MimeType)
				}
				binaryBytes, decodeError := base64.StdEncoding.DecodeString(onePixelPNGBase64Content)
				if decodeError != nil {
					t.Fatalf("failed to decode binary fixture during validation: %v", decodeError)
				}
				expectedSize := utils.FormatFileSize(int64(len(binaryBytes)))
				if child.Size != expectedSize {
					t.Fatalf("expected size %s, got %s", expectedSize, child.Size)
				}
				info, err := os.Stat(child.Path)
				if err != nil {
					t.Fatalf("stat failed for %s: %v", child.Path, err)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if child.LastModified != expectedTimestamp {
					t.Fatalf("expected last modified %s, got %s", expectedTimestamp, child.LastModified)
				}
			},
		},
		{
			name: "BinaryFileContentIgnoreDefault",
			arguments: withJSONFormat([]string{
				appTypes.CommandContent,
				".",
			}),
			prepare: func(t *testing.T) string {
				binaryBytes, decodeError := base64.StdEncoding.DecodeString(onePixelPNGBase64Content)
				if decodeError != nil {
					t.Fatalf("failed to decode binary fixture: %v", decodeError)
				}
				ignoreContent := binarySectionHeader + "\n" + unmatchedBinaryFixtureName + "\n"
				return setupTestDirectory(t, map[string]string{
					binaryFixtureFileName: string(binaryBytes),
					ignoreFileName:        ignoreContent,
				})
			},
			validate: func(t *testing.T, output string) {
				files := decodeJSONFiles(t, output)
				if len(files) != 1 {
					t.Fatalf("expected one item, got %d", len(files))
				}
				fileOutput := files[0]
				if fileOutput.Content != "" {
					t.Fatalf("expected empty content for binary file, got %q", fileOutput.Content)
				}
				if fileOutput.MimeType != expectedBinaryMimeType {
					t.Fatalf("expected MIME type %q, got %q", expectedBinaryMimeType, fileOutput.MimeType)
				}
			},
		},
		{
			name: "BinaryFileContentBinarySection",
			arguments: withJSONFormat([]string{
				appTypes.CommandContent,
				".",
			}),
			prepare: func(t *testing.T) string {
				binaryBytes, decodeError := base64.StdEncoding.DecodeString(onePixelPNGBase64Content)
				if decodeError != nil {
					t.Fatalf("failed to decode binary fixture: %v", decodeError)
				}
				ignoreContent := binarySectionHeader + "\n" + binaryFixtureFileName + "\n"
				return setupTestDirectory(t, map[string]string{
					binaryFixtureFileName: string(binaryBytes),
					ignoreFileName:        ignoreContent,
				})
			},
			validate: func(t *testing.T, output string) {
				files := decodeJSONFiles(t, output)
				if len(files) != 1 {
					t.Fatalf("expected one item, got %d", len(files))
				}
				fileOutput := files[0]
				if fileOutput.Content != onePixelPNGBase64Content {
					t.Fatalf("expected base64 content for binary file")
				}
				if fileOutput.MimeType != expectedBinaryMimeType {
					t.Fatalf("expected MIME type %q, got %q", expectedBinaryMimeType, fileOutput.MimeType)
				}
			},
		},
		{
			name: "InvalidFormatValue",
			arguments: []string{
				appTypes.CommandContent,
				"a.txt",
				"--format",
				"yaml",
			},
			prepare:     func(t *testing.T) string { return setupTestDirectory(t, map[string]string{"a.txt": "A"}) },
			expectError: true,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "Invalid format value 'yaml'") {
					t.Errorf("expected error about invalid format value, got:\n%s", output)
				}
			},
		},
		{
			name: "CallChainMainDoc",
			arguments: []string{
				appTypes.CommandCallChain,
				"main.main",
				documentationFlag,
				"--format",
				appTypes.FormatRaw,
			},
			prepare: func(t *testing.T) string { return getModuleRoot(t) },
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "main.main") {
					t.Fatalf("expected main.main documentation in output, got:\n%s", output)
				}

				for _, line := range strings.Split(output, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "Error:") {
						t.Fatalf("unexpected error message in output:\n%s", output)
					}
				}
			},
		},
		{
			name: "GitDirectoryExcludedByDefault",
			arguments: withJSONFormat([]string{
				appTypes.CommandTree,
			}),
			prepare: func(t *testing.T) string {
				layout := map[string]string{
					filepath.Join(utils.GitDirectoryName, hiddenFileName): hiddenFileContent,
					visibleFileName: visibleFileContent,
				}
				return setupTestDirectory(t, layout)
			},
			validate: func(t *testing.T, output string) {
				nodes := decodeJSONRoots(t, output)
				if len(nodes) != 1 {
					t.Fatalf("expected one root node, got %d", len(nodes))
				}
				children := childrenToSlice(nodes[0].Children)
				if len(children) != 1 || children[0].Name != visibleFileName {
					t.Fatalf("expected only %s, got %#v", visibleFileName, children)
				}
				if children[0].MimeType != expectedTextMimeType {
					t.Fatalf("expected MIME type %s", expectedTextMimeType)
				}
				expectedSize := utils.FormatFileSize(int64(len(visibleFileContent)))
				if children[0].Size != expectedSize {
					t.Fatalf("expected size %s, got %s", expectedSize, children[0].Size)
				}
				info, err := os.Stat(children[0].Path)
				if err != nil {
					t.Fatalf("stat failed for %s: %v", children[0].Path, err)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if children[0].LastModified != expectedTimestamp {
					t.Fatalf("expected last modified %s, got %s", expectedTimestamp, children[0].LastModified)
				}
			},
		},
		{
			name: "GitDirectoryIncludedWithFlag",
			arguments: withJSONFormat([]string{
				appTypes.CommandTree,
				includeGitFlag,
			}),
			prepare: func(t *testing.T) string {
				layout := map[string]string{
					filepath.Join(utils.GitDirectoryName, hiddenFileName): hiddenFileContent,
					visibleFileName: visibleFileContent,
				}
				return setupTestDirectory(t, layout)
			},
			validate: func(t *testing.T, output string) {
				nodes := decodeJSONRoots(t, output)
				if len(nodes) != 1 {
					t.Fatalf("expected one root node, got %d", len(nodes))
				}
				names := make(map[string]appTypes.TreeOutputNode)
				for _, child := range childrenToSlice(nodes[0].Children) {
					names[child.Name] = child
				}
				if _, ok := names[utils.GitDirectoryName]; !ok {
					t.Fatalf("expected %s in output", utils.GitDirectoryName)
				}
				visibleNode, ok := names[visibleFileName]
				if !ok {
					t.Fatalf("expected %s in output", visibleFileName)
				}
				if visibleNode.MimeType != expectedTextMimeType {
					t.Fatalf("expected MIME type %s", expectedTextMimeType)
				}
				expectedSize := utils.FormatFileSize(int64(len(visibleFileContent)))
				if visibleNode.Size != expectedSize {
					t.Fatalf("expected size %s, got %s", expectedSize, visibleNode.Size)
				}
				info, err := os.Stat(visibleNode.Path)
				if err != nil {
					t.Fatalf("stat failed for %s: %v", visibleNode.Path, err)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if visibleNode.LastModified != expectedTimestamp {
					t.Fatalf("expected last modified %s, got %s", expectedTimestamp, visibleNode.LastModified)
				}
			},
		},
		{
			name:      "GoogleSheetsAddonTreeOmitsIgnoredEntries",
			arguments: []string{appTypes.CommandTree},
			prepare:   setupGoogleSheetsAddonFixture,
			validate: func(testingHandle *testing.T, output string) {
				if strings.Contains(output, nodeModulesDirName) {
					testingHandle.Fatalf("expected tree output to exclude %s\n%s", nodeModulesDirName, output)
				}
				if strings.Contains(output, claspConfigurationFileName) {
					testingHandle.Fatalf("expected tree output to exclude %s\n%s", claspConfigurationFileName, output)
				}
				if !strings.Contains(output, visibleFileName) {
					testingHandle.Fatalf("expected tree output to include %s\n%s", visibleFileName, output)
				}
				if !strings.Contains(output, expectedTextMimeType) {
					testingHandle.Fatalf("expected MIME type %s in output\n%s", expectedTextMimeType, output)
				}
			},
		},
		{
			name:      "GoogleSheetsAddonContentOmitsIgnoredEntries",
			arguments: []string{appTypes.CommandContent},
			prepare:   setupGoogleSheetsAddonFixture,
			validate: func(testingHandle *testing.T, output string) {
				if strings.Contains(output, dependencyFileName) || strings.Contains(output, dependencyFileContent) {
					testingHandle.Fatalf("expected content output to exclude %s\n%s", dependencyFileName, output)
				}
				if strings.Contains(output, claspConfigurationFileName) || strings.Contains(output, claspConfigurationContent) {
					testingHandle.Fatalf("expected content output to exclude %s\n%s", claspConfigurationFileName, output)
				}
				if !strings.Contains(output, visibleFileName) || !strings.Contains(output, visibleFileContent) {
					testingHandle.Fatalf("expected content output to include %s and its content\n%s", visibleFileName, output)
				}
				if !strings.Contains(output, expectedTextMimeType) {
					testingHandle.Fatalf("expected MIME type %s in output\n%s", expectedTextMimeType, output)
				}
			},
		},
		{
			name:      "NestedGitignoreExcluded",
			arguments: withJSONFormat([]string{treeAlias}),
			prepare: func(t *testing.T) string {
				layout := map[string]string{
					filepath.Join(subDirectoryName, gitignoreFileName):                      nodeModulesPattern,
					filepath.Join(subDirectoryName, nodeModulesDirName, dependencyFileName): dependencyFileContent,
					filepath.Join(subDirectoryName, visibleFileName):                        visibleFileContent,
				}
				return setupTestDirectory(t, layout)
			},
			validate: func(t *testing.T, output string) {
				nodes := decodeJSONRoots(t, output)
				if len(nodes) != 1 {
					t.Fatalf("expected one root node, got %d", len(nodes))
				}
				rootChildren := childrenToSlice(nodes[0].Children)
				if len(rootChildren) != 1 || rootChildren[0].Name != subDirectoryName {
					t.Fatalf("expected root child %s, got %#v", subDirectoryName, rootChildren)
				}
				subNode := rootChildren[0]
				if subNode.Size != "" {
					t.Fatalf("expected directory %s to omit size, got %q", subDirectoryName, subNode.Size)
				}
				children := childrenToSlice(subNode.Children)
				if len(children) != 1 || children[0].Name != visibleFileName {
					t.Fatalf("expected only %s in %s, got %#v", visibleFileName, subDirectoryName, children)
				}
				if children[0].MimeType != expectedTextMimeType {
					t.Fatalf("expected MIME type %s", expectedTextMimeType)
				}
				expectedSize := utils.FormatFileSize(int64(len(visibleFileContent)))
				if children[0].Size != expectedSize {
					t.Fatalf("expected size %s, got %s", expectedSize, children[0].Size)
				}
				info, err := os.Stat(children[0].Path)
				if err != nil {
					t.Fatalf("stat failed for %s: %v", children[0].Path, err)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if children[0].LastModified != expectedTimestamp {
					t.Fatalf("expected last modified %s, got %s", expectedTimestamp, children[0].LastModified)
				}
			},
		},
		{
			name:      "ParentGitignoreExcluded",
			arguments: withJSONFormat([]string{treeAlias}),
			prepare: func(t *testing.T) string {
				layout := map[string]string{
					gitignoreFileName: "site_data/\napp/logs/\n",
					filepath.Join("app", "site_data", hiddenFileName): hiddenFileContent,
					filepath.Join("app", "logs", hiddenFileName):      hiddenFileContent,
					filepath.Join("app", visibleFileName):             visibleFileContent,
				}
				root := setupTestDirectory(t, layout)
				return filepath.Join(root, "app")
			},
			validate: func(t *testing.T, output string) {
				nodes := decodeJSONRoots(t, output)
				if len(nodes) != 1 {
					t.Fatalf("expected one root node, got %d", len(nodes))
				}
				rootNode := nodes[0]
				children := childrenToSlice(rootNode.Children)
				if len(children) != 1 || children[0].Name != visibleFileName {
					t.Fatalf("expected only %s in output, got %#v", visibleFileName, children)
				}
				if strings.Contains(output, "site_data") {
					t.Fatalf("unexpected site_data entry in output\n%s", output)
				}
				if strings.Contains(output, "logs") {
					t.Fatalf("unexpected logs entry in output\n%s", output)
				}
			},
		},
		{
			name:      "NestedGitignoreContentExcluded",
			arguments: withJSONFormat([]string{contentAlias}),
			prepare: func(testingHandle *testing.T) string {
				directoryLayout := map[string]string{
					filepath.Join(subDirectoryName, gitignoreFileName):                      nodeModulesPattern + ignoredFileName + "\n",
					filepath.Join(subDirectoryName, nodeModulesDirName, dependencyFileName): dependencyFileContent,
					filepath.Join(subDirectoryName, ignoredFileName):                        ignoredFileContent,
					filepath.Join(subDirectoryName, visibleFileName):                        visibleFileContent,
				}
				return setupTestDirectory(testingHandle, directoryLayout)
			},
			validate: func(testingHandle *testing.T, output string) {
				if strings.Contains(output, nodeModulesDirName) {
					testingHandle.Fatalf("expected content output to exclude %s\n%s", nodeModulesDirName, output)
				}
				if strings.Contains(output, ignoredFileName) {
					testingHandle.Fatalf("expected content output to exclude %s\n%s", ignoredFileName, output)
				}
				fileOutputs := decodeJSONFiles(testingHandle, output)
				if len(fileOutputs) != 1 {
					testingHandle.Fatalf("expected one file, got %d", len(fileOutputs))
				}
				expectedSuffix := filepath.Join(subDirectoryName, visibleFileName)
				fileOutput := fileOutputs[0]
				if !strings.HasSuffix(fileOutput.Path, expectedSuffix) {
					testingHandle.Fatalf("expected path suffix %s, got %s", expectedSuffix, fileOutput.Path)
				}
				if fileOutput.Content != visibleFileContent {
					testingHandle.Fatalf("expected content %s, got %s", visibleFileContent, fileOutput.Content)
				}
				if fileOutput.MimeType != expectedTextMimeType {
					testingHandle.Fatalf("expected MIME type %s", expectedTextMimeType)
				}
				expectedSize := utils.FormatFileSize(int64(len(visibleFileContent)))
				if fileOutput.Size != expectedSize {
					testingHandle.Fatalf("expected size %s, got %s", expectedSize, fileOutput.Size)
				}
				info, err := os.Stat(fileOutput.Path)
				if err != nil {
					testingHandle.Fatalf("stat failed for %s: %v", fileOutput.Path, err)
				}
				expectedTimestamp := utils.FormatTimestamp(info.ModTime())
				if fileOutput.LastModified != expectedTimestamp {
					testingHandle.Fatalf("expected last modified %s, got %s", expectedTimestamp, fileOutput.LastModified)
				}
			},
		},
		{
			name:      "ExcludePatternsTree",
			arguments: withJSONFormat([]string{appTypes.CommandTree, "-e", toolsDirectoryName, "-e", githubDirectoryName, "-e", yamlPattern}),
			prepare:   setupExclusionPatternFixture,
			validate: func(testingHandle *testing.T, output string) {
				outputNodes := decodeJSONRoots(testingHandle, output)
				if len(outputNodes) != 1 {
					testingHandle.Fatalf("expected one root node, got %d", len(outputNodes))
				}
				rootChildren := childrenToSlice(outputNodes[0].Children)
				if len(rootChildren) != 2 {
					testingHandle.Fatalf("expected two root children, got %d", len(rootChildren))
				}
				for _, childNode := range rootChildren {
					if childNode.Name == toolsDirectoryName || childNode.Name == githubDirectoryName || childNode.Name == yamlRootFileName {
						testingHandle.Fatalf("unexpected child %s", childNode.Name)
					}
					if childNode.Name == nestedDirectoryName {
						if len(childNode.Children) != 1 || childNode.Children[0].Name != nestedTextFileName {
							testingHandle.Fatalf("expected only %s inside %s", nestedTextFileName, nestedDirectoryName)
						}
						if childNode.Children[0].MimeType != expectedTextMimeType {
							testingHandle.Fatalf("expected MIME type %s", expectedTextMimeType)
						}
					}
				}
			},
		},
		{
			name:      "ExcludePatternsContent",
			arguments: withJSONFormat([]string{appTypes.CommandContent, "-e", toolsDirectoryName, "-e", githubDirectoryName, "-e", yamlPattern}),
			prepare:   setupExclusionPatternFixture,
			validate: func(testingHandle *testing.T, output string) {
				fileOutputs := decodeJSONFiles(testingHandle, output)
				if len(fileOutputs) != 2 {
					testingHandle.Fatalf("expected two files, got %d", len(fileOutputs))
				}
				for _, fileOutput := range fileOutputs {
					if strings.HasSuffix(fileOutput.Path, yamlRootFileName) || strings.HasSuffix(fileOutput.Path, nestedYamlFileName) {
						testingHandle.Fatalf("unexpected YAML file %s", fileOutput.Path)
					}
					if strings.Contains(fileOutput.Path, toolsDirectoryName) || strings.Contains(fileOutput.Path, githubDirectoryName) {
						testingHandle.Fatalf("unexpected excluded path %s", fileOutput.Path)
					}
					if fileOutput.MimeType != expectedTextMimeType {
						testingHandle.Fatalf("expected MIME type %s", expectedTextMimeType)
					}
				}
			},
		},
	}

	for _, testCase := range testCases {
		testingHandle.Run(testCase.name, func(t *testing.T) {
			if testCase.requiresHelpers {
				if os.Getenv("CTX_TEST_RUN_HELPERS") != "1" {
					t.Skip("set CTX_TEST_RUN_HELPERS=1 to run helper-dependent tests")
				}
				uvExecutable := os.Getenv("CTX_TEST_UV")
				if uvExecutable == "" {
					uvExecutable = "uv"
				}
				if _, err := exec.LookPath(uvExecutable); err != nil {
					t.Skipf("uv executable %q unavailable: %v", uvExecutable, err)
				}
				t.Setenv("CTX_UV", uvExecutable)
			}
			workingDir := testCase.prepare(t)

			var output string
			if testCase.expectError {
				output = runCommandExpectError(t, binary, testCase.arguments, workingDir)
			} else if testCase.expectWarning {
				output = runCommandWithWarnings(t, binary, testCase.arguments, workingDir)
			} else {
				output = runCommand(t, binary, testCase.arguments, workingDir)
			}

			testCase.validate(t, output)
		})
	}
}

// TestGoRunAllPackagesInvokesRootCommand ensures `go run ./...` executes the CLI entrypoint.
func TestGoRunAllPackagesInvokesRootCommand(testingHandle *testing.T) {
	moduleRoot := getModuleRoot(testingHandle)
	command := exec.Command("go", "run", "./...", versionFlag)
	command.Dir = moduleRoot

	var stdoutBuffer, stderrBuffer bytes.Buffer
	command.Stdout = &stdoutBuffer
	command.Stderr = &stderrBuffer

	if runError := command.Run(); runError != nil {
		testingHandle.Fatalf("`go run ./...` failed: %v\nstdout:\n%s\nstderr:\n%s", runError, stdoutBuffer.String(), stderrBuffer.String())
	}

	const prefix = "ctx version:"
	if !strings.HasPrefix(stdoutBuffer.String(), prefix) {
		testingHandle.Fatalf("expected version output to start with %q\n%s", prefix, stdoutBuffer.String())
	}
}
