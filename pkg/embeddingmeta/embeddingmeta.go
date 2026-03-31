package embeddingmeta

import (
	"fmt"
	"strings"
)

const (
	ProviderOpenAI = "openai"
	ProviderOllama = "ollama"

	DefaultOpenAIModel = "text-embedding-3-small"
	DefaultOllamaModel = "nomic-embed-text"
)

// NormalizeProvider keeps provider handling consistent across config, clients, and ES setup.
func NormalizeProvider(provider string) string {
	if strings.EqualFold(strings.TrimSpace(provider), ProviderOllama) {
		return ProviderOllama
	}
	return ProviderOpenAI
}

func ResolveModel(provider, model string) string {
	model = strings.TrimSpace(model)
	if model != "" {
		return model
	}

	if NormalizeProvider(provider) == ProviderOllama {
		return DefaultOllamaModel
	}

	return DefaultOpenAIModel
}

func ResolveDimensions(provider, model string, override int) (int, error) {
	if override > 0 {
		return override, nil
	}
	return InferDimensions(provider, model)
}

func InferDimensions(provider, model string) (int, error) {
	provider = NormalizeProvider(provider)
	model = strings.ToLower(strings.TrimSpace(ResolveModel(provider, model)))

	switch model {
	case "text-embedding-3-small", "text-embedding-ada-002":
		return 1536, nil
	case "text-embedding-3-large":
		return 3072, nil
	case "text-embedding-v4":
		return 1024, nil
	case "nomic-embed-text":
		return 768, nil
	case "mxbai-embed-large", "bge-m3":
		return 1024, nil
	default:
		return 0, fmt.Errorf(
			"unknown embedding dimensions for provider=%s model=%s, set EMBEDDING_DIMENSIONS explicitly",
			provider,
			ResolveModel(provider, model),
		)
	}
}
