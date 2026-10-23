//go:build r21_contracts

package r21

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// EXPECTED TO FAIL until R2.1 implementation and the disposable seed adapter land.
// Deliberately excluded from default suites/CI; never skip or claim it passed.
func TestHTTPContract(t *testing.T) {
	if os.Getenv("MEOWGRAM_DISPOSABLE_R21_CONTRACT") != "1" {
		t.Fatal("requires explicitly provisioned disposable R2.1 fixture")
	}
	base := loopbackURL(t, "R21_CONTRACT_URL")
	control := loopbackURL(t, "R21_CONTRACT_CONTROL_URL")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, f := range load(t) {
		t.Run(f.ID, func(t *testing.T) {
			// Seed controller resets all state AND issues synthetic auth for each case.
			// It is a test-adapter contract, not an application endpoint.
			seedBody, _ := json.Marshal(map[string]string{"seed": "lounge-and-groups-v1", "case": f.ID})
			reset, err := client.Post(control+"/reset", "application/json", bytes.NewReader(seedBody))
			if err != nil {
				t.Fatal("seed adapter unavailable")
			}
			defer reset.Body.Close()
			if reset.StatusCode != 200 {
				t.Fatalf("seed reset status %d", reset.StatusCode)
			}
			var auth map[string]string
			if err := json.NewDecoder(io.LimitReader(reset.Body, 65536)).Decode(&auth); err != nil {
				t.Fatal("invalid seed response")
			}
			if auth[f.Actor] == "" {
				t.Fatal("missing synthetic actor authorization")
			}
			req, err := http.NewRequest(f.Method, base+f.Path, strings.NewReader(f.Body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", auth[f.Actor])
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "r21-contract-"+f.ID)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal("fixture request failed")
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			if err != nil {
				t.Fatal("response read failed")
			}
			if resp.StatusCode != f.Status {
				t.Fatalf("status=%d want=%d (response omitted)", resp.StatusCode, f.Status)
			}
			for k, v := range f.Headers {
				if resp.Header.Get(k) != v {
					t.Errorf("header %s does not match", k)
				}
			}
			if f.Expect.MembershipVersion && resp.Header.Get("X-Membership-Version") == "" {
				t.Error("missing membership revision, including on empty pages")
			}
			if f.Status == 204 {
				if len(body) != 0 {
					t.Error("204 body must be empty")
				}
				return
			}
			if len(f.Expect.JSON) > 0 || len(f.Expect.Fields) > 0 || len(f.Expect.RequiredFields) > 0 {
				var object map[string]any
				if err := json.Unmarshal(body, &object); err != nil {
					t.Fatal("expected JSON object")
				}
				if len(f.Expect.JSON) > 0 && !reflect.DeepEqual(object, f.Expect.JSON) {
					t.Error("exact error body differs")
				}
				fields(t, object, f.Expect.Fields, f.Expect.RequiredFields)
			}
			if f.Expect.ArrayScope != "" || f.Expect.Empty {
				var rows []map[string]any
				if err := json.Unmarshal(body, &rows); err != nil || rows == nil {
					t.Fatal("expected JSON array")
				}
				if f.Expect.Empty && len(rows) != 0 {
					t.Error("expected empty page")
				}
				if f.Expect.Nonempty && len(rows) == 0 {
					t.Fatal("positive-control messages absent")
				}
				if len(rows) > 500 {
					t.Error("page exceeds 500")
				}
				if len(rows) == 500 && resp.Header.Get("X-Has-More") != "true" {
					t.Error("full page must expose X-Has-More")
				}
				scopes := map[string]bool{}
				var previousTime time.Time
				previousID := ""
				ids := map[string]bool{}
				for _, row := range rows {
					fields(t, row, nil, f.Expect.RequiredRowFields)
					for _, key := range f.Expect.ForbiddenRowFields {
						if _, ok := row[key]; ok {
							t.Errorf("unexpected row field %s", key)
						}
					}
					channel, present := row["channel_id"]
					if !present {
						t.Error("missing channel_id")
					}
					key := fmt.Sprint(channel)
					scopes[key] = true
					switch f.Expect.ArrayScope {
					case "lounge":
						if channel != nil {
							t.Error("private row leaked into legacy lounge")
						}
					case "member":
						if channel != nil && channel != "11111111-1111-4111-8111-111111111111" {
							t.Error("unauthorized group row")
						}
					default:
						if channel != f.Expect.ArrayScope {
							t.Error("wrong group row")
						}
					}
					count := row["conversation_member_count"]
					if channel == nil {
						if count != nil {
							t.Error("lounge member count must be null")
						}
					} else if count != float64(3) {
						t.Error("group member count must reflect three seeded current members")
					}
					if f.Actor == "human-member" {
						if value, ok := row["conversation_muted"].(bool); !ok || value {
							t.Error("expected seeded unmuted human metadata")
						}
					}
					id, _ := row["id"].(string)
					stamp, _ := row["created_at"].(string)
					at, err := time.Parse(time.RFC3339Nano, stamp)
					if err != nil || !strings.HasSuffix(stamp, "Z") || id == "" {
						t.Error("invalid UTC time/message identity")
					}
					if ids[id] {
						t.Error("duplicate message UUID")
					}
					ids[id] = true
					if at.Before(previousTime) || at.Equal(previousTime) && id < previousID {
						t.Error("unstable time/UUID order")
					}
					previousTime, previousID = at, id
				}
				if f.Expect.ArrayScope == "member" && (!scopes["<nil>"] || !scopes["11111111-1111-4111-8111-111111111111"]) {
					t.Error("merged positive controls must include lounge and authorized group")
				}
			}
		})
	}
}
func fields(t *testing.T, object, expected map[string]any, required []string) {
	t.Helper()
	for _, key := range required {
		if _, ok := object[key]; !ok {
			t.Errorf("missing field %s", key)
		}
	}
	for key, value := range expected {
		actual, ok := object[key]
		if !ok || !reflect.DeepEqual(actual, value) {
			t.Errorf("field %s differs", key)
		}
	}
}
func loopbackURL(t *testing.T, key string) string {
	t.Helper()
	raw := os.Getenv(key)
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		t.Fatalf("invalid %s", key)
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "http" || ip == nil || !ip.IsLoopback() || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		t.Fatalf("%s must be an explicit loopback HTTP origin for disposable fixtures", key)
	}
	return strings.TrimRight(raw, "/")
}
