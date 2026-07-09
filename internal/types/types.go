// Package types defines every cross‑package data structure used by the ctx CLI.
package types

import "encoding/xml"

const (
	NodeTypeFile      = "file"
	NodeTypeDirectory = "directory"
	NodeTypeBinary    = "binary"

	CommandTree      = "tree"
	CommandContent   = "content"
	CommandCallChain = "callchain"
	CommandDoc       = "doc"
	CommandBundle    = "bundle"

	FormatRaw  = "raw"
	FormatToon = "toon"
	FormatJSON = "json"
	FormatXML  = "xml"

	DocumentationModeDisabled = "disabled"
	DocumentationModeRelevant = "relevant"
	DocumentationModeFull     = "full"
)

// ValidatedPath is an absolute input path that already passed existence checks.
type ValidatedPath struct {
	AbsolutePath string
	IsDir        bool
}

// DocumentationEntry is a single piece of documentation attached to output.
type DocumentationEntry struct {
	Kind string `json:"type" xml:"type"`
	Name string `json:"name" xml:"name"`
	Doc  string `json:"documentation" xml:"documentation"`
}

// FileOutput represents one file returned by the content command.
type FileOutput struct {
	Path          string               `json:"path" xml:"path"`
	Type          string               `json:"type" xml:"type"`
	Content       string               `json:"content" xml:"content"`
	Size          string               `json:"size,omitempty" xml:"size,omitempty"`
	SizeBytes     int64                `json:"-" xml:"-"`
	LastModified  string               `json:"lastModified,omitempty" xml:"lastModified,omitempty"`
	MimeType      string               `json:"mimeType,omitempty" xml:"mimeType,omitempty"`
	Tokens        int                  `json:"tokens,omitempty" xml:"tokens,omitempty"`
	Model         string               `json:"model,omitempty" xml:"model,omitempty"`
	Documentation []DocumentationEntry `json:"documentation,omitempty" xml:"documentation>entry,omitempty"`
}

// TreeOutputNode represents a node of a directory tree returned by the tree command.
type TreeOutputNode struct {
	XMLName       xml.Name             `json:"-" xml:"node"`
	Path          string               `json:"path" xml:"path"`
	Name          string               `json:"name" xml:"name"`
	Type          string               `json:"type" xml:"type"`
	Size          string               `json:"size,omitempty" xml:"size,omitempty"`
	SizeBytes     int64                `json:"-" xml:"-"`
	LastModified  string               `json:"lastModified,omitempty" xml:"lastModified,omitempty"`
	MimeType      string               `json:"mimeType,omitempty" xml:"mimeType,omitempty"`
	Tokens        int                  `json:"tokens,omitempty" xml:"tokens,omitempty"`
	Model         string               `json:"model,omitempty" xml:"model,omitempty"`
	Children      []*TreeOutputNode    `json:"children,omitempty" xml:"children>node,omitempty"`
	TotalFiles    int                  `json:"totalFiles,omitempty" xml:"totalFiles,omitempty"`
	TotalSize     string               `json:"totalSize,omitempty" xml:"totalSize,omitempty"`
	TotalTokens   int                  `json:"totalTokens,omitempty" xml:"totalTokens,omitempty"`
	Content       string               `json:"content,omitempty" xml:"content,omitempty"`
	Documentation []DocumentationEntry `json:"documentation,omitempty" xml:"documentation>entry,omitempty"`
}

// CallChainOutput is the result of the callchain command.
type CallChainOutput struct {
	TargetFunction string               `json:"targetFunction" xml:"targetFunction"`
	Callers        []string             `json:"callers" xml:"callers>caller"`
	Callees        *[]string            `json:"callees,omitempty" xml:"callees>callee,omitempty"`
	Functions      map[string]string    `json:"functions" xml:"-"`
	Documentation  []DocumentationEntry `json:"documentation,omitempty" xml:"documentation>entry,omitempty"`
}

// OutputSummary captures aggregate information about rendered files.
type OutputSummary struct {
	TotalFiles  int    `json:"totalFiles" xml:"totalFiles"`
	TotalSize   string `json:"totalSize" xml:"totalSize"`
	TotalTokens int    `json:"totalTokens,omitempty" xml:"totalTokens,omitempty"`
	Model       string `json:"model,omitempty" xml:"model,omitempty"`
}

// ContextBundleRequest describes the goal-oriented repository context to assemble.
type ContextBundleRequest struct {
	RepositoryRoot    string            `json:"repositoryRoot"`
	Goal              ContextBundleGoal `json:"goal"`
	IssueDocumentPath string            `json:"issueDocumentPath,omitempty"`
	PlanDocumentPath  string            `json:"planDocumentPath,omitempty"`
	ValidationTarget  string            `json:"validationTarget,omitempty"`
	MaxTokens         int               `json:"maxTokens,omitempty"`
	Model             string            `json:"model,omitempty"`
	IncludePaths      []string          `json:"includePaths,omitempty"`
	ExcludePaths      []string          `json:"excludePaths,omitempty"`
}

// ContextBundleGoal is the concrete work item the context bundle should support.
type ContextBundleGoal struct {
	ID       string `json:"id,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"body,omitempty"`
	Category string `json:"category,omitempty"`
}

// ContextBundleOutput is the canonical JSON contract emitted by ctx bundle.
type ContextBundleOutput struct {
	Version     int                      `json:"version"`
	Repository  ContextBundleRepository  `json:"repository"`
	Goal        ContextBundleGoal        `json:"goal"`
	Budget      ContextBundleBudget      `json:"budget"`
	Terms       []string                 `json:"terms"`
	Contracts   []ContextBundleItem      `json:"contracts"`
	Files       []ContextBundleItem      `json:"files"`
	Symbols     []ContextBundleSymbol    `json:"symbols,omitempty"`
	Exclusions  []ContextBundleExclusion `json:"exclusions,omitempty"`
	GeneratedBy string                   `json:"generatedBy"`
}

// ContextBundleRepository identifies the repository scanned for the bundle.
type ContextBundleRepository struct {
	Root string `json:"root"`
	Name string `json:"name"`
}

// ContextBundleBudget records the token accounting used for selection.
type ContextBundleBudget struct {
	MaxTokens  int    `json:"maxTokens"`
	UsedTokens int    `json:"usedTokens"`
	Model      string `json:"model"`
}

// ContextBundleItem is a selected contract or implementation file excerpt.
type ContextBundleItem struct {
	Path      string `json:"path"`
	Role      string `json:"role"`
	Reason    string `json:"reason"`
	Score     int    `json:"score,omitempty"`
	Tokens    int    `json:"tokens"`
	SHA256    string `json:"sha256"`
	LineStart int    `json:"lineStart"`
	LineEnd   int    `json:"lineEnd"`
	Content   string `json:"content"`
}

// ContextBundleSymbol describes a structural source symbol found in a selected file.
type ContextBundleSymbol struct {
	Path          string `json:"path"`
	Language      string `json:"language"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	QualifiedName string `json:"qualifiedName"`
	LineStart     int    `json:"lineStart"`
	LineEnd       int    `json:"lineEnd"`
}

// ContextBundleExclusion records a file that was considered but left out of the bundle.
type ContextBundleExclusion struct {
	Path   string `json:"path"`
	Role   string `json:"role,omitempty"`
	Reason string `json:"reason"`
	Score  int    `json:"score,omitempty"`
}
