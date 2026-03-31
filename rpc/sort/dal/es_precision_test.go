package dal

import (
	"strings"
	"testing"
)

func TestParseSearchResponsePreservesLargeGroupID(t *testing.T) {
	body := `{
		"hits": {
			"total": {"value": 1},
			"hits": [
				{
					"_id": "290051107751723008",
					"_score": 12.0714,
					"_source": {
						"id": 290051107751723008,
						"content": "content",
						"summary": "summary",
						"type": "info",
						"domains": ["technology"],
						"keywords": ["AI"],
						"group_id": 290047059094929408,
						"source_type": "original",
						"created_at": "2026-03-11T17:19:55.210869+08:00",
						"updated_at": "2026-03-11T17:19:55.210869+08:00"
					}
				}
			]
		}
	}`

	parsed, err := parseSearchResponse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parseSearchResponse failed: %v", err)
	}
	if got := parsed.Hits.Hits[0].Source.GroupID; got != 290047059094929408 {
		t.Fatalf("expected exact group_id 290047059094929408, got %d", got)
	}
}
