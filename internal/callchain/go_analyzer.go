package callchain

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/tyemirov/ctx/internal/types"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const (
	goPackagePattern             = "./..."
	errorFailedToLoadPackages    = "failed to load packages: %w"
	errorLoadingPackagesProduced = "errors encountered while loading packages"
	errorGoFunctionNotFound      = "target function %q not found in call graph"
	qualifiedNameSeparator       = "."
)

// GoAnalyzer resolves call chains for Go functions.
type GoAnalyzer struct{}

// NewGoAnalyzer constructs a GoAnalyzer instance.
func NewGoAnalyzer() *GoAnalyzer {
	return &GoAnalyzer{}
}

// Analyze performs Go call graph traversal.
func (analyzer *GoAnalyzer) Analyze(request AnalyzerRequest) (*types.CallChainOutput, error) {
	if !goModuleExists(request.RepositoryRootDirectory) {
		return nil, fmt.Errorf("%w: go module not found", ErrSymbolNotFound)
	}
	configuration := &packages.Config{
		Mode: packages.LoadAllSyntax,
		Dir:  request.RepositoryRootDirectory,
		Fset: token.NewFileSet(),
	}
	loadedPackages, loadError := packages.Load(configuration, goPackagePattern)
	if loadError != nil {
		return nil, fmt.Errorf(errorFailedToLoadPackages, loadError)
	}
	if packages.PrintErrors(loadedPackages) > 0 {
		return nil, errors.New(errorLoadingPackagesProduced)
	}

	ssaProgram, _ := ssautil.Packages(loadedPackages, ssa.BuilderMode(0))
	ssaProgram.Build()

	callGraphRoot := static.CallGraph(ssaProgram)
	callGraphRoot.DeleteSyntheticNodes()

	targetNode := selectFunctionNode(callGraphRoot, request.TargetSymbol)
	if targetNode == nil {
		return nil, fmt.Errorf("%w: %s", ErrSymbolNotFound, fmt.Sprintf(errorGoFunctionNotFound, request.TargetSymbol))
	}

	callerNames := traverseCallers(targetNode, request.MaximumDepth)
	calleeNames := traverseCallees(targetNode, request.MaximumDepth)

	relevantFunctionNames := map[string]struct{}{targetNode.Func.String(): {}}
	for _, name := range callerNames {
		relevantFunctionNames[name] = struct{}{}
	}
	for _, name := range calleeNames {
		relevantFunctionNames[name] = struct{}{}
	}

	functionSources := make(map[string]string)
	extractedFilePaths := make(map[string]struct{})

	for _, loadedPackage := range loadedPackages {
		for _, file := range loadedPackage.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				functionDeclaration, ok := node.(*ast.FuncDecl)
				if !ok || functionDeclaration.Name == nil {
					return true
				}
				qualifiedName := composeGoQualifiedName(loadedPackage, functionDeclaration)
				if _, needed := relevantFunctionNames[qualifiedName]; !needed {
					return true
				}
				var functionBuffer bytes.Buffer
				(&printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}).Fprint(&functionBuffer, configuration.Fset, functionDeclaration)
				functionSources[qualifiedName] = functionBuffer.String()
				if request.IncludeDocumentation {
					if position := configuration.Fset.File(functionDeclaration.Pos()); position != nil {
						extractedFilePaths[position.Name()] = struct{}{}
					}
				}
				return true
			})
		}
	}

	output := &types.CallChainOutput{
		TargetFunction: request.TargetSymbol,
		Callers:        callerNames,
		Functions:      functionSources,
	}
	if len(calleeNames) > 0 {
		output.Callees = &calleeNames
	}

	if request.IncludeDocumentation && request.DocumentationCollector != nil {
		repositoryPrefix := request.RepositoryRootDirectory + string(os.PathSeparator)
		for filePath := range extractedFilePaths {
			if filePath != request.RepositoryRootDirectory && !strings.HasPrefix(filePath, repositoryPrefix) {
				continue
			}
			documentationEntries, documentationError := request.DocumentationCollector.CollectFromFile(filePath)
			if documentationError == nil && len(documentationEntries) > 0 {
				output.Documentation = append(output.Documentation, documentationEntries...)
			}
		}
	}

	return output, nil
}

func traverseCallers(targetNode *callgraph.Node, maximumDepth int) []string {
	visited := map[*callgraph.Node]struct{}{targetNode: {}}
	queue := []*callgraph.Node{targetNode}
	var callers []string
	for depth := 0; depth < maximumDepth; depth++ {
		var next []*callgraph.Node
		for _, node := range queue {
			for _, inEdge := range node.In {
				if inEdge.Caller == nil || inEdge.Caller.Func == nil {
					continue
				}
				if _, seen := visited[inEdge.Caller]; seen {
					continue
				}
				visited[inEdge.Caller] = struct{}{}
				callers = append(callers, inEdge.Caller.Func.String())
				next = append(next, inEdge.Caller)
			}
		}
		queue = next
	}
	return callers
}

func traverseCallees(targetNode *callgraph.Node, maximumDepth int) []string {
	visited := map[*callgraph.Node]struct{}{targetNode: {}}
	queue := []*callgraph.Node{targetNode}
	var callees []string
	for depth := 0; depth < maximumDepth; depth++ {
		var next []*callgraph.Node
		for _, node := range queue {
			for _, outEdge := range node.Out {
				if outEdge.Callee == nil || outEdge.Callee.Func == nil {
					continue
				}
				if _, seen := visited[outEdge.Callee]; seen {
					continue
				}
				visited[outEdge.Callee] = struct{}{}
				callees = append(callees, outEdge.Callee.Func.String())
				next = append(next, outEdge.Callee)
			}
		}
		queue = next
	}
	return callees
}

func composeGoQualifiedName(packageData *packages.Package, functionDeclaration *ast.FuncDecl) string {
	name := functionDeclaration.Name.Name
	if functionDeclaration.Recv != nil && len(functionDeclaration.Recv.List) > 0 {
		var buffer bytes.Buffer
		printer.Fprint(&buffer, packageData.Fset, functionDeclaration.Recv.List[0].Type)
		return packageData.PkgPath + qualifiedNameSeparator + strings.TrimSpace(buffer.String()) + qualifiedNameSeparator + name
	}
	return packageData.PkgPath + qualifiedNameSeparator + name
}

func selectFunctionNode(graph *callgraph.Graph, candidate string) *callgraph.Node {
	short := candidate
	if lastDotIndex := strings.LastIndex(candidate, qualifiedNameSeparator); lastDotIndex >= 0 && lastDotIndex < len(candidate)-1 {
		short = candidate[lastDotIndex+1:]
	}
	var best *callgraph.Node
	for _, node := range graph.Nodes {
		if node.Func == nil {
			continue
		}
		full := node.Func.String()
		if full == candidate {
			return node
		}
		if strings.HasSuffix(full, qualifiedNameSeparator+candidate) || strings.HasSuffix(full, qualifiedNameSeparator+short) {
			if best == nil || len(full) > len(best.Func.String()) {
				best = node
			}
		}
	}
	return best
}

func goModuleExists(startingDirectory string) bool {
	current := startingDirectory
	for {
		if current == "" {
			return false
		}
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}
