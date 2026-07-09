// Package cli provides the command line interface.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tyemirov/ctx/internal/commands"
	"github.com/tyemirov/ctx/internal/config"
	"github.com/tyemirov/ctx/internal/discover"
	"github.com/tyemirov/ctx/internal/docs"
	"github.com/tyemirov/ctx/internal/docs/githubdoc"
	"github.com/tyemirov/ctx/internal/output"
	"github.com/tyemirov/ctx/internal/services/clipboard"
	"github.com/tyemirov/ctx/internal/services/mcp"
	"github.com/tyemirov/ctx/internal/services/stream"
	"github.com/tyemirov/ctx/internal/tokenizer"
	"github.com/tyemirov/ctx/internal/types"
	"github.com/tyemirov/ctx/internal/utils"
	"golang.org/x/sync/errgroup"
)

const (
	exclusionFlagName     = "e"
	noGitignoreFlagName   = "no-gitignore"
	noIgnoreFlagName      = "no-ignore"
	includeGitFlagName    = "git"
	formatFlagName        = "format"
	summaryFlagName       = "summary"
	tokensFlagName        = "tokens"
	modelFlagName         = "model"
	documentationFlagName = "doc"
	contentFlagName       = "content"
	versionFlagName       = "version"
	versionTemplate       = "ctx version: %s\n"
	defaultPath           = "."
	rootUse               = "ctx"
	rootShortDescription  = "ctx command line interface"
	rootLongDescription   = `ctx inspects project structure and source code.
It renders directory trees, shows file content, and analyzes call chains.
Use --format to select toon, raw, json, or xml output. Use --doc to include documentation for supported commands, and --version to print the application version.`
	versionFlagDescription    = "display application version"
	copyFlagName              = "copy"
	copyFlagDescription       = "copy command output to the system clipboard"
	copyFlagAlias             = "c"
	copyOnlyFlagName          = "copy-only"
	copyOnlyFlagAlias         = "co"
	copyOnlyFlagDescription   = "copy command output to the system clipboard without writing to stdout"
	configFlagName            = "config"
	configFlagDescription     = "path to an application configuration file"
	initFlagName              = "init"
	initFlagDescription       = "generate a configuration file (local or global)"
	forceFlagName             = "force"
	forceFlagDescription      = "overwrite configuration if it already exists"
	mcpFlagName               = "mcp"
	mcpFlagDescription        = "run the program as an MCP server"
	mcpListenAddress          = "127.0.0.1:0"
	mcpStartupMessageFormat   = "MCP server listening on %s\n"
	mcpFlagConflictMessage    = "--mcp cannot be combined with subcommands"
	mcpShutdownTimeout        = 5 * time.Second
	treeUse                   = "tree [paths...]"
	contentUse                = "content [paths...]"
	callchainUse              = "callchain <function>"
	bundleUse                 = "bundle"
	treeAlias                 = "t"
	contentAlias              = "c"
	callchainAlias            = "cc"
	treeShortDescription      = "display directory tree (" + treeAlias + ")"
	contentShortDescription   = "show file contents (" + contentAlias + ")"
	callchainShortDescription = "analyze call chains (" + callchainAlias + ")"
	bundleShortDescription    = "build goal-oriented execution context"

	// treeLongDescription provides detailed help for the tree command.
	treeLongDescription = `List directories and files for one or more paths.
Use --format to select toon, raw, json, or xml output.`
	// treeUsageExample demonstrates tree command usage.
	treeUsageExample = `  # Render the tree in XML format
  ctx tree --format xml ./cmd

  # Exclude vendor directory
  ctx tree -e vendor .`

	// contentLongDescription provides detailed help for the content command.
	contentLongDescription = `Display file content for provided paths.
Use --format to select toon, raw, json, or xml output and --doc to include collected documentation.`
	// contentUsageExample demonstrates content command usage.
	contentUsageExample = `  # Show project files with documentation
  ctx content --doc .

  # Display a file in raw format
  ctx content --format raw main.go`

	// callchainLongDescription provides detailed help for the callchain command.
	callchainLongDescription = `Analyze the call chain of a function.
Use --depth to control traversal depth, --format for output selection, and --doc to include documentation.`
	// callchainUsageExample demonstrates callchain command usage.
	callchainUsageExample = `  # Analyze call chain up to depth two in JSON
  ctx callchain fmt.Println --depth 2

  # Produce XML output including documentation
  ctx callchain mypkg.MyFunc --format xml --doc`

	bundleLongDescription = `Build a goal-oriented repository context bundle from a JSON request.
The bundle command emits the canonical context contract used by agentic execution. It supports json and toon output.`
	bundleUsageExample = `  # Render an execution context bundle in TOON
  ctx bundle --request /tmp/context-request.json --format toon`

	docUse              = "doc"
	docAlias            = "d"
	docShortDescription = "retrieve documentation from GitHub or the web (" + docAlias + ")"
	docLongDescription  = `Fetch and render documentation stored in a GitHub repository path or a public web page.

Required parameters:
  --path accepts owner/repo[/path], https://github.com/owner/repo[...path], or another https:// URL to capture.

Optional parameters:
  --ref selects the git branch, tag, or commit to read (GitHub repositories only).
  --rules points to a documentation cleanup rule set for pruning files.
  --doc controls the amount of fetched documentation included in the output.
  --web-depth controls link traversal depth for external pages (0–3; defaults to 1).
  --copy copies the rendered documentation to the system clipboard.`
	docUsageExample = `  # Use explicit repository coordinates
  ctx doc --path example/project/docs

  # Fetch documentation from the repository root on a branch
  ctx doc --path example/project --ref main

  # Derive coordinates from a repository URL
  ctx doc --path https://github.com/example/project/tree/main/docs

  # Capture documentation from a public site with depth 1
  ctx doc --path https://getbootstrap.com/docs/5.0/getting-started/introduction/ --web-depth 1`
	docDiscoverUse              = "discover [path]"
	docDiscoverShortDescription = "generate local dependency documentation"
	docDiscoverLongDescription  = `Scan a project for dependencies (Go modules, npm packages, Python requirements) and write curated documentation bundles to docs/dependencies.

Provide an optional path argument or --root to set the project directory. Use --format json to emit a manifest instead of human-readable output.`
	docDiscoverFormatText              = "text"
	docDiscoverFormatJSON              = "json"
	docDiscoverRootFlagName            = "root"
	docDiscoverOutputFlagName          = "output-dir"
	docDiscoverEcosystemsFlag          = "ecosystems"
	docDiscoverIncludeFlagName         = "include"
	docDiscoverExcludeFlagName         = "exclude"
	docDiscoverIncludeDevFlag          = "include-dev"
	docDiscoverIncludeIndirectFlagName = "include-indirect"
	docDiscoverRulesFlagName           = "rules"
	docDiscoverConcurrencyFlag         = "concurrency"
	docDiscoverFormatFlagName          = "format"
	docDiscoverAPIBaseFlagName         = "api-base"
	docDiscoverNPMBaseFlagName         = "npm-registry-base"
	docDiscoverPyPIBaseFlagName        = "pypi-registry-base"
	docDiscoverFormatDescription       = "output format (text|json)"
	docDiscoverSummaryTemplate         = "Dependencies processed: %d (written: %d, skipped: %d, failed: %d)\n"
	bundleRequestFlagName              = "request"
	bundleRequestFlagDescription       = "path to the context bundle request JSON"

	docWebDepthFlagName    = "web-depth"
	docWebDepthDescription = "maximum link depth for external pages (0-3); 0 fetches only the provided page"

	docsAttemptFlagName        = "docs-attempt"
	docsAttemptFlagDescription = "attempt to retrieve GitHub documentation for imported modules"
	docsAPIBaseFlagName        = "docs-api-base"
	docsAPIBaseFlagDescription = "GitHub API base URL for documentation attempts"

	docCoordinatesRequiredMessage     = "doc command requires repository coordinates or an HTTPS documentation URL"
	docCoordinatesGuidanceMessage     = "Provide --path with owner/repo[/path], a GitHub URL, or another https:// documentation URL."
	docCoordinatesHelpMessage         = `Run "ctx doc --help" for complete flag help.`
	docMissingCoordinatesErrorMessage = docCoordinatesRequiredMessage + ". " + docCoordinatesGuidanceMessage + " " + docCoordinatesHelpMessage
	docMissingPathErrorMessage        = "doc command requires a documentation path. " + docCoordinatesGuidanceMessage + " " + docCoordinatesHelpMessage
	docPathFlagDescription            = "Repository coordinates (owner/repo[/path]), GitHub URL (tree/blob), or https:// documentation link"
	docReferenceFlagDescription       = "Git reference to fetch (branch, tag, or commit)"
	docRulesFlagDescription           = "path to cleanup rules file for documentation pruning"
	docAPIBaseFlagDescription         = "GitHub API base URL"

	repositoryRefFlagName     = "ref"
	repositoryPathFlagName    = "path"
	repositoryRulesFlagName   = "rules"
	repositoryAPIBaseFlagName = "api-base"
	githubTokenEnvPrimary     = "GH_TOKEN"
	githubTokenEnvSecondary   = "GITHUB_TOKEN"
	githubTokenEnvTertiary    = "GITHUB_API_TOKEN"

	docTitleTemplate    = "# Documentation for %s/%s (%s)\n\n"
	docHeaderTemplate   = "## %s\n\n"
	docSectionSeparator = "\n\n"
	githubTreeSegment   = "tree"
	githubBlobSegment   = "blob"

	callChainDepthFlagName          = "depth"
	unsupportedCommandMessage       = "unsupported command"
	defaultCallChainDepth           = 1
	callChainDepthDescription       = "traversal depth"
	exclusionFlagDescription        = "exclude path pattern"
	disableGitignoreFlagDescription = "do not use .gitignore"
	disableIgnoreFlagDescription    = "do not use .ignore"
	includeGitFlagDescription       = "include git directory"
	formatFlagDescription           = "output format"
	summaryFlagDescription          = "include summary of resulting files"
	tokensFlagDescription           = "include token counts"
	modelFlagDescription            = "tokenizer model to use for token counting"
	defaultTokenizerModelName       = "gpt-4o"
	documentationFlagDescription    = "documentation mode (disabled|relevant|full)"
	contentFlagDescription          = "include file content in output"
	invalidFormatMessage            = "Invalid format value '%s'"
	invalidBundleFormatMessage      = "bundle supports toon or json output"
	invalidDocumentationModeMessage = "invalid documentation mode '%s'"
	warningSkipPathFormat           = "Warning: skipping %s: %v\n"
	warningTokenCountFormat         = "Warning: failed to count tokens for %s: %v\n"
	workingDirectoryErrorFormat     = "unable to determine working directory: %w"
	// errorAbsolutePathFormat reports failure to resolve an absolute path.
	errorAbsolutePathFormat = "abs failed for '%s': %w"
	// errorPathMissingFormat reports a missing path.
	errorPathMissingFormat = "path '%s' does not exist"
	// errorStatFormat reports failure to retrieve file statistics.
	errorStatFormat = "stat failed for '%s': %w"
	// errorNoValidPaths indicates that all paths are invalid.
	errorNoValidPaths              = "no valid paths"
	clipboardCopyErrorFormat       = "failed to copy output to clipboard: %w"
	clipboardServiceMissingMessage = "clipboard functionality requested but unavailable"
	configurationLoadErrorFormat   = "failed to load configuration: %w"
	configurationInitSuccessFormat = "configuration written to %s\n"
)

// isSupportedFormat reports whether the provided format is recognized.
func isSupportedFormat(format string) bool {
	switch format {
	case types.FormatRaw, types.FormatToon, types.FormatJSON, types.FormatXML:
		return true
	default:
		return false
	}
}

// Execute runs the ctx application.
func Execute() error {
	rootCommand := createRootCommand(clipboard.NewService())
	normalizedArguments := normalizeBooleanFlagArguments(rootCommand, os.Args[1:])
	rootCommand.SetArgs(normalizedArguments)
	return rootCommand.Execute()
}

// createRootCommand builds the root Cobra command.
func createRootCommand(clipboardProvider clipboard.Copier) *cobra.Command {
	var showVersion bool
	var copyFlagValue bool
	var copyOnlyFlagValue bool
	var explicitConfigPath string
	var initTarget string
	var forceInit bool
	var applicationConfig config.ApplicationConfiguration
	var configurationLoaded bool
	var runMCP bool

	callChainService := commands.NewCallChainService()

	rootCommand := &cobra.Command{
		Use:          rootUse,
		Short:        rootShortDescription,
		Long:         rootLongDescription,
		SilenceUsage: true,
		RunE: func(command *cobra.Command, arguments []string) error {
			if runMCP {
				if len(arguments) > 0 {
					return fmt.Errorf("%s accepts no arguments", mcpFlagName)
				}
				return startMCPServer(command.Context(), command.OutOrStdout())
			}
			return command.Help()
		},
		PersistentPreRunE: func(command *cobra.Command, arguments []string) error {
			if showVersion {
				fmt.Printf(versionTemplate, utils.GetApplicationVersion())
				os.Exit(0)
			}
			if runMCP && command.Name() != rootUse {
				return fmt.Errorf(mcpFlagConflictMessage)
			}
			if configurationLoaded {
				return nil
			}
			workingDirectory, workingDirectoryError := os.Getwd()
			if workingDirectoryError != nil {
				return fmt.Errorf(workingDirectoryErrorFormat, workingDirectoryError)
			}
			if command.InheritedFlags().Changed(initFlagName) {
				target := config.InitTarget(initTarget)
				if target == "" {
					target = config.InitTargetLocal
				}
				initializedPath, initError := config.InitializeConfiguration(config.InitOptions{
					Target:           target,
					Force:            forceInit,
					WorkingDirectory: workingDirectory,
				})
				if initError != nil {
					return initError
				}
				fmt.Fprintf(command.OutOrStdout(), configurationInitSuccessFormat, initializedPath)
				os.Exit(0)
			}
			loadedConfiguration, loadError := config.LoadApplicationConfiguration(config.LoadOptions{
				WorkingDirectory: workingDirectory,
				ExplicitFilePath: explicitConfigPath,
			})
			if loadError != nil {
				return fmt.Errorf(configurationLoadErrorFormat, loadError)
			}
			applicationConfig = loadedConfiguration
			configurationLoaded = true
			return nil
		},
	}
	registerBooleanFlag(rootCommand.PersistentFlags(), &showVersion, versionFlagName, false, versionFlagDescription)
	registerBooleanFlag(rootCommand.PersistentFlags(), &copyFlagValue, copyFlagName, false, copyFlagDescription)
	registerBooleanFlag(rootCommand.PersistentFlags(), &copyFlagValue, copyFlagAlias, false, copyFlagDescription)
	registerBooleanFlag(rootCommand.PersistentFlags(), &copyOnlyFlagValue, copyOnlyFlagName, false, copyOnlyFlagDescription)
	registerBooleanFlag(rootCommand.PersistentFlags(), &copyOnlyFlagValue, copyOnlyFlagAlias, false, copyOnlyFlagDescription)
	if copyAliasFlag := rootCommand.PersistentFlags().Lookup(copyFlagAlias); copyAliasFlag != nil {
		copyAliasFlag.Hidden = true
	}
	if aliasFlag := rootCommand.PersistentFlags().Lookup(copyOnlyFlagAlias); aliasFlag != nil {
		aliasFlag.Hidden = true
	}
	rootCommand.PersistentFlags().StringVar(&explicitConfigPath, configFlagName, "", configFlagDescription)
	rootCommand.PersistentFlags().StringVar(&initTarget, initFlagName, "", initFlagDescription)
	if initFlag := rootCommand.PersistentFlags().Lookup(initFlagName); initFlag != nil {
		initFlag.NoOptDefVal = string(config.InitTargetLocal)
	}
	registerBooleanFlag(rootCommand.PersistentFlags(), &forceInit, forceFlagName, false, forceFlagDescription)
	registerBooleanFlag(rootCommand.PersistentFlags(), &runMCP, mcpFlagName, false, mcpFlagDescription)
	rootCommand.AddCommand(
		createTreeCommand(clipboardProvider, &copyFlagValue, &copyOnlyFlagValue, &applicationConfig),
		createContentCommand(clipboardProvider, &copyFlagValue, &copyOnlyFlagValue, &applicationConfig),
		createCallChainCommand(clipboardProvider, &copyFlagValue, &copyOnlyFlagValue, &applicationConfig, callChainService),
		createBundleCommand(clipboardProvider, &copyFlagValue, &copyOnlyFlagValue),
		createDocCommand(clipboardProvider, &copyFlagValue, &copyOnlyFlagValue, &applicationConfig),
	)
	rootCommand.InitDefaultHelpCmd()
	rootCommand.InitDefaultCompletionCmd()
	return rootCommand
}

// pathOptions stores configuration for path-related flags.
type pathOptions struct {
	exclusionPatterns []string
	disableGitignore  bool
	disableIgnoreFile bool
	includeGit        bool
}

type tokenOptions struct {
	enabled bool
	model   string
}

func (options tokenOptions) toConfig(workingDirectory string) tokenizer.Config {
	return tokenizer.Config{
		Model:            options.model,
		WorkingDirectory: workingDirectory,
	}
}

// addPathFlags registers path-related flags on the command.
func addPathFlags(command *cobra.Command, options *pathOptions) {
	command.Flags().StringArrayVarP(&options.exclusionPatterns, exclusionFlagName, exclusionFlagName, nil, exclusionFlagDescription)
	registerBooleanFlag(command.Flags(), &options.disableGitignore, noGitignoreFlagName, false, disableGitignoreFlagDescription)
	registerBooleanFlag(command.Flags(), &options.disableIgnoreFile, noIgnoreFlagName, false, disableIgnoreFlagDescription)
	registerBooleanFlag(command.Flags(), &options.includeGit, includeGitFlagName, false, includeGitFlagDescription)
}

// createTreeCommand returns the tree subcommand.
func createTreeCommand(clipboardProvider clipboard.Copier, copyFlag *bool, copyOnlyFlag *bool, applicationConfig *config.ApplicationConfiguration) *cobra.Command {
	var pathConfiguration pathOptions
	var outputFormat string = types.FormatToon
	var summaryEnabled bool = true
	var tokenConfiguration tokenOptions
	tokenConfiguration.model = defaultTokenizerModelName
	var includeContent bool

	treeCommand := &cobra.Command{
		Use:     treeUse,
		Aliases: []string{treeAlias},
		Short:   treeShortDescription,
		Long:    treeLongDescription,
		Example: treeUsageExample,
		Args:    cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, arguments []string) error {
			if applicationConfig != nil {
				applyStreamConfiguration(command, applicationConfig.Tree, &pathConfiguration, &outputFormat, nil, &summaryEnabled, &includeContent, nil, &tokenConfiguration)
			}
			if len(arguments) == 0 {
				arguments = []string{defaultPath}
			}
			outputFormatLower := strings.ToLower(outputFormat)
			if !isSupportedFormat(outputFormatLower) {
				return fmt.Errorf(invalidFormatMessage, outputFormatLower)
			}
			copyFlagValue := copyFlag != nil && *copyFlag
			copyOnlyFlagValue := copyOnlyFlag != nil && *copyOnlyFlag
			var configCopy, configCopyOnly *bool
			if applicationConfig != nil {
				copySettings := applicationConfig.Tree.CopySettings()
				configCopy = copySettings.Copy
				configCopyOnly = copySettings.CopyOnly
			}
			copyEnabledForCommand, copyOnlyForCommand := resolveClipboardPreferences(command, copyFlagValue, copyOnlyFlagValue, configCopy, configCopyOnly)
			descriptor := commandDescriptor{
				ctx:                command.Context(),
				commandName:        types.CommandTree,
				paths:              arguments,
				exclusionPatterns:  pathConfiguration.exclusionPatterns,
				useGitignore:       !pathConfiguration.disableGitignore,
				useIgnoreFile:      !pathConfiguration.disableIgnoreFile,
				includeGit:         pathConfiguration.includeGit,
				callChainDepth:     defaultCallChainDepth,
				format:             outputFormatLower,
				documentation:      documentationOptions{},
				summaryEnabled:     summaryEnabled,
				includeContent:     includeContent,
				tokenConfiguration: tokenConfiguration,
				outputWriter:       command.OutOrStdout(),
				errorWriter:        command.ErrOrStderr(),
				clipboardEnabled:   copyEnabledForCommand,
				copyOnly:           copyOnlyForCommand,
				clipboard:          clipboardProvider,
			}
			return runTool(descriptor)
		},
	}

	addPathFlags(treeCommand, &pathConfiguration)
	treeCommand.Flags().StringVar(&outputFormat, formatFlagName, types.FormatToon, formatFlagDescription)
	registerBooleanFlag(treeCommand.Flags(), &summaryEnabled, summaryFlagName, true, summaryFlagDescription)
	registerBooleanFlag(treeCommand.Flags(), &tokenConfiguration.enabled, tokensFlagName, false, tokensFlagDescription)
	treeCommand.Flags().StringVar(&tokenConfiguration.model, modelFlagName, defaultTokenizerModelName, modelFlagDescription)
	registerBooleanFlag(treeCommand.Flags(), &includeContent, contentFlagName, false, contentFlagDescription)
	return treeCommand
}

// createContentCommand returns the content subcommand.
func createContentCommand(clipboardProvider clipboard.Copier, copyFlag *bool, copyOnlyFlag *bool, applicationConfig *config.ApplicationConfiguration) *cobra.Command {
	var pathConfiguration pathOptions
	var outputFormat string = types.FormatToon
	documentationMode := types.DocumentationModeDisabled
	var summaryEnabled bool = true
	var tokenConfiguration tokenOptions
	tokenConfiguration.model = defaultTokenizerModelName
	includeContent := true
	var docsAttempt bool
	var docsAPIBase string

	contentCommand := &cobra.Command{
		Use:     contentUse,
		Aliases: []string{contentAlias},
		Short:   contentShortDescription,
		Long:    contentLongDescription,
		Example: contentUsageExample,
		Args:    cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, arguments []string) error {
			if applicationConfig != nil {
				applyStreamConfiguration(command, applicationConfig.Content, &pathConfiguration, &outputFormat, &documentationMode, &summaryEnabled, &includeContent, &docsAttempt, &tokenConfiguration)
			}
			if len(arguments) == 0 {
				arguments = []string{defaultPath}
			}
			if docsAttempt {
				docFlag := command.Flags().Lookup(documentationFlagName)
				if (docFlag == nil || !docFlag.Changed) && documentationMode != types.DocumentationModeFull {
					documentationMode = types.DocumentationModeFull
				}
			}
			outputFormatLower := strings.ToLower(outputFormat)
			if !isSupportedFormat(outputFormatLower) {
				return fmt.Errorf(invalidFormatMessage, outputFormatLower)
			}
			effectiveDocumentationMode, documentationErr := normalizeDocumentationMode(documentationMode)
			if documentationErr != nil {
				return fmt.Errorf("configure documentation: %w", documentationErr)
			}
			tokenResolver := newEnvironmentGitHubTokenResolver(
				githubTokenEnvPrimary,
				githubTokenEnvSecondary,
				githubTokenEnvTertiary,
			)
			documentationOptions, documentationOptionsErr := newDocumentationOptions(documentationOptionsParameters{
				Mode:          effectiveDocumentationMode,
				RemoteEnabled: docsAttempt,
				RemoteAPIBase: docsAPIBase,
				TokenResolver: tokenResolver,
			})
			if documentationOptionsErr != nil {
				return fmt.Errorf("configure documentation: %w", documentationOptionsErr)
			}
			copyFlagValue := copyFlag != nil && *copyFlag
			copyOnlyFlagValue := copyOnlyFlag != nil && *copyOnlyFlag
			var configCopy, configCopyOnly *bool
			if applicationConfig != nil {
				copySettings := applicationConfig.Content.CopySettings()
				configCopy = copySettings.Copy
				configCopyOnly = copySettings.CopyOnly
			}
			copyEnabledForCommand, copyOnlyForCommand := resolveClipboardPreferences(command, copyFlagValue, copyOnlyFlagValue, configCopy, configCopyOnly)
			descriptor := commandDescriptor{
				ctx:                command.Context(),
				commandName:        types.CommandContent,
				paths:              arguments,
				exclusionPatterns:  pathConfiguration.exclusionPatterns,
				useGitignore:       !pathConfiguration.disableGitignore,
				useIgnoreFile:      !pathConfiguration.disableIgnoreFile,
				includeGit:         pathConfiguration.includeGit,
				callChainDepth:     defaultCallChainDepth,
				format:             outputFormatLower,
				documentation:      documentationOptions,
				summaryEnabled:     summaryEnabled,
				includeContent:     includeContent,
				tokenConfiguration: tokenConfiguration,
				outputWriter:       command.OutOrStdout(),
				errorWriter:        command.ErrOrStderr(),
				clipboardEnabled:   copyEnabledForCommand,
				copyOnly:           copyOnlyForCommand,
				clipboard:          clipboardProvider,
			}
			return runTool(descriptor)
		},
	}

	addPathFlags(contentCommand, &pathConfiguration)
	contentCommand.Flags().StringVar(&outputFormat, formatFlagName, types.FormatToon, formatFlagDescription)
	contentCommand.Flags().StringVar(&documentationMode, documentationFlagName, types.DocumentationModeDisabled, documentationFlagDescription)
	if docFlag := contentCommand.Flags().Lookup(documentationFlagName); docFlag != nil {
		docFlag.NoOptDefVal = types.DocumentationModeRelevant
	}
	registerBooleanFlag(contentCommand.Flags(), &summaryEnabled, summaryFlagName, true, summaryFlagDescription)
	registerBooleanFlag(contentCommand.Flags(), &tokenConfiguration.enabled, tokensFlagName, false, tokensFlagDescription)
	contentCommand.Flags().StringVar(&tokenConfiguration.model, modelFlagName, defaultTokenizerModelName, modelFlagDescription)
	registerBooleanFlag(contentCommand.Flags(), &includeContent, contentFlagName, true, contentFlagDescription)
	registerBooleanFlag(contentCommand.Flags(), &docsAttempt, docsAttemptFlagName, false, docsAttemptFlagDescription)
	contentCommand.Flags().StringVar(&docsAPIBase, docsAPIBaseFlagName, "", docsAPIBaseFlagDescription)
	if docsAPIFlag := contentCommand.Flags().Lookup(docsAPIBaseFlagName); docsAPIFlag != nil {
		docsAPIFlag.Hidden = true
	}
	return contentCommand
}

// createCallChainCommand returns the callchain subcommand.
func createCallChainCommand(clipboardProvider clipboard.Copier, copyFlag *bool, copyOnlyFlag *bool, applicationConfig *config.ApplicationConfiguration, callChainService commands.CallChainService) *cobra.Command {
	if callChainService == nil {
		callChainService = commands.NewCallChainService()
	}
	var outputFormat string = types.FormatToon
	documentationMode := types.DocumentationModeDisabled
	var callChainDepth int = defaultCallChainDepth
	var docsAttempt bool
	var docsAPIBase string

	callChainCommand := &cobra.Command{
		Use:     callchainUse,
		Aliases: []string{callchainAlias},
		Short:   callchainShortDescription,
		Long:    callchainLongDescription,
		Example: callchainUsageExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			if applicationConfig != nil {
				applyCallChainConfiguration(command, applicationConfig.CallChain, &outputFormat, &callChainDepth, &documentationMode, &docsAttempt)
			}
			if docsAttempt {
				docFlag := command.Flags().Lookup(documentationFlagName)
				if (docFlag == nil || !docFlag.Changed) && documentationMode != types.DocumentationModeFull {
					documentationMode = types.DocumentationModeFull
				}
			}
			outputFormatLower := strings.ToLower(outputFormat)
			if !isSupportedFormat(outputFormatLower) {
				return fmt.Errorf(invalidFormatMessage, outputFormatLower)
			}
			effectiveDocumentationMode, documentationErr := normalizeDocumentationMode(documentationMode)
			if documentationErr != nil {
				return documentationErr
			}
			tokenResolver := newEnvironmentGitHubTokenResolver(
				githubTokenEnvPrimary,
				githubTokenEnvSecondary,
				githubTokenEnvTertiary,
			)
			documentationOptions, documentationOptionsErr := newDocumentationOptions(documentationOptionsParameters{
				Mode:          effectiveDocumentationMode,
				RemoteEnabled: docsAttempt,
				RemoteAPIBase: docsAPIBase,
				TokenResolver: tokenResolver,
			})
			if documentationOptionsErr != nil {
				return fmt.Errorf("configure documentation: %w", documentationOptionsErr)
			}
			copyFlagValue := copyFlag != nil && *copyFlag
			copyOnlyFlagValue := copyOnlyFlag != nil && *copyOnlyFlag
			var configCopy, configCopyOnly *bool
			if applicationConfig != nil {
				copySettings := applicationConfig.CallChain.CopySettings()
				configCopy = copySettings.Copy
				configCopyOnly = copySettings.CopyOnly
			}
			copyEnabledForCommand, copyOnlyForCommand := resolveClipboardPreferences(command, copyFlagValue, copyOnlyFlagValue, configCopy, configCopyOnly)
			descriptor := commandDescriptor{
				ctx:                command.Context(),
				commandName:        types.CommandCallChain,
				paths:              []string{arguments[0]},
				exclusionPatterns:  nil,
				useGitignore:       true,
				useIgnoreFile:      true,
				includeGit:         false,
				callChainDepth:     callChainDepth,
				format:             outputFormatLower,
				documentation:      documentationOptions,
				summaryEnabled:     false,
				includeContent:     false,
				tokenConfiguration: tokenOptions{},
				outputWriter:       command.OutOrStdout(),
				errorWriter:        command.ErrOrStderr(),
				clipboardEnabled:   copyEnabledForCommand,
				copyOnly:           copyOnlyForCommand,
				clipboard:          clipboardProvider,
				callChainService:   callChainService,
			}
			return runTool(descriptor)
		},
	}

	callChainCommand.Flags().StringVar(&outputFormat, formatFlagName, types.FormatToon, formatFlagDescription)
	callChainCommand.Flags().StringVar(&documentationMode, documentationFlagName, types.DocumentationModeDisabled, documentationFlagDescription)
	if docFlag := callChainCommand.Flags().Lookup(documentationFlagName); docFlag != nil {
		docFlag.NoOptDefVal = types.DocumentationModeRelevant
	}
	callChainCommand.Flags().IntVar(&callChainDepth, callChainDepthFlagName, defaultCallChainDepth, callChainDepthDescription)
	registerBooleanFlag(callChainCommand.Flags(), &docsAttempt, docsAttemptFlagName, false, docsAttemptFlagDescription)
	callChainCommand.Flags().StringVar(&docsAPIBase, docsAPIBaseFlagName, "", docsAPIBaseFlagDescription)
	if docsAPIFlag := callChainCommand.Flags().Lookup(docsAPIBaseFlagName); docsAPIFlag != nil {
		docsAPIFlag.Hidden = true
	}
	return callChainCommand
}

func createBundleCommand(clipboardProvider clipboard.Copier, copyFlag *bool, copyOnlyFlag *bool) *cobra.Command {
	var outputFormat string = types.FormatToon
	var requestPath string

	bundleCommand := &cobra.Command{
		Use:     bundleUse,
		Short:   bundleShortDescription,
		Long:    bundleLongDescription,
		Example: bundleUsageExample,
		Args:    cobra.NoArgs,
		RunE: func(command *cobra.Command, arguments []string) error {
			outputFormatLower := strings.ToLower(outputFormat)
			if outputFormatLower != types.FormatToon && outputFormatLower != types.FormatJSON {
				return fmt.Errorf(invalidBundleFormatMessage)
			}
			if strings.TrimSpace(requestPath) == "" {
				return fmt.Errorf("%s is required", bundleRequestFlagName)
			}
			requestBytes, readErr := os.ReadFile(requestPath)
			if readErr != nil {
				return fmt.Errorf("read bundle request: %w", readErr)
			}
			var request types.ContextBundleRequest
			if decodeErr := json.Unmarshal(requestBytes, &request); decodeErr != nil {
				return fmt.Errorf("decode bundle request: %w", decodeErr)
			}
			if strings.TrimSpace(request.Model) == "" {
				request.Model = defaultTokenizerModelName
			}
			counter, resolvedModel, counterErr := tokenizer.NewCounter(tokenizer.Config{
				Model:            request.Model,
				WorkingDirectory: request.RepositoryRoot,
			})
			if counterErr != nil {
				if errors.Is(counterErr, tokenizer.ErrHelperUnavailable) {
					return fmt.Errorf("token helper unavailable: %w", counterErr)
				}
				return counterErr
			}
			bundle, bundleErr := commands.BuildContextBundle(command.Context(), commands.ContextBundleOptions{
				Request:      request,
				TokenCounter: counter,
				TokenModel:   resolvedModel,
			})
			if bundleErr != nil {
				return bundleErr
			}
			renderedOutput := output.RenderContextBundleToon(bundle)
			if outputFormatLower == types.FormatJSON {
				renderedJSON, renderErr := output.RenderContextBundleJSON(bundle)
				if renderErr != nil {
					return renderErr
				}
				renderedOutput = renderedJSON
			}
			writer := command.OutOrStdout()
			copyEnabled := copyFlag != nil && *copyFlag
			copyOnlyEnabled := copyOnlyFlag != nil && *copyOnlyFlag
			if copyOnlyEnabled {
				copyEnabled = true
			}
			var clipboardBuffer *bytes.Buffer
			if copyEnabled {
				if clipboardProvider == nil {
					return errors.New(clipboardServiceMissingMessage)
				}
				clipboardBuffer = &bytes.Buffer{}
				if copyOnlyEnabled {
					writer = clipboardBuffer
				} else {
					writer = io.MultiWriter(writer, clipboardBuffer)
				}
			}
			if _, writeErr := fmt.Fprintln(writer, renderedOutput); writeErr != nil {
				return writeErr
			}
			if clipboardBuffer != nil {
				if copyErr := clipboardProvider.Copy(clipboardBuffer.String()); copyErr != nil {
					return fmt.Errorf(clipboardCopyErrorFormat, copyErr)
				}
			}
			return nil
		},
	}
	bundleCommand.Flags().StringVar(&requestPath, bundleRequestFlagName, "", bundleRequestFlagDescription)
	bundleCommand.Flags().StringVar(&outputFormat, formatFlagName, types.FormatToon, formatFlagDescription)
	return bundleCommand
}

func createDocCommand(clipboardProvider clipboard.Copier, copyFlag *bool, copyOnlyFlag *bool, applicationConfig *config.ApplicationConfiguration) *cobra.Command {
	var repositoryPathSpec string
	var repositoryReference string
	var rulesPath string
	var apiBase string
	var webDepth int = 1
	documentationMode := types.DocumentationModeFull

	docCommand := &cobra.Command{
		Use:     docUse,
		Aliases: []string{docAlias},
		Short:   docShortDescription,
		Long:    docLongDescription,
		Example: docUsageExample,
		Args:    cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, arguments []string) error {
			if len(arguments) > 0 {
				if len(arguments) == 1 && command.Flags().Changed(documentationFlagName) {
					normalizedMode, normalizeErr := normalizeDocumentationMode(arguments[0])
					if normalizeErr != nil {
						return normalizeErr
					}
					documentationMode = normalizedMode
				} else {
					return fmt.Errorf("%s accepts no arguments", docUse)
				}
			}
			mode, modeErr := normalizeDocumentationMode(documentationMode)
			if modeErr != nil {
				return modeErr
			}
			writer := command.OutOrStdout()
			copyEnabled := copyFlag != nil && *copyFlag
			copyOnlyEnabled := copyOnlyFlag != nil && *copyOnlyFlag
			if copyOnlyEnabled {
				copyEnabled = true
			}
			trimmedPath := strings.TrimSpace(repositoryPathSpec)
			if isWebDocumentationPath(trimmedPath) {
				webOptions := docWebCommandOptions{
					Path:             trimmedPath,
					Depth:            webDepth,
					ClipboardEnabled: copyEnabled,
					CopyOnly:         copyOnlyEnabled,
					Clipboard:        clipboardProvider,
					Writer:           writer,
				}
				return runDocWebCommand(command.Context(), webOptions)
			}
			coordinates, coordinatesErr := resolveRepositoryCoordinates(repositoryPathSpec, "", "", repositoryReference, "")
			if coordinatesErr != nil {
				return coordinatesErr
			}
			var ruleSet githubdoc.RuleSet
			if rulesPath != "" {
				loadedRuleSet, loadErr := githubdoc.LoadRuleSet(rulesPath)
				if loadErr != nil {
					return loadErr
				}
				ruleSet = loadedRuleSet
			}
			tokenResolver := newEnvironmentGitHubTokenResolver(
				githubTokenEnvPrimary,
				githubTokenEnvSecondary,
				githubTokenEnvTertiary,
			)
			documentationOptions, documentationErr := newDocumentationOptions(documentationOptionsParameters{
				Mode:          mode,
				TokenResolver: tokenResolver,
			})
			if documentationErr != nil {
				return documentationErr
			}
			options := docCommandOptions{
				Coordinates:      coordinates,
				RuleSet:          ruleSet,
				Documentation:    documentationOptions,
				APIBase:          apiBase,
				ClipboardEnabled: copyEnabled,
				CopyOnly:         copyOnlyEnabled,
				Clipboard:        clipboardProvider,
				Writer:           writer,
			}
			if runErr := runDocCommand(command.Context(), options); runErr != nil {
				return runErr
			}
			return nil
		},
	}

	docCommand.Flags().StringVar(&repositoryPathSpec, repositoryPathFlagName, "", docPathFlagDescription)
	docCommand.Flags().StringVar(&repositoryReference, repositoryRefFlagName, "", docReferenceFlagDescription)
	docCommand.Flags().StringVar(&rulesPath, repositoryRulesFlagName, "", docRulesFlagDescription)
	docCommand.Flags().StringVar(&apiBase, repositoryAPIBaseFlagName, "", docAPIBaseFlagDescription)
	docCommand.Flags().IntVar(&webDepth, docWebDepthFlagName, 1, docWebDepthDescription)
	docCommand.Flags().StringVar(&documentationMode, documentationFlagName, types.DocumentationModeFull, documentationFlagDescription)
	if docFlag := docCommand.Flags().Lookup(documentationFlagName); docFlag != nil {
		docFlag.NoOptDefVal = types.DocumentationModeFull
	}
	if apiFlag := docCommand.Flags().Lookup(repositoryAPIBaseFlagName); apiFlag != nil {
		apiFlag.Hidden = true
	}
	docCommand.AddCommand(createDocDiscoverCommand(clipboardProvider, copyFlag, copyOnlyFlag, applicationConfig))
	return docCommand
}

func createDocDiscoverCommand(clipboardProvider clipboard.Copier, copyFlag *bool, copyOnlyFlag *bool, applicationConfig *config.ApplicationConfiguration) *cobra.Command {
	var rootPath string
	var outputDir string
	var ecosystems []string
	var includePatterns []string
	var excludePatterns []string
	var includeDev bool
	var includeIndirect bool
	var rulesPath string
	var concurrency int
	var format string
	var apiBase string
	var npmRegistryBase string
	var pypiRegistryBase string

	command := &cobra.Command{
		Use:   docDiscoverUse,
		Short: docDiscoverShortDescription,
		Long:  docDiscoverLongDescription,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, arguments []string) error {
			var configDefaults config.DocDiscoverConfiguration
			if applicationConfig != nil {
				configDefaults = applicationConfig.DocDiscover
			}
			if len(arguments) > 0 {
				rootPath = arguments[0]
			}
			if rootPath == "" {
				rootPath = defaultPath
			}
			selectedOutputDir := resolveStringOption(cmd, docDiscoverOutputFlagName, outputDir, configDefaults.OutputDir)
			selectedRules := resolveStringOption(cmd, docDiscoverRulesFlagName, rulesPath, configDefaults.Rules)
			selectedInclude := resolveSliceOption(cmd, docDiscoverIncludeFlagName, includePatterns, configDefaults.Include)
			selectedExclude := resolveSliceOption(cmd, docDiscoverExcludeFlagName, excludePatterns, configDefaults.Exclude)
			selectedEcosystems := resolveSliceOption(cmd, docDiscoverEcosystemsFlag, ecosystems, configDefaults.Ecosystems)
			selectedFormat := format
			if selectedFormat == "" {
				selectedFormat = docDiscoverFormatText
			}
			selectedConcurrency := resolveIntOption(cmd, docDiscoverConcurrencyFlag, concurrency, configDefaults.Concurrency)
			selectedIncludeDev := resolveBoolOption(cmd, docDiscoverIncludeDevFlag, includeDev, configDefaults.IncludeDev)
			selectedIncludeIndirect := resolveBoolOption(cmd, docDiscoverIncludeIndirectFlagName, includeIndirect, configDefaults.IncludeIndirect)
			selectedNPM := resolveStringOption(cmd, docDiscoverNPMBaseFlagName, npmRegistryBase, configDefaults.NPMRegistry)
			selectedPyPI := resolveStringOption(cmd, docDiscoverPyPIBaseFlagName, pypiRegistryBase, configDefaults.PyPIRegistry)
			selectedAPIBase := strings.TrimSpace(apiBase)

			ruleSet, ruleErr := loadRuleSet(selectedRules)
			if ruleErr != nil {
				return ruleErr
			}
			ecosystemSet, ecosystemErr := parseEcosystemSelection(selectedEcosystems)
			if ecosystemErr != nil {
				return ecosystemErr
			}

			tokenResolver := newEnvironmentGitHubTokenResolver(
				githubTokenEnvPrimary,
				githubTokenEnvSecondary,
				githubTokenEnvTertiary,
			)
			authorizationToken, tokenErr := tokenResolver.Resolve()
			if tokenErr != nil && !errors.Is(tokenErr, errGitHubTokenMissing) {
				return fmt.Errorf("resolve GitHub token: %w", tokenErr)
			}

			copySettings := configDefaults.CopySettings()
			var configCopy, configCopyOnly *bool
			if copySettings.Copy != nil {
				configCopy = copySettings.Copy
			}
			if copySettings.CopyOnly != nil {
				configCopyOnly = copySettings.CopyOnly
			}

			cliCopy := copyFlag != nil && *copyFlag
			cliCopyOnly := copyOnlyFlag != nil && *copyOnlyFlag
			copyEnabled, copyOnlyEnabled := resolveClipboardPreferences(cmd, cliCopy, cliCopyOnly, configCopy, configCopyOnly)

			options := discover.Options{
				RootPath:           rootPath,
				OutputDir:          selectedOutputDir,
				Ecosystems:         ecosystemSet,
				IncludePatterns:    utils.DeduplicatePatterns(selectedInclude),
				ExcludePatterns:    utils.DeduplicatePatterns(selectedExclude),
				IncludeDev:         selectedIncludeDev,
				IncludeIndirect:    selectedIncludeIndirect,
				RuleSet:            ruleSet,
				Concurrency:        selectedConcurrency,
				APIBase:            selectedAPIBase,
				AuthorizationToken: authorizationToken,
				NPMRegistryBase:    selectedNPM,
				PyPIRegistryBase:   selectedPyPI,
			}
			runner := discover.NewRunner(options)
			summary, runErr := runner.Run(cmd.Context())
			if runErr != nil {
				return runErr
			}
			formattedOutput, renderErr := renderDocDiscoverOutput(summary, selectedFormat)
			if renderErr != nil {
				return renderErr
			}

			writer := cmd.OutOrStdout()
			var clipboardBuffer *bytes.Buffer
			clipboardRequested := copyEnabled || copyOnlyEnabled
			if clipboardRequested {
				if clipboardProvider == nil {
					return errors.New(clipboardServiceMissingMessage)
				}
				clipboardBuffer = &bytes.Buffer{}
				if copyOnlyEnabled {
					writer = clipboardBuffer
				} else {
					writer = io.MultiWriter(writer, clipboardBuffer)
				}
			}
			if formattedOutput != "" && !copyOnlyEnabled {
				if _, writeErr := fmt.Fprint(writer, formattedOutput); writeErr != nil {
					return writeErr
				}
				if !strings.HasSuffix(formattedOutput, "\n") {
					if _, writeErr := fmt.Fprintln(writer); writeErr != nil {
						return writeErr
					}
				}
			} else if formattedOutput != "" && copyOnlyEnabled {
				if _, writeErr := fmt.Fprint(writer, formattedOutput); writeErr != nil {
					return writeErr
				}
			} else if !copyOnlyEnabled {
				if _, writeErr := fmt.Fprintln(writer); writeErr != nil {
					return writeErr
				}
			}
			if clipboardRequested && clipboardBuffer != nil {
				if copyErr := clipboardProvider.Copy(clipboardBuffer.String()); copyErr != nil {
					return fmt.Errorf(clipboardCopyErrorFormat, copyErr)
				}
			}
			return nil
		},
	}

	command.Flags().StringVar(&rootPath, docDiscoverRootFlagName, "", "project root path (defaults to current directory)")
	command.Flags().StringSliceVar(&ecosystems, docDiscoverEcosystemsFlag, nil, "comma-separated ecosystems to process (go,js,python)")
	command.Flags().StringSliceVar(&includePatterns, docDiscoverIncludeFlagName, nil, "dependency patterns to include (glob syntax)")
	command.Flags().StringSliceVar(&excludePatterns, docDiscoverExcludeFlagName, nil, "dependency patterns to exclude (glob syntax)")
	registerBooleanFlag(command.Flags(), &includeDev, docDiscoverIncludeDevFlag, false, "include dev dependencies")
	registerBooleanFlag(command.Flags(), &includeIndirect, docDiscoverIncludeIndirectFlagName, false, "include indirect Go modules")
	command.Flags().StringVar(&rulesPath, docDiscoverRulesFlagName, "", docRulesFlagDescription)
	command.Flags().IntVar(&concurrency, docDiscoverConcurrencyFlag, 0, "maximum concurrent fetches (default derived from CPU)")
	command.Flags().StringVar(&format, docDiscoverFormatFlagName, docDiscoverFormatText, docDiscoverFormatDescription)
	command.Flags().StringVar(&apiBase, docDiscoverAPIBaseFlagName, "", "GitHub API base URL override")
	command.Flags().StringVar(&npmRegistryBase, docDiscoverNPMBaseFlagName, "", "npm registry base URL")
	command.Flags().StringVar(&pypiRegistryBase, docDiscoverPyPIBaseFlagName, "", "PyPI registry base URL")
	command.Flags().StringVar(&outputDir, docDiscoverOutputFlagName, "", "output directory for generated documentation")

	if apiFlag := command.Flags().Lookup(docDiscoverAPIBaseFlagName); apiFlag != nil {
		apiFlag.Hidden = true
	}
	if npmFlag := command.Flags().Lookup(docDiscoverNPMBaseFlagName); npmFlag != nil {
		npmFlag.Hidden = true
	}
	if pypiFlag := command.Flags().Lookup(docDiscoverPyPIBaseFlagName); pypiFlag != nil {
		pypiFlag.Hidden = true
	}

	return command
}

type commandDescriptor struct {
	ctx                context.Context
	commandName        string
	paths              []string
	exclusionPatterns  []string
	useGitignore       bool
	useIgnoreFile      bool
	includeGit         bool
	callChainDepth     int
	format             string
	documentation      documentationOptions
	summaryEnabled     bool
	includeContent     bool
	tokenConfiguration tokenOptions
	outputWriter       io.Writer
	errorWriter        io.Writer
	clipboardEnabled   bool
	copyOnly           bool
	clipboard          clipboard.Copier
	callChainService   commands.CallChainService
}

type executionContext struct {
	documentationMode string
	collector         *docs.Collector
	tokenCounter      tokenizer.Counter
	tokenModel        string
	workingDirectory  string
}

// runTool executes the command described by descriptor, assembling documentation collectors,
// token counters, and clipboard behaviour as required.
func runTool(descriptor commandDescriptor) error {
	commandContext := descriptor.ctx
	if commandContext == nil {
		commandContext = context.Background()
	}

	workingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		return fmt.Errorf(workingDirectoryErrorFormat, workingDirectoryError)
	}

	executionContext, contextError := buildExecutionContext(commandContext, descriptor, workingDirectory)
	if contextError != nil {
		return contextError
	}

	outputWriter := descriptor.outputWriter
	if outputWriter == nil {
		outputWriter = os.Stdout
	}
	errorWriter := descriptor.errorWriter
	if errorWriter == nil {
		errorWriter = os.Stderr
	}

	var clipboardBuffer *bytes.Buffer
	clipboardRequested := descriptor.clipboardEnabled || descriptor.copyOnly
	targetWriter := outputWriter
	if clipboardRequested {
		if descriptor.clipboard == nil {
			return errors.New(clipboardServiceMissingMessage)
		}
		clipboardBuffer = &bytes.Buffer{}
		if descriptor.copyOnly {
			targetWriter = clipboardBuffer
		} else {
			targetWriter = io.MultiWriter(targetWriter, clipboardBuffer)
		}
	}

	switch descriptor.commandName {
	case types.CommandCallChain:
		if len(descriptor.paths) == 0 {
			return fmt.Errorf("call chain command requires a target function")
		}
		if descriptor.callChainService == nil {
			return fmt.Errorf("call chain service is not configured")
		}
		if err := runCallChain(descriptor.paths[0], descriptor.format, descriptor.callChainDepth, executionContext.documentationMode, executionContext.collector, executionContext.workingDirectory, targetWriter, descriptor.callChainService); err != nil {
			return err
		}
	case types.CommandTree, types.CommandContent:
		if err := runStreamCommand(
			commandContext,
			descriptor.commandName,
			descriptor.paths,
			descriptor.exclusionPatterns,
			descriptor.useGitignore,
			descriptor.useIgnoreFile,
			descriptor.includeGit,
			descriptor.format,
			executionContext.documentationMode,
			descriptor.summaryEnabled,
			descriptor.includeContent,
			executionContext.tokenCounter,
			executionContext.tokenModel,
			executionContext.collector,
			targetWriter,
			errorWriter,
		); err != nil {
			return err
		}
	default:
		return fmt.Errorf(unsupportedCommandMessage)
	}

	if clipboardBuffer != nil {
		if copyErr := descriptor.clipboard.Copy(clipboardBuffer.String()); copyErr != nil {
			return fmt.Errorf(clipboardCopyErrorFormat, copyErr)
		}
	}

	return nil
}

func buildExecutionContext(commandContext context.Context, descriptor commandDescriptor, workingDirectory string) (executionContext, error) {
	mode := descriptor.documentation.Mode()
	result := executionContext{
		documentationMode: mode,
		workingDirectory:  workingDirectory,
	}

	if mode != types.DocumentationModeDisabled {
		collector, collectorErr := docs.NewCollectorWithOptions(workingDirectory, descriptor.documentation.CollectorOptions())
		if collectorErr != nil {
			return executionContext{}, collectorErr
		}
		result.collector = collector
		if descriptor.documentation.RemoteDocumentationEnabled() {
			if commandContext == nil {
				commandContext = context.Background()
			}
			collector.ActivateRemoteDocumentation(commandContext)
		}
	}

	if descriptor.tokenConfiguration.enabled {
		counter, resolvedModel, counterErr := tokenizer.NewCounter(descriptor.tokenConfiguration.toConfig(workingDirectory))
		if counterErr != nil {
			if errors.Is(counterErr, tokenizer.ErrHelperUnavailable) {
				return executionContext{}, fmt.Errorf("token helper unavailable: %w", counterErr)
			}
			return executionContext{}, counterErr
		}
		result.tokenCounter = counter
		result.tokenModel = resolvedModel
	}

	return result, nil
}

// runCallChain processes the callchain command for the specified target and depth.

func runCallChain(
	target string,
	format string,
	callChainDepth int,
	documentationMode string,
	collector *docs.Collector,
	moduleRoot string,
	outputWriter io.Writer,
	service commands.CallChainService,
) error {
	withDocumentation := documentationMode != types.DocumentationModeDisabled
	callChainData, callChainDataError := service.GetCallChainData(target, callChainDepth, withDocumentation, collector, moduleRoot)
	if callChainDataError != nil {
		return callChainDataError
	}
	if outputWriter == nil {
		outputWriter = os.Stdout
	}
	if format == types.FormatJSON {
		renderedCallChainJSONOutput, renderCallChainJSONError := output.RenderCallChainJSON(callChainData)
		if renderCallChainJSONError != nil {
			return renderCallChainJSONError
		}
		fmt.Fprintln(outputWriter, renderedCallChainJSONOutput)
	} else if format == types.FormatToon {
		fmt.Fprintln(outputWriter, output.RenderCallChainToon(callChainData))
	} else if format == types.FormatXML {
		renderedCallChainXMLOutput, renderCallChainXMLError := output.RenderCallChainXML(callChainData)
		if renderCallChainXMLError != nil {
			return renderCallChainXMLError
		}
		fmt.Fprintln(outputWriter, renderedCallChainXMLOutput)
	} else {
		fmt.Fprintln(outputWriter, output.RenderCallChainRaw(callChainData))
	}
	return nil
}

// runStreamCommand executes tree or content commands for the given paths.
func runStreamCommand(
	commandContext context.Context,
	commandName string,
	paths []string,
	exclusionPatterns []string,
	useGitignore bool,
	useIgnoreFile bool,
	includeGit bool,
	format string,
	documentationMode string,
	withSummary bool,
	includeContent bool,
	tokenCounter tokenizer.Counter,
	tokenModel string,
	collector *docs.Collector,
	outputWriter io.Writer,
	errorWriter io.Writer,
) (err error) {
	if commandContext == nil {
		commandContext = context.Background()
	}
	validatedPaths, pathValidationError := resolveAndValidatePaths(paths)
	if pathValidationError != nil {
		return pathValidationError
	}

	totalRootPaths := len(validatedPaths)

	renderCommandName := commandName
	if includeContent {
		renderCommandName = types.CommandContent
	} else {
		renderCommandName = types.CommandTree
	}

	var renderer output.StreamRenderer
	switch format {
	case types.FormatRaw:
		renderer = output.NewRawStreamRenderer(outputWriter, errorWriter, renderCommandName, withSummary)
	case types.FormatToon:
		renderer = output.NewToonStreamRenderer(outputWriter, errorWriter, renderCommandName, withSummary)
	case types.FormatJSON:
		renderer = output.NewJSONStreamRenderer(outputWriter, errorWriter, renderCommandName, totalRootPaths, withSummary, includeContent)
	case types.FormatXML:
		renderer = output.NewXMLStreamRenderer(outputWriter, errorWriter, renderCommandName, totalRootPaths, withSummary, includeContent)
	default:
		return fmt.Errorf(invalidFormatMessage, format)
	}

	defer func() {
		if renderer == nil {
			return
		}
		if flushErr := renderer.Flush(); flushErr != nil && err == nil {
			err = flushErr
		}
	}()

	for _, info := range validatedPaths {
		streamErr := runStreamPath(commandContext, renderer, info, exclusionPatterns, useGitignore, useIgnoreFile, includeGit, includeContent, documentationMode, tokenCounter, tokenModel, collector)
		if streamErr != nil && !errors.Is(streamErr, context.Canceled) {
			if errorWriter != nil {
				fmt.Fprintf(errorWriter, warningSkipPathFormat, info.AbsolutePath, streamErr)
			}
		}
	}

	return nil
}

func runStreamPath(
	ctx context.Context,
	renderer output.StreamRenderer,
	path types.ValidatedPath,
	exclusionPatterns []string,
	useGitignore bool,
	useIgnoreFile bool,
	includeGit bool,
	includeContent bool,
	documentationMode string,
	tokenCounter tokenizer.Counter,
	tokenModel string,
	collector *docs.Collector,
) error {
	var ignorePatterns []string
	var binaryPatterns []string
	if path.IsDir {
		patterns, binary, loadErr := config.LoadRecursiveIgnorePatterns(path.AbsolutePath, exclusionPatterns, useGitignore, useIgnoreFile, includeGit)
		if loadErr != nil {
			return loadErr
		}
		ignorePatterns = patterns
		if includeContent {
			binaryPatterns = binary
		}
	}

	producer := func(streamCtx context.Context, ch chan<- stream.Event) error {
		options := stream.TreeOptions{
			Root:                  path.AbsolutePath,
			IgnorePatterns:        ignorePatterns,
			TokenCounter:          tokenCounter,
			TokenModel:            tokenModel,
			IncludeContent:        includeContent,
			BinaryContentPatterns: binaryPatterns,
		}
		return stream.StreamTree(streamCtx, options, ch)
	}

	consumer := func(event stream.Event) error {
		if documentationMode != types.DocumentationModeDisabled && collector != nil && event.Kind == stream.EventKindFile && event.File != nil {
			if entries, docErr := collector.CollectFromFile(event.File.Path); docErr == nil && len(entries) > 0 {
				event.File.Documentation = entries
			}
		}
		return renderer.Handle(event)
	}

	return dispatchStream(ctx, producer, consumer)
}

func dispatchStream(
	ctx context.Context,
	produce func(context.Context, chan<- stream.Event) error,
	consume func(stream.Event) error,
) error {
	group, streamCtx := errgroup.WithContext(ctx)
	events := make(chan stream.Event)

	group.Go(func() error {
		defer close(events)
		return produce(streamCtx, events)
	})

	group.Go(func() error {
		for {
			select {
			case <-streamCtx.Done():
				return streamCtx.Err()
			case event, ok := <-events:
				if !ok {
					return nil
				}
				if err := consume(event); err != nil {
					return err
				}
			}
		}
	})

	if err := group.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func applyStreamConfiguration(
	command *cobra.Command,
	configuration config.StreamCommandConfiguration,
	paths *pathOptions,
	outputFormat *string,
	documentationMode *string,
	summaryEnabled *bool,
	includeContent *bool,
	docsAttempt *bool,
	tokens *tokenOptions,
) {
	if command == nil {
		return
	}
	if configuration.Format != "" && outputFormat != nil && !command.Flags().Changed(formatFlagName) {
		*outputFormat = configuration.Format
	}
	if configuration.Summary != nil && summaryEnabled != nil && !command.Flags().Changed(summaryFlagName) {
		*summaryEnabled = *configuration.Summary
	}
	if documentationMode != nil && !command.Flags().Changed(documentationFlagName) {
		if configuration.DocumentationMode != "" {
			if normalized, err := normalizeDocumentationMode(configuration.DocumentationMode); err == nil {
				*documentationMode = normalized
			}
		} else if configuration.Documentation != nil {
			if *configuration.Documentation {
				*documentationMode = types.DocumentationModeRelevant
			} else {
				*documentationMode = types.DocumentationModeDisabled
			}
		}
	}
	if configuration.IncludeContent != nil && includeContent != nil && !command.Flags().Changed(contentFlagName) {
		*includeContent = *configuration.IncludeContent
	}
	if configuration.DocsAttempt != nil && docsAttempt != nil && !command.Flags().Changed(docsAttemptFlagName) {
		*docsAttempt = *configuration.DocsAttempt
	}
	if tokens != nil {
		if configuration.Tokens.Enabled != nil && !command.Flags().Changed(tokensFlagName) {
			tokens.enabled = *configuration.Tokens.Enabled
		}
		if configuration.Tokens.Model != "" && !command.Flags().Changed(modelFlagName) {
			tokens.model = configuration.Tokens.Model
		}
	}
	if paths != nil {
		if len(configuration.Paths.Exclude) > 0 && !command.Flags().Changed(exclusionFlagName) {
			paths.exclusionPatterns = append([]string{}, configuration.Paths.Exclude...)
		}
		if configuration.Paths.UseGitignore != nil && !command.Flags().Changed(noGitignoreFlagName) {
			paths.disableGitignore = !*configuration.Paths.UseGitignore
		}
		if configuration.Paths.UseIgnoreFile != nil && !command.Flags().Changed(noIgnoreFlagName) {
			paths.disableIgnoreFile = !*configuration.Paths.UseIgnoreFile
		}
		if configuration.Paths.IncludeGit != nil && !command.Flags().Changed(includeGitFlagName) {
			paths.includeGit = *configuration.Paths.IncludeGit
		}
	}
}

func applyCallChainConfiguration(
	command *cobra.Command,
	configuration config.CallChainConfiguration,
	outputFormat *string,
	depth *int,
	documentationMode *string,
	docsAttempt *bool,
) {
	if command == nil {
		return
	}
	if configuration.Format != "" && outputFormat != nil && !command.Flags().Changed(formatFlagName) {
		*outputFormat = configuration.Format
	}
	if configuration.Depth != nil && depth != nil && !command.Flags().Changed(callChainDepthFlagName) {
		*depth = *configuration.Depth
	}
	if documentationMode != nil && !command.Flags().Changed(documentationFlagName) {
		if configuration.DocumentationMode != "" {
			if normalized, err := normalizeDocumentationMode(configuration.DocumentationMode); err == nil {
				*documentationMode = normalized
			}
		} else if configuration.Documentation != nil {
			if *configuration.Documentation {
				*documentationMode = types.DocumentationModeRelevant
			} else {
				*documentationMode = types.DocumentationModeDisabled
			}
		}
	}
	if configuration.DocsAttempt != nil && docsAttempt != nil && !command.Flags().Changed(docsAttemptFlagName) {
		*docsAttempt = *configuration.DocsAttempt
	}
}

func resolveCopyDefault(command *cobra.Command, cliValue bool, configurationValue *bool) bool {
	if configurationValue == nil {
		return cliValue
	}
	if booleanFlagChanged(command, copyFlagName, copyFlagAlias) {
		return cliValue
	}
	return *configurationValue
}

func resolveCopyOnlyDefault(command *cobra.Command, cliValue bool, configurationValue *bool) bool {
	if configurationValue == nil {
		return cliValue
	}
	if booleanFlagChanged(command, copyOnlyFlagName, copyOnlyFlagAlias) {
		return cliValue
	}
	return *configurationValue
}

func resolveClipboardPreferences(command *cobra.Command, cliCopy bool, cliCopyOnly bool, configurationCopy *bool, configurationCopyOnly *bool) (bool, bool) {
	copyEnabled := resolveCopyDefault(command, cliCopy, configurationCopy)
	copyOnlyEnabled := resolveCopyOnlyDefault(command, cliCopyOnly, configurationCopyOnly)
	if copyOnlyEnabled {
		copyEnabled = true
	}
	return copyEnabled, copyOnlyEnabled
}

func booleanFlagChanged(command *cobra.Command, names ...string) bool {
	if command == nil {
		return false
	}
	for _, name := range names {
		if flag := command.Flag(name); flag != nil && flag.Changed {
			return true
		}
	}
	inherited := command.InheritedFlags()
	local := command.Flags()
	persistent := command.PersistentFlags()
	for _, name := range names {
		if inherited != nil && inherited.Changed(name) {
			return true
		}
		if local != nil && local.Changed(name) {
			return true
		}
		if persistent != nil && persistent.Changed(name) {
			return true
		}
	}
	return false
}

func normalizeDocumentationMode(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case "":
		return types.DocumentationModeDisabled, nil
	case types.DocumentationModeDisabled, types.DocumentationModeRelevant, types.DocumentationModeFull:
		return trimmed, nil
	case "true":
		return types.DocumentationModeRelevant, nil
	case "false":
		return types.DocumentationModeDisabled, nil
	default:
		return "", fmt.Errorf(invalidDocumentationModeMessage, value)
	}
}

// resolveAndValidatePaths converts input paths to absolute form and validates their existence.
func resolveAndValidatePaths(inputs []string) ([]types.ValidatedPath, error) {
	seen := make(map[string]struct{})
	var result []types.ValidatedPath
	for _, inputPath := range inputs {
		absolutePath, absolutePathError := filepath.Abs(inputPath)
		if absolutePathError != nil {
			return nil, fmt.Errorf(errorAbsolutePathFormat, inputPath, absolutePathError)
		}
		cleanPath := filepath.Clean(absolutePath)
		if _, ok := seen[cleanPath]; ok {
			continue
		}
		info, fileStatusError := os.Stat(cleanPath)
		if fileStatusError != nil {
			if os.IsNotExist(fileStatusError) {
				return nil, fmt.Errorf(errorPathMissingFormat, inputPath)
			}
			return nil, fmt.Errorf(errorStatFormat, inputPath, fileStatusError)
		}
		seen[cleanPath] = struct{}{}
		result = append(result, types.ValidatedPath{AbsolutePath: cleanPath, IsDir: info.IsDir()})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf(errorNoValidPaths)
	}
	return result, nil
}

type docCommandOptions struct {
	Coordinates      repositoryCoordinates
	RuleSet          githubdoc.RuleSet
	Documentation    documentationOptions
	APIBase          string
	ClipboardEnabled bool
	CopyOnly         bool
	Clipboard        clipboard.Copier
	Writer           io.Writer
}

func runDocCommand(ctx context.Context, options docCommandOptions) error {
	outputWriter := options.Writer
	if outputWriter == nil {
		outputWriter = os.Stdout
	}
	mode := options.Documentation.Mode()
	fetcher := githubdoc.NewFetcher(nil)
	if options.APIBase != "" {
		fetcher = fetcher.WithAPIBase(options.APIBase)
	}
	if authorizationToken := options.Documentation.AuthorizationToken(); authorizationToken != "" {
		fetcher = fetcher.WithAuthorizationToken(authorizationToken)
	}
	documents, fetchErr := fetcher.Fetch(ctx, githubdoc.FetchOptions{
		Owner:      options.Coordinates.Owner,
		Repository: options.Coordinates.Repository,
		Reference:  options.Coordinates.Reference,
		RootPath:   options.Coordinates.RootPath,
		RuleSet:    options.RuleSet,
	})
	if fetchErr != nil {
		return fetchErr
	}
	filtered := trimDocumentsForMode(documents, mode)
	rendered := renderDocumentationOutput(options.Coordinates, filtered)
	var clipboardBuffer *bytes.Buffer
	clipboardRequested := options.ClipboardEnabled || options.CopyOnly
	if clipboardRequested {
		if options.Clipboard == nil {
			return errors.New(clipboardServiceMissingMessage)
		}
		clipboardBuffer = &bytes.Buffer{}
		if options.CopyOnly {
			outputWriter = clipboardBuffer
		} else {
			outputWriter = io.MultiWriter(outputWriter, clipboardBuffer)
		}
	}
	if rendered != "" {
		if _, writeErr := fmt.Fprint(outputWriter, rendered); writeErr != nil {
			return writeErr
		}
		if !strings.HasSuffix(rendered, "\n") {
			if _, writeErr := fmt.Fprintln(outputWriter); writeErr != nil {
				return writeErr
			}
		}
	} else {
		if _, writeErr := fmt.Fprintln(outputWriter); writeErr != nil {
			return writeErr
		}
	}
	if clipboardRequested && clipboardBuffer != nil {
		if copyErr := options.Clipboard.Copy(clipboardBuffer.String()); copyErr != nil {
			return fmt.Errorf(clipboardCopyErrorFormat, copyErr)
		}
	}
	return nil
}

func resolveStringOption(command *cobra.Command, flagName string, flagValue string, configValue string) string {
	if command.Flags().Changed(flagName) {
		return strings.TrimSpace(flagValue)
	}
	if flagValue != "" {
		return strings.TrimSpace(flagValue)
	}
	return strings.TrimSpace(configValue)
}

func resolveSliceOption(command *cobra.Command, flagName string, flagValues []string, configValues []string) []string {
	if command.Flags().Changed(flagName) {
		return append([]string(nil), flagValues...)
	}
	if len(flagValues) > 0 {
		return append([]string(nil), flagValues...)
	}
	if len(configValues) == 0 {
		return nil
	}
	return append([]string(nil), configValues...)
}

func resolveBoolOption(command *cobra.Command, flagName string, flagValue bool, configValue *bool) bool {
	if command.Flags().Changed(flagName) {
		return flagValue
	}
	if configValue != nil {
		return *configValue
	}
	return flagValue
}

func resolveIntOption(command *cobra.Command, flagName string, flagValue int, configValue *int) int {
	if command.Flags().Changed(flagName) {
		return flagValue
	}
	if configValue != nil {
		return *configValue
	}
	return flagValue
}

func loadRuleSet(path string) (githubdoc.RuleSet, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return githubdoc.RuleSet{}, nil
	}
	ruleSet, err := githubdoc.LoadRuleSet(trimmed)
	if err != nil {
		return githubdoc.RuleSet{}, fmt.Errorf("load rules from %s: %w", trimmed, err)
	}
	return ruleSet, nil
}

func parseEcosystemSelection(values []string) (map[discover.Ecosystem]bool, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := map[discover.Ecosystem]bool{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "":
			continue
		case "go":
			result[discover.EcosystemGo] = true
		case "js", "javascript", "node":
			result[discover.EcosystemJavaScript] = true
		case "python", "py":
			result[discover.EcosystemPython] = true
		default:
			return nil, fmt.Errorf("unsupported ecosystem %s", value)
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func renderDocDiscoverOutput(summary discover.Summary, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", docDiscoverFormatText:
		return renderDocDiscoverText(summary), nil
	case docDiscoverFormatJSON:
		encoded, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return "", fmt.Errorf("encode manifest: %w", err)
		}
		return string(encoded) + "\n", nil
	default:
		return "", fmt.Errorf("unsupported format %s", format)
	}
}

func renderDocDiscoverText(summary discover.Summary) string {
	builder := &strings.Builder{}
	total := len(summary.Entries)
	written := summary.Count(discover.StatusWritten)
	skipped := summary.Count(discover.StatusSkipped)
	failed := summary.Count(discover.StatusFailed)
	fmt.Fprintf(builder, docDiscoverSummaryTemplate, total, written, skipped, failed)
	entries := append([]discover.ManifestEntry(nil), summary.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		left := strings.ToLower(entries[i].Name)
		right := strings.ToLower(entries[j].Name)
		if left == right {
			return entries[i].Ecosystem < entries[j].Ecosystem
		}
		return left < right
	})
	for _, entry := range entries {
		line := fmt.Sprintf("- [%s] %s (%s)", entry.Status, entry.Name, entry.Repository)
		if entry.OutputPath != "" {
			line += fmt.Sprintf(" -> %s", entry.OutputPath)
		}
		if entry.DocFileCount > 0 {
			line += fmt.Sprintf(" [%d files]", entry.DocFileCount)
		}
		if entry.Status != discover.StatusWritten && entry.Reason != "" {
			line += fmt.Sprintf(" (%s)", entry.Reason)
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

type repositoryCoordinates struct {
	Owner      string
	Repository string
	Reference  string
	RootPath   string
}

func resolveRepositoryCoordinates(pathSpec string, owner string, repository string, reference string, rootPath string) (repositoryCoordinates, error) {
	parsed, parseErr := parseRepositoryPathSpec(pathSpec)
	if parseErr != nil {
		return repositoryCoordinates{}, fmt.Errorf("parse --path: %w", parseErr)
	}
	coordinates := repositoryCoordinates{
		Owner:      owner,
		Repository: repository,
		RootPath:   rootPath,
	}
	if parsed.Owner != "" {
		coordinates.Owner = parsed.Owner
	}
	if parsed.Repository != "" {
		coordinates.Repository = parsed.Repository
	}
	if parsed.RootPath != "" {
		coordinates.RootPath = parsed.RootPath
	}
	if parsed.Reference != "" {
		coordinates.Reference = parsed.Reference
	}
	if reference != "" {
		coordinates.Reference = reference
	}
	if coordinates.Owner == "" || coordinates.Repository == "" {
		return repositoryCoordinates{}, fmt.Errorf(docMissingCoordinatesErrorMessage)
	}
	normalizedRoot := normalizeRepositoryRootPath(coordinates.RootPath)
	if normalizedRoot == "" {
		return repositoryCoordinates{}, fmt.Errorf(docMissingPathErrorMessage)
	}
	coordinates.RootPath = normalizedRoot
	return coordinates, nil
}

func parseRepositoryPathSpec(raw string) (repositoryCoordinates, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return repositoryCoordinates{}, nil
	}
	if strings.Contains(trimmed, "://") {
		return parseGitHubRepositoryURL(trimmed)
	}
	segments := strings.Split(trimmed, "/")
	if len(segments) < 2 {
		return repositoryCoordinates{}, fmt.Errorf("repository path must include owner and repository")
	}
	owner := strings.TrimSpace(segments[0])
	repositorySegment := strings.TrimSpace(segments[1])
	if owner == "" || repositorySegment == "" {
		return repositoryCoordinates{}, fmt.Errorf("repository path must include owner and repository")
	}
	reference := ""
	repository := repositorySegment
	if at := strings.Index(repositorySegment, "@"); at >= 0 {
		repository = strings.TrimSpace(repositorySegment[:at])
		reference = strings.TrimSpace(repositorySegment[at+1:])
	}
	if repository == "" {
		return repositoryCoordinates{}, fmt.Errorf("repository path must include owner and repository")
	}
	remaining := segments[2:]
	rootSegments := remaining
	if len(remaining) >= 2 {
		switch strings.ToLower(remaining[0]) {
		case githubTreeSegment, githubBlobSegment:
			if len(remaining) >= 2 {
				if reference == "" {
					reference = strings.TrimSpace(remaining[1])
				}
				rootSegments = remaining[2:]
			}
		}
	}
	rootPath := strings.Join(rootSegments, "/")
	if strings.TrimSpace(rootPath) == "" {
		rootPath = "."
	}
	return repositoryCoordinates{
		Owner:      owner,
		Repository: repository,
		Reference:  reference,
		RootPath:   strings.Trim(rootPath, "/"),
	}, nil
}

func normalizeRepositoryRootPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if trimmed == "." {
		return "."
	}
	cleaned := strings.Trim(trimmed, "/")
	for strings.HasPrefix(cleaned, "./") {
		cleaned = strings.TrimPrefix(cleaned, "./")
	}
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "" {
		return "."
	}
	return cleaned
}

func parseGitHubRepositoryURL(raw string) (repositoryCoordinates, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return repositoryCoordinates{}, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return repositoryCoordinates{}, fmt.Errorf("parse repository url: %w", err)
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 {
		return repositoryCoordinates{}, fmt.Errorf("repository url must include owner and name")
	}
	coords := repositoryCoordinates{
		Owner:      segments[0],
		Repository: segments[1],
	}
	if len(segments) >= 4 {
		switch strings.ToLower(segments[2]) {
		case githubTreeSegment, githubBlobSegment:
			coords.Reference = segments[3]
			if len(segments) > 4 {
				coords.RootPath = strings.Join(segments[4:], "/")
			}
		}
	}
	if coords.Reference == "" && len(segments) > 2 {
		coords.RootPath = strings.Join(segments[2:], "/")
	}
	if coords.RootPath == "" {
		coords.RootPath = "."
	}
	return coords, nil
}

func isWebDocumentationPath(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return false
	}
	host := strings.ToLower(parsed.Host)
	if host == "" {
		return false
	}
	if strings.Contains(host, "github.com") || strings.Contains(host, "githubusercontent.com") {
		return false
	}
	return true
}

func trimDocumentsForMode(documents []githubdoc.Document, mode string) []githubdoc.Document {
	if mode != types.DocumentationModeRelevant {
		return documents
	}
	if len(documents) == 0 {
		return documents
	}
	return documents[:1]
}

func renderDocumentationOutput(coordinates repositoryCoordinates, documents []githubdoc.Document) string {
	var builder strings.Builder
	referenceLabel := coordinates.Reference
	if strings.TrimSpace(referenceLabel) == "" {
		referenceLabel = "default"
	}
	fmt.Fprintf(&builder, docTitleTemplate, coordinates.Owner, coordinates.Repository, referenceLabel)
	trimmedRoot := strings.Trim(strings.TrimSpace(coordinates.RootPath), "/")
	for index, document := range documents {
		if index > 0 {
			builder.WriteString(docSectionSeparator)
		}
		relativePath := document.Path
		if trimmedRoot != "" {
			prefix := trimmedRoot + "/"
			relativePath = strings.TrimPrefix(relativePath, prefix)
		}
		if relativePath == "" {
			relativePath = document.Path
		}
		fmt.Fprintf(&builder, docHeaderTemplate, relativePath)
		builder.WriteString(strings.TrimSpace(document.Content))
	}
	builder.WriteString("\n")
	return builder.String()
}

func mcpCapabilities() []mcp.Capability {
	return []mcp.Capability{
		{
			Name:        types.CommandTree,
			Description: "Display directory tree as JSON. Paths must be absolute or resolved relative to the reported root directory. Flags: summary (bool), exclude (string[]), includeContent (bool), useGitignore (bool), useIgnore (bool), tokens (bool), model (string), includeGit (bool).",
		},
		{
			Name:        types.CommandContent,
			Description: "Show file contents as JSON. Paths must be absolute or resolved relative to the reported root directory. Flags: summary (bool), documentation (string), includeContent (bool), exclude (string[]), useGitignore (bool), useIgnore (bool), tokens (bool), model (string), includeGit (bool).",
		},
		{
			Name:        types.CommandCallChain,
			Description: "Analyze Go/Python/JavaScript call chains as JSON. Target must be fully qualified or resolvable in the project. Flags: depth (int), documentation (string).",
		},
		{
			Name:        types.CommandDoc,
			Description: "Retrieve documentation from GitHub or external HTTPS pages as Markdown. Flags: path (string), ref (string), rules (string), doc (string), web-depth (int), copy (bool).",
		},
	}
}

func startMCPServer(parent context.Context, output io.Writer) error {
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	writer := output
	if writer == nil {
		writer = os.Stdout
	}

	workingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		return fmt.Errorf(workingDirectoryErrorFormat, workingDirectoryError)
	}

	server := mcp.NewServer(mcp.Config{
		Address:         mcpListenAddress,
		Capabilities:    mcpCapabilities(),
		Executors:       mcpCommandExecutors(),
		RootDirectory:   workingDirectory,
		ShutdownTimeout: mcpShutdownTimeout,
	})

	notify := func(address string) {
		fmt.Fprintf(writer, mcpStartupMessageFormat, address)
	}

	if err := server.Run(ctx, notify); err != nil {
		return fmt.Errorf("run MCP server: %w", err)
	}
	return nil
}
