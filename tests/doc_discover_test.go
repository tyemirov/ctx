package tests

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocDiscoverGeneratesDependencyDocs(t *testing.T) {
	if testing.Short() {
		t.Skip("skip doc discover integration in short mode")
	}

	binary := buildBinary(t)
	workingDirectory := t.TempDir()
	writeFile(t, filepath.Join(workingDirectory, "go.mod"), `module example.com/project

go 1.22

require github.com/test/go-lib v1.0.0
`)
	writeFile(t, filepath.Join(workingDirectory, "package.json"), `{
  "name": "web-app",
  "version": "0.0.1",
  "dependencies": {
    "bootstrap": "^5.3.0"
  }
}
`)
	writeFile(t, filepath.Join(workingDirectory, "requirements.txt"), "requests==2.31.0\n")

	githubServer := startDocDiscoverGitHubServer(t, map[string]docRepoFixture{
		"test/go-lib": {
			Reference: "v1.0.0",
			Files: map[string]string{
				"docs/index.md": "# Go Lib\n\nAPI overview.",
			},
		},
		"twbs/bootstrap": {
			Reference: "main",
			Files: map[string]string{
				"docs/bootstrap.md": "# Bootstrap\n\nComponents overview.",
			},
		},
		"psf/requests": {
			Reference: "main",
			Files: map[string]string{
				"docs/requests.md": "# Requests\n\nHTTP client overview.",
			},
		},
	})
	defer githubServer.Close()

	npmServer := startDocDiscoverNPMRegistry(t, map[string]docRegistryPackage{
		"bootstrap": {
			RepositoryURL: "https://github.com/twbs/bootstrap",
		},
	})
	defer npmServer.Close()

	pypiServer := startDocDiscoverPyPIRegistry(t, map[string]string{
		"requests": "https://github.com/psf/requests",
	})
	defer pypiServer.Close()

	args := []string{
		docCommandName,
		"discover",
		"--api-base", githubServer.URL,
		"--npm-registry-base", npmServer.URL,
		"--pypi-registry-base", pypiServer.URL,
		"--output-dir", "docs-out",
		"--format", "json",
	}
	output := runCommand(t, binary, args, workingDirectory)

	var manifest struct {
		Entries []struct {
			Name       string `json:"name"`
			Ecosystem  string `json:"ecosystem"`
			Status     string `json:"status"`
			OutputPath string `json:"outputPath"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(output), &manifest); err != nil {
		t.Fatalf("parse manifest: %v\n%s", err, output)
	}
	if len(manifest.Entries) != 3 {
		t.Fatalf("expected 3 manifest entries, got %d", len(manifest.Entries))
	}
	for _, entry := range manifest.Entries {
		if entry.Status != "written" {
			t.Fatalf("expected entry %s to be written, got %s", entry.Name, entry.Status)
		}
		if entry.OutputPath == "" {
			t.Fatalf("expected output path for %s", entry.Name)
		}
		fullPath := filepath.Join(workingDirectory, entry.OutputPath)
		data, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			t.Fatalf("read generated doc for %s: %v", entry.Name, readErr)
		}
		if !strings.Contains(string(data), "# "+entry.Name) {
			t.Fatalf("expected file to contain heading for %s\n%s", entry.Name, string(data))
		}
	}
}

type docRepoFixture struct {
	Reference string
	Files     map[string]string
}

func startDocDiscoverGitHubServer(t *testing.T, repositories map[string]docRepoFixture) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		path := strings.Trim(request.URL.Path, "/")
		if !strings.HasPrefix(path, "repos/") {
			http.NotFound(writer, request)
			return
		}
		segments := strings.Split(path, "/")
		if len(segments) < 4 {
			http.NotFound(writer, request)
			return
		}
		owner := segments[1]
		repo := segments[2]
		itemPath := strings.Join(segments[4:], "/")
		key := fmt.Sprintf("%s/%s", owner, repo)
		fixture, ok := repositories[key]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		if fixture.Reference != "" && request.URL.Query().Get("ref") != fixture.Reference {
			http.NotFound(writer, request)
			return
		}
		if itemPath == "" {
			itemPath = "."
		}
		writer.Header().Set("Content-Type", "application/json")
		if itemPath == "." || hasDirectory(fixture.Files, itemPath) {
			directory := strings.Trim(itemPath, "/")
			var entries []map[string]interface{}
			for filePath := range fixture.Files {
				if directory != "" && !strings.HasPrefix(filePath, directory+"/") {
					continue
				}
				relative := filePath
				if directory != "" {
					relative = strings.TrimPrefix(filePath, directory+"/")
					if strings.Contains(relative, "/") {
						continue
					}
					relative = directory + "/" + relative
				} else {
					if strings.Contains(filePath, "/") {
						continue
					}
				}
				entries = append(entries, map[string]interface{}{
					"name": filepath.Base(relative),
					"path": relative,
					"type": "file",
				})
			}
			if err := json.NewEncoder(writer).Encode(entries); err != nil {
				t.Fatalf("encode directory listing: %v", err)
			}
			return
		}
		content, ok := fixture.Files[itemPath]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		payload := map[string]interface{}{
			"name":     filepath.Base(itemPath),
			"path":     itemPath,
			"type":     "file",
			"encoding": "base64",
			"content":  base64.StdEncoding.EncodeToString([]byte(content)),
		}
		if err := json.NewEncoder(writer).Encode(payload); err != nil {
			t.Fatalf("encode file payload: %v", err)
		}
	})
	return httptest.NewServer(handler)
}

func hasDirectory(files map[string]string, directory string) bool {
	clean := strings.Trim(directory, "/")
	if clean == "" || clean == "." {
		return true
	}
	for path := range files {
		if strings.HasPrefix(path, clean+"/") {
			return true
		}
	}
	return false
}

type docRegistryPackage struct {
	RepositoryURL string
}

func startDocDiscoverNPMRegistry(t *testing.T, packages map[string]docRegistryPackage) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		name, err := url.PathUnescape(strings.TrimPrefix(request.URL.Path, "/"))
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		fixture, ok := packages[name]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		payload := map[string]interface{}{
			"repository": map[string]string{
				"type": "git",
				"url":  fixture.RepositoryURL,
			},
			"dist-tags": map[string]string{
				"latest": "1.0.0",
			},
			"versions": map[string]map[string]interface{}{
				"1.0.0": {
					"repository": map[string]string{
						"type": "git",
						"url":  fixture.RepositoryURL,
					},
				},
			},
		}
		if err := json.NewEncoder(writer).Encode(payload); err != nil {
			t.Fatalf("encode npm payload: %v", err)
		}
	})
	return httptest.NewServer(handler)
}

func startDocDiscoverPyPIRegistry(t *testing.T, packages map[string]string) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := strings.TrimPrefix(request.URL.Path, "/")
		path = strings.TrimSuffix(path, "/json")
		projectURL, ok := packages[path]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		payload := map[string]interface{}{
			"info": map[string]interface{}{
				"project_urls": map[string]string{
					"Source": projectURL,
				},
			},
		}
		if err := json.NewEncoder(writer).Encode(payload); err != nil {
			t.Fatalf("encode pypi payload: %v", err)
		}
	})
	return httptest.NewServer(handler)
}
