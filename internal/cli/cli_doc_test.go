package cli

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/tyemirov/ctx/internal/docs/webdoc"
)

func TestParseGitHubRepositoryURL(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    repositoryCoordinates
		expectError bool
	}{
		{
			name:  "tree path with explicit branch",
			input: "https://github.com/example/project/tree/main/docs",
			expected: repositoryCoordinates{
				Owner:      "example",
				Repository: "project",
				Reference:  "main",
				RootPath:   "docs",
			},
		},
		{
			name:  "blob path with branch and nested directory",
			input: "https://github.com/jspreadsheet/ce/blob/master/docs/jspreadsheet/docs",
			expected: repositoryCoordinates{
				Owner:      "jspreadsheet",
				Repository: "ce",
				Reference:  "master",
				RootPath:   "docs/jspreadsheet/docs",
			},
		},
		{
			name:     "empty keeps defaults",
			input:    "",
			expected: repositoryCoordinates{},
		},
	}
	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {
			result, err := parseGitHubRepositoryURL(testCase.input)
			if testCase.expectError {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if !reflect.DeepEqual(result, testCase.expected) {
				t.Fatalf("expected %+v, got %+v", testCase.expected, result)
			}
		})
	}
}

func TestResolveRepositoryCoordinatesAcceptsUnifiedPath(t *testing.T) {
	result, err := resolveRepositoryCoordinates("jspreadsheet/ce/docs/jspreadsheet", "", "", "", "")
	if err != nil {
		t.Fatalf("expected unified path to resolve without error, got %v", err)
	}
	expected := repositoryCoordinates{
		Owner:      "jspreadsheet",
		Repository: "ce",
		Reference:  "",
		RootPath:   "docs/jspreadsheet",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %+v, got %+v", expected, result)
	}
}

func TestResolveRepositoryCoordinatesDefaultsRootForOwnerRepoFormat(t *testing.T) {
	result, err := resolveRepositoryCoordinates("example/documentation", "", "", "", "")
	if err != nil {
		t.Fatalf("expected owner/repo format to resolve without error, got %v", err)
	}
	expected := repositoryCoordinates{
		Owner:      "example",
		Repository: "documentation",
		Reference:  "",
		RootPath:   ".",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %+v, got %+v", expected, result)
	}
}

func TestRunDocWebCommandRequiresPath(t *testing.T) {
	t.Parallel()
	err := runDocWebCommand(context.Background(), docWebCommandOptions{
		Depth:   1,
		Fetcher: &stubDocWebFetcher{},
		Writer:  &bytes.Buffer{},
	})
	if err == nil {
		t.Fatalf("expected error when path is missing")
	}
	if !strings.Contains(err.Error(), "path") {
		t.Fatalf("expected path validation error, got %v", err)
	}
}

func TestRunDocWebCommandValidatesDepth(t *testing.T) {
	t.Parallel()
	err := runDocWebCommand(context.Background(), docWebCommandOptions{
		Path:    "https://example.com/docs",
		Depth:   -1,
		Fetcher: &stubDocWebFetcher{},
		Writer:  &bytes.Buffer{},
	})
	if err == nil {
		t.Fatalf("expected error when depth is negative")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected depth validation error, got %v", err)
	}
}

func TestRunDocWebCommandCopiesOutput(t *testing.T) {
	t.Parallel()
	pages := []webdoc.Page{
		{
			URL:     "https://example.com/docs",
			Title:   "Docs",
			Content: "Root page body",
		},
		{
			URL:     "https://example.com/docs/setup",
			Title:   "Setup",
			Content: "Setup instructions",
		},
	}
	clipboard := &recordingClipboard{}
	fetcher := &stubDocWebFetcher{pages: pages}
	var output bytes.Buffer
	err := runDocWebCommand(context.Background(), docWebCommandOptions{
		Path:             "https://example.com/docs",
		Depth:            1,
		Fetcher:          fetcher,
		Writer:           &output,
		ClipboardEnabled: true,
		Clipboard:        clipboard,
	})
	if err != nil {
		t.Fatalf("expected run to succeed, got %v", err)
	}
	rendered := output.String()
	if !strings.Contains(rendered, "Docs") || !strings.Contains(rendered, "Setup instructions") {
		t.Fatalf("expected rendered output to contain sanitized content, got %q", rendered)
	}
	if clipboard.invocations != 1 {
		t.Fatalf("expected clipboard to be invoked once, got %d", clipboard.invocations)
	}
	if clipboard.copiedText != rendered {
		t.Fatalf("clipboard text mismatch\nexpected: %q\nactual: %q", rendered, clipboard.copiedText)
	}
	if fetcher.requestedPath != "https://example.com/docs" {
		t.Fatalf("expected fetcher to receive request path, got %s", fetcher.requestedPath)
	}
	if fetcher.requestedDepth != 1 {
		t.Fatalf("expected fetcher depth to be 1, got %d", fetcher.requestedDepth)
	}
}

type stubDocWebFetcher struct {
	pages          []webdoc.Page
	err            error
	requestedPath  string
	requestedDepth int
}

func (stub *stubDocWebFetcher) Fetch(ctx context.Context, path string, depth int) ([]webdoc.Page, error) {
	stub.requestedPath = path
	stub.requestedDepth = depth
	if stub.err != nil {
		return nil, stub.err
	}
	return append([]webdoc.Page(nil), stub.pages...), nil
}

type recordingClipboard struct {
	invocations int
	copiedText  string
	err         error
}

func (clipboard *recordingClipboard) Copy(text string) error {
	clipboard.invocations++
	clipboard.copiedText = text
	return clipboard.err
}

func TestIsWebDocumentationPath(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "external url",
			input:    "https://getbootstrap.com/docs/5.0/getting-started/introduction/",
			expected: true,
		},
		{
			name:     "github url",
			input:    "https://github.com/example/project/tree/main/docs",
			expected: false,
		},
		{
			name:     "raw githubusercontent",
			input:    "https://raw.githubusercontent.com/example/project/README.md",
			expected: false,
		},
		{
			name:     "owner repo",
			input:    "example/project",
			expected: false,
		},
	}
	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			result := isWebDocumentationPath(testCase.input)
			if result != testCase.expected {
				t.Fatalf("expected %v for %q, got %v", testCase.expected, testCase.input, result)
			}
		})
	}
}
