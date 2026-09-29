package callchain

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	python "github.com/smacker/go-tree-sitter/python"
	"github.com/tyemirov/ctx/internal/types"
)

const (
	pythonFileExtension          = ".py"
	pythonFunctionNodeType       = "function_definition"
	pythonClassNodeType          = "class_definition"
	pythonCallNodeType           = "call"
	pythonNameField              = "name"
	pythonBodyField              = "body"
	pythonCallFunctionField      = "function"
	pythonAttributeNodeType      = "attribute"
	pythonObjectField            = "object"
	pythonAttributeField         = "attribute"
	pythonSelfIdentifier         = "self"
	pythonClassIdentifier        = "cls"
	pythonUnresolvedSymbolFormat = "%w: python symbol %s"
)

type pythonAnalyzer struct {
	parser *sitter.Parser
}

type pythonFunction struct {
	qualifiedName      string
	moduleName         string
	classQualifiedName string
	classNames         []string
	simpleName         string
	calls              []string
	source             string
	filePath           string
}

// NewPythonAnalyzer constructs an Analyzer for Python source files.
func NewPythonAnalyzer() Analyzer {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	return &pythonAnalyzer{parser: parser}
}

func (analyzer *pythonAnalyzer) Analyze(request AnalyzerRequest) (*types.CallChainOutput, error) {
	functions, collectError := analyzer.collectFunctions(request.RepositoryRootDirectory)
	if collectError != nil {
		return nil, collectError
	}
	if len(functions) == 0 {
		return nil, fmt.Errorf(pythonUnresolvedSymbolFormat, ErrSymbolNotFound, request.TargetSymbol)
	}

	recordsByQualifiedName := map[string]*pythonFunction{}
	moduleToFunctions := map[string]map[string]*pythonFunction{}
	classToFunctions := map[string]map[string]*pythonFunction{}
	simpleNameToFunctions := map[string][]*pythonFunction{}
	for index := range functions {
		record := &functions[index]
		recordsByQualifiedName[record.qualifiedName] = record
		if moduleToFunctions[record.moduleName] == nil {
			moduleToFunctions[record.moduleName] = map[string]*pythonFunction{}
		}
		moduleToFunctions[record.moduleName][record.simpleName] = record
		if record.classQualifiedName != "" {
			if classToFunctions[record.classQualifiedName] == nil {
				classToFunctions[record.classQualifiedName] = map[string]*pythonFunction{}
			}
			classToFunctions[record.classQualifiedName][record.simpleName] = record
		}
		simpleNameToFunctions[record.simpleName] = append(simpleNameToFunctions[record.simpleName], record)
	}

	targetRecord := findPythonTarget(recordsByQualifiedName, simpleNameToFunctions, request.TargetSymbol)
	if targetRecord == nil {
		return nil, fmt.Errorf(pythonUnresolvedSymbolFormat, ErrSymbolNotFound, request.TargetSymbol)
	}

	resolvedCallers := map[string]map[string]struct{}{}
	resolvedCallees := map[string]map[string]struct{}{}
	for _, record := range recordsByQualifiedName {
		for _, callText := range record.calls {
			resolved := analyzer.resolvePythonCall(callText, record, recordsByQualifiedName, moduleToFunctions, classToFunctions, simpleNameToFunctions)
			if resolved == "" {
				continue
			}
			callers := resolvedCallers[resolved]
			if callers == nil {
				callers = map[string]struct{}{}
				resolvedCallers[resolved] = callers
			}
			callers[record.qualifiedName] = struct{}{}
			callees := resolvedCallees[record.qualifiedName]
			if callees == nil {
				callees = map[string]struct{}{}
				resolvedCallees[record.qualifiedName] = callees
			}
			callees[resolved] = struct{}{}
		}
	}

	callerNames := collectReachable(targetRecord.qualifiedName, resolvedCallers, request.MaximumDepth)
	calleeNames := collectReachable(targetRecord.qualifiedName, resolvedCallees, request.MaximumDepth)

	relevantNames := map[string]struct{}{targetRecord.qualifiedName: {}}
	for _, caller := range callerNames {
		relevantNames[caller] = struct{}{}
	}
	for _, callee := range calleeNames {
		relevantNames[callee] = struct{}{}
	}

	functionsMap := map[string]string{}
	documentationFiles := map[string]struct{}{}
	for name := range relevantNames {
		record := recordsByQualifiedName[name]
		if record == nil {
			continue
		}
		functionsMap[name] = record.source
		if request.IncludeDocumentation {
			documentationFiles[record.filePath] = struct{}{}
		}
	}

	output := &types.CallChainOutput{
		TargetFunction: targetRecord.qualifiedName,
		Callers:        callerNames,
		Functions:      functionsMap,
	}
	if len(calleeNames) > 0 {
		output.Callees = &calleeNames
	}

	if request.IncludeDocumentation && request.DocumentationCollector != nil {
		for filePath := range documentationFiles {
			entries, documentationError := request.DocumentationCollector.CollectFromFile(filePath)
			if documentationError == nil && len(entries) > 0 {
				output.Documentation = append(output.Documentation, entries...)
			}
		}
	}

	return output, nil
}

func (analyzer *pythonAnalyzer) collectFunctions(root string) ([]pythonFunction, error) {
	var functions []pythonFunction
	walkError := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() != "." && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != pythonFileExtension {
			return nil
		}
		content, readError := os.ReadFile(path)
		if readError != nil {
			return readError
		}
		tree, parseError := analyzer.parser.ParseCtx(context.Background(), nil, content)
		if parseError != nil {
			return fmt.Errorf("parse %s: %w", path, parseError)
		}
		defer tree.Close()
		moduleName := pythonModuleName(root, path)
		analyzer.walkPythonTree(tree.RootNode(), content, moduleName, path, &functions)
		return nil
	})
	if walkError != nil {
		return nil, walkError
	}
	return functions, nil
}

func (analyzer *pythonAnalyzer) walkPythonTree(node *sitter.Node, content []byte, moduleName string, filePath string, functions *[]pythonFunction) {
	if node == nil {
		return
	}
	if node.Type() == pythonFunctionNodeType {
		nameNode := node.ChildByFieldName(pythonNameField)
		if nameNode != nil {
			functionName := strings.TrimSpace(string(content[nameNode.StartByte():nameNode.EndByte()]))
			classNames := pythonClassStack(node, content)
			classQualifiedName := pythonComposeClassQualifiedName(moduleName, classNames)
			qualifiedName := pythonComposeQualifiedName(moduleName, classNames, functionName)
			calls := collectPythonCalls(node, content)
			*functions = append(*functions, pythonFunction{
				qualifiedName:      qualifiedName,
				moduleName:         moduleName,
				classQualifiedName: classQualifiedName,
				classNames:         classNames,
				simpleName:         functionName,
				calls:              calls,
				source:             string(content[node.StartByte():node.EndByte()]),
				filePath:           filePath,
			})
		}
	}
	for index := 0; index < int(node.ChildCount()); index++ {
		analyzer.walkPythonTree(node.Child(index), content, moduleName, filePath, functions)
	}
}

func collectPythonCalls(functionNode *sitter.Node, content []byte) []string {
	var calls []string
	var walk func(current *sitter.Node)
	walk = func(current *sitter.Node) {
		if current == nil {
			return
		}
		if current != functionNode && current.Type() == pythonFunctionNodeType {
			return
		}
		if current.Type() == pythonCallNodeType {
			targetNode := current.ChildByFieldName(pythonCallFunctionField)
			if targetNode != nil {
				calls = append(calls, extractPythonCallTarget(targetNode, content))
			}
		}
		for index := 0; index < int(current.ChildCount()); index++ {
			walk(current.Child(index))
		}
	}
	body := functionNode.ChildByFieldName(pythonBodyField)
	if body != nil {
		walk(body)
	} else {
		walk(functionNode)
	}
	return calls
}

func extractPythonCallTarget(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	if node.Type() == pythonAttributeNodeType {
		objectNode := node.ChildByFieldName(pythonObjectField)
		attributeNode := node.ChildByFieldName(pythonAttributeField)
		if objectNode == nil || attributeNode == nil {
			return strings.TrimSpace(string(content[node.StartByte():node.EndByte()]))
		}
		left := extractPythonCallTarget(objectNode, content)
		right := strings.TrimSpace(string(content[attributeNode.StartByte():attributeNode.EndByte()]))
		if left == "" {
			return right
		}
		return left + qualifiedNameSeparator + right
	}
	return strings.TrimSpace(string(content[node.StartByte():node.EndByte()]))
}

func (analyzer *pythonAnalyzer) resolvePythonCall(callText string, origin *pythonFunction, records map[string]*pythonFunction, moduleToFunctions map[string]map[string]*pythonFunction, classToFunctions map[string]map[string]*pythonFunction, simpleNameToFunctions map[string][]*pythonFunction) string {
	cleaned := strings.TrimSpace(callText)
	if cleaned == "" {
		return ""
	}
	segments := strings.Split(cleaned, qualifiedNameSeparator)
	if len(segments) == 1 {
		if bucket := moduleToFunctions[origin.moduleName]; bucket != nil {
			if candidate := bucket[segments[0]]; candidate != nil {
				return candidate.qualifiedName
			}
		}
		if origin.classQualifiedName != "" {
			if bucket := classToFunctions[origin.classQualifiedName]; bucket != nil {
				if candidate := bucket[segments[0]]; candidate != nil {
					return candidate.qualifiedName
				}
			}
		}
		if candidates := simpleNameToFunctions[segments[0]]; len(candidates) == 1 {
			return candidates[0].qualifiedName
		}
		return ""
	}
	first := segments[0]
	if first == pythonSelfIdentifier || first == pythonClassIdentifier {
		if origin.classQualifiedName != "" {
			if bucket := classToFunctions[origin.classQualifiedName]; bucket != nil {
				if candidate := bucket[segments[len(segments)-1]]; candidate != nil {
					return candidate.qualifiedName
				}
			}
		}
		return ""
	}
	joined := cleaned
	if candidate := records[joined]; candidate != nil {
		return candidate.qualifiedName
	}
	modulePrefixed := origin.moduleName + qualifiedNameSeparator + joined
	if candidate := records[modulePrefixed]; candidate != nil {
		return candidate.qualifiedName
	}
	suffix := qualifiedNameSeparator + joined
	for name, record := range records {
		if strings.HasSuffix(name, suffix) {
			return record.qualifiedName
		}
	}
	return ""
}

func findPythonTarget(records map[string]*pythonFunction, simpleNameToFunctions map[string][]*pythonFunction, target string) *pythonFunction {
	if record := records[target]; record != nil {
		return record
	}
	if candidates := simpleNameToFunctions[target]; len(candidates) == 1 {
		return candidates[0]
	}
	suffix := qualifiedNameSeparator + target
	for name, record := range records {
		if strings.HasSuffix(name, suffix) {
			return record
		}
	}
	return nil
}

func pythonClassStack(node *sitter.Node, content []byte) []string {
	var stack []string
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Type() != pythonClassNodeType {
			continue
		}
		nameNode := parent.ChildByFieldName(pythonNameField)
		if nameNode == nil {
			continue
		}
		className := strings.TrimSpace(string(content[nameNode.StartByte():nameNode.EndByte()]))
		stack = append([]string{className}, stack...)
	}
	return stack
}

func pythonComposeClassQualifiedName(moduleName string, classNames []string) string {
	if len(classNames) == 0 {
		return ""
	}
	if moduleName == "" {
		return strings.Join(classNames, qualifiedNameSeparator)
	}
	return moduleName + qualifiedNameSeparator + strings.Join(classNames, qualifiedNameSeparator)
}

func pythonComposeQualifiedName(moduleName string, classNames []string, functionName string) string {
	if len(classNames) == 0 {
		if moduleName == "" {
			return functionName
		}
		return moduleName + qualifiedNameSeparator + functionName
	}
	classPath := strings.Join(classNames, qualifiedNameSeparator)
	if moduleName != "" {
		classPath = moduleName + qualifiedNameSeparator + classPath
	}
	return classPath + qualifiedNameSeparator + functionName
}

func pythonModuleName(root string, filePath string) string {
	relative, err := filepath.Rel(root, filePath)
	if err != nil {
		relative = filePath
	}
	withoutExt := strings.TrimSuffix(relative, filepath.Ext(relative))
	normalized := strings.ReplaceAll(withoutExt, string(filepath.Separator), qualifiedNameSeparator)
	normalized = strings.TrimSuffix(normalized, qualifiedNameSeparator+"__init__")
	normalized = strings.Trim(normalized, qualifiedNameSeparator)
	return normalized
}
