package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	defaultListenAddress     = "127.0.0.1:0"
	defaultShutdownDuration  = 5 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
	headerContentType        = "Content-Type"
	mimeTypeJSON             = "application/json"
	capabilitiesPath         = "/capabilities"
	rootPath                 = "/"
	commandsPrefix           = "/commands/"
	environmentPath          = "/environment"
	errorFieldName           = "error"
	errorCommandNotFound     = "command not found"
)

// Capability describes a feature exposed by the MCP server.
type Capability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CommandRequest holds the raw payload supplied by clients.
type CommandRequest struct {
	Payload json.RawMessage
}

// CommandResponse contains the outcome of a command execution.
type CommandResponse struct {
	Output   string   `json:"output"`
	Format   string   `json:"format"`
	Warnings []string `json:"warnings,omitempty"`
}

// CommandExecutor executes a command based on an incoming request.
type CommandExecutor interface {
	Execute(ctx context.Context, request CommandRequest) (CommandResponse, error)
}

// CommandExecutorFunc adapts a function into a CommandExecutor.
type CommandExecutorFunc func(context.Context, CommandRequest) (CommandResponse, error)

// Execute invokes the underlying function.
func (executor CommandExecutorFunc) Execute(ctx context.Context, request CommandRequest) (CommandResponse, error) {
	return executor(ctx, request)
}

// CommandExecutionError represents a failure accompanied by an HTTP status code.
type CommandExecutionError struct {
	statusCode int
	err        error
}

// Error returns the error string.
func (executionError CommandExecutionError) Error() string {
	return executionError.err.Error()
}

// Unwrap exposes the wrapped error.
func (executionError CommandExecutionError) Unwrap() error {
	return executionError.err
}

// StatusCode reports the associated HTTP status code.
func (executionError CommandExecutionError) StatusCode() int {
	return executionError.statusCode
}

// NewCommandExecutionError creates a new CommandExecutionError.
func NewCommandExecutionError(statusCode int, err error) error {
	if err == nil {
		return nil
	}
	return CommandExecutionError{statusCode: statusCode, err: err}
}

// Config defines runtime options for the MCP server.
type Config struct {
	Address         string
	Capabilities    []Capability
	Executors       map[string]CommandExecutor
	RootDirectory   string
	ShutdownTimeout time.Duration
}

// Server serves capability metadata and executes commands over HTTP.
type Server struct {
	config Config
}

// NewServer creates a new Server with defaults applied.
func NewServer(config Config) Server {
	normalized := config
	if normalized.Address == "" {
		normalized.Address = defaultListenAddress
	}
	if normalized.ShutdownTimeout <= 0 {
		normalized.ShutdownTimeout = defaultShutdownDuration
	}
	if normalized.Capabilities == nil {
		normalized.Capabilities = []Capability{}
	}
	if normalized.Executors == nil {
		normalized.Executors = map[string]CommandExecutor{}
	}
	if normalized.RootDirectory == "" {
		if workingDirectory, workingDirectoryErr := os.Getwd(); workingDirectoryErr == nil {
			normalized.RootDirectory = workingDirectory
		}
	}
	return Server{config: normalized}
}

// Run starts the MCP server and blocks until the provided context is canceled.
// The notify callback receives the bound address once the listener is active.
func (server Server) Run(ctx context.Context, notify func(string)) error {
	listener, listenErr := net.Listen("tcp", server.config.Address)
	if listenErr != nil {
		return fmt.Errorf("listen on %s: %w", server.config.Address, listenErr)
	}
	actualAddress := listener.Addr().String()

	router := http.NewServeMux()
	router.HandleFunc(capabilitiesPath, server.handleCapabilities)
	router.HandleFunc(rootPath, server.handleRoot)
	router.HandleFunc(commandsPrefix, server.handleCommand)
	router.HandleFunc(environmentPath, server.handleEnvironment)

	httpServer := &http.Server{Handler: router, ReadHeaderTimeout: defaultReadHeaderTimeout}
	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		serveErr := httpServer.Serve(listener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve MCP: %w", serveErr)
		}
		return nil
	})

	if notify != nil {
		notify(actualAddress)
	}

	group.Go(func() error {
		<-groupCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), server.config.ShutdownTimeout)
		defer cancel()
		shutdownErr := httpServer.Shutdown(shutdownCtx)
		if shutdownErr != nil && !errors.Is(shutdownErr, context.Canceled) && !errors.Is(shutdownErr, http.ErrServerClosed) {
			return fmt.Errorf("shutdown MCP: %w", shutdownErr)
		}
		return nil
	})

	return group.Wait()
}

func (server Server) handleCapabilities(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	payload := struct {
		Capabilities  []Capability `json:"capabilities"`
		RootDirectory string       `json:"rootDirectory"`
	}{
		Capabilities:  server.config.Capabilities,
		RootDirectory: server.config.RootDirectory,
	}
	server.writeJSON(writer, http.StatusOK, payload)
}

func (server Server) handleRoot(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (server Server) handleEnvironment(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	payload := struct {
		RootDirectory string `json:"rootDirectory"`
	}{
		RootDirectory: server.config.RootDirectory,
	}
	server.writeJSON(writer, http.StatusOK, payload)
}

func (server Server) handleCommand(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	commandName := strings.TrimPrefix(request.URL.Path, commandsPrefix)
	if commandName == "" || strings.Contains(commandName, "/") {
		server.writeJSON(writer, http.StatusNotFound, map[string]string{errorFieldName: errorCommandNotFound})
		return
	}
	executor, found := server.config.Executors[commandName]
	if !found {
		server.writeJSON(writer, http.StatusNotFound, map[string]string{errorFieldName: errorCommandNotFound})
		return
	}
	body, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		server.writeJSON(writer, http.StatusBadRequest, map[string]string{errorFieldName: fmt.Sprintf("read request body: %v", readErr)})
		return
	}
	commandRequest := CommandRequest{Payload: json.RawMessage(body)}
	commandResponse, executeErr := executor.Execute(request.Context(), commandRequest)
	if executeErr != nil {
		statusCode := server.statusCodeFromError(executeErr)
		server.writeJSON(writer, statusCode, map[string]string{errorFieldName: executeErr.Error()})
		return
	}
	server.writeJSON(writer, http.StatusOK, commandResponse)
}

func (server Server) writeJSON(writer http.ResponseWriter, statusCode int, payload interface{}) {
	var buffer bytes.Buffer
	if encodeErr := json.NewEncoder(&buffer).Encode(payload); encodeErr != nil {
		fallback := map[string]string{errorFieldName: fmt.Sprintf("encode response: %v", encodeErr)}
		writer.Header().Set(headerContentType, mimeTypeJSON)
		writer.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(writer).Encode(fallback)
		return
	}
	writer.Header().Set(headerContentType, mimeTypeJSON)
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(buffer.Bytes())
}

func (server Server) statusCodeFromError(err error) int {
	var executionError CommandExecutionError
	if errors.As(err, &executionError) {
		return executionError.StatusCode()
	}
	return http.StatusInternalServerError
}
