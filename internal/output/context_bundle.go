package output

import (
	"encoding/json"

	"github.com/tyemirov/ctx/internal/types"
)

// RenderContextBundleJSON marshals a context bundle as deterministic JSON.
func RenderContextBundleJSON(bundle *types.ContextBundleOutput) (string, error) {
	encoded, encodeErr := json.MarshalIndent(bundle, indentPrefix, indentSpacer)
	return string(encoded), encodeErr
}

// RenderContextBundleToon renders the context bundle in TOON format for prompt inclusion.
func RenderContextBundleToon(bundle *types.ContextBundleOutput) string {
	if bundle == nil {
		return ""
	}
	builder := toonBuilder{}
	builder.writeIndent(0)
	builder.buffer.WriteString("contextBundle:\n")
	builder.writeField(1, toonField{key: "version", value: bundle.Version})
	builder.writeField(1, toonField{key: "generatedBy", value: bundle.GeneratedBy})
	builder.writeIndent(1)
	builder.buffer.WriteString("repository:\n")
	builder.writeField(2, toonField{key: "root", value: bundle.Repository.Root})
	builder.writeField(2, toonField{key: "name", value: bundle.Repository.Name})
	builder.writeIndent(1)
	builder.buffer.WriteString("goal:\n")
	if bundle.Goal.ID != "" {
		builder.writeField(2, toonField{key: "id", value: bundle.Goal.ID})
	}
	if bundle.Goal.Kind != "" {
		builder.writeField(2, toonField{key: "kind", value: bundle.Goal.Kind})
	}
	if bundle.Goal.Category != "" {
		builder.writeField(2, toonField{key: "category", value: bundle.Goal.Category})
	}
	builder.writeField(2, toonField{key: "title", value: bundle.Goal.Title})
	if bundle.Goal.Body != "" {
		builder.writeField(2, toonField{key: "body", value: bundle.Goal.Body})
	}
	builder.writeIndent(1)
	builder.buffer.WriteString("budget:\n")
	builder.writeField(2, toonField{key: "maxTokens", value: bundle.Budget.MaxTokens})
	builder.writeField(2, toonField{key: "usedTokens", value: bundle.Budget.UsedTokens})
	builder.writeField(2, toonField{key: "model", value: bundle.Budget.Model})
	builder.writeScalarArray(1, "terms", bundle.Terms)
	writeContextBundleItems(&builder, 1, "contracts", bundle.Contracts)
	writeContextBundleItems(&builder, 1, "files", bundle.Files)
	writeContextBundleSymbols(&builder, 1, bundle.Symbols)
	writeContextBundleExclusions(&builder, 1, bundle.Exclusions)
	return builder.String()
}

func writeContextBundleItems(builder *toonBuilder, indent int, name string, items []types.ContextBundleItem) {
	builder.writeArrayHeader(indent, name, len(items))
	for _, item := range items {
		fields := []toonField{
			{key: "path", value: item.Path},
			{key: "role", value: item.Role},
			{key: "reason", value: item.Reason},
		}
		if item.Score != 0 {
			fields = append(fields, toonField{key: "score", value: item.Score})
		}
		fields = append(fields,
			toonField{key: "tokens", value: item.Tokens},
			toonField{key: "sha256", value: item.SHA256},
			toonField{key: "lineStart", value: item.LineStart},
			toonField{key: "lineEnd", value: item.LineEnd},
			toonField{key: "content", value: item.Content},
		)
		builder.writeObjectInArray(indent+1, fields)
	}
}

func writeContextBundleSymbols(builder *toonBuilder, indent int, symbols []types.ContextBundleSymbol) {
	builder.writeArrayHeader(indent, "symbols", len(symbols))
	for _, symbol := range symbols {
		fields := []toonField{
			{key: "path", value: symbol.Path},
			{key: "language", value: symbol.Language},
			{key: "kind", value: symbol.Kind},
			{key: "name", value: symbol.Name},
			{key: "qualifiedName", value: symbol.QualifiedName},
			{key: "lineStart", value: symbol.LineStart},
			{key: "lineEnd", value: symbol.LineEnd},
		}
		builder.writeObjectInArray(indent+1, fields)
	}
}

func writeContextBundleExclusions(builder *toonBuilder, indent int, exclusions []types.ContextBundleExclusion) {
	builder.writeArrayHeader(indent, "exclusions", len(exclusions))
	for _, exclusion := range exclusions {
		fields := []toonField{
			{key: "path", value: exclusion.Path},
		}
		if exclusion.Role != "" {
			fields = append(fields, toonField{key: "role", value: exclusion.Role})
		}
		fields = append(fields, toonField{key: "reason", value: exclusion.Reason})
		if exclusion.Score != 0 {
			fields = append(fields, toonField{key: "score", value: exclusion.Score})
		}
		builder.writeObjectInArray(indent+1, fields)
	}
}
