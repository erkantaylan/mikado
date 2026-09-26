package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"mikado/internal/github"
	"mikado/internal/store"
)

type noGitHub struct{}

func (noGitHub) Issues(context.Context, []github.Ref) (map[string]github.Issue, error) {
	return map[string]github.Issue{}, nil
}
func (noGitHub) Assign(context.Context, github.Ref, []string, []string) error { return nil }
func (noGitHub) Assignees(context.Context, string, string) ([]string, error) {
	return []string{}, nil
}

func TestAPI(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "mikado.db"), noGitHub{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h, err := Handler(s)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, host, ctype string) (int, map[string]any) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = host
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	const js, local = "application/json", "127.0.0.1:47291"
	steps := []struct {
		method, path, body, host, ctype string
		want                            int
	}{
		{"POST", "/api/journeys", `{"title":"Ship it"}`, local, js, 201}, // J1
		{"POST", "/api/journeys", `{"title":"Ship it","slug":"ship"}`, local, js, 400},
		{"POST", "/api/journeys", `{"title":"x"}`, "evil.example:47291", js, 403},
		{"POST", "/api/journeys", `{"title":"x"}`, local, "text/plain", 415},
		{"POST", "/api/journeys", `{"titel":"x"}`, local, js, 400},
		// Journey-scoped alias: final:true makes it that journey's final (Q1).
		{"POST", "/api/journeys/j1/cards", `{"kind":"errand","title":"a","final":true}`, "localhost:5173", js, 201},
		{"POST", "/api/journeys/J99/cards", `{"kind":"errand","title":"x"}`, local, js, 404},
		{"POST", "/api/journeys/ship/cards", `{"kind":"errand","title":"x"}`, local, js, 400},
		// Global: Q2, needed by Q1, so in journey J1.
		{"POST", "/api/cards", `{"kind":"errand","title":"b","neededBy":[1]}`, local, js, 201},
		{"POST", "/api/needs", `{"from":2,"to":1}`, local, js, 409},
		{"POST", "/api/cards", `{"kind":"issue","ref":"studio/game#1"}`, local, js, 400},
		{"PATCH", "/api/cards/Q-2", `{"done":true}`, local, js, 200},
		{"PATCH", "/api/cards/2", `{"final":true}`, local, js, 400},
		{"PATCH", "/api/journeys/J1/cards/q2", `{"working":false}`, local, js, 200},
		{"GET", "/api/cards/Q2", "", local, "", 200},
		{"GET", "/api/cards/M2", "", local, "", 400},
		{"GET", "/api/cards/studio/game%231", "", local, "", 404},
		{"POST", "/api/journeys", `{"title":"Other","final":"Q2"}`, local, js, 201}, // J2
		{"PATCH", "/api/journeys/J2", `{"title":"Other two"}`, local, js, 200},
		{"PATCH", "/api/journeys/J2", `{"slug":"other-two"}`, local, js, 400},
		{"DELETE", "/api/journeys/J1/needs", `{"from":1,"to":2}`, local, js, 204},
		{"DELETE", "/api/cards/q2", `{"reason":"parked"}`, local, js, 204},
		{"GET", "/api/journeys/J99", "", local, "", 404},
		{"GET", "/api/journeys/nope", "", local, "", 400},
		{"GET", "/api/nope", "", local, "", 404},
	}
	for _, st := range steps {
		if code, out := call(st.method, st.path, st.body, st.host, st.ctype); code != st.want {
			t.Errorf("%s %s %s: %d %v, want %d", st.method, st.path, st.body, code, out, st.want)
		}
	}
	code, out := call("GET", "/api/journeys/J1", "", local, "")
	if code != 200 || len(out["cards"].([]any)) != 1 || out["journey"].(map[string]any)["key"] != "J1" {
		t.Errorf("journey view: %d %v", code, out)
	}
	card := out["cards"].([]any)[0].(map[string]any)
	if card["key"] != "Q1" || card["final"] != true || len(card["alsoIn"].([]any)) != 0 {
		t.Errorf("card JSON: %v", card)
	}
	if code, out := call("GET", "/api/journeys/J2", "", local, ""); code != 200 ||
		out["journey"].(map[string]any)["finalCardId"] != nil || out["journey"].(map[string]any)["title"] != "Other two" {
		t.Errorf("journey whose final was removed: %d %v", code, out)
	}
}

func TestHostMatcher(t *testing.T) {
	ours := HostMatcher([]string{"Mikado.Home", "*.ts.net", " "})
	for host, want := range map[string]bool{
		"localhost:47291":       true,
		"mikado.localhost:5173": true,
		"a.b.LOCALHOST.":        true,
		"localhost.evil.test":   false,
		"evillocalhost":         false,
		"127.0.0.1":             true,
		"[::1]:47291":           true,
		"mikado.home":           true,
		"MIKADO.HOME.:8080":     true,
		"box.tail1.ts.net":      true,
		"ts.net":                false,
		"evilts.net":            false,
		"mikado.home.evil.test": false,
		"evil.example":          false,
		"":                      false,
	} {
		if got := ours(host); got != want {
			t.Errorf("%q: %v, want %v", host, got, want)
		}
	}
	if HostMatcher(nil)("mikado.home") {
		t.Error("no accepted hosts should mean localhost (and *.localhost) only")
	}
}

func TestHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikado.db")
	s, err := store.Open(path, noGitHub{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }() // s is reopened below
	h, err := Handler(s, "Mikado.Home")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, host string) (int, string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = host
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, strings.TrimSpace(rec.Body.String())
	}
	const local = "127.0.0.1:47291"
	steps := []struct {
		method, path, body, host string
		want                     int
	}{
		{"GET", "/api/journeys", "", "foo.test", 403},
		{"POST", "/api/hosts", `{"name":"Foo.Test"}`, "foo.test", 403},
		{"POST", "/api/hosts", `{"name":"foo.test:80"}`, local, 400},
		{"POST", "/api/hosts", `{"name":"Foo.Test."}`, local, 201},
		// Accepted from the very next request, no restart.
		{"GET", "/api/journeys", "", "foo.test:8080", 200},
		{"POST", "/api/hosts", `{"name":"foo.test"}`, "localhost:47291", 200},
		// Readable through any accepted host, changeable only through localhost.
		{"GET", "/api/hosts", "", "foo.test", 200},
		{"POST", "/api/hosts", `{"name":"bar.test"}`, "foo.test", 403},
		{"POST", "/api/hosts", `{"name":"bar.test"}`, "mikado.localhost", 403},
		{"DELETE", "/api/hosts/foo.test", "", "mikado.home", 403},
		{"DELETE", "/api/hosts/mikado.home", "", local, 409},
		{"POST", "/api/hosts", `{"name":"mikado.home"}`, local, 200},
		{"POST", "/api/hosts", `{"name":"*.ts.net"}`, "[::1]:47291", 201},
		{"DELETE", "/api/hosts/FOO.test", "", local, 204},
		{"GET", "/api/journeys", "", "foo.test", 403},
		{"DELETE", "/api/hosts/foo.test", "", local, 404},
		{"GET", "/api/journeys", "", "box.ts.net", 200},
		{"POST", "/api/hosts", `name=x`, local, 400},
	}
	for _, st := range steps {
		if code, out := call(st.method, st.path, st.body, st.host); code != st.want {
			t.Errorf("%s %s %s (Host %s): %d %s, want %d", st.method, st.path, st.body, st.host, code, out, st.want)
		}
	}
	if code, out := call("GET", "/api/hosts", "", local); code != 200 ||
		!strings.HasPrefix(out, `[{"name":"mikado.home","source":"flag"},{"name":"*.ts.net","source":"stored","addedAt":"`) {
		t.Errorf("GET /api/hosts: %d %s", code, out)
	}

	// Stored hosts outlive the server; flag hosts do not.
	s.Close()
	if s, err = store.Open(path, noGitHub{}); err != nil {
		t.Fatal(err)
	}
	if h, err = Handler(s); err != nil {
		t.Fatal(err)
	}
	if code, _ := call("GET", "/api/journeys", "", "box.ts.net"); code != 200 {
		t.Errorf("stored host after a restart: %d", code)
	}
	if code, _ := call("GET", "/api/journeys", "", "mikado.home"); code != 403 {
		t.Errorf("flag host after a restart without the flag: %d", code)
	}
	if _, err := Handler(s, "http://x"); err == nil {
		t.Error("a bad flag host should be refused")
	}
}

func TestJourneyKeyRefsAndCrowns(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "mikado.db"), noGitHub{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h, err := Handler(s)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) (int, map[string]any) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = "localhost"
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	for _, st := range []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/journeys", `{"title":"Controller support"}`, 201},                              // J1
		{"POST", "/api/journeys/J1/cards", `{"kind":"errand","title":"ship pads","final":true}`, 201}, // Q1
		{"POST", "/api/cards", `{"kind":"errand","title":"glyphs","neededBy":[1]}`, 201},              // Q2
		{"POST", "/api/journeys", `{"title":"Season two","final":"J1"}`, 201},                         // J2, crowned by Q1 too
		{"POST", "/api/journeys", `{"title":"Launch"}`, 201},                                          // J3
		{"POST", "/api/cards", `{"kind":"errand","title":"launch it","finalOf":"J3"}`, 201},           // Q3
		{"PATCH", "/api/journeys/J3", `{"final":"J3"}`, 200},
		{"GET", "/api/cards/J99", "", 404},
		{"GET", "/api/cards/nope", "", 400},
		{"POST", "/api/journeys", `{"title":"Empty"}`, 201}, // J4
		{"GET", "/api/cards/J4", "", 400},
		{"PATCH", "/api/cards/j3", `{"working":true}`, 200},
	} {
		if code, out := call(st.method, st.path, st.body); code != st.want {
			t.Errorf("%s %s %s: %d %v, want %d", st.method, st.path, st.body, code, out, st.want)
		}
	}
	// Launch requires the whole of J1: Q3 requires Q1.
	if code, out := call("POST", "/api/needs", `{"from":3,"to":1}`); code != 201 {
		t.Fatalf("need: %d %v", code, out)
	}
	code, out := call("GET", "/api/cards/J1", "")
	if code != 200 || out["card"].(map[string]any)["key"] != "Q1" {
		t.Fatalf("card by journey key: %d %v", code, out)
	}
	if cr, _ := out["card"].(map[string]any)["crowns"].(map[string]any); cr == nil || cr["key"] != "J1" {
		t.Errorf("card view crowns: %v", out["card"])
	}
	if js, _ := out["journeys"].([]any); len(js) != 3 { // J1, J2 it crowns; J3 holds it as a journey card
		t.Errorf("card view journeys: %v", out["journeys"])
	}
	_, out = call("GET", "/api/journeys/J3", "")
	cards := out["cards"].([]any)
	if len(cards) != 2 {
		t.Fatalf("launch cards: %v", cards)
	}
	for _, c := range cards {
		c := c.(map[string]any)
		cr, has := c["crowns"].(map[string]any)
		switch c["key"] {
		case "Q1":
			if !has || cr["key"] != "J1" || cr["done"] != 0.0 || cr["total"] != 2.0 || cr["state"] != "active" || len(cr["open"].([]any)) != 2 {
				t.Errorf("journey card: %v", c)
			}
		case "Q3":
			if has {
				t.Errorf("own crowning quest has crowns: %v", c)
			}
		}
	}
}
