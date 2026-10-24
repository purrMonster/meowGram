// Package r21 holds executable, implementation-independent R2.1 HTTP fixtures.
package r21

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

type fixture struct {
	ID             string            `json:"id"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Body           string            `json:"body"`
	Actor          string            `json:"actor"`
	Status         int               `json:"status"`
	Headers        map[string]string `json:"headers"`
	Expect         expectation       `json:"expect"`
	Implementation string            `json:"implementation"`
	IdempotencyKey string            `json:"idempotency_key"`
	Setup          []requestStep     `json:"setup"`
}

type requestStep struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	Body           string `json:"body"`
	Actor          string `json:"actor"`
	Status         int    `json:"status"`
	IdempotencyKey string `json:"idempotency_key"`
}
type expectation struct {
	JSON              map[string]any `json:"json"`
	Fields            map[string]any `json:"fields"`
	RequiredFields    []string       `json:"required_fields"`
	AllowedFields     []string       `json:"allowed_fields"`
	ArrayScope        string         `json:"array_scope"`
	RequiredRowFields []string       `json:"required_row_fields"`
	AllowedRowFields  []string       `json:"allowed_row_fields"`
	Nonempty          bool           `json:"nonempty"`
	Empty             bool           `json:"empty"`
	MembershipVersion bool           `json:"membership_version"`
}

func load(t *testing.T) []fixture {
	t.Helper()
	data, err := os.ReadFile("http.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version int       `json:"version"`
		Source  string    `json:"source"`
		Seed    string    `json:"seed"`
		Cases   []fixture `json:"cases"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	if document.Version != 1 || document.Seed != "lounge-and-groups-v1" || len(document.Cases) == 0 {
		t.Fatal("unsupported or empty contract")
	}
	return document.Cases
}

// This verifies fixture integrity, NOT implementation compliance. The opt-in
// r21_contracts build tag separately activates real HTTP assertions.
func TestFixtureIntegrity(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range load(t) {
		t.Run(f.ID, func(t *testing.T) {
			if f.ID == "" || seen[f.ID] {
				t.Fatal("empty/duplicate case ID")
			}
			seen[f.ID] = true
			switch f.Method {
			case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			default:
				t.Fatal("invalid method")
			}
			u, err := url.ParseRequestURI(f.Path)
			if err != nil || u.IsAbs() || !strings.HasPrefix(f.Path, "/api/") {
				t.Fatal("fixture must use relative API route")
			}
			if f.Actor != "human-member" && f.Actor != "bot-member" {
				t.Fatal("unknown seeded actor")
			}
			if f.Status < 200 || f.Status > 599 || f.Implementation != "pending" {
				t.Fatal("invalid status/implementation marker")
			}
			if f.Headers["Cache-Control"] != "no-store" {
				t.Fatal("missing no-store contract")
			}
			if f.Body != "" && !json.Valid([]byte(f.Body)) {
				t.Fatal("body is not JSON; duplicate keys must remain raw strings")
			}
			if f.Status == 403 && (len(f.Expect.JSON) != 2 || f.Expect.JSON["error"] != "forbidden" || f.Expect.JSON["message"] != "request not permitted") {
				t.Fatal("403 existence oracle")
			}
			for _, step := range f.Setup {
				if step.Actor != "human-member" && step.Actor != "human-peer" {
					t.Fatal("unknown setup actor")
				}
				if step.Status != 201 && step.Status != 204 {
					t.Fatal("setup must succeed before replay assertion")
				}
				if step.Method == http.MethodPost && (step.Path != "/api/messages" || step.IdempotencyKey == "" || step.IdempotencyKey != f.IdempotencyKey || !json.Valid([]byte(step.Body))) {
					t.Fatal("setup must seed same replay key with valid send")
				}
				if step.Method != http.MethodPost && step.Method != http.MethodDelete {
					t.Fatal("unsupported setup method")
				}
			}
		})
	}
}
