package mcp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServerClosesIncompleteHeaders(testingHandle *testing.T) {
	for _, partialRequest := range []string{
		"GET /capabilities HTTP/1.1\r\nHost: localhost\r\nX-Partial: ",
		"GET /capabilities HTTP/1.1\r\n",
	} {
		testingHandle.Run(partialRequest, func(testingHandle *testing.T) {
			server := NewServer(Config{Address: "127.0.0.1:0", ShutdownTimeout: time.Second})
			serverContext, cancelServer := context.WithCancel(context.Background())
			addressChannel := make(chan string, 1)
			serverResult := make(chan error, 1)
			go func() { serverResult <- server.Run(serverContext, func(address string) { addressChannel <- address }) }()
			address := waitForAddress(testingHandle, addressChannel)
			testingHandle.Cleanup(func() {
				cancelServer()
				if serverError := <-serverResult; serverError != nil {
					testingHandle.Errorf("stop server: %v", serverError)
				}
			})
			connection, dialError := net.DialTimeout("tcp", address, time.Second)
			if dialError != nil {
				testingHandle.Fatalf("connect: %v", dialError)
			}
			testingHandle.Cleanup(func() {
				if closeError := connection.Close(); closeError != nil {
					testingHandle.Errorf("close connection: %v", closeError)
				}
			})
			if deadlineError := connection.SetDeadline(time.Now().Add(6 * time.Second)); deadlineError != nil {
				testingHandle.Fatalf("set deadline: %v", deadlineError)
			}
			if _, writeError := io.WriteString(connection, partialRequest); writeError != nil {
				testingHandle.Fatalf("write incomplete headers: %v", writeError)
			}
			if strings.Contains(partialRequest, "X-Partial") {
				time.Sleep(2 * time.Second)
				if _, writeError := io.WriteString(connection, "progress"); writeError != nil {
					testingHandle.Fatalf("write partial header progress: %v", writeError)
				}
			}
			var received [1]byte
			_, readError := connection.Read(received[:])
			var networkError net.Error
			if errors.As(readError, &networkError) && networkError.Timeout() {
				testingHandle.Fatal("server kept incomplete headers open past the header deadline")
			}
			if !errors.Is(readError, io.EOF) {
				testingHandle.Fatalf("expected closed connection, got %v", readError)
			}
			client := http.Client{Timeout: time.Second}
			response, requestError := client.Get("http://" + address + "/capabilities")
			if requestError != nil {
				testingHandle.Fatalf("ordinary request: %v", requestError)
			}
			if closeError := response.Body.Close(); closeError != nil {
				testingHandle.Fatalf("close response: %v", closeError)
			}
			if response.StatusCode != http.StatusOK {
				testingHandle.Fatalf("ordinary request status: %d", response.StatusCode)
			}
		})
	}
}

func TestServerAllowsBodyAfterHeaderDeadline(testingHandle *testing.T) {
	server := NewServer(Config{
		Address:         "127.0.0.1:0",
		ShutdownTimeout: time.Second,
		Executors: map[string]CommandExecutor{
			"sample": CommandExecutorFunc(func(_ context.Context, request CommandRequest) (CommandResponse, error) {
				return CommandResponse{Output: string(request.Payload)}, nil
			}),
		},
	})
	serverContext, cancelServer := context.WithCancel(context.Background())
	addressChannel := make(chan string, 1)
	resultChannel := make(chan error, 1)
	go func() { resultChannel <- server.Run(serverContext, func(address string) { addressChannel <- address }) }()
	address := waitForAddress(testingHandle, addressChannel)
	testingHandle.Cleanup(func() {
		cancelServer()
		if serverError := <-resultChannel; serverError != nil {
			testingHandle.Errorf("stop server: %v", serverError)
		}
	})
	connection, dialError := net.DialTimeout("tcp", address, time.Second)
	if dialError != nil {
		testingHandle.Fatalf("connect: %v", dialError)
	}
	testingHandle.Cleanup(func() {
		if closeError := connection.Close(); closeError != nil {
			testingHandle.Errorf("close connection: %v", closeError)
		}
	})
	if deadlineError := connection.SetDeadline(time.Now().Add(8 * time.Second)); deadlineError != nil {
		testingHandle.Fatalf("set deadline: %v", deadlineError)
	}
	body := `{}`
	headers := "POST /commands/sample HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: " + fmt.Sprint(len(body)) + "\r\n\r\n"
	if _, writeError := io.WriteString(connection, headers); writeError != nil {
		testingHandle.Fatalf("write headers: %v", writeError)
	}
	time.Sleep(5200 * time.Millisecond)
	if _, writeError := io.WriteString(connection, body); writeError != nil {
		testingHandle.Fatalf("write body: %v", writeError)
	}
	response, responseError := http.ReadResponse(bufio.NewReader(connection), nil)
	if responseError != nil {
		testingHandle.Fatalf("read response: %v", responseError)
	}
	responseBody, bodyError := io.ReadAll(response.Body)
	if bodyError != nil {
		testingHandle.Fatalf("read body: %v", bodyError)
	}
	if closeError := response.Body.Close(); closeError != nil {
		testingHandle.Fatalf("close response: %v", closeError)
	}
	if response.StatusCode != http.StatusOK || !bytes.Contains(responseBody, []byte(`"output":"{}"`)) {
		testingHandle.Fatalf("ordinary slow body rejected: %d %s", response.StatusCode, responseBody)
	}
}
