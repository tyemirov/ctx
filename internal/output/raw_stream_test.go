package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tyemirov/ctx/internal/output"
	"github.com/tyemirov/ctx/internal/services/stream"
	"github.com/tyemirov/ctx/internal/types"
)

func TestRawStreamRendererStreamsTreeEvents(t *testing.T) {
	t.Parallel()

	directoryPath := "/tmp/root"
	filePath := directoryPath + "/file.txt"
	treeNode := &types.TreeOutputNode{
		Path: directoryPath,
		Name: "root",
		Type: types.NodeTypeDirectory,
		Children: []*types.TreeOutputNode{
			{
				Path:   filePath,
				Name:   "file.txt",
				Type:   types.NodeTypeFile,
				Tokens: 2,
			},
		},
	}

	testCases := []struct {
		name              string
		events            []stream.Event
		expectedFragments []string
	}{
		{
			name: "streams tree with connectors",
			events: []stream.Event{
				{Kind: stream.EventKindStart, Path: directoryPath},
				{Kind: stream.EventKindDirectory, Directory: &stream.DirectoryEvent{Phase: stream.DirectoryEnter, Path: directoryPath, Depth: 0}},
				{Kind: stream.EventKindFile, File: &stream.FileEvent{Path: filePath, Depth: 1, Tokens: 2}},
				{Kind: stream.EventKindDirectory, Directory: &stream.DirectoryEvent{Phase: stream.DirectoryLeave, Path: directoryPath, Depth: 0, Summary: &stream.SummaryEvent{Files: 1, Bytes: 4, Tokens: 2}}},
				{Kind: stream.EventKindTree, Tree: treeNode},
				{Kind: stream.EventKindSummary, Summary: &stream.SummaryEvent{Files: 1, Bytes: 4, Tokens: 2, Model: "stub"}},
				{Kind: stream.EventKindDone},
			},
			expectedFragments: []string{
				directoryPath,
				"└── [File] " + filePath + " (2 tokens)",
				"Summary: 1 file",
			},
		},
	}

	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			renderer := output.NewRawStreamRenderer(&stdout, &stderr, types.CommandTree, true)

			for index, event := range testCase.events {
				if err := renderer.Handle(event); err != nil {
					t.Fatalf("handle event %d failed: %v", index, err)
				}
			}

			if err := renderer.Flush(); err != nil {
				t.Fatalf("flush failed: %v", err)
			}

			for _, fragment := range testCase.expectedFragments {
				if !strings.Contains(stdout.String(), fragment) {
					t.Fatalf("expected fragment %q in output: %s", fragment, stdout.String())
				}
			}

			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output")
			}
		})
	}
}

func TestRawStreamRendererStreamsContentEvents(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	renderer := output.NewRawStreamRenderer(&stdout, &stderr, types.CommandContent, true)

	filePath := "/tmp/file.txt"
	treeNode := &types.TreeOutputNode{Path: "/tmp", Name: "tmp", Type: types.NodeTypeDirectory}
	events := []stream.Event{
		{Kind: stream.EventKindStart, Path: "/tmp"},
		{Kind: stream.EventKindWarning, Message: &stream.LogEvent{Level: "warning", Message: "alert"}},
		{Kind: stream.EventKindFile, File: &stream.FileEvent{Path: filePath, Type: types.NodeTypeFile}},
		{Kind: stream.EventKindContentChunk, Chunk: &stream.ChunkEvent{Path: filePath, Data: "content", IsFinal: true}},
		{Kind: stream.EventKindSummary, Summary: &stream.SummaryEvent{Files: 1, Bytes: 7}},
		{Kind: stream.EventKindTree, Tree: treeNode},
		{Kind: stream.EventKindDone},
	}

	for _, event := range events {
		if err := renderer.Handle(event); err != nil {
			t.Fatalf("handle event failed: %v", err)
		}
	}

	if err := renderer.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	outputText := stdout.String()
	if !strings.Contains(outputText, "File: "+filePath) {
		t.Fatalf("expected file header in output")
	}
	if !strings.Contains(outputText, "content") {
		t.Fatalf("expected file content in output")
	}
	if !strings.Contains(outputText, "End of file: "+filePath) {
		t.Fatalf("expected end marker in output")
	}
	if !strings.Contains(outputText, "--- Directory Tree: /tmp ---") {
		t.Fatalf("expected tree header in output")
	}
	if !strings.Contains(outputText, "Summary: 1 file") {
		t.Fatalf("expected summary line in output")
	}
	if !strings.Contains(stderr.String(), "alert") {
		t.Fatalf("expected warning on stderr")
	}
}
