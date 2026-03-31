package dal

import (
	"encoding/json"
	"testing"
	"time"
)

func getTopLevelBoolQuery(t *testing.T, query map[string]interface{}) map[string]interface{} {
	t.Helper()
	queryObj, ok := query["query"].(map[string]interface{})
	if !ok {
		t.Fatalf("query.query is missing or invalid")
	}
	boolQuery, ok := queryObj["bool"].(map[string]interface{})
	if !ok {
		t.Fatalf("query.query.bool is missing or invalid")
	}
	return boolQuery
}

func getRelevanceShouldClauses(t *testing.T, query map[string]interface{}) []interface{} {
	t.Helper()
	boolQuery := getTopLevelBoolQuery(t, query)
	must, ok := boolQuery["must"].([]interface{})
	if !ok {
		t.Fatalf("query.query.bool.must is missing or invalid")
	}

	var shouldClauses []interface{}
	for _, clause := range must {
		clauseMap, ok := clause.(map[string]interface{})
		if !ok {
			continue
		}
		boolClause, ok := clauseMap["bool"].(map[string]interface{})
		if !ok {
			continue
		}
		should, ok := boolClause["should"].([]interface{})
		if !ok {
			continue
		}
		// The expire_time filter is always the first bool/should clause.
		// Relevance should clauses (domains/keywords/geo) are appended later.
		shouldClauses = should
	}
	return shouldClauses
}

// TestBuildSearchQuery tests query building logic (no ES service required)
func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		req      *SearchItemsRequest
		validate func(t *testing.T, query map[string]interface{})
	}{
		{
			name: "Basic query - limit only",
			req: &SearchItemsRequest{
				Limit: 20,
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				if query["size"] != 20 {
					t.Errorf("Expected size=20, got %v", query["size"])
				}

				// Check sorting
				sort := query["sort"].([]interface{})
				if len(sort) == 0 {
					t.Error("Expected sort field")
				}
			},
		},
		{
			name: "domains keyword matching",
			req: &SearchItemsRequest{
				Domains: []string{"AI", "technology"},
				Limit:   10,
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				should := getRelevanceShouldClauses(t, query)
				if len(should) < 2 {
					t.Errorf("Expected at least 2 should clauses for domains, got %d", len(should))
				}
			},
		},
		{
			name: "keywords keyword matching",
			req: &SearchItemsRequest{
				Keywords: []string{"machine learning", "deep learning"},
				Limit:    10,
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				should := getRelevanceShouldClauses(t, query)
				if len(should) < 2 {
					t.Errorf("Expected at least 2 should clauses for keywords, got %d", len(should))
				}
			},
		},
		{
			name: "geo fuzzy matching",
			req: &SearchItemsRequest{
				Geo:   "Beijing",
				Limit: 10,
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				should := getRelevanceShouldClauses(t, query)
				if len(should) == 0 {
					t.Error("Expected should clause for geo")
				}

				// Check if there is a match query
				found := false
				for _, clause := range should {
					clauseMap := clause.(map[string]interface{})
					if _, ok := clauseMap["match"]; ok {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected match query for geo")
				}
			},
		},
		{
			name: "cursor pagination",
			req: &SearchItemsRequest{
				LastUpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Limit:         10,
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				queryObj := query["query"].(map[string]interface{})
				boolQuery := queryObj["bool"].(map[string]interface{})
				must := boolQuery["must"].([]interface{})

				// Check if there is a range query for updated_at
				foundRange := false
				for _, clause := range must {
					clauseMap := clause.(map[string]interface{})
					if rangeQuery, ok := clauseMap["range"]; ok {
						rangeMap := rangeQuery.(map[string]interface{})
						if _, ok := rangeMap["updated_at"]; ok {
							foundRange = true
							break
						}
					}
				}
				if !foundRange {
					t.Error("Expected range query for updated_at")
				}
			},
		},
		{
			name: "combined query",
			req: &SearchItemsRequest{
				Domains:  []string{"AI", "technology"},
				Keywords: []string{"machine learning"},
				Geo:      "Beijing",
				Limit:    20,
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				boolQuery := getTopLevelBoolQuery(t, query)

				// Check must clause (expire time filter)
				must := boolQuery["must"].([]interface{})
				if len(must) == 0 {
					t.Error("Expected must clause for expire_time filter")
				}

				// Check should clause (domains + keywords + geo)
				should := getRelevanceShouldClauses(t, query)
				// 2 domains + 1 keyword + 1 geo = 4
				expectedMinShould := 4
				if len(should) < expectedMinShould {
					t.Errorf("Expected at least %d should clauses, got %d", expectedMinShould, len(should))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := buildSearchQuery(tt.req)

			// Print query structure (for debugging)
			queryJSON, _ := json.MarshalIndent(query, "", "  ")
			t.Logf("Query:\n%s", string(queryJSON))

			// Validate query
			tt.validate(t, query)
		})
	}
}

// TestExpireTimeFilter tests expire time filtering logic
func TestExpireTimeFilter(t *testing.T) {
	req := &SearchItemsRequest{
		Limit: 10,
	}

	query := buildSearchQuery(req)
	queryJSON, _ := json.MarshalIndent(query, "", "  ")
	t.Logf("Expire time filter query:\n%s", string(queryJSON))

	// Check if must clause contains expire time filter
	queryObj := query["query"].(map[string]interface{})
	boolQuery := queryObj["bool"].(map[string]interface{})
	must := boolQuery["must"].([]interface{})

	if len(must) == 0 {
		t.Fatal("Expected must clause for expire_time filter")
	}

	// Check if first must clause is expire time filter
	firstMust := must[0].(map[string]interface{})
	if _, ok := firstMust["bool"]; !ok {
		t.Error("Expected bool query for expire_time filter")
	}

	boolFilter := firstMust["bool"].(map[string]interface{})
	should := boolFilter["should"].([]interface{})

	// Should have two conditions: expire_time does not exist or expire_time > now
	if len(should) != 2 {
		t.Errorf("Expected 2 should clauses for expire_time filter, got %d", len(should))
	}

	t.Log("✓ Expire time filter is correctly configured")
}

// TestQuerySorting tests sorting logic (relevance scoring priority)
func TestQuerySorting(t *testing.T) {
	req := &SearchItemsRequest{
		Limit: 10,
	}

	query := buildSearchQuery(req)

	// Check sort fields
	sort := query["sort"].([]interface{})
	if len(sort) < 2 {
		t.Fatal("Expected at least 2 sort fields (_score and updated_at)")
	}

	// First sort should be _score
	firstSort := sort[0].(map[string]interface{})
	if _, ok := firstSort["_score"]; !ok {
		t.Error("Expected first sort field to be _score")
	}

	scoreSort := firstSort["_score"].(map[string]interface{})
	if scoreSort["order"] != "desc" {
		t.Errorf("Expected _score order=desc, got %v", scoreSort["order"])
	}

	// Second sort should be updated_at
	secondSort := sort[1].(map[string]interface{})
	updatedAtSort := secondSort["updated_at"].(map[string]interface{})

	if updatedAtSort["order"] != "desc" {
		t.Errorf("Expected updated_at order=desc, got %v", updatedAtSort["order"])
	}

	t.Log("✓ Sorting by _score DESC, then updated_at DESC is correctly configured")
}

// TestCaseInsensitiveMatch tests case-insensitive matching
func TestCaseInsensitiveMatch(t *testing.T) {
	req := &SearchItemsRequest{
		Domains: []string{"AI", "Technology"},
		Limit:   10,
	}

	query := buildSearchQuery(req)
	queryJSON, _ := json.MarshalIndent(query, "", "  ")
	t.Logf("Case-insensitive query:\n%s", string(queryJSON))

	// Check if domains in query are converted to lowercase
	shouldClauses := getRelevanceShouldClauses(t, query)

	if len(shouldClauses) == 0 {
		t.Fatal("Expected should clauses for domains")
	}

	// Verify at least one term query uses lowercase
	foundLowercase := false
	for _, shouldItem := range shouldClauses {
		shouldMap := shouldItem.(map[string]interface{})
		if boolClause, ok := shouldMap["bool"]; ok {
			boolMap := boolClause.(map[string]interface{})
			if innerShould, ok := boolMap["should"]; ok {
				innerShouldList := innerShould.([]interface{})
				for _, innerItem := range innerShouldList {
					innerMap := innerItem.(map[string]interface{})
					if termQuery, ok := innerMap["term"]; ok {
						termMap := termQuery.(map[string]interface{})
						if domainsQuery, ok := termMap["domains"]; ok {
							domainsMap := domainsQuery.(map[string]interface{})
							if value, ok := domainsMap["value"].(string); ok {
								// Check if lowercase
								if value == "ai" || value == "technology" {
									foundLowercase = true
									t.Logf("✓ Found lowercase term query: %s", value)
								}
							}
						}
					}
				}
			}
		}
	}

	if !foundLowercase {
		t.Error("Expected at least one lowercase term query")
	}

	t.Log("✓ Case-insensitive matching is correctly configured")
}

// TestFuzzyMatch tests fuzzy matching
func TestFuzzyMatch(t *testing.T) {
	req := &SearchItemsRequest{
		Keywords: []string{"tech"},
		Limit:    10,
	}

	query := buildSearchQuery(req)
	queryJSON, _ := json.MarshalIndent(query, "", "  ")
	t.Logf("Fuzzy match query:\n%s", string(queryJSON))

	// Check if there is a match query against the .text subfield
	queryObj := query["query"].(map[string]interface{})
	boolQuery := queryObj["bool"].(map[string]interface{})
	must := boolQuery["must"].([]interface{})

	foundMatchQuery := false
	for _, mustItem := range must {
		mustMap := mustItem.(map[string]interface{})
		if boolClause, ok := mustMap["bool"]; ok {
			boolMap := boolClause.(map[string]interface{})
			if should, ok := boolMap["should"]; ok {
				shouldList := should.([]interface{})
				for _, shouldItem := range shouldList {
					shouldMap := shouldItem.(map[string]interface{})
					if boolClause, ok := shouldMap["bool"]; ok {
						boolMap := boolClause.(map[string]interface{})
						if innerShould, ok := boolMap["should"]; ok {
							innerShouldList := innerShould.([]interface{})
							for _, innerItem := range innerShouldList {
								innerMap := innerItem.(map[string]interface{})
								if matchQuery, ok := innerMap["match"]; ok {
									matchMap := matchQuery.(map[string]interface{})
									// Check if there is keywords.text field
									if _, ok := matchMap["keywords.text"]; ok {
										foundMatchQuery = true
										t.Log("✓ Found match query on keywords.text field")
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if !foundMatchQuery {
		t.Error("Expected match query on keywords.text field for fuzzy matching")
	}

	t.Log("✓ Fuzzy matching is correctly configured")
}

// TestRelevanceScoring tests relevance scoring weights
func TestRelevanceScoring(t *testing.T) {
	req := &SearchItemsRequest{
		Domains: []string{"ai"},
		Limit:   10,
	}

	query := buildSearchQuery(req)
	queryJSON, _ := json.MarshalIndent(query, "", "  ")
	t.Logf("Relevance scoring query:\n%s", string(queryJSON))

	// Check boost weights
	queryObj := query["query"].(map[string]interface{})
	boolQuery := queryObj["bool"].(map[string]interface{})
	must := boolQuery["must"].([]interface{})

	foundTermBoost := false
	foundMatchBoost := false

	for _, mustItem := range must {
		mustMap := mustItem.(map[string]interface{})
		if boolClause, ok := mustMap["bool"]; ok {
			boolMap := boolClause.(map[string]interface{})
			if should, ok := boolMap["should"]; ok {
				shouldList := should.([]interface{})
				for _, shouldItem := range shouldList {
					shouldMap := shouldItem.(map[string]interface{})
					if boolClause, ok := shouldMap["bool"]; ok {
						boolMap := boolClause.(map[string]interface{})
						if innerShould, ok := boolMap["should"]; ok {
							innerShouldList := innerShould.([]interface{})
							for _, innerItem := range innerShouldList {
								innerMap := innerItem.(map[string]interface{})

								// Check boost for term query
								if termQuery, ok := innerMap["term"]; ok {
									termMap := termQuery.(map[string]interface{})
									if domainsQuery, ok := termMap["domains"]; ok {
										domainsMap := domainsQuery.(map[string]interface{})
										if boost, ok := domainsMap["boost"].(float64); ok {
											if boost == 3.0 {
												foundTermBoost = true
												t.Logf("✓ Found term query with boost=3.0")
											}
										}
									}
								}

								// Check boost for match query
								if matchQuery, ok := innerMap["match"]; ok {
									matchMap := matchQuery.(map[string]interface{})
									if domainsTextQuery, ok := matchMap["domains.text"]; ok {
										domainsTextMap := domainsTextQuery.(map[string]interface{})
										if boost, ok := domainsTextMap["boost"].(float64); ok {
											if boost == 2.0 {
												foundMatchBoost = true
												t.Logf("✓ Found match query with boost=2.0")
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if !foundTermBoost {
		t.Error("Expected term query with boost=3.0 for exact matching")
	}

	if !foundMatchBoost {
		t.Error("Expected match query with boost=2.0 for fuzzy matching")
	}

	t.Log("✓ Relevance scoring with correct boost weights is configured")
}
