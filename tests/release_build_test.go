//go:build releasebuild

package tests

import (
	"debug/buildinfo"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	appTypes "github.com/tyemirov/ctx/internal/types"
	"gopkg.in/yaml.v3"
)

const (
	releaseManifestPath = ".mprlab/deploy/resources.yml"
	releaseBinaryKind   = "github_release_binary"
	releaseCGOSetting   = "CGO_ENABLED"
	releaseGOOSSetting  = "GOOS"
	releaseArchSetting  = "GOARCH"
	releaseWindowsOS    = "windows"
)

type releaseBuildManifest struct {
	Resources struct {
		Items map[string]struct {
			Kind  string `yaml:"kind"`
			Build struct {
				Package     string            `yaml:"package"`
				Binary      string            `yaml:"binary"`
				Platforms   []string          `yaml:"platforms"`
				Environment map[string]string `yaml:"environment"`
			} `yaml:"build"`
		} `yaml:"resources"`
	} `yaml:"mprlab_resources"`
}

func TestReleaseBinaries(testingHandle *testing.T) {
	repositoryRoot := getModuleRoot(testingHandle)
	manifestBytes, readError := os.ReadFile(filepath.Join(repositoryRoot, releaseManifestPath))
	if readError != nil {
		testingHandle.Fatal(readError)
	}
	var manifest releaseBuildManifest
	if parseError := yaml.Unmarshal(manifestBytes, &manifest); parseError != nil {
		testingHandle.Fatal(parseError)
	}
	expectedPlatforms := map[string]bool{
		"linux/amd64":   false,
		"darwin/amd64":  false,
		"darwin/arm64":  false,
		"windows/amd64": false,
	}
	for _, resource := range manifest.Resources.Items {
		if resource.Kind != releaseBinaryKind {
			continue
		}
		for _, platform := range resource.Build.Platforms {
			seen, supported := expectedPlatforms[platform]
			if !supported || seen {
				testingHandle.Fatalf("unexpected or repeated release platform %q", platform)
			}
			expectedPlatforms[platform] = true
			testingHandle.Run(strings.ReplaceAll(platform, "/", "-"), func(testingHandle *testing.T) {
				platformParts := strings.Split(platform, "/")
				binaryName := resource.Build.Binary
				if platformParts[0] == releaseWindowsOS {
					binaryName += ".exe"
				}
				binaryPath := filepath.Join(testingHandle.TempDir(), binaryName)
				buildCommand := exec.CommandContext(testingHandle.Context(), "go", "build", "-buildvcs=false", "-trimpath", "-o", binaryPath, resource.Build.Package)
				buildCommand.Dir = repositoryRoot
				buildCommand.Env = releaseBuildEnvironment(resource.Build.Environment, platformParts)
				buildOutput, buildError := buildCommand.CombinedOutput()
				if buildError != nil {
					testingHandle.Fatalf("release build %s failed: %v\n%s", platform, buildError, buildOutput)
				}
				verifyReleaseArtifact(testingHandle, binaryPath, platformParts)
				if platform == runtime.GOOS+"/"+runtime.GOARCH {
					verifyReleaseParsers(testingHandle, binaryPath)
				}
			})
		}
	}
	for platform, seen := range expectedPlatforms {
		if !seen {
			testingHandle.Errorf("release platform %s is absent", platform)
		}
	}
}

func releaseBuildEnvironment(declared map[string]string, platformParts []string) []string {
	selected := map[string]string{releaseCGOSetting: "0", releaseGOOSSetting: platformParts[0], releaseArchSetting: platformParts[1]}
	for name, value := range declared {
		selected[name] = value
	}
	environment := make([]string, 0, len(os.Environ())+len(selected))
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, replaced := selected[name]; !replaced {
			environment = append(environment, entry)
		}
	}
	for name, value := range selected {
		environment = append(environment, name+"="+value)
	}
	return environment
}

func verifyReleaseArtifact(testingHandle *testing.T, binaryPath string, platformParts []string) {
	testingHandle.Helper()
	buildInfo, readError := buildinfo.ReadFile(binaryPath)
	if readError != nil {
		testingHandle.Fatal(readError)
	}
	settings := make(map[string]string, len(buildInfo.Settings))
	for _, setting := range buildInfo.Settings {
		settings[setting.Key] = setting.Value
	}
	for name, expected := range map[string]string{releaseCGOSetting: "1", releaseGOOSSetting: platformParts[0], releaseArchSetting: platformParts[1]} {
		if settings[name] != expected {
			testingHandle.Errorf("artifact %s=%q, expected %q", name, settings[name], expected)
		}
	}
	switch platformParts[0] {
	case "linux":
		artifact, openError := elf.Open(binaryPath)
		if openError != nil {
			testingHandle.Fatal(openError)
		}
		defer artifact.Close()
		if artifact.Machine != elf.EM_X86_64 {
			testingHandle.Fatalf("unexpected ELF machine %v", artifact.Machine)
		}
		if artifact.Section(".interp") != nil {
			testingHandle.Fatal("Linux release artifact requires a dynamic loader")
		}
	case "darwin":
		artifact, openError := macho.Open(binaryPath)
		if openError != nil {
			testingHandle.Fatal(openError)
		}
		defer artifact.Close()
		expectedCPU := map[string]macho.Cpu{"amd64": macho.CpuAmd64, "arm64": macho.CpuArm64}[platformParts[1]]
		if artifact.Cpu != expectedCPU {
			testingHandle.Fatalf("Mach-O CPU=%v, expected %v", artifact.Cpu, expectedCPU)
		}
	case releaseWindowsOS:
		artifact, openError := pe.Open(binaryPath)
		if openError != nil {
			testingHandle.Fatal(openError)
		}
		defer artifact.Close()
		if artifact.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			testingHandle.Fatalf("unexpected PE machine %v", artifact.Machine)
		}
	}
}

func verifyReleaseParsers(testingHandle *testing.T, binaryPath string) {
	testingHandle.Helper()
	for _, scenario := range []struct {
		name, target, caller string
		files                map[string]string
	}{
		{
			name: "python", target: pythonCallchainTargetFunction, caller: "consumer.run",
			files: map[string]string{
				filepath.Join(pythonCallchainModuleDirectoryName, pythonCallchainServiceFileName): pythonCallchainServiceContent,
				pythonCallchainConsumerFileName: pythonCallchainConsumerContent,
			},
		},
		{
			name: "javascript", target: javaScriptCallchainTargetFunction, caller: "consumer.run",
			files: map[string]string{
				filepath.Join(javaScriptCallchainModuleDirectoryName, javaScriptCallchainServiceFileName): javaScriptCallchainServiceContent,
				javaScriptCallchainConsumerFileName: javaScriptCallchainConsumerContent,
			},
		},
	} {
		testingHandle.Run(scenario.name, func(testingHandle *testing.T) {
			workingDirectory := setupTestDirectory(testingHandle, scenario.files)
			result := runCommand(testingHandle, binaryPath, []string{appTypes.CommandCallChain, scenario.target, depthFlag, depthThreeValue, formatFlag, appTypes.FormatJSON}, workingDirectory)
			var chains []appTypes.CallChainOutput
			if parseError := json.Unmarshal([]byte(result), &chains); parseError != nil {
				testingHandle.Fatal(parseError)
			}
			if len(chains) != 1 || chains[0].TargetFunction != scenario.target {
				testingHandle.Fatalf("unexpected call chain %s", result)
			}
			for _, caller := range chains[0].Callers {
				if caller == scenario.caller {
					return
				}
			}
			testingHandle.Fatalf("caller %s is absent: %s", scenario.caller, result)
		})
	}
}
