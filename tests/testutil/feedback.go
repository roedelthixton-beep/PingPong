package testutil

import "testing"

func SubmitFeedback(t *testing.T, token string, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	resp := DoPost(t, "/api/v1/items/feedback", body, token)
	if int(resp["code"].(float64)) != 0 {
		t.Fatalf("submit feedback failed: %v", resp["msg"])
	}
	data := resp["data"].(map[string]interface{})
	return data
}
