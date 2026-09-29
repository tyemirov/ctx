package tokenizer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

// Counter estimates token counts for text content.
type Counter interface {
	Name() string
	CountString(input string) (int, error)
}

type runeCountingCounter struct{}

func (runeCountingCounter) Name() string {
	return "rune-counter"
}

func (runeCountingCounter) CountString(input string) (int, error) {
	return len([]rune(input)), nil
}

// Config captures tokenizer selection parameters provided by the CLI.
type Config struct {
	Model            string
	WorkingDirectory string
	Timeout          time.Duration
}

const (
	defaultModel        = "gpt-4o"
	defaultEncodingName = "cl100k_base"
	defaultUVTimeout    = 120 * time.Second
)

var ErrHelperUnavailable = errors.New("tokenizer helper unavailable")

// NewCounter returns a Counter implementation for the requested model.
func NewCounter(cfg Config) (Counter, string, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}

	if resolved, ok := resolveModelAlias(model); ok {
		model = resolved
	}

	lowerModel := strings.ToLower(model)

	if isOpenAIModel(lowerModel) {
		encoding, err := tiktoken.EncodingForModel(lowerModel)
		if err == nil && encoding != nil {
			return openAICounter{encoding: encoding, name: lowerModel}, model, nil
		}
		fallbackEncoding, fallbackErr := tiktoken.GetEncoding(defaultEncodingName)
		if fallbackErr != nil {
			return runeCountingCounter{}, model, nil
		}
		return openAICounter{encoding: fallbackEncoding, name: defaultEncodingName}, model, nil
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultUVTimeout
	}

	switch {
	case strings.HasPrefix(lowerModel, "claude-"):
		if strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) == "" {
			return nil, "", errors.New("counting tokens for Claude models requires ANTHROPIC_API_KEY to be set; export your Anthropic API key before running ctx")
		}
		uvExecutable, detectErr := detectUVExecutable()
		if detectErr != nil {
			return nil, "", fmt.Errorf("%w: %w", ErrHelperUnavailable, detectErr)
		}
		directory, err := materializeHelperScripts("")
		if err != nil {
			return nil, "", err
		}
		scriptPath := filepath.Join(directory, anthropicScriptName)
		return scriptCounter{
			runner:     uvExecutable,
			scriptPath: scriptPath,
			args:       []string{"--model", model},
			helperName: "anthropic_tokenizer",
			timeout:    timeout,
		}, model, nil
	case strings.HasPrefix(lowerModel, "llama-"):
		uvExecutable, detectErr := detectUVExecutable()
		if detectErr != nil {
			return nil, "", fmt.Errorf("%w: %w", ErrHelperUnavailable, detectErr)
		}
		directory, err := materializeHelperScripts("")
		if err != nil {
			return nil, "", err
		}
		scriptPath := filepath.Join(directory, llamaScriptName)
		return scriptCounter{
			runner:     uvExecutable,
			scriptPath: scriptPath,
			args:       []string{"--model", model},
			helperName: "sentencepiece",
			timeout:    timeout,
		}, model, nil
	default:
		encoding, err := tiktoken.GetEncoding(defaultEncodingName)
		if err != nil {
			return runeCountingCounter{}, model, nil
		}
		return openAICounter{encoding: encoding, name: defaultEncodingName}, model, nil
	}
}

type aliasEntry struct {
	canonical string
	aliases   []string
}

var modelAliasLookup = buildModelAliasLookup()

func resolveModelAlias(model string) (string, bool) {
	normalized := normalizeAliasKey(model)
	if normalized == "" {
		return "", false
	}

	if canonical, ok := modelAliasLookup[normalized]; ok {
		return canonical, true
	}
	return "", false
}

func buildModelAliasLookup() map[string]string {
	entries := []aliasEntry{
		{
			canonical: "claude-3-5-haiku-20241022",
			aliases: []string{
				"claude-3.5-haiku",
				"claude-3-5-haiku",
				"claude-3.5",
				"claude-3-5",
			},
		},
		{
			canonical: "claude-3-5-sonnet-20240620",
			aliases: []string{
				"claude-3.5-sonnet-20240620",
			},
		},
		{
			canonical: "claude-3-5-sonnet-20241022",
			aliases: []string{
				"claude-3.5-sonnet",
				"claude-3-5-sonnet",
				"claude-3.5",
				"claude-3-5",
			},
		},
		{
			canonical: "claude-3-7-sonnet-20250219",
			aliases: []string{
				"claude-3.7",
				"claude-3-7",
				"claude-3.7-sonnet",
				"claude-3-7-sonnet",
			},
		},
		{
			canonical: "claude-3-haiku-20240307",
			aliases: []string{
				"claude-3-haiku",
			},
		},
		{
			canonical: "claude-3-opus-20240229",
			aliases: []string{
				"claude-3-opus",
				"claude-opus-3",
			},
		},
		{
			canonical: "claude-opus-4-1-20250805",
			aliases: []string{
				"claude-4.1",
				"claude-4-1",
				"claude-opus-4.1",
				"claude-opus-4-1",
			},
		},
		{
			canonical: "claude-opus-4-20250514",
			aliases: []string{
				"claude-4-opus",
				"claude-4",
				"claude-opus-4",
			},
		},
		{
			canonical: "claude-sonnet-4-20250514",
			aliases: []string{
				"claude-sonnet-4",
				"claude-4-sonnet",
				"claude-4",
			},
		},
		{
			canonical: "claude-sonnet-4-5-20250929",
			aliases: []string{
				"claude-4.5",
				"claude-4-5",
				"claude-sonnet-4.5",
				"claude-sonnet-4-5",
				"claude-4.5-sonnet",
			},
		},
	}

	lookup := make(map[string]string, len(entries)*4)
	for _, entry := range entries {
		normalizedCanonical := normalizeAliasKey(entry.canonical)
		if normalizedCanonical != "" {
			lookup[normalizedCanonical] = entry.canonical
		}
		base := stripClaudeDate(entry.canonical)
		if base != "" {
			lookup[normalizeAliasKey(base)] = entry.canonical
		}
		for _, alias := range entry.aliases {
			if normalized := normalizeAliasKey(alias); normalized != "" {
				lookup[normalized] = entry.canonical
			}
		}
	}

	return lookup
}

func normalizeAliasKey(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "-", "_", "-", ".", "-")
	normalized := replacer.Replace(trimmed)
	normalized = collapseHyphens(normalized)
	return normalized
}

func collapseHyphens(input string) string {
	for strings.Contains(input, "--") {
		input = strings.ReplaceAll(input, "--", "-")
	}
	return input
}

func stripClaudeDate(model string) string {
	parts := strings.Split(model, "-")
	if len(parts) == 0 {
		return model
	}
	last := parts[len(parts)-1]
	if len(last) == 8 && isDigits(last) {
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return model
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isOpenAIModel(model string) bool {
	prefixes := []string{
		"gpt-",
		"text-embedding",
		"davinci",
		"curie",
		"babbage",
		"ada",
		"code-",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}

func detectUVExecutable() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("CTX_UV")); explicit != "" {
		if path, err := exec.LookPath(explicit); err == nil {
			return path, nil
		}
		if _, err := os.Stat(explicit); err == nil {
			return explicit, nil
		}
		return "", fmt.Errorf("uv executable specified via CTX_UV (%s) not found", explicit)
	}
	if path, err := exec.LookPath("uv"); err == nil {
		return path, nil
	}
	return "", errors.New("uv executable not found; install uv from https://github.com/astral-sh/uv or expose it via CTX_UV")
}
