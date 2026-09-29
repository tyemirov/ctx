package output

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/tyemirov/ctx/internal/types"
	"github.com/tyemirov/ctx/internal/utils"
)

const (
	indentPrefix = ""
	indentSpacer = "  "

	separatorLine         = "----------------------------------------"
	callchainMetaHeader   = "----- CALLCHAIN METADATA -----"
	callchainFunctionsHdr = "----- FUNCTIONS -----"
	callchainDocsHeader   = "--- DOCS ---"

	xmlHeader         = xml.Header
	xmlRootElement    = "result"
	xmlCallchainsName = "callchains"

	binaryContentOmitted = "(binary content omitted)"
	mimeTypeLabel        = "Mime Type: "
	binaryNodeFormat     = "[Binary] %s (%s%s)\n"
	binaryTreeFormat     = "%s[Binary] %s (%s%s)\n"

	treeBranchConnector = "├── "
	treeLastConnector   = "└── "
	treeBranchPadding   = "│   "
	treeLastPadding     = "    "

	// filePrefix is the key prefix used when deduplicating file outputs.
	filePrefix = "file:"
	// nodePrefix is the key prefix used when deduplicating tree nodes.
	nodePrefix = "node:"
	// callChainPrefix is the key prefix used when deduplicating call chain outputs.
	callChainPrefix = "callchain:"
)

// RenderCallChainRaw returns the call‑chain output in raw text format.
func RenderCallChainRaw(data *types.CallChainOutput) string {
	var buffer bytes.Buffer

	buffer.WriteString(callchainMetaHeader + "\n")
	buffer.WriteString("Target Function: " + data.TargetFunction + "\n")
	buffer.WriteString("Callers:\n")
	if len(data.Callers) == 0 {
		buffer.WriteString("  (none)\n")
	} else {
		for _, callerName := range data.Callers {
			buffer.WriteString(" " + callerName + "\n")
		}
	}
	buffer.WriteString("Callees:\n")
	if data.Callees == nil || len(*data.Callees) == 0 {
		buffer.WriteString("  (none)\n")
	} else {
		for _, calleeName := range *data.Callees {
			buffer.WriteString(" " + calleeName + "\n")
		}
	}
	buffer.WriteString("\n")
	buffer.WriteString(callchainFunctionsHdr + "\n")
	for _, name := range orderedFunctionNames(data) {
		buffer.WriteString("Function: " + name + "\n")
		buffer.WriteString(separatorLine + "\n")
		buffer.WriteString(data.Functions[name] + "\n")
		buffer.WriteString(separatorLine + "\n\n")
	}
	if len(data.Documentation) > 0 {
		buffer.WriteString(callchainDocsHeader + "\n")
		for _, documentationEntry := range data.Documentation {
			buffer.WriteString(documentationEntry.Kind + " " + documentationEntry.Name + "\n")
			if documentationEntry.Doc != "" {
				buffer.WriteString(documentationEntry.Doc + "\n")
			}
			buffer.WriteString("\n")
		}
	}

	return buffer.String()
}

// RenderCallChainJSON marshals the call‑chain output as a JSON array.
func RenderCallChainJSON(data *types.CallChainOutput) (string, error) {
	array := []types.CallChainOutput{*data}
	encoded, jsonEncodeError := json.MarshalIndent(array, indentPrefix, indentSpacer)
	return string(encoded), jsonEncodeError
}

// RenderCallChainXML marshals the call-chain output as an XML document.
func RenderCallChainXML(data *types.CallChainOutput) (string, error) {
	var callees []string
	if data.Callees != nil {
		callees = *data.Callees
	}
	type xmlFunction struct {
		Name string `xml:"name"`
		Body string `xml:"body"`
	}
	var functionList []xmlFunction
	names := make([]string, 0, len(data.Functions))
	for name := range data.Functions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		functionList = append(functionList, xmlFunction{Name: name, Body: data.Functions[name]})
	}
	type xmlCallChain struct {
		TargetFunction string                     `xml:"targetFunction"`
		Callers        []string                   `xml:"callers>caller"`
		Callees        []string                   `xml:"callees>callee,omitempty"`
		Functions      []xmlFunction              `xml:"functions>function"`
		Documentation  []types.DocumentationEntry `xml:"documentation>entry,omitempty"`
	}
	wrapper := struct {
		XMLName xml.Name       `xml:""`
		Chains  []xmlCallChain `xml:"callchain"`
	}{
		XMLName: xml.Name{Local: xmlCallchainsName},
		Chains: []xmlCallChain{
			{
				TargetFunction: data.TargetFunction,
				Callers:        data.Callers,
				Callees:        callees,
				Functions:      functionList,
				Documentation:  data.Documentation,
			},
		},
	}
	encoded, xmlMarshalError := xml.MarshalIndent(wrapper, indentPrefix, indentSpacer)
	if xmlMarshalError != nil {
		return "", xmlMarshalError
	}
	return xmlHeader + string(encoded), nil
}

// RenderJSON deduplicates and marshals results to JSON.
func RenderJSON(collected []interface{}) (string, error) {
	dedupedItems := removeDuplicateCollectedItems(collected)
	if len(dedupedItems) == 0 {
		return "[]", nil
	}
	treeNodes := extractTreeNodes(dedupedItems)
	if len(treeNodes) > 0 {
		if len(treeNodes) == 1 {
			encoded, jsonEncodeError := json.MarshalIndent(treeNodes[0], indentPrefix, indentSpacer)
			return string(encoded), jsonEncodeError
		}
		encoded, jsonEncodeError := json.MarshalIndent(treeNodes, indentPrefix, indentSpacer)
		return string(encoded), jsonEncodeError
	}
	if len(dedupedItems) == 1 {
		encoded, jsonEncodeError := json.MarshalIndent(dedupedItems[0], indentPrefix, indentSpacer)
		return string(encoded), jsonEncodeError
	}
	encoded, jsonEncodeError := json.MarshalIndent(dedupedItems, indentPrefix, indentSpacer)
	return string(encoded), jsonEncodeError
}

// RenderXML deduplicates and marshals results to XML.
func RenderXML(collected []interface{}) (string, error) {
	dedupedItems := removeDuplicateCollectedItems(collected)
	treeNodes := extractTreeNodes(dedupedItems)
	if len(treeNodes) == 0 {
		wrapper := struct {
			XMLName xml.Name      `xml:"results"`
			Items   []interface{} `xml:"item"`
		}{Items: dedupedItems}
		encoded, xmlMarshalError := xml.MarshalIndent(wrapper, indentPrefix, indentSpacer)
		if xmlMarshalError != nil {
			return "", xmlMarshalError
		}
		return xmlHeader + string(encoded), nil
	}
	if len(treeNodes) == 1 {
		encoded, xmlMarshalError := xml.MarshalIndent(treeNodes[0], indentPrefix, indentSpacer)
		if xmlMarshalError != nil {
			return "", xmlMarshalError
		}
		return xmlHeader + string(encoded), nil
	}
	wrapper := struct {
		XMLName xml.Name                `xml:"results"`
		Nodes   []*types.TreeOutputNode `xml:"node"`
	}{Nodes: treeNodes}
	encoded, xmlMarshalError := xml.MarshalIndent(wrapper, indentPrefix, indentSpacer)
	if xmlMarshalError != nil {
		return "", xmlMarshalError
	}
	return xmlHeader + string(encoded), nil
}

// RenderRaw deduplicates and prints results in raw text format.
func RenderRaw(commandName string, collected []interface{}, includeSummary bool) error {
	dedupedItems := removeDuplicateCollectedItems(collected)

	if includeSummary {
		summary := computeSummary(dedupedItems)
		fmt.Println(FormatSummaryLine(summary))
		fmt.Println()
	}

	for _, item := range dedupedItems {
		switch outputItem := item.(type) {
		case *types.FileOutput:
			if commandName == types.CommandContent {
				fmt.Printf("File: %s\n", outputItem.Path)
				if outputItem.Type == types.NodeTypeBinary {
					fmt.Printf("%s%s\n", mimeTypeLabel, outputItem.MimeType)
					if outputItem.Content == "" {
						fmt.Println(binaryContentOmitted)
					} else {
						fmt.Println(outputItem.Content)
					}
				} else {
					fmt.Println(outputItem.Content)
				}
				fmt.Printf("End of file: %s\n", outputItem.Path)
				fmt.Println(separatorLine)
			}
		case *types.TreeOutputNode:
			if commandName == types.CommandTree {
				switch outputItem.Type {
				case types.NodeTypeFile:
					fmt.Printf("[File] %s\n", outputItem.Path)
				case types.NodeTypeBinary:
					fmt.Printf(binaryNodeFormat, outputItem.Path, mimeTypeLabel, outputItem.MimeType)
				default:
					fmt.Printf("\n--- Directory Tree: %s ---\n", outputItem.Path)
					renderTreeNode(os.Stdout, outputItem, "", includeSummary, true, true)
				}
			}
		case *types.CallChainOutput:
			if commandName == types.CommandCallChain {
				fmt.Print(RenderCallChainRaw(outputItem))
			}
		}
	}

	return nil
}

func extractTreeNodes(items []interface{}) []*types.TreeOutputNode {
	var nodes []*types.TreeOutputNode
	for _, item := range items {
		node, ok := item.(*types.TreeOutputNode)
		if ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// removeDuplicateCollectedItems returns a slice with duplicate collected items removed.
func removeDuplicateCollectedItems(items []interface{}) []interface{} {
	seen := make(map[string]struct{}, len(items))
	var out []interface{}
	for _, item := range items {
		var key string
		switch outputItem := item.(type) {
		case *types.FileOutput:
			key = filePrefix + outputItem.Path
		case *types.TreeOutputNode:
			key = nodePrefix + outputItem.Path
		case *types.CallChainOutput:
			key = callChainPrefix + outputItem.TargetFunction
		default:
			continue
		}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

// computeSummary aggregates file counts and sizes from collected items.
func computeSummary(items []interface{}) *types.OutputSummary {
	var totalFiles int
	var totalBytes int64
	var totalTokens int
	var summaryModel string
	hasFileOutputs := false
	for _, item := range items {
		if _, ok := item.(*types.FileOutput); ok {
			hasFileOutputs = true
			break
		}
	}
	for _, item := range items {
		switch outputItem := item.(type) {
		case *types.FileOutput:
			totalFiles++
			totalBytes += outputItem.SizeBytes
			totalTokens += outputItem.Tokens
			if outputItem.Model != "" && summaryModel == "" {
				summaryModel = outputItem.Model
			}
		case *types.TreeOutputNode:
			if hasFileOutputs {
				continue
			}
			files, bytes, tokens := summarizeTree(outputItem)
			totalFiles += files
			totalBytes += bytes
			totalTokens += tokens
			if outputItem.Model != "" && summaryModel == "" {
				summaryModel = outputItem.Model
			}
		}
	}
	return &types.OutputSummary{
		TotalFiles:  totalFiles,
		TotalSize:   utils.FormatFileSize(totalBytes),
		TotalTokens: totalTokens,
		Model:       summaryModel,
	}
}

// summarizeTree returns the file count, total size, and tokens for a tree node.
func summarizeTree(node *types.TreeOutputNode) (int, int64, int) {
	if node == nil {
		return 0, 0, 0
	}
	var totalFiles int
	var totalBytes int64
	var totalTokens int
	if node.Type == types.NodeTypeFile || node.Type == types.NodeTypeBinary {
		totalFiles++
		totalBytes += node.SizeBytes
		totalTokens += node.Tokens
	}
	for _, child := range node.Children {
		childFiles, childBytes, childTokens := summarizeTree(child)
		totalFiles += childFiles
		totalBytes += childBytes
		totalTokens += childTokens
	}
	return totalFiles, totalBytes, totalTokens
}

// writeTree recursively prints a directory tree to the provided writer.
func directorySummaryLine(node *types.TreeOutputNode, includeSummary bool) string {
	if !includeSummary || node == nil || node.Type != types.NodeTypeDirectory {
		return ""
	}
	label := "files"
	count := node.TotalFiles
	size := node.TotalSize
	tokens := node.TotalTokens
	if count == 0 || size == "" || tokens == 0 {
		files, bytes, countedTokens := summarizeTree(node)
		if count == 0 {
			count = files
		}
		if size == "" {
			size = utils.FormatFileSize(bytes)
		}
		if tokens == 0 {
			tokens = countedTokens
		}
	}
	if count == 1 {
		label = "file"
	}
	tokenSuffix := ""
	if tokens > 0 {
		tokenSuffix = fmt.Sprintf(", %d tokens", tokens)
	}
	return fmt.Sprintf("Summary: %d %s, %s%s", count, label, size, tokenSuffix)
}

func treeNodeLinePrefix(prefix string, isRoot bool, isLast bool) (string, string) {
	if isRoot {
		return "", ""
	}
	connector := treeBranchConnector
	childPrefix := prefix + treeBranchPadding
	if isLast {
		connector = treeLastConnector
		childPrefix = prefix + treeLastPadding
	}
	return prefix + connector, childPrefix
}

func renderTreeNode(writer io.Writer, node *types.TreeOutputNode, prefix string, includeSummary bool, isRoot bool, isLast bool) {
	if node == nil {
		return
	}
	linePrefix, childPrefix := treeNodeLinePrefix(prefix, isRoot, isLast)
	switch node.Type {
	case types.NodeTypeFile:
		if node.Tokens > 0 {
			fmt.Fprintf(writer, "%s[File] %s (%d tokens)\n", linePrefix, node.Path, node.Tokens)
		} else {
			fmt.Fprintf(writer, "%s[File] %s\n", linePrefix, node.Path)
		}
		return
	case types.NodeTypeBinary:
		fmt.Fprintf(writer, binaryTreeFormat, linePrefix, node.Path, mimeTypeLabel, node.MimeType)
		return
	}
	fmt.Fprintf(writer, "%s%s\n", linePrefix, node.Path)
	summaryLine := directorySummaryLine(node, includeSummary)
	if summaryLine != "" {
		if isRoot {
			fmt.Fprintf(writer, "%s\n", summaryLine)
		} else {
			fmt.Fprintf(writer, "%s%s\n", childPrefix, summaryLine)
		}
	}
	for index, child := range node.Children {
		if child == nil {
			continue
		}
		renderTreeNode(writer, child, childPrefix, includeSummary, false, index == len(node.Children)-1)
	}
}

// PrintTreeRaw renders a directory tree using the raw formatter.
func PrintTreeRaw(node *types.TreeOutputNode, includeSummary bool) {
	WriteTreeRaw(os.Stdout, node, includeSummary)
}

// WriteTreeRaw renders a directory tree to the provided writer.
func WriteTreeRaw(writer io.Writer, node *types.TreeOutputNode, includeSummary bool) {
	if node == nil {
		return
	}
	renderTreeNode(writer, node, "", includeSummary, true, true)
}

// PrintFileRaw renders a single file output in raw format.
func PrintFileRaw(file types.FileOutput) {
	WriteFileRaw(os.Stdout, file)
}

// WriteFileRaw renders a single file output to the provided writer.
func WriteFileRaw(writer io.Writer, file types.FileOutput) {
	fmt.Fprintf(writer, "File: %s\n", file.Path)
	if file.Type == types.NodeTypeBinary {
		fmt.Fprintf(writer, "%s%s\n", mimeTypeLabel, file.MimeType)
		if file.Content == "" {
			fmt.Fprintln(writer, binaryContentOmitted)
		} else {
			fmt.Fprintln(writer, file.Content)
		}
	} else {
		fmt.Fprintln(writer, file.Content)
	}
	fmt.Fprintf(writer, "End of file: %s\n", file.Path)
	fmt.Fprintln(writer, separatorLine)
}

// FormatSummaryLine formats an OutputSummary into the raw summary line.
func FormatSummaryLine(summary *types.OutputSummary) string {
	if summary == nil {
		summary = &types.OutputSummary{}
	}
	label := "files"
	if summary.TotalFiles == 1 {
		label = "file"
	}
	extra := ""
	if summary.TotalTokens > 0 {
		extra = fmt.Sprintf(", %d tokens", summary.TotalTokens)
	}
	modelSuffix := ""
	if summary.Model != "" {
		modelSuffix = fmt.Sprintf(" (model: %s)", summary.Model)
	}
	return fmt.Sprintf("Summary: %d %s, %s%s%s", summary.TotalFiles, label, summary.TotalSize, extra, modelSuffix)
}

// orderedFunctionNames returns a deterministic ordering of function names.
func orderedFunctionNames(data *types.CallChainOutput) []string {
	seen := map[string]struct{}{}
	var order []string
	add := func(name string) {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			order = append(order, name)
		}
	}
	add(data.TargetFunction)
	for _, callerName := range data.Callers {
		add(callerName)
	}
	if data.Callees != nil {
		for _, calleeName := range *data.Callees {
			add(calleeName)
		}
	}
	var others []string
	for name := range data.Functions {
		if _, ok := seen[name]; !ok {
			others = append(others, name)
		}
	}
	sort.Strings(others)
	order = append(order, others...)
	return order
}
