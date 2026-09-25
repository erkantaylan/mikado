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
	h := Handler(s)
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
		t.Error("no accepted hosts should mean localhost only")
	}
}
