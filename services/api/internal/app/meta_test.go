package app

import (
	"net/http"
	"testing"
)

// The meta endpoint is the UI's single source of vocabulary; it must list every
// category the API accepts and nothing more.
func TestMetaListsCategoriesAndSorts(t *testing.T) {
	_, httpServer := newTestServer(t)

	status, body := requestJSON(t, &http.Client{}, http.MethodGet, httpServer.URL+"/api/meta", nil)
	if status != http.StatusOK {
		t.Fatalf("meta: %d", status)
	}
	raw, ok := body["categories"].([]any)
	if !ok || len(raw) != len(postCategoryList) {
		t.Fatalf("categories: %v", body["categories"])
	}
	for i, item := range raw {
		if item != postCategoryList[i] {
			t.Fatalf("category order: got %v want %v", raw, postCategoryList)
		}
		if !postCategories[item.(string)] {
			t.Fatalf("meta exposes a category the API rejects: %v", item)
		}
	}
	sorts, ok := body["sorts"].([]any)
	if !ok || len(sorts) != len(postSorts) {
		t.Fatalf("sorts: %v", body["sorts"])
	}
}
