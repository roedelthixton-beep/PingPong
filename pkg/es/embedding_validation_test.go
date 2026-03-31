package es

import (
	"strings"
	"testing"
)

func TestParseEmbeddingDimensions(t *testing.T) {
	t.Parallel()

	raw := `{
		"items-000001": {
			"mappings": {
				"properties": {
					"embedding": {"type": "dense_vector", "dims": 768}
				}
			}
		},
		"items-000002": {
			"mappings": {
				"properties": {
					"embedding": {"type": "dense_vector", "dims": 1024}
				}
			}
		}
	}`

	got, err := parseEmbeddingDimensions(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parseEmbeddingDimensions() error = %v", err)
	}
	if got["items-000001"] != 768 {
		t.Fatalf("items-000001 dims = %d, want 768", got["items-000001"])
	}
	if got["items-000002"] != 1024 {
		t.Fatalf("items-000002 dims = %d, want 1024", got["items-000002"])
	}
}

func TestBuildIndexMappingUsesProvidedDims(t *testing.T) {
	t.Parallel()

	mapping := BuildIndexMapping(1024)
	properties := mapping["properties"].(map[string]interface{})
	embedding := properties["embedding"].(map[string]interface{})
	if got := embedding["dims"].(int); got != 1024 {
		t.Fatalf("embedding dims = %d, want 1024", got)
	}
}
