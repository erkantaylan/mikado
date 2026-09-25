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
		{"POST", "/api/quests", `{"title":"Ship it","slug":"ship"}`, local, js, 201},
		{"POST", "/api/quests", `{"title":"Ship it","slug":"ship"}`, local, js, 409},
		{"POST", "/api/quests", `{"title":"x"}`, "evil.example:47291", js, 403},
		{"POST", "/api/quests", `{"title":"x"}`, local, "text/plain", 415},
		{"POST", "/api/quests", `{"titel":"x"}`, local, js, 400},
		// Quest-scoped alias: final:true makes it that quest's final (M1).
		{"POST", "/api/quests/SHIP/cards", `{"kind":"errand","title":"a","final":true}`, "localhost:5173", js, 201},
		{"POST", "/api/quests/nope/cards", `{"kind":"errand","title":"x"}`, local, js, 404},
		// Global: M2, needed by M1, so in quest ship.
		{"POST", "/api/cards", `{"kind":"errand","title":"b","neededBy":[1]}`, local, js, 201},
		{"POST", "/api/needs", `{"from":2,"to":1}`, local, js, 409},
		{"POST", "/api/cards", `{"kind":"issue","ref":"studio/game#1"}`, local, js, 400},
		{"PATCH", "/api/cards/M-2", `{"done":true}`, local, js, 200},
		{"PATCH", "/api/cards/2", `{"final":true}`, local, js, 400},
		{"PATCH", "/api/quests/ship/cards/m2", `{"working":false}`, local, js, 200},
		{"GET", "/api/cards/M2", "", local, "", 200},
		{"GET", "/api/cards/studio/game%231", "", local, "", 404},
		{"POST", "/api/quests", `{"title":"Other","final":"M2"}`, local, js, 201},
		{"PATCH", "/api/quests/other", `{"slug":"Other-Two"}`, local, js, 200},
		{"PATCH", "/api/quests/other-two", `{"slug":"bad slug"}`, local, js, 400},
		{"DELETE", "/api/quests/ship/needs", `{"from":1,"to":2}`, local, js, 204},
		{"DELETE", "/api/cards/c2", `{"reason":"parked"}`, local, js, 204},
		{"GET", "/api/quests/nope", "", local, "", 404},
		{"GET", "/api/nope", "", local, "", 404},
	}
	for _, st := range steps {
		if code, out := call(st.method, st.path, st.body, st.host, st.ctype); code != st.want {
			t.Errorf("%s %s %s: %d %v, want %d", st.method, st.path, st.body, code, out, st.want)
		}
	}
	code, out := call("GET", "/api/quests/ship", "", local, "")
	if code != 200 || len(out["cards"].([]any)) != 1 {
		t.Errorf("quest view: %d %v", code, out)
	}
	card := out["cards"].([]any)[0].(map[string]any)
	if card["key"] != "M1" || card["final"] != true || len(card["alsoIn"].([]any)) != 0 {
		t.Errorf("card JSON: %v", card)
	}
	if code, out := call("GET", "/api/quests/other-two", "", local, ""); code != 200 || out["quest"].(map[string]any)["finalCardId"] != nil {
		t.Errorf("quest whose final was removed: %d %v", code, out)
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
		{"GET", "/api/quests", "", "foo.test", 403},
		{"POST", "/api/hosts", `{"name":"Foo.Test"}`, "foo.test", 403},
		{"POST", "/api/hosts", `{"name":"foo.test:80"}`, local, 400},
		{"POST", "/api/hosts", `{"name":"Foo.Test."}`, local, 201},
		// Accepted from the very next request, no restart.
		{"GET", "/api/quests", "", "foo.test:8080", 200},
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
		{"GET", "/api/quests", "", "foo.test", 403},
		{"DELETE", "/api/hosts/foo.test", "", local, 404},
		{"GET", "/api/quests", "", "box.ts.net", 200},
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
	if code, _ := call("GET", "/api/quests", "", "box.ts.net"); code != 200 {
		t.Errorf("stored host after a restart: %d", code)
	}
	if code, _ := call("GET", "/api/quests", "", "mikado.home"); code != 403 {
		t.Errorf("flag host after a restart without the flag: %d", code)
	}
	if _, err := Handler(s, "http://x"); err == nil {
		t.Error("a bad flag host should be refused")
	}
}

func TestQuestSlugRefsAndCrowns(t *testing.T) {
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
		{"POST", "/api/quests", `{"title":"Controller support","slug":"pads"}`, 201},
		{"POST", "/api/quests/pads/cards", `{"kind":"errand","title":"ship pads","final":true}`, 201}, // M1
		{"POST", "/api/cards", `{"kind":"errand","title":"glyphs","neededBy":[1]}`, 201},              // M2
		{"POST", "/api/quests", `{"title":"Season two","slug":"season","final":"pads"}`, 201},         // crowned by M1 too
		{"POST", "/api/quests", `{"title":"Launch","slug":"launch"}`, 201},
		{"POST", "/api/cards", `{"kind":"errand","title":"launch it","finalOf":"launch"}`, 201}, // M3
		{"PATCH", "/api/quests/launch", `{"final":"launch"}`, 200},
		{"GET", "/api/cards/nope", "", 404},
		{"POST", "/api/quests", `{"title":"Empty","slug":"empty"}`, 201},
		{"GET", "/api/cards/empty", "", 400},
		{"PATCH", "/api/cards/launch", `{"working":true}`, 200},
	} {
		if code, out := call(st.method, st.path, st.body); code != st.want {
			t.Errorf("%s %s %s: %d %v, want %d", st.method, st.path, st.body, code, out, st.want)
		}
	}
	// launch requires the whole of pads: M3 requires M1.
	if code, out := call("POST", "/api/needs", `{"from":3,"to":1}`); code != 201 {
		t.Fatalf("need: %d %v", code, out)
	}
	code, out := call("GET", "/api/cards/pads", "")
	if code != 200 || out["card"].(map[string]any)["key"] != "M1" {
		t.Fatalf("card by slug: %d %v", code, out)
	}
	if cr, _ := out["card"].(map[string]any)["crowns"].(map[string]any); cr == nil || cr["slug"] != "pads" {
		t.Errorf("card view crowns: %v", out["card"])
	}
	_, out = call("GET", "/api/quests/launch", "")
	cards := out["cards"].([]any)
	if len(cards) != 2 {
		t.Fatalf("launch cards: %v", cards)
	}
	for _, c := range cards {
		c := c.(map[string]any)
		cr, has := c["crowns"].(map[string]any)
		switch c["key"] {
		case "M1":
			if !has || cr["slug"] != "pads" || cr["done"] != 0.0 || cr["total"] != 2.0 || cr["state"] != "active" || len(cr["open"].([]any)) != 2 {
				t.Errorf("quest card: %v", c)
			}
		case "M3":
			if has {
				t.Errorf("own crowning deed has crowns: %v", c)
			}
		}
	}
}
