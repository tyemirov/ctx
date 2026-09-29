package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tyemirov/ctx/internal/config"
	"github.com/tyemirov/ctx/internal/docs"
	"github.com/tyemirov/ctx/internal/tokenizer"
	"github.com/tyemirov/ctx/internal/types"
	"github.com/tyemirov/ctx/internal/utils"
)

type treeStubCounter struct{}

func (treeStubCounter) Name() string { return "stub" }

func (treeStubCounter) CountString(input string) (int, error) {
	return len([]rune(input)), nil
}

type clipboardStub struct {
	copiedText      string
	errorToReturn   error
	invocationCount int
}

func (stub *clipboardStub) Copy(text string) error {
	stub.invocationCount++
	stub.copiedText = text
	return stub.errorToReturn
}

type callChainServiceStub struct {
	output          *types.CallChainOutput
	returnedError   error
	invocationCount int
}

func (stub *callChainServiceStub) GetCallChainData(targetFunctionQualifiedName string, callChainDepth int, includeDocumentation bool, documentationCollector *docs.Collector, repositoryRootDirectory string) (*types.CallChainOutput, error) {
	stub.invocationCount++
	if stub.returnedError != nil {
		return nil, stub.returnedError
	}
	if stub.output != nil {
		return stub.output, nil
	}
	return &types.CallChainOutput{TargetFunction: targetFunctionQualifiedName}, nil
}

func boolPtr(value bool) *bool {
	pointer := value
	return &pointer
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = writePipe

	var buffer bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buffer, readPipe)
		close(done)
	}()

	fn()

	writePipe.Close()
	os.Stdout = original
	<-done
	return buffer.String()
}

func TestRunTreeRawStreamingOutputsSummaryAfterFiles(t *testing.T) {
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "nested")
	if err := os.Mkdir(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	rootFile := filepath.Join(tempDir, "root.txt")
	nestedFile := filepath.Join(nestedDir, "inner.txt")

	if err := os.WriteFile(rootFile, []byte("token"), 0o600); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	if err := os.WriteFile(nestedFile, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	outputText := captureStdout(t, func() {
		if err := runStreamCommand(
			context.Background(),
			types.CommandTree,
			[]string{tempDir},
			nil,
			true,
			true,
			false,
			types.FormatRaw,
			types.DocumentationModeDisabled,
			true,
			false,
			treeStubCounter{},
			"stub-model",
			nil,
			os.Stdout,
			os.Stderr,
		); err != nil {
			t.Fatalf("runStreamCommand error: %v", err)
		}
	})

	if !strings.Contains(outputText, tempDir) {
		t.Fatalf("expected directory path in output")
	}
	if !strings.Contains(outputText, nestedDir) {
		t.Fatalf("expected nested directory path in output")
	}
	if !strings.Contains(outputText, rootFile) {
		t.Fatalf("expected root file in output")
	}
	if !strings.Contains(outputText, nestedFile) {
		t.Fatalf("expected nested file in output")
	}

	if !strings.Contains(outputText, "├──") {
		t.Fatalf("expected tree branch connector in output")
	}
	if !strings.Contains(outputText, "└──") {
		t.Fatalf("expected tree leaf connector in output")
	}

	nestedDirIndex := strings.Index(outputText, nestedDir)
	if nestedDirIndex == -1 {
		t.Fatalf("expected nested directory entry in output")
	}
	nestedFileIndex := strings.Index(outputText, nestedFile)
	nestedSummaryKey := fmt.Sprintf("Summary: 1 file, %s", utils.FormatFileSize(3))
	nestedSummaryIndex := strings.Index(outputText, nestedSummaryKey)
	if nestedSummaryIndex == -1 {
		t.Fatalf("expected nested summary line in output")
	}
	if nestedDirIndex >= nestedSummaryIndex || nestedSummaryIndex >= nestedFileIndex {
		t.Fatalf("nested summary ordering incorrect: %s", outputText)
	}

	if !strings.Contains(outputText, "Summary: 2 files") {
		t.Fatalf("expected global summary in output")
	}
}

func TestRunContentRawStreamingStreamsBeforeSummary(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "content.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	outputText := captureStdout(t, func() {
		if err := runStreamCommand(
			context.Background(),
			types.CommandContent,
			[]string{tempDir},
			nil,
			true,
			true,
			false,
			types.FormatRaw,
			types.DocumentationModeDisabled,
			true,
			true,
			treeStubCounter{},
			"stub-model",
			nil,
			os.Stdout,
			os.Stderr,
		); err != nil {
			t.Fatalf("runStreamCommand error: %v", err)
		}
	})

	if !strings.Contains(outputText, "File: "+filePath) {
		t.Fatalf("expected file header in output")
	}
	fileHeaderIndex := strings.Index(outputText, "File: "+filePath)
	endMarker := "End of file: " + filePath
	endIndex := strings.Index(outputText, endMarker)
	if endIndex == -1 {
		t.Fatalf("expected end of file marker")
	}
	if endIndex < fileHeaderIndex {
		t.Fatalf("end marker appeared before file header")
	}
	summaryIndex := strings.LastIndex(outputText, "Summary:")
	if summaryIndex == -1 {
		t.Fatalf("expected summary in output")
	}
	if summaryIndex < endIndex {
		t.Fatalf("summary appeared before file finished streaming")
	}
}

func TestRunToolCopiesOutputToClipboard(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "clipboard.txt")
	if err := os.WriteFile(filePath, []byte("clipboard data"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	testCases := []struct {
		name           string
		clipboardError error
		expectError    bool
	}{
		{
			name:           "copies output when clipboard succeeds",
			clipboardError: nil,
			expectError:    false,
		},
		{
			name:           "returns error when clipboard copy fails",
			clipboardError: errors.New("clipboard failure"),
			expectError:    true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := &clipboardStub{errorToReturn: testCase.clipboardError}
			var outputBuffer bytes.Buffer
			descriptor := commandDescriptor{
				ctx:                context.Background(),
				commandName:        types.CommandContent,
				paths:              []string{tempDir},
				exclusionPatterns:  nil,
				useGitignore:       false,
				useIgnoreFile:      false,
				includeGit:         false,
				callChainDepth:     defaultCallChainDepth,
				format:             types.FormatJSON,
				documentation:      documentationOptions{},
				summaryEnabled:     false,
				includeContent:     true,
				tokenConfiguration: tokenOptions{},
				outputWriter:       &outputBuffer,
				errorWriter:        io.Discard,
				clipboardEnabled:   true,
				copyOnly:           false,
				clipboard:          stub,
			}
			executionError := runTool(descriptor)

			if testCase.expectError {
				if executionError == nil {
					t.Fatalf("expected error when clipboard copy fails")
				}
				return
			}

			if executionError != nil {
				t.Fatalf("unexpected error: %v", executionError)
			}
			if stub.invocationCount != 1 {
				t.Fatalf("expected clipboard to be used once, got %d", stub.invocationCount)
			}
			if stub.copiedText != outputBuffer.String() {
				t.Fatalf("clipboard text mismatch\nexpected: %q\nactual: %q", outputBuffer.String(), stub.copiedText)
			}
		})
	}
}

func TestRunToolCopyOnlySuppressesStdout(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "copy-only.txt")
	if err := os.WriteFile(filePath, []byte("copy only"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	stub := &clipboardStub{}
	var outputBuffer bytes.Buffer
	descriptor := commandDescriptor{
		ctx:                context.Background(),
		commandName:        types.CommandContent,
		paths:              []string{tempDir},
		exclusionPatterns:  nil,
		useGitignore:       false,
		useIgnoreFile:      false,
		includeGit:         false,
		callChainDepth:     defaultCallChainDepth,
		format:             types.FormatJSON,
		documentation:      documentationOptions{},
		summaryEnabled:     false,
		includeContent:     true,
		tokenConfiguration: tokenOptions{},
		outputWriter:       &outputBuffer,
		errorWriter:        io.Discard,
		clipboardEnabled:   false,
		copyOnly:           true,
		clipboard:          stub,
	}
	executionError := runTool(descriptor)

	if executionError != nil {
		t.Fatalf("unexpected error: %v", executionError)
	}
	if outputBuffer.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", outputBuffer.String())
	}
	if stub.invocationCount != 1 {
		t.Fatalf("expected clipboard to be used once, got %d", stub.invocationCount)
	}
	if stub.copiedText == "" {
		t.Fatalf("expected clipboard to receive content")
	}
	if !strings.Contains(stub.copiedText, filepath.Base(filePath)) {
		t.Fatalf("expected clipboard content to reference %s, got %q", filepath.Base(filePath), stub.copiedText)
	}
}

func TestContentCommandCopyOnlyWritesClipboard(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "copy-only.txt")
	if err := os.WriteFile(filePath, []byte("copy-only-data"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stub := &clipboardStub{}
	var copyFlag bool
	var copyOnlyFlag bool
	root := &cobra.Command{Use: "ctx"}
	registerBooleanFlag(root.PersistentFlags(), &copyFlag, copyFlagName, false, "")
	registerBooleanFlag(root.PersistentFlags(), &copyFlag, copyFlagAlias, false, "")
	registerBooleanFlag(root.PersistentFlags(), &copyOnlyFlag, copyOnlyFlagName, false, "")
	registerBooleanFlag(root.PersistentFlags(), &copyOnlyFlag, copyOnlyFlagAlias, false, "")
	command := createContentCommand(stub, &copyFlag, &copyOnlyFlag, nil)
	root.AddCommand(command)
	var outputBuffer bytes.Buffer
	var errorBuffer bytes.Buffer
	root.SetOut(&outputBuffer)
	root.SetErr(&errorBuffer)
	root.SetArgs([]string{"content", "--copy-only", tempDir})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute content command: %v\nstderr: %s", err, errorBuffer.String())
	}
	if outputBuffer.Len() != 0 {
		t.Fatalf("expected copy-only command to suppress stdout, got %q", outputBuffer.String())
	}
	if stub.invocationCount != 1 {
		t.Fatalf("expected clipboard to be invoked once, got %d", stub.invocationCount)
	}
	if !strings.Contains(stub.copiedText, filepath.Base(filePath)) {
		t.Fatalf("expected clipboard text to include %s, got %q", filepath.Base(filePath), stub.copiedText)
	}
}

func TestResolveCopyDefaultRespectsAlias(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)
	var copyValue bool
	registerBooleanFlag(root.PersistentFlags(), &copyValue, copyFlagName, false, "")
	registerBooleanFlag(root.PersistentFlags(), &copyValue, copyFlagAlias, false, "")
	if alias := root.PersistentFlags().Lookup(copyFlagAlias); alias != nil {
		alias.Hidden = true
	}
	if err := root.PersistentFlags().Set(copyFlagAlias, "true"); err != nil {
		t.Fatalf("set copy alias: %v", err)
	}
	if lookup := child.InheritedFlags().Lookup(copyFlagAlias); lookup == nil || !lookup.Changed {
		t.Fatalf("expected inherited alias flag to be marked changed")
	}
	if flag := child.Flag(copyFlagAlias); flag == nil || !flag.Changed {
		t.Fatalf("expected command flag lookup to report alias changed")
	}
	result := resolveCopyDefault(child, copyValue, boolPtr(false))
	if !result {
		t.Fatalf("expected copy alias to override configuration default")
	}
}

func TestResolveClipboardPreferences(t *testing.T) {
	trueValue := boolPtr(true)
	falseValue := boolPtr(false)
	testCases := []struct {
		name             string
		cliCopy          bool
		cliCopyOnly      bool
		configCopy       *bool
		configCopyOnly   *bool
		setCopyFlag      bool
		setCopyOnlyFlag  bool
		expectedCopy     bool
		expectedCopyOnly bool
	}{
		{
			name:             "configuration enables copy",
			cliCopy:          false,
			cliCopyOnly:      false,
			configCopy:       trueValue,
			configCopyOnly:   nil,
			expectedCopy:     true,
			expectedCopyOnly: false,
		},
		{
			name:             "configuration copy-only enables both flags",
			cliCopy:          false,
			cliCopyOnly:      false,
			configCopy:       nil,
			configCopyOnly:   trueValue,
			expectedCopy:     true,
			expectedCopyOnly: true,
		},
		{
			name:             "cli copy overrides configuration default",
			cliCopy:          true,
			cliCopyOnly:      false,
			configCopy:       falseValue,
			configCopyOnly:   nil,
			setCopyFlag:      true,
			expectedCopy:     true,
			expectedCopyOnly: false,
		},
		{
			name:             "cli copy-only overrides configuration default",
			cliCopy:          false,
			cliCopyOnly:      true,
			configCopy:       nil,
			configCopyOnly:   falseValue,
			setCopyOnlyFlag:  true,
			expectedCopy:     true,
			expectedCopyOnly: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			command, copyValue, copyOnlyValue := newClipboardCommand()
			*copyValue = testCase.cliCopy
			*copyOnlyValue = testCase.cliCopyOnly
			if testCase.setCopyFlag {
				if err := command.Flags().Set(copyFlagName, fmt.Sprintf("%t", testCase.cliCopy)); err != nil {
					t.Fatalf("set copy flag: %v", err)
				}
			}
			if testCase.setCopyOnlyFlag {
				if err := command.Flags().Set(copyOnlyFlagName, fmt.Sprintf("%t", testCase.cliCopyOnly)); err != nil {
					t.Fatalf("set copy-only flag: %v", err)
				}
			}
			copyEnabled, copyOnlyEnabled := resolveClipboardPreferences(command, *copyValue, *copyOnlyValue, testCase.configCopy, testCase.configCopyOnly)
			if copyEnabled != testCase.expectedCopy {
				t.Fatalf("expected copy=%t, got %t", testCase.expectedCopy, copyEnabled)
			}
			if copyOnlyEnabled != testCase.expectedCopyOnly {
				t.Fatalf("expected copyOnly=%t, got %t", testCase.expectedCopyOnly, copyOnlyEnabled)
			}
		})
	}
}

func TestRunToolCallChainRequiresService(t *testing.T) {
	descriptor := commandDescriptor{
		ctx:            context.Background(),
		commandName:    types.CommandCallChain,
		paths:          []string{"fmt.Println"},
		callChainDepth: defaultCallChainDepth,
		format:         types.FormatJSON,
		documentation:  documentationOptions{},
		outputWriter:   io.Discard,
		errorWriter:    io.Discard,
	}

	err := runTool(descriptor)
	if err == nil {
		t.Fatalf("expected error when call chain service is missing")
	}
	if !strings.Contains(err.Error(), "call chain service") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunToolCallChainUsesInjectedService(t *testing.T) {
	stub := &callChainServiceStub{}
	var outputBuffer bytes.Buffer
	descriptor := commandDescriptor{
		ctx:              context.Background(),
		commandName:      types.CommandCallChain,
		paths:            []string{"fmt.Println"},
		callChainDepth:   1,
		format:           types.FormatRaw,
		documentation:    documentationOptions{},
		outputWriter:     &outputBuffer,
		errorWriter:      io.Discard,
		callChainService: stub,
	}

	if err := runTool(descriptor); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.invocationCount != 1 {
		t.Fatalf("expected service to be invoked once, got %d", stub.invocationCount)
	}
	if outputBuffer.Len() == 0 {
		t.Fatalf("expected output to be rendered")
	}
}

func TestBuildExecutionContextSurfacesHelperSentinel(t *testing.T) {
	t.Setenv("CTX_UV", filepath.Join(os.TempDir(), "missing-uv"))
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	descriptor := commandDescriptor{
		documentation: documentationOptions{},
		tokenConfiguration: tokenOptions{
			enabled: true,
			model:   "claude-3-5-sonnet",
		},
	}
	_, err := buildExecutionContext(context.Background(), descriptor, t.TempDir())
	if err == nil {
		t.Fatalf("expected error when helpers are unavailable")
	}
	if !errors.Is(err, tokenizer.ErrHelperUnavailable) {
		t.Fatalf("expected helper unavailable error, got %v", err)
	}
}

func newClipboardCommand() (*cobra.Command, *bool, *bool) {
	command := &cobra.Command{Use: "clipboard"}
	var copyValue bool
	var copyOnlyValue bool
	registerBooleanFlag(command.Flags(), &copyValue, copyFlagName, copyValue, "")
	registerBooleanFlag(command.Flags(), &copyValue, copyFlagAlias, copyValue, "")
	if alias := command.Flags().Lookup(copyFlagAlias); alias != nil {
		alias.Hidden = true
	}
	registerBooleanFlag(command.Flags(), &copyOnlyValue, copyOnlyFlagName, copyOnlyValue, "")
	registerBooleanFlag(command.Flags(), &copyOnlyValue, copyOnlyFlagAlias, copyOnlyValue, "")
	if alias := command.Flags().Lookup(copyOnlyFlagAlias); alias != nil {
		alias.Hidden = true
	}
	return command, &copyValue, &copyOnlyValue
}

func TestApplyStreamConfigurationUsesDefaults(t *testing.T) {
	var pathConfiguration pathOptions
	format := types.FormatJSON
	summaryEnabled := true
	documentationMode := types.DocumentationModeDisabled
	tokens := tokenOptions{model: "initial"}
	includeContent := false

	command := &cobra.Command{Use: "test"}
	command.Flags().StringVar(&format, formatFlagName, format, "")
	registerBooleanFlag(command.Flags(), &summaryEnabled, summaryFlagName, summaryEnabled, "")
	command.Flags().StringVar(&documentationMode, documentationFlagName, documentationMode, "")
	registerBooleanFlag(command.Flags(), &tokens.enabled, tokensFlagName, tokens.enabled, "")
	command.Flags().StringVar(&tokens.model, modelFlagName, tokens.model, "")
	command.Flags().StringArrayVarP(&pathConfiguration.exclusionPatterns, exclusionFlagName, exclusionFlagName, pathConfiguration.exclusionPatterns, "")
	registerBooleanFlag(command.Flags(), &pathConfiguration.disableGitignore, noGitignoreFlagName, pathConfiguration.disableGitignore, "")
	registerBooleanFlag(command.Flags(), &pathConfiguration.disableIgnoreFile, noIgnoreFlagName, pathConfiguration.disableIgnoreFile, "")
	registerBooleanFlag(command.Flags(), &pathConfiguration.includeGit, includeGitFlagName, pathConfiguration.includeGit, "")
	registerBooleanFlag(command.Flags(), &includeContent, contentFlagName, includeContent, "")
	docsAttempt := false

	configuration := config.StreamCommandConfiguration{
		Format:         types.FormatXML,
		Summary:        boolPtr(false),
		Documentation:  boolPtr(true),
		IncludeContent: boolPtr(true),
		Tokens: config.TokenConfiguration{
			Enabled: boolPtr(true),
			Model:   "custom",
		},
		Paths: config.PathConfiguration{
			Exclude:       []string{"vendor"},
			UseGitignore:  boolPtr(false),
			UseIgnoreFile: boolPtr(false),
			IncludeGit:    boolPtr(true),
		},
	}

	applyStreamConfiguration(command, configuration, &pathConfiguration, &format, &documentationMode, &summaryEnabled, &includeContent, &docsAttempt, &tokens)

	if format != types.FormatXML {
		t.Fatalf("expected format %s, got %s", types.FormatXML, format)
	}
	if summaryEnabled {
		t.Fatalf("expected summary to be disabled")
	}
	if documentationMode != types.DocumentationModeRelevant {
		t.Fatalf("expected documentation mode relevant, got %s", documentationMode)
	}
	if !includeContent {
		t.Fatalf("expected includeContent to be enabled")
	}
	if !tokens.enabled {
		t.Fatalf("expected tokens to be enabled")
	}
	if tokens.model != "custom" {
		t.Fatalf("expected token model custom, got %s", tokens.model)
	}
	if len(pathConfiguration.exclusionPatterns) != 1 || pathConfiguration.exclusionPatterns[0] != "vendor" {
		t.Fatalf("expected exclusion patterns to include vendor: %v", pathConfiguration.exclusionPatterns)
	}
	if !pathConfiguration.disableGitignore {
		t.Fatalf("expected gitignore to be disabled")
	}
	if !pathConfiguration.disableIgnoreFile {
		t.Fatalf("expected ignore file to be disabled")
	}
	if !pathConfiguration.includeGit {
		t.Fatalf("expected git directory to be included")
	}
}

func TestApplyStreamConfigurationRespectsCliOverrides(t *testing.T) {
	pathConfiguration := pathOptions{
		exclusionPatterns: []string{"cli"},
		disableGitignore:  true,
		disableIgnoreFile: true,
		includeGit:        false,
	}
	format := "cli"
	summaryEnabled := true
	documentationMode := types.DocumentationModeFull
	tokens := tokenOptions{enabled: true, model: "cli"}
	includeContent := true

	command := &cobra.Command{Use: "test"}
	command.Flags().StringVar(&format, formatFlagName, format, "")
	registerBooleanFlag(command.Flags(), &summaryEnabled, summaryFlagName, summaryEnabled, "")
	command.Flags().StringVar(&documentationMode, documentationFlagName, documentationMode, "")
	registerBooleanFlag(command.Flags(), &tokens.enabled, tokensFlagName, tokens.enabled, "")
	command.Flags().StringVar(&tokens.model, modelFlagName, tokens.model, "")
	command.Flags().StringArrayVarP(&pathConfiguration.exclusionPatterns, exclusionFlagName, exclusionFlagName, pathConfiguration.exclusionPatterns, "")
	registerBooleanFlag(command.Flags(), &pathConfiguration.disableGitignore, noGitignoreFlagName, pathConfiguration.disableGitignore, "")
	registerBooleanFlag(command.Flags(), &pathConfiguration.disableIgnoreFile, noIgnoreFlagName, pathConfiguration.disableIgnoreFile, "")
	registerBooleanFlag(command.Flags(), &pathConfiguration.includeGit, includeGitFlagName, pathConfiguration.includeGit, "")
	registerBooleanFlag(command.Flags(), &includeContent, contentFlagName, includeContent, "")
	docsAttempt := false

	// Simulate CLI overrides.
	if err := command.Flags().Set(formatFlagName, format); err != nil {
		t.Fatalf("set format flag: %v", err)
	}
	if err := command.Flags().Set(summaryFlagName, "true"); err != nil {
		t.Fatalf("set summary flag: %v", err)
	}
	if err := command.Flags().Set(documentationFlagName, types.DocumentationModeFull); err != nil {
		t.Fatalf("set doc flag: %v", err)
	}
	if err := command.Flags().Set(tokensFlagName, "true"); err != nil {
		t.Fatalf("set tokens flag: %v", err)
	}
	if err := command.Flags().Set(modelFlagName, tokens.model); err != nil {
		t.Fatalf("set model flag: %v", err)
	}
	if err := command.Flags().Set(exclusionFlagName, "cli"); err != nil {
		t.Fatalf("set exclusion flag: %v", err)
	}
	if err := command.Flags().Set(noGitignoreFlagName, "true"); err != nil {
		t.Fatalf("set gitignore flag: %v", err)
	}
	if err := command.Flags().Set(noIgnoreFlagName, "true"); err != nil {
		t.Fatalf("set ignore flag: %v", err)
	}
	if err := command.Flags().Set(includeGitFlagName, "false"); err != nil {
		t.Fatalf("set include git flag: %v", err)
	}
	if err := command.Flags().Set(contentFlagName, "true"); err != nil {
		t.Fatalf("set content flag: %v", err)
	}

	configuration := config.StreamCommandConfiguration{
		Format:         types.FormatXML,
		Summary:        boolPtr(false),
		Documentation:  boolPtr(false),
		IncludeContent: boolPtr(false),
		Tokens: config.TokenConfiguration{
			Enabled: boolPtr(false),
			Model:   "config",
		},
		Paths: config.PathConfiguration{
			Exclude:       []string{"config"},
			UseGitignore:  boolPtr(true),
			UseIgnoreFile: boolPtr(true),
			IncludeGit:    boolPtr(true),
		},
	}

	applyStreamConfiguration(command, configuration, &pathConfiguration, &format, &documentationMode, &summaryEnabled, &includeContent, &docsAttempt, &tokens)

	if format != "cli" {
		t.Fatalf("expected format to remain cli, got %s", format)
	}
	if !summaryEnabled {
		t.Fatalf("expected summary to remain true")
	}
	if documentationMode != types.DocumentationModeFull {
		t.Fatalf("expected documentation mode to remain full, got %s", documentationMode)
	}
	if !includeContent {
		t.Fatalf("expected includeContent to remain true")
	}
	if !tokens.enabled {
		t.Fatalf("expected tokens to remain enabled")
	}
	if tokens.model != "cli" {
		t.Fatalf("expected token model to remain cli, got %s", tokens.model)
	}
	if len(pathConfiguration.exclusionPatterns) != 1 || pathConfiguration.exclusionPatterns[0] != "cli" {
		t.Fatalf("expected exclusion patterns to remain cli: %v", pathConfiguration.exclusionPatterns)
	}
	if !pathConfiguration.disableGitignore {
		t.Fatalf("expected gitignore to remain disabled")
	}
	if !pathConfiguration.disableIgnoreFile {
		t.Fatalf("expected ignore file to remain disabled")
	}
	if pathConfiguration.includeGit {
		t.Fatalf("expected include git to remain false")
	}
}

func TestApplyCallChainConfigurationUsesDefaults(t *testing.T) {
	format := types.FormatJSON
	documentationMode := types.DocumentationModeDisabled
	depth := defaultCallChainDepth

	command := &cobra.Command{Use: "callchain"}
	command.Flags().StringVar(&format, formatFlagName, format, "")
	command.Flags().StringVar(&documentationMode, documentationFlagName, documentationMode, "")
	command.Flags().IntVar(&depth, callChainDepthFlagName, depth, "")
	docsAttempt := false

	configuration := config.CallChainConfiguration{
		Format:        types.FormatXML,
		Depth:         func() *int { value := 2; return &value }(),
		Documentation: boolPtr(true),
	}

	applyCallChainConfiguration(command, configuration, &format, &depth, &documentationMode, &docsAttempt)

	if format != types.FormatXML {
		t.Fatalf("expected format to change to xml")
	}
	if depth != 2 {
		t.Fatalf("expected depth to be updated to 2")
	}
	if documentationMode != types.DocumentationModeRelevant {
		t.Fatalf("expected documentation mode relevant, got %s", documentationMode)
	}
}

func TestTreeCommandRejectsInvalidFormat(t *testing.T) {
	var copyValue bool
	var copyOnlyValue bool
	command := createTreeCommand(&clipboardStub{}, &copyValue, &copyOnlyValue, nil)
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--format", "invalid-format"})

	err := command.Execute()
	if err == nil {
		t.Fatalf("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "Invalid format value") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
