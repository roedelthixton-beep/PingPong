package es

import "testing"

func TestIsAlreadyExistsCreateError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{
			name:       "resource already exists from es",
			statusCode: 400,
			body:       `{"error":{"type":"resource_already_exists_exception","reason":"index [items-000001] already exists"}}`,
			want:       true,
		},
		{
			name:       "generic bad request",
			statusCode: 400,
			body:       `{"error":{"type":"illegal_argument_exception","reason":"bad request"}}`,
			want:       false,
		},
		{
			name:       "conflict already exists",
			statusCode: 409,
			body:       `{"error":{"type":"already_exists_exception","reason":"already exists"}}`,
			want:       true,
		},
		{
			name:       "non error status",
			statusCode: 200,
			body:       `{"acknowledged":true}`,
			want:       false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isAlreadyExistsCreateError(tc.statusCode, []byte(tc.body))
			if got != tc.want {
				t.Fatalf("isAlreadyExistsCreateError() = %v, want %v", got, tc.want)
			}
		})
	}
}
