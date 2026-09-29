package output_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/ctx/internal/output"
	"github.com/tyemirov/ctx/internal/types"
	"github.com/tyemirov/ctx/internal/utils"
)

// textMimeTypeExpected defines the MIME type expected for text files.
const textMimeTypeExpected = "text/plain; charset=utf-8"

var sampleLastModified = time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)

const sampleFileSize int64 = 123

const sampleFileSizeHuman = "123b"

var sampleLastModifiedFormatted = utils.FormatTimestamp(sampleLastModified)

// callChainRawExpected defines the expected raw rendering of a call chain.
const callChainRawExpected = "----- CALLCHAIN METADATA -----\n" +
	"Target Function: target\n" +
	"Callers:\n" +
	" caller\n" +
	"Callees:\n" +
	" callee\n\n" +
	"----- FUNCTIONS -----\n" +
	"Function: target\n" +
	"----------------------------------------\n" +
	"body\n" +
	"----------------------------------------\n\n" +
	"Function: caller\n" +
	"----------------------------------------\n" +
	"\n" +
	"----------------------------------------\n\n" +
	"Function: callee\n" +
	"----------------------------------------\n" +
	"\n" +
	"----------------------------------------\n\n" +
	"--- DOCS ---\n" +
	"kind target\n" +
	"documentation\n\n"

// TestRenderCallChainRaw verifies RenderCallChainRaw output.
func TestRenderCallChainRaw(testingInstance *testing.T) {
	callees := []string{"callee"}
	data := &types.CallChainOutput{
		TargetFunction: "target",
		Callers:        []string{"caller"},
		Callees:        &callees,
		Functions:      map[string]string{"target": "body"},
		Documentation:  []types.DocumentationEntry{{Kind: "kind", Name: "target", Doc: "documentation"}},
	}
	actual := output.RenderCallChainRaw(data)
	if actual != callChainRawExpected {
		testingInstance.Errorf("unexpected output: %q", actual)
	}
}

func TestWriteTreeRawRendersConnectors(testingInstance *testing.T) {
	testingInstance.Parallel()

	rootPath := "/tmp/project"
	readmePath := rootPath + "/README.md"
	docsPath := rootPath + "/docs"
	indexPath := docsPath + "/index.md"

	testCases := []struct {
		name            string
		node            *types.TreeOutputNode
		includeSummary  bool
		expectedOutputs []string
	}{
		{
			name: "directory with nested file",
			node: &types.TreeOutputNode{
				Path: rootPath,
				Name: "project",
				Type: types.NodeTypeDirectory,
				Children: []*types.TreeOutputNode{
					{
						Path: readmePath,
						Name: "README.md",
						Type: types.NodeTypeFile,
					},
					{
						Path: docsPath,
						Name: "docs",
						Type: types.NodeTypeDirectory,
						Children: []*types.TreeOutputNode{
							{
								Path: indexPath,
								Name: "index.md",
								Type: types.NodeTypeFile,
							},
						},
					},
				},
			},
			includeSummary: true,
			expectedOutputs: []string{
				rootPath,
				"├── [File] " + readmePath,
				"└── " + docsPath,
				"    └── [File] " + indexPath,
				"Summary: 2 files",
				"    Summary: 1 file",
			},
		},
	}

	for _, testCase := range testCases {

		testingInstance.Run(testCase.name, func(testingInstance *testing.T) {
			testingInstance.Parallel()

			var buffer bytes.Buffer
			output.WriteTreeRaw(&buffer, testCase.node, testCase.includeSummary)
			rendered := buffer.String()
			for _, expected := range testCase.expectedOutputs {
				if !strings.Contains(rendered, expected) {
					testingInstance.Fatalf("expected %q in output: %s", expected, rendered)
				}
			}
		})
	}
}

// jsonExpected defines the expected JSON rendering.
var jsonExpected = "{\n" +
	"  \"path\": \"file.txt\",\n" +
	"  \"type\": \"file\",\n" +
	"  \"content\": \"data\",\n" +
	"  \"size\": \"" + sampleFileSizeHuman + "\",\n" +
	"  \"lastModified\": \"" + sampleLastModifiedFormatted + "\",\n" +
	"  \"mimeType\": \"" + textMimeTypeExpected + "\",\n" +
	"  \"documentation\": [\n" +
	"    {\n" +
	"      \"type\": \"kind\",\n" +
	"      \"name\": \"name\",\n" +
	"      \"documentation\": \"doc\"\n" +
	"    }\n" +
	"  ]\n" +
	"}"

// renderJSONErrorMessage defines the error message for JSON rendering failures.
const renderJSONErrorMessage = "render json error"

// TestRenderJSON verifies RenderJSON output and deduplication.
func TestRenderJSON(testingInstance *testing.T) {
	items := []interface{}{
		&types.FileOutput{
			Path:         "file.txt",
			Type:         types.NodeTypeFile,
			Content:      "data",
			Size:         sampleFileSizeHuman,
			SizeBytes:    sampleFileSize,
			LastModified: sampleLastModifiedFormatted,
			MimeType:     textMimeTypeExpected,
			Documentation: []types.DocumentationEntry{
				{Kind: "kind", Name: "name", Doc: "doc"},
			},
		},
	}
	actual, renderJSONError := output.RenderJSON(items)
	if renderJSONError != nil {
		testingInstance.Fatalf("%s: %v", renderJSONErrorMessage, renderJSONError)
	}
	if actual != jsonExpected {
		testingInstance.Errorf("unexpected output: %q", actual)
	}
}

// TestRenderJSONWithSummaryTreeRecursive verifies tree summaries include nested file totals.
func TestRenderJSONWithSummaryTreeRecursive(testingInstance *testing.T) {
	const nestedFileSize int64 = 2048
	totalSize := sampleFileSize + nestedFileSize
	rootFile := &types.TreeOutputNode{
		Path:       "root/file.txt",
		Name:       "file.txt",
		Type:       types.NodeTypeFile,
		Size:       utils.FormatFileSize(sampleFileSize),
		SizeBytes:  sampleFileSize,
		TotalFiles: 1,
		TotalSize:  utils.FormatFileSize(sampleFileSize),
	}
	subFile := &types.TreeOutputNode{
		Path:       "root/sub/b.bin",
		Name:       "b.bin",
		Type:       types.NodeTypeBinary,
		Size:       utils.FormatFileSize(nestedFileSize),
		SizeBytes:  nestedFileSize,
		TotalFiles: 1,
		TotalSize:  utils.FormatFileSize(nestedFileSize),
	}
	subDir := &types.TreeOutputNode{
		Path:       "root/sub",
		Name:       "sub",
		Type:       types.NodeTypeDirectory,
		Children:   []*types.TreeOutputNode{subFile},
		SizeBytes:  nestedFileSize,
		TotalFiles: 1,
		TotalSize:  utils.FormatFileSize(nestedFileSize),
	}
	root := &types.TreeOutputNode{
		Path:       "root",
		Name:       "root",
		Type:       types.NodeTypeDirectory,
		Children:   []*types.TreeOutputNode{rootFile, subDir},
		SizeBytes:  totalSize,
		TotalFiles: 2,
		TotalSize:  utils.FormatFileSize(totalSize),
	}
	items := []interface{}{root}
	actual, renderJSONError := output.RenderJSON(items)
	if renderJSONError != nil {
		testingInstance.Fatalf("%s: %v", renderJSONErrorMessage, renderJSONError)
	}
	type treeNodeDTO struct {
		Path       string        `json:"path"`
		Name       string        `json:"name"`
		Type       string        `json:"type"`
		TotalFiles int           `json:"totalFiles"`
		TotalSize  string        `json:"totalSize"`
		Children   []treeNodeDTO `json:"children"`
	}
	var parsed treeNodeDTO
	if jsonDecodeError := json.Unmarshal([]byte(actual), &parsed); jsonDecodeError != nil {
		testingInstance.Fatalf("json decode error: %v", jsonDecodeError)
	}
	if parsed.TotalFiles != 2 {
		testingInstance.Fatalf("expected 2 files, got %d", parsed.TotalFiles)
	}
	expectedSize := utils.FormatFileSize(totalSize)
	if parsed.TotalSize != expectedSize {
		testingInstance.Fatalf("expected total size %s, got %s", expectedSize, parsed.TotalSize)
	}
	if len(parsed.Children) != 2 {
		testingInstance.Fatalf("expected two children under root, got %d", len(parsed.Children))
	}
	var foundDir, foundFile bool
	for _, child := range parsed.Children {
		switch child.Type {
		case types.NodeTypeDirectory:
			foundDir = true
			if child.TotalFiles != 1 {
				testingInstance.Fatalf("directory summary incorrect: %+v", child)
			}
			if len(child.Children) != 1 {
				testingInstance.Fatalf("expected nested child under directory, got %d", len(child.Children))
			}
			nested := child.Children[0]
			if nested.TotalFiles != 1 {
				testingInstance.Fatalf("nested directory file summary incorrect: %+v", nested)
			}
		case types.NodeTypeFile, types.NodeTypeBinary:
			foundFile = true
			if child.TotalFiles != 1 {
				testingInstance.Fatalf("file summary incorrect: %+v", child)
			}
		}
	}
	if !foundDir || !foundFile {
		testingInstance.Fatalf("expected both directory and file children summaries, got %+v", parsed.Children)
	}
}

// xmlExpected defines the expected XML rendering.
var xmlExpected = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
	"<results>\n" +
	"  <item>\n" +
	"    <path>file.txt</path>\n" +
	"    <type>file</type>\n" +
	"    <content>data</content>\n" +
	"    <size>" + sampleFileSizeHuman + "</size>\n" +
	"    <lastModified>" + sampleLastModifiedFormatted + "</lastModified>\n" +
	"    <mimeType>" + textMimeTypeExpected + "</mimeType>\n" +
	"    <documentation>\n" +
	"      <entry>\n" +
	"        <type>kind</type>\n" +
	"        <name>name</name>\n" +
	"        <documentation>doc</documentation>\n" +
	"      </entry>\n" +
	"    </documentation>\n" +
	"  </item>\n" +
	"</results>"

// renderXMLErrorMessage defines the error message for XML rendering failures.
const renderXMLErrorMessage = "render xml error"

// TestRenderXML verifies RenderXML output and deduplication.
func TestRenderXML(testingInstance *testing.T) {
	items := []interface{}{
		&types.FileOutput{
			Path:         "file.txt",
			Type:         types.NodeTypeFile,
			Content:      "data",
			Size:         sampleFileSizeHuman,
			SizeBytes:    sampleFileSize,
			LastModified: sampleLastModifiedFormatted,
			MimeType:     textMimeTypeExpected,
			Documentation: []types.DocumentationEntry{
				{Kind: "kind", Name: "name", Doc: "doc"},
			},
		},
	}
	actual, renderXMLError := output.RenderXML(items)
	if renderXMLError != nil {
		testingInstance.Fatalf("%s: %v", renderXMLErrorMessage, renderXMLError)
	}
	if actual != xmlExpected {
		testingInstance.Errorf("unexpected output: %q", actual)
	}
}

// TestRenderXMLWithSummary verifies XML rendering when summary is requested.
func TestRenderXMLWithSummary(testingInstance *testing.T) {
	root := &types.TreeOutputNode{
		Path:       "root",
		Name:       "root",
		Type:       types.NodeTypeDirectory,
		TotalFiles: 2,
		Children: []*types.TreeOutputNode{
			{Path: "root/file.txt", Name: "file.txt", Type: types.NodeTypeFile, TotalFiles: 1},
			{Path: "root/other.txt", Name: "other.txt", Type: types.NodeTypeFile, TotalFiles: 1},
		},
	}
	items := []interface{}{root}
	actual, renderXMLError := output.RenderXML(items)
	if renderXMLError != nil {
		testingInstance.Fatalf("%s: %v", renderXMLErrorMessage, renderXMLError)
	}
	if !strings.Contains(actual, "<totalFiles>2</totalFiles>") {
		testingInstance.Errorf("expected totalFiles in XML output: %q", actual)
	}
}

// rawExpected defines the expected raw output for documentation and content.
const rawExpected = "File: file.txt\n" +
	"data\n" +
	"End of file: file.txt\n" +
	"----------------------------------------\n"

// rawSummaryExpected defines the expected raw output when a summary is requested.
const rawSummaryExpected = "Summary: 1 file, " + sampleFileSizeHuman + "\n\n" +
	"File: file.txt\n" +
	"data\n" +
	"End of file: file.txt\n" +
	"----------------------------------------\n"

// pipeCreationErrorMessage defines the error message for pipe creation failures.
const pipeCreationErrorMessage = "pipe creation error"

// renderRawErrorMessage defines the error message for raw rendering failures.
const renderRawErrorMessage = "render raw error"

// bufferReadErrorMessage defines the error message for buffer read failures.
const bufferReadErrorMessage = "buffer read error"

// TestRenderRaw verifies RenderRaw printing.
func TestRenderRaw(testingInstance *testing.T) {
	items := []interface{}{
		&types.FileOutput{Path: "file.txt", Type: types.NodeTypeFile, Content: "data", Size: sampleFileSizeHuman, SizeBytes: sampleFileSize, LastModified: sampleLastModifiedFormatted, MimeType: textMimeTypeExpected},
	}
	reader, writer, pipeCreationError := os.Pipe()
	if pipeCreationError != nil {
		testingInstance.Fatalf("%s: %v", pipeCreationErrorMessage, pipeCreationError)
	}
	originalStdout := os.Stdout
	os.Stdout = writer
	renderRawError := output.RenderRaw(types.CommandContent, items, false)
	writer.Close()
	os.Stdout = originalStdout
	if renderRawError != nil {
		testingInstance.Fatalf("%s: %v", renderRawErrorMessage, renderRawError)
	}
	var buffer bytes.Buffer
	_, bufferReadError := buffer.ReadFrom(reader)
	if bufferReadError != nil {
		testingInstance.Fatalf("%s: %v", bufferReadErrorMessage, bufferReadError)
	}
	actual := buffer.String()
	if actual != rawExpected {
		testingInstance.Errorf("unexpected output: %q", actual)
	}
}

// TestRenderRawWithSummary verifies raw rendering when summary is requested.
func TestRenderRawWithSummary(testingInstance *testing.T) {
	items := []interface{}{
		&types.FileOutput{Path: "file.txt", Type: types.NodeTypeFile, Content: "data", Size: sampleFileSizeHuman, SizeBytes: sampleFileSize, LastModified: sampleLastModifiedFormatted, MimeType: textMimeTypeExpected},
	}
	reader, writer, pipeCreationError := os.Pipe()
	if pipeCreationError != nil {
		testingInstance.Fatalf("%s: %v", pipeCreationErrorMessage, pipeCreationError)
	}
	originalStdout := os.Stdout
	os.Stdout = writer
	renderRawError := output.RenderRaw(types.CommandContent, items, true)
	writer.Close()
	os.Stdout = originalStdout
	if renderRawError != nil {
		testingInstance.Fatalf("%s: %v", renderRawErrorMessage, renderRawError)
	}
	var buffer bytes.Buffer
	_, bufferReadError := buffer.ReadFrom(reader)
	if bufferReadError != nil {
		testingInstance.Fatalf("%s: %v", bufferReadErrorMessage, bufferReadError)
	}
	actual := buffer.String()
	if actual != rawSummaryExpected {
		testingInstance.Errorf("unexpected output: %q", actual)
	}
}
