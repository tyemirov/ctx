package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/ctx/internal/services/mcp"
	"github.com/tyemirov/ctx/internal/types"
)

func TestStartMCPServerServesCapabilities(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
	}{{name: "serves capabilities"}}

	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var buffer bytes.Buffer
			done := make(chan error, 1)

			go func() {
				done <- startMCPServer(ctx, &buffer)
			}()

			address := waitForMCPAddress(t, &buffer)

			request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/capabilities", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}

			client := http.Client{Timeout: 2 * time.Second}
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("perform request: %v", err)
			}
			defer response.Body.Close()

			if response.StatusCode != http.StatusOK {
				t.Fatalf("unexpected status: %d", response.StatusCode)
			}

			var body struct {
				Capabilities  []mcp.Capability `json:"capabilities"`
				RootDirectory string           `json:"rootDirectory"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}

			expected := mcpCapabilities()
			if len(body.Capabilities) != len(expected) {
				t.Fatalf("expected %d capabilities, got %d", len(expected), len(body.Capabilities))
			}

			for index, capability := range expected {
				payload := body.Capabilities[index]
				if payload != capability {
					t.Fatalf("capability %d mismatch: got %+v, want %+v", index, payload, capability)
				}
			}

			workingDirectory, workingDirectoryError := os.Getwd()
			if workingDirectoryError != nil {
				t.Fatalf("get working directory: %v", workingDirectoryError)
			}
			if body.RootDirectory != workingDirectory {
				t.Fatalf("unexpected root directory: %s", body.RootDirectory)
			}

			cancel()
			if err := <-done; err != nil {
				t.Fatalf("server shutdown error: %v", err)
			}
		})
	}
}

func waitForMCPAddress(t *testing.T, buffer *bytes.Buffer) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		output := buffer.String()
		if output != "" {
			for _, line := range strings.Split(output, "\n") {
				if strings.HasPrefix(line, "MCP server listening on ") {
					return strings.TrimPrefix(line, "MCP server listening on ")
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server address not reported: %s", buffer.String())
	return ""
}

func TestStartMCPServerExecutesTreeCommand(t *testing.T) {
	t.Parallel()

	temporaryDirectory := t.TempDir()
	filePath := temporaryDirectory + "/file.txt"
	if writeErr := os.WriteFile(filePath, []byte("content"), 0o600); writeErr != nil {
		t.Fatalf("write file: %v", writeErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buffer bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- startMCPServer(ctx, &buffer)
	}()

	address := waitForMCPAddress(t, &buffer)

	client := http.Client{Timeout: 2 * time.Second}
	requestBody := bytes.NewBufferString(fmt.Sprintf(`{"paths":["%s"],"summary":false}`, temporaryDirectory))
	request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address+"/commands/tree", requestBody)
	if requestErr != nil {
		t.Fatalf("create request: %v", requestErr)
	}
	response, responseErr := client.Do(request)
	if responseErr != nil {
		t.Fatalf("execute request: %v", responseErr)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.StatusCode)
	}

	var body mcp.CommandResponse
	if decodeErr := json.NewDecoder(response.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if !strings.Contains(body.Output, temporaryDirectory) {
		t.Fatalf("expected output to include path %s", temporaryDirectory)
	}
	if body.Format != "json" {
		t.Fatalf("unexpected format: %s", body.Format)
	}
	if len(body.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", body.Warnings)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("server shutdown error: %v", err)
	}
}

func TestStartMCPServerReportsEnvironment(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buffer bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- startMCPServer(ctx, &buffer)
	}()

	address := waitForMCPAddress(t, &buffer)

	client := http.Client{Timeout: 2 * time.Second}
	request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/environment", nil)
	if requestErr != nil {
		t.Fatalf("create request: %v", requestErr)
	}

	response, responseErr := client.Do(request)
	if responseErr != nil {
		t.Fatalf("execute request: %v", responseErr)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.StatusCode)
	}

	var payload struct {
		RootDirectory string `json:"rootDirectory"`
	}
	if decodeErr := json.NewDecoder(response.Body).Decode(&payload); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}

	workingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		t.Fatalf("get working directory: %v", workingDirectoryError)
	}

	if payload.RootDirectory != workingDirectory {
		t.Fatalf("unexpected root directory: %s", payload.RootDirectory)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("server shutdown error: %v", err)
	}
}

func TestExecuteDocCommandHandlesWebURL(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(writer, `<html><head><title>Sample Docs</title></head><body><p>Content</p></body></html>`)
	}))
	t.Cleanup(server.Close)

	payload := docRequest{
		RepositoryURL: server.URL,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	response, execErr := executeDocCommand(context.Background(), mcp.CommandRequest{
		Payload: data,
	})
	if execErr != nil {
		t.Fatalf("execute doc command: %v", execErr)
	}
	if !strings.Contains(response.Output, "Sample Docs") {
		t.Fatalf("expected output to include page title, got %q", response.Output)
	}
	if response.Format != types.FormatRaw {
		t.Fatalf("unexpected format %s", response.Format)
	}
}

func TestStartMCPServerProvidesDocumentationForMultipleLanguages(t *testing.T) {
	t.Parallel()

	currentDirectory, currentDirectoryError := os.Getwd()
	if currentDirectoryError != nil {
		t.Fatalf("get working directory: %v", currentDirectoryError)
	}

	temporaryDirectory := t.TempDir()
	if strings.HasPrefix(temporaryDirectory, currentDirectory) {
		t.Fatalf("temporary directory %s must be outside current directory %s", temporaryDirectory, currentDirectory)
	}

	goFilePath := filepath.Join(temporaryDirectory, "sample.go")
	goSource := `package sample

import "net/http"

func UseMethod() string {
	return http.MethodGet
}
`
	if writeErr := os.WriteFile(goFilePath, []byte(goSource), 0o600); writeErr != nil {
		t.Fatalf("write go file: %v", writeErr)
	}

	javaScriptFilePath := filepath.Join(temporaryDirectory, "module.js")
	javaScriptSource := `/**
 * Adds two numbers.
 */
export function add(a, b) {
	return a + b;
}
`
	if writeErr := os.WriteFile(javaScriptFilePath, []byte(javaScriptSource), 0o600); writeErr != nil {
		t.Fatalf("write javascript file: %v", writeErr)
	}

	pythonFilePath := filepath.Join(temporaryDirectory, "module.py")
	pythonSource := `"""Sample module docstring."""

def greet(name):
	"""Return greeting text."""
	return f"Hello {name}"
`
	if writeErr := os.WriteFile(pythonFilePath, []byte(pythonSource), 0o600); writeErr != nil {
		t.Fatalf("write python file: %v", writeErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buffer bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- startMCPServer(ctx, &buffer)
	}()

	address := waitForMCPAddress(t, &buffer)

	client := http.Client{Timeout: 5 * time.Second}
	requestBody := bytes.NewBufferString(fmt.Sprintf(`{"paths":["%s"],"summary":false,"documentation":true}`, temporaryDirectory))
	request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address+"/commands/content", requestBody)
	if requestErr != nil {
		t.Fatalf("create request: %v", requestErr)
	}
	request.Header.Set("Content-Type", "application/json")

	response, responseErr := client.Do(request)
	if responseErr != nil {
		t.Fatalf("execute request: %v", responseErr)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.StatusCode)
	}

	var commandResponse mcp.CommandResponse
	if decodeErr := json.NewDecoder(response.Body).Decode(&commandResponse); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if commandResponse.Format != "json" {
		t.Fatalf("unexpected format: %s", commandResponse.Format)
	}
	if commandResponse.Output == "" {
		t.Fatalf("expected output data")
	}

	type contentNode struct {
		Path          string                     `json:"path"`
		Type          string                     `json:"type"`
		Children      []contentNode              `json:"children"`
		Documentation []types.DocumentationEntry `json:"documentation"`
	}

	var root contentNode
	if unmarshalErr := json.Unmarshal([]byte(commandResponse.Output), &root); unmarshalErr != nil {
		t.Fatalf("unmarshal content output: %v", unmarshalErr)
	}
	if root.Path != temporaryDirectory {
		t.Fatalf("expected root path %s, got %s", temporaryDirectory, root.Path)
	}

	foundGoDocumentation := false
	foundJavaScriptDocumentation := false
	foundPythonDocumentation := false

	var traverse func(node contentNode)
	traverse = func(node contentNode) {
		if len(node.Documentation) > 0 {
			for _, entry := range node.Documentation {
				if entry.Name == "" || entry.Doc == "" {
					t.Fatalf("documentation entry missing fields for path %s: %+v", node.Path, entry)
				}
				if node.Path == goFilePath && strings.Contains(entry.Name, "net/http") {
					foundGoDocumentation = true
				}
				if node.Path == javaScriptFilePath && strings.Contains(entry.Name, "module.add") {
					foundJavaScriptDocumentation = true
				}
				if node.Path == pythonFilePath && (strings.Contains(entry.Name, "module") || strings.Contains(entry.Name, "greet")) {
					foundPythonDocumentation = true
				}
			}
		}
		for _, child := range node.Children {
			traverse(child)
		}
	}

	traverse(root)

	if !foundGoDocumentation {
		t.Fatalf("expected Go documentation entries for %s", goFilePath)
	}
	if !foundJavaScriptDocumentation {
		t.Fatalf("expected JavaScript documentation entries for %s", javaScriptFilePath)
	}
	if !foundPythonDocumentation {
		t.Fatalf("expected Python documentation entries for %s", pythonFilePath)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("server shutdown error: %v", err)
	}
}
