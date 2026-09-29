package callchain

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	javascript "github.com/smacker/go-tree-sitter/javascript"
	"github.com/tyemirov/ctx/internal/types"
)

const (
	javaScriptFileExtension          = ".js"
	javaScriptFunctionNodeType       = "function_declaration"
	javaScriptMethodNodeType         = "method_definition"
	javaScriptClassNodeType          = "class_declaration"
	javaScriptCallExpressionType     = "call_expression"
	javaScriptNameField              = "name"
	javaScriptBodyField              = "body"
	javaScriptFunctionField          = "function"
	javaScriptObjectField            = "object"
	javaScriptPropertyField          = "property"
	javaScriptThisIdentifier         = "this"
	javaScriptUnresolvedSymbolFormat = "%w: javascript symbol %s"
)

var javaScriptNestedCallableNodeTypes = map[string]struct{}{
	"function_declaration": {},
	"method_definition":    {},
	"generator_function":   {},
	"function":             {},
	"function_expression":  {},
	"arrow_function":       {},
}

type javaScriptAnalyzer struct {
	parser *sitter.Parser
}

type javaScriptFunction struct {
	qualifiedName      string
	moduleName         string
	classQualifiedName string
	classNames         []string
	simpleName         string
	calls              []string
	source             string
	filePath           string
}

// NewJavaScriptAnalyzer constructs an Analyzer for JavaScript source files.
func NewJavaScriptAnalyzer() Analyzer {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())
	return &javaScriptAnalyzer{parser: parser}
}

func (analyzer *javaScriptAnalyzer) Analyze(request AnalyzerRequest) (*types.CallChainOutput, error) {
	functions, collectError := analyzer.collectFunctions(request.RepositoryRootDirectory)
	if collectError != nil {
		return nil, collectError
	}
	if len(functions) == 0 {
		return nil, fmt.Errorf(javaScriptUnresolvedSymbolFormat, ErrSymbolNotFound, request.TargetSymbol)
	}

	recordsByQualifiedName := map[string]*javaScriptFunction{}
	moduleToFunctions := map[string]map[string]*javaScriptFunction{}
	classToFunctions := map[string]map[string]*javaScriptFunction{}
	simpleNameToFunctions := map[string][]*javaScriptFunction{}
	for index := range functions {
		record := &functions[index]
		recordsByQualifiedName[record.qualifiedName] = record
		if moduleToFunctions[record.moduleName] == nil {
			moduleToFunctions[record.moduleName] = map[string]*javaScriptFunction{}
		}
		moduleToFunctions[record.moduleName][record.simpleName] = record
		if record.classQualifiedName != "" {
			if classToFunctions[record.classQualifiedName] == nil {
				classToFunctions[record.classQualifiedName] = map[string]*javaScriptFunction{}
			}
			classToFunctions[record.classQualifiedName][record.simpleName] = record
		}
		simpleNameToFunctions[record.simpleName] = append(simpleNameToFunctions[record.simpleName], record)
	}

	targetRecord := findJavaScriptTarget(recordsByQualifiedName, simpleNameToFunctions, request.TargetSymbol)
	if targetRecord == nil {
		return nil, fmt.Errorf(javaScriptUnresolvedSymbolFormat, ErrSymbolNotFound, request.TargetSymbol)
	}

	resolvedCallers := map[string]map[string]struct{}{}
	resolvedCallees := map[string]map[string]struct{}{}
	for _, record := range recordsByQualifiedName {
		for _, callText := range record.calls {
			resolved := analyzer.resolveJavaScriptCall(callText, record, recordsByQualifiedName, moduleToFunctions, classToFunctions, simpleNameToFunctions)
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

func (analyzer *javaScriptAnalyzer) collectFunctions(root string) ([]javaScriptFunction, error) {
	var functions []javaScriptFunction
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
		if filepath.Ext(path) != javaScriptFileExtension {
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
		moduleName := javaScriptModuleName(root, path)
		analyzer.walkJavaScriptTree(tree.RootNode(), content, moduleName, path, &functions)
		return nil
	})
	if walkError != nil {
		return nil, walkError
	}
	return functions, nil
}

func (analyzer *javaScriptAnalyzer) walkJavaScriptTree(node *sitter.Node, content []byte, moduleName string, filePath string, functions *[]javaScriptFunction) {
	if node == nil {
		return
	}
	switch node.Type() {
	case javaScriptFunctionNodeType:
		nameNode := node.ChildByFieldName(javaScriptNameField)
		if nameNode != nil {
			functionName := strings.TrimSpace(string(content[nameNode.StartByte():nameNode.EndByte()]))
			classNames := javaScriptClassStack(node, content)
			classQualifiedName := javaScriptComposeClassQualifiedName(moduleName, classNames)
			qualifiedName := javaScriptComposeQualifiedName(moduleName, classNames, functionName)
			calls := collectJavaScriptCalls(node, content)
			*functions = append(*functions, javaScriptFunction{
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
	case javaScriptMethodNodeType:
		propertyNode := node.ChildByFieldName(javaScriptPropertyField)
		if propertyNode != nil {
			functionName := strings.TrimSpace(string(content[propertyNode.StartByte():propertyNode.EndByte()]))
			classNames := javaScriptClassStack(node, content)
			classQualifiedName := javaScriptComposeClassQualifiedName(moduleName, classNames)
			qualifiedName := javaScriptComposeQualifiedName(moduleName, classNames, functionName)
			calls := collectJavaScriptCalls(node, content)
			*functions = append(*functions, javaScriptFunction{
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
		analyzer.walkJavaScriptTree(node.Child(index), content, moduleName, filePath, functions)
	}
}

func collectJavaScriptCalls(node *sitter.Node, content []byte) []string {
	var calls []string
	var walk func(current *sitter.Node)
	walk = func(current *sitter.Node) {
		if current == nil {
			return
		}
		if current != node {
			if _, isCallable := javaScriptNestedCallableNodeTypes[current.Type()]; isCallable {
				return
			}
		}
		if current.Type() == javaScriptCallExpressionType {
			targetNode := current.ChildByFieldName(javaScriptFunctionField)
			if targetNode != nil {
				calls = append(calls, extractJavaScriptCallTarget(targetNode, content))
			}
		}
		for index := 0; index < int(current.ChildCount()); index++ {
			walk(current.Child(index))
		}
	}
	body := node.ChildByFieldName(javaScriptBodyField)
	if body != nil {
		walk(body)
	} else {
		walk(node)
	}
	return calls
}

func extractJavaScriptCallTarget(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	if node.Type() == "member_expression" {
		objectNode := node.ChildByFieldName(javaScriptObjectField)
		propertyNode := node.ChildByFieldName(javaScriptPropertyField)
		if objectNode == nil || propertyNode == nil {
			return strings.TrimSpace(string(content[node.StartByte():node.EndByte()]))
		}
		left := extractJavaScriptCallTarget(objectNode, content)
		right := strings.TrimSpace(string(content[propertyNode.StartByte():propertyNode.EndByte()]))
		if left == "" {
			return right
		}
		return left + qualifiedNameSeparator + right
	}
	return strings.TrimSpace(string(content[node.StartByte():node.EndByte()]))
}

func (analyzer *javaScriptAnalyzer) resolveJavaScriptCall(callText string, origin *javaScriptFunction, records map[string]*javaScriptFunction, moduleToFunctions map[string]map[string]*javaScriptFunction, classToFunctions map[string]map[string]*javaScriptFunction, simpleNameToFunctions map[string][]*javaScriptFunction) string {
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
	if first == javaScriptThisIdentifier {
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

func findJavaScriptTarget(records map[string]*javaScriptFunction, simpleNameToFunctions map[string][]*javaScriptFunction, target string) *javaScriptFunction {
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

func javaScriptClassStack(node *sitter.Node, content []byte) []string {
	var stack []string
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Type() != javaScriptClassNodeType {
			continue
		}
		nameNode := parent.ChildByFieldName(javaScriptNameField)
		if nameNode == nil {
			continue
		}
		className := strings.TrimSpace(string(content[nameNode.StartByte():nameNode.EndByte()]))
		stack = append([]string{className}, stack...)
	}
	return stack
}

func javaScriptComposeClassQualifiedName(moduleName string, classNames []string) string {
	if len(classNames) == 0 {
		return ""
	}
	if moduleName == "" {
		return strings.Join(classNames, qualifiedNameSeparator)
	}
	return moduleName + qualifiedNameSeparator + strings.Join(classNames, qualifiedNameSeparator)
}

func javaScriptComposeQualifiedName(moduleName string, classNames []string, functionName string) string {
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

func javaScriptModuleName(root string, filePath string) string {
	relative, err := filepath.Rel(root, filePath)
	if err != nil {
		relative = filePath
	}
	withoutExt := strings.TrimSuffix(relative, filepath.Ext(relative))
	normalized := strings.ReplaceAll(withoutExt, string(filepath.Separator), qualifiedNameSeparator)
	normalized = strings.TrimSuffix(normalized, qualifiedNameSeparator+"index")
	normalized = strings.Trim(normalized, qualifiedNameSeparator)
	return normalized
}
