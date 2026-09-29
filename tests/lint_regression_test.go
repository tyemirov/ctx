package tests

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestContentIncludesGoStandardLibraryDocumentation(testingHandle *testing.T) {
	repositoryRoot := testingHandle.TempDir()
	writeFile(testingHandle, filepath.Join(repositoryRoot, "go.mod"), "module example.com/docsfixture\n\ngo 1.25.4\n")
	writeFile(testingHandle, filepath.Join(repositoryRoot, "main.go"), "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"fixture\") }\n")
	binary := buildBinary(testingHandle)
	result := runCommand(testingHandle, binary, []string{"content", "--doc=relevant", "--format=json", "main.go"}, repositoryRoot)
	if !strings.Contains(result, "Println formats using the default formats") {
		testingHandle.Fatalf("standard library documentation is absent: %s", result)
	}
}
