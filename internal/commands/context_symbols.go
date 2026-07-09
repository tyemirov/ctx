package commands

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	javascript "github.com/smacker/go-tree-sitter/javascript"
	python "github.com/smacker/go-tree-sitter/python"
	"github.com/tyemirov/ctx/internal/types"
)

const (
	symbolLanguageGo         = "go"
	symbolLanguageJavaScript = "javascript"
	symbolLanguagePython     = "python"
	symbolKindClass          = "class"
	symbolKindFunction       = "function"
	symbolKindMethod         = "method"
	symbolKindType           = "type"
)

func extractContextSymbols(root string, relativePath string, content []byte) []types.ContextBundleSymbol {
	switch strings.ToLower(filepath.Ext(relativePath)) {
	case ".go":
		return extractGoContextSymbols(root, relativePath, content)
	case ".js", ".mjs", ".cjs":
		return extractJavaScriptContextSymbols(relativePath, content)
	case ".py":
		return extractPythonContextSymbols(relativePath, content)
	default:
		return nil
	}
}

func extractGoContextSymbols(root string, relativePath string, content []byte) []types.ContextBundleSymbol {
	fileSet := token.NewFileSet()
	parsedFile, parseErr := parser.ParseFile(fileSet, filepath.Join(root, relativePath), content, parser.SkipObjectResolution)
	if parseErr != nil {
		return nil
	}
	moduleName := contextSymbolModuleName(relativePath)
	var symbols []types.ContextBundleSymbol
	ast.Inspect(parsedFile, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncDecl:
			if typed.Name == nil {
				return true
			}
			name := typed.Name.Name
			kind := symbolKindFunction
			if typed.Recv != nil && len(typed.Recv.List) > 0 {
				kind = symbolKindMethod
				name = goReceiverName(fileSet, typed.Recv.List[0].Type) + "." + name
			}
			symbols = append(symbols, types.ContextBundleSymbol{
				Path:          relativePath,
				Language:      symbolLanguageGo,
				Kind:          kind,
				Name:          typed.Name.Name,
				QualifiedName: moduleName + "." + name,
				LineStart:     fileSet.Position(typed.Pos()).Line,
				LineEnd:       fileSet.Position(typed.End()).Line,
			})
			return true
		case *ast.TypeSpec:
			if typed.Name == nil {
				return true
			}
			symbols = append(symbols, types.ContextBundleSymbol{
				Path:          relativePath,
				Language:      symbolLanguageGo,
				Kind:          symbolKindType,
				Name:          typed.Name.Name,
				QualifiedName: moduleName + "." + typed.Name.Name,
				LineStart:     fileSet.Position(typed.Pos()).Line,
				LineEnd:       fileSet.Position(typed.End()).Line,
			})
			return true
		default:
			return true
		}
	})
	return symbols
}

func goReceiverName(fileSet *token.FileSet, expression ast.Expr) string {
	var buffer bytes.Buffer
	if printErr := printer.Fprint(&buffer, fileSet, expression); printErr != nil {
		return "receiver"
	}
	name := strings.TrimSpace(buffer.String())
	name = strings.TrimPrefix(name, "*")
	return name
}

func extractJavaScriptContextSymbols(relativePath string, content []byte) []types.ContextBundleSymbol {
	parserHandle := sitter.NewParser()
	parserHandle.SetLanguage(javascript.GetLanguage())
	tree := parserHandle.Parse(nil, content)
	if tree == nil {
		return nil
	}
	var symbols []types.ContextBundleSymbol
	walkJavaScriptContextSymbols(tree.RootNode(), relativePath, content, nil, &symbols)
	return symbols
}

func walkJavaScriptContextSymbols(node *sitter.Node, relativePath string, content []byte, classStack []string, symbols *[]types.ContextBundleSymbol) {
	if node == nil {
		return
	}
	switch node.Type() {
	case "class_declaration":
		name := contextNodeName(node, content)
		if name != "" {
			qualifiedName := contextQualifiedName(relativePath, classStack, name)
			*symbols = append(*symbols, contextSitterSymbol(relativePath, symbolLanguageJavaScript, symbolKindClass, name, qualifiedName, node))
			nextStack := append(append([]string{}, classStack...), name)
			for index := 0; index < int(node.ChildCount()); index++ {
				walkJavaScriptContextSymbols(node.Child(index), relativePath, content, nextStack, symbols)
			}
			return
		}
	case "function_declaration":
		name := contextNodeName(node, content)
		if name != "" {
			qualifiedName := contextQualifiedName(relativePath, classStack, name)
			*symbols = append(*symbols, contextSitterSymbol(relativePath, symbolLanguageJavaScript, symbolKindFunction, name, qualifiedName, node))
		}
	case "method_definition":
		name := contextNodeName(node, content)
		if name != "" {
			qualifiedName := contextQualifiedName(relativePath, classStack, name)
			*symbols = append(*symbols, contextSitterSymbol(relativePath, symbolLanguageJavaScript, symbolKindMethod, name, qualifiedName, node))
		}
	case "variable_declarator":
		nameNode := node.ChildByFieldName("name")
		valueNode := node.ChildByFieldName("value")
		if nameNode != nil && valueNode != nil && isJavaScriptCallableValue(valueNode.Type()) {
			name := strings.TrimSpace(string(content[nameNode.StartByte():nameNode.EndByte()]))
			qualifiedName := contextQualifiedName(relativePath, classStack, name)
			*symbols = append(*symbols, contextSitterSymbol(relativePath, symbolLanguageJavaScript, symbolKindFunction, name, qualifiedName, node))
		}
	}
	for index := 0; index < int(node.ChildCount()); index++ {
		walkJavaScriptContextSymbols(node.Child(index), relativePath, content, classStack, symbols)
	}
}

func isJavaScriptCallableValue(nodeType string) bool {
	switch nodeType {
	case "arrow_function", "function", "function_expression", "generator_function":
		return true
	default:
		return false
	}
}

func extractPythonContextSymbols(relativePath string, content []byte) []types.ContextBundleSymbol {
	parserHandle := sitter.NewParser()
	parserHandle.SetLanguage(python.GetLanguage())
	tree := parserHandle.Parse(nil, content)
	if tree == nil {
		return nil
	}
	var symbols []types.ContextBundleSymbol
	walkPythonContextSymbols(tree.RootNode(), relativePath, content, nil, &symbols)
	return symbols
}

func walkPythonContextSymbols(node *sitter.Node, relativePath string, content []byte, classStack []string, symbols *[]types.ContextBundleSymbol) {
	if node == nil {
		return
	}
	switch node.Type() {
	case "class_definition":
		name := contextNodeName(node, content)
		if name != "" {
			qualifiedName := contextQualifiedName(relativePath, classStack, name)
			*symbols = append(*symbols, contextSitterSymbol(relativePath, symbolLanguagePython, symbolKindClass, name, qualifiedName, node))
			nextStack := append(append([]string{}, classStack...), name)
			for index := 0; index < int(node.ChildCount()); index++ {
				walkPythonContextSymbols(node.Child(index), relativePath, content, nextStack, symbols)
			}
			return
		}
	case "function_definition":
		name := contextNodeName(node, content)
		if name != "" {
			kind := symbolKindFunction
			if len(classStack) > 0 {
				kind = symbolKindMethod
			}
			qualifiedName := contextQualifiedName(relativePath, classStack, name)
			*symbols = append(*symbols, contextSitterSymbol(relativePath, symbolLanguagePython, kind, name, qualifiedName, node))
		}
	}
	for index := 0; index < int(node.ChildCount()); index++ {
		walkPythonContextSymbols(node.Child(index), relativePath, content, classStack, symbols)
	}
}

func contextNodeName(node *sitter.Node, content []byte) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return ""
	}
	return strings.TrimSpace(string(content[nameNode.StartByte():nameNode.EndByte()]))
}

func contextSitterSymbol(relativePath string, language string, kind string, name string, qualifiedName string, node *sitter.Node) types.ContextBundleSymbol {
	return types.ContextBundleSymbol{
		Path:          relativePath,
		Language:      language,
		Kind:          kind,
		Name:          name,
		QualifiedName: qualifiedName,
		LineStart:     int(node.StartPoint().Row) + 1,
		LineEnd:       int(node.EndPoint().Row) + 1,
	}
}

func contextQualifiedName(relativePath string, stack []string, name string) string {
	parts := []string{contextSymbolModuleName(relativePath)}
	parts = append(parts, stack...)
	parts = append(parts, name)
	return strings.Join(parts, ".")
}

func contextSymbolModuleName(relativePath string) string {
	withoutExtension := strings.TrimSuffix(filepath.ToSlash(relativePath), filepath.Ext(relativePath))
	segments := strings.Split(withoutExtension, "/")
	cleaned := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment != "" {
			cleaned = append(cleaned, segment)
		}
	}
	if len(cleaned) == 0 {
		return "root"
	}
	return strings.Join(cleaned, ".")
}
