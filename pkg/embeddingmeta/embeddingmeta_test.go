package embeddingmeta

import "testing"

func TestResolveModel(t *testing.T) {
	t.Parallel()

	if got := ResolveModel("openai", ""); got != DefaultOpenAIModel {
		t.Fatalf("ResolveModel(openai) = %q, want %q", got, DefaultOpenAIModel)
	}
	if got := ResolveModel("ollama", ""); got != DefaultOllamaModel {
		t.Fatalf("ResolveModel(ollama) = %q, want %q", got, DefaultOllamaModel)
	}
}

func TestInferDimensions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider string
		model    string
		want     int
		wantErr  bool
	}{
		{name: "openai default", provider: "openai", model: "text-embedding-3-small", want: 1536},
		{name: "aliyun text-embedding-v4", provider: "openai", model: "text-embedding-v4", want: 1024},
		{name: "ollama nomic", provider: "ollama", model: "nomic-embed-text", want: 768},
		{name: "ollama mxbai", provider: "ollama", model: "mxbai-embed-large", want: 1024},
		{name: "ollama bge", provider: "ollama", model: "bge-m3", want: 1024},
		{name: "unknown model", provider: "ollama", model: "custom-embed", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := InferDimensions(tc.provider, tc.model)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("InferDimensions(%q, %q) expected error", tc.provider, tc.model)
				}
				return
			}
			if err != nil {
				t.Fatalf("InferDimensions(%q, %q) unexpected error: %v", tc.provider, tc.model, err)
			}
			if got != tc.want {
				t.Fatalf("InferDimensions(%q, %q) = %d, want %d", tc.provider, tc.model, got, tc.want)
			}
		})
	}
}
