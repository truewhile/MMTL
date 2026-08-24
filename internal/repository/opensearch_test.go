package repository

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ShukeBta/MMTL/internal/config"
)

func TestOpenSearchMediaBackendSearchesIDs(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["from"].(float64) != 5 || body["size"].(float64) != 10 {
			t.Fatalf("paging body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hits": map[string]any{
				"total": map[string]any{"value": 2},
				"hits": []any{
					map[string]any{"_id": "m-1", "_source": map[string]any{"id": "m-1"}},
					map[string]any{"_id": "m-2", "_source": map[string]any{"id": "m-2"}},
				},
			},
		})
	}))
	defer upstream.Close()

	backend := NewOpenSearchMediaBackend(config.SearchConfig{
		Backend:       "opensearch",
		OpenSearchURL: upstream.URL,
		Index:         "media-test",
	})
	ids, total, err := backend.SearchMediaIDs(t.Context(), "流浪地球", 5, 10, MediaQueryFilter{
		IncludeNSFW:       false,
		AllowedLibraryIDs: []string{"lib-1"},
		HiddenLibraryIDs:  []string{"adult"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/media-test/_search" {
		t.Fatalf("path = %q", gotPath)
	}
	if total != 2 || len(ids) != 2 || ids[0] != "m-1" || ids[1] != "m-2" {
		t.Fatalf("ids=%#v total=%d", ids, total)
	}
}
