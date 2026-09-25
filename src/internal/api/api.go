// Package api is mikado's JSON API, mounted under /api. It is the only way in
// to the data for the CLI and the dashboard alike, so authentication can be
// added here later without touching either.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"mikado/internal/store"
)

// Handler returns the API handler. Paths include the /api prefix. Cards and
// needs are global; the quest-scoped card and need routes are kept as aliases
// (they only check that the quest exists, and let "final" mean that quest's).
// Requests must be addressed to localhost or to one of hosts (see HostMatcher).
func Handler(s *store.Store, hosts ...string) http.Handler {
	h := &handler{s: s}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/quests", h.listQuests)
	mux.HandleFunc("POST /api/quests", h.createQuest)
	mux.HandleFunc("GET /api/quests/{slug}", h.getQuest)
	mux.HandleFunc("PATCH /api/quests/{slug}", h.patchQuest)

	mux.HandleFunc("POST /api/cards", h.addCard)
	mux.HandleFunc("GET /api/cards/{ref...}", h.getCard)
	mux.HandleFunc("PATCH /api/cards/{id}", h.patchCard)
	mux.HandleFunc("DELETE /api/cards/{id}", h.removeCard)
	mux.HandleFunc("POST /api/cards/{id}/assignees", h.assign)
	mux.HandleFunc("POST /api/needs", h.addNeed)
	mux.HandleFunc("DELETE /api/needs", h.removeNeed)

	// Quest-scoped aliases.
	mux.HandleFunc("POST /api/quests/{slug}/cards", h.inQuest(h.addCard))
	mux.HandleFunc("PATCH /api/quests/{slug}/cards/{id}", h.inQuest(h.patchCard))
	mux.HandleFunc("DELETE /api/quests/{slug}/cards/{id}", h.inQuest(h.removeCard))
	mux.HandleFunc("POST /api/quests/{slug}/cards/{id}/assignees", h.inQuest(h.assign))
	mux.HandleFunc("POST /api/quests/{slug}/needs", h.inQuest(h.addNeed))
	mux.HandleFunc("DELETE /api/quests/{slug}/needs", h.inQuest(h.removeNeed))

	mux.HandleFunc("GET /api/repos/{owner}/{repo}/assignees", h.repoAssignees)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
	})
	return guard(mux, HostMatcher(hosts))
}

// HostMatcher reports whether a request's Host names this server: localhost,
// a loopback IP, or one of the accepted hosts. An accepted host is a name
// ("mikado.home") or a wildcard for its subdomains ("*.ts.net"); names are
// compared ignoring case, a port and a trailing dot.
func HostMatcher(accepted []string) func(host string) bool {
	var exact, suffixes []string
	for _, a := range accepted {
		a = normHost(a)
		if rest, ok := strings.CutPrefix(a, "*."); ok {
			suffixes = append(suffixes, "."+rest)
		} else if a != "" {
			exact = append(exact, a)
		}
	}
	return func(host string) bool {
		host = normHost(host)
		if host == "localhost" || isLoopback(host) || slices.Contains(exact, host) {
			return true
		}
		return slices.ContainsFunc(suffixes, func(s string) bool { return strings.HasSuffix(host, s) })
	}
}

func normHost(host string) string {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
}

// guard protects a server with no authentication against other web pages
// the browser has open: it refuses Host headers that are not ours (DNS
// rebinding) and bodies that are not JSON (a cross-site form or no-cors fetch
// cannot send application/json without a CORS preflight, which we never
// answer).
func guard(next http.Handler, ours func(host string) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ours(r.Host) {
			writeError(w, http.StatusForbidden, "mikado does not answer requests addressed to "+normHost(r.Host)+
				": accept that host with `mikado serve --allow-host`")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.ContentLength != 0 {
			if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, "send the body as Content-Type: application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type handler struct{ s *store.Store }

// inQuest checks the {slug} of a quest-scoped alias exists before handing
// over to the global handler.
func (h *handler) inQuest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h.s.CheckQuest(r.Context(), r.PathValue("slug")); err != nil {
			fail(w, err)
			return
		}
		next(w, r)
	}
}

// cardRef is a card named in a body by id (142) or by any reference string
// ("M142", "studio/game#7").
type cardRef struct{ raw json.RawMessage }

func (c *cardRef) UnmarshalJSON(b []byte) error { c.raw = append(json.RawMessage{}, b...); return nil }

func (h *handler) resolve(r *http.Request, c *cardRef) (*int64, error) {
	if c == nil || len(c.raw) == 0 || string(c.raw) == "null" {
		return nil, nil
	}
	var ref string
	var n int64
	switch {
	case json.Unmarshal(c.raw, &n) == nil:
		ref = strconv.FormatInt(n, 10)
	case json.Unmarshal(c.raw, &ref) == nil:
	default:
		return nil, &store.Error{Kind: store.ErrInvalid, Msg: "a deed is a number or a string like \"M142\""}
	}
	id, err := h.s.Resolve(r.Context(), ref)
	return &id, err
}

func (h *handler) listQuests(w http.ResponseWriter, r *http.Request) {
	qs, warning, err := h.s.Quests(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	if warning != "" {
		// The body is a bare array, so the GitHub warning travels as a header.
		w.Header().Set("X-Mikado-GitHub", warning)
	}
	writeJSON(w, http.StatusOK, qs)
}

func (h *handler) createQuest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string   `json:"title"`
		Slug  string   `json:"slug"`
		Final *cardRef `json:"final"`
	}
	if !decode(w, r, &in) {
		return
	}
	final, err := h.resolve(r, in.Final)
	if err != nil {
		fail(w, err)
		return
	}
	q, err := h.s.CreateQuest(r.Context(), in.Title, in.Slug, final)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, q)
}

func (h *handler) getQuest(w http.ResponseWriter, r *http.Request) {
	v, err := h.s.Quest(r.Context(), r.PathValue("slug"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *handler) patchQuest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Slug  *string  `json:"slug"`
		Title *string  `json:"title"`
		Final *cardRef `json:"final"`
	}
	if !decode(w, r, &in) {
		return
	}
	final, err := h.resolve(r, in.Final)
	if err != nil {
		fail(w, err)
		return
	}
	q, err := h.s.UpdateQuest(r.Context(), r.PathValue("slug"), store.QuestPatch{Slug: in.Slug, Title: in.Title, Final: final})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (h *handler) addCard(w http.ResponseWriter, r *http.Request) {
	var in store.NewCard
	if !decode(w, r, &in) {
		return
	}
	c, created, err := h.s.AddCard(r.Context(), in, r.PathValue("slug"))
	if err != nil {
		fail(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, c)
}

func (h *handler) getCard(w http.ResponseWriter, r *http.Request) {
	id, err := h.s.Resolve(r.Context(), r.PathValue("ref"))
	if err != nil {
		fail(w, err)
		return
	}
	v, err := h.s.CardView(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *handler) patchCard(w http.ResponseWriter, r *http.Request) {
	id, ok := h.cardID(w, r)
	if !ok {
		return
	}
	var in store.CardPatch
	if !decode(w, r, &in) {
		return
	}
	c, err := h.s.UpdateCard(r.Context(), id, in, r.PathValue("slug"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *handler) removeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := h.cardID(w, r)
	if !ok {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.s.RemoveCard(r.Context(), id, in.Reason); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) assign(w http.ResponseWriter, r *http.Request) {
	id, ok := h.cardID(w, r)
	if !ok {
		return
	}
	var in struct {
		Add    []string `json:"add"`
		Remove []string `json:"remove"`
	}
	if !decode(w, r, &in) {
		return
	}
	c, err := h.s.Assign(r.Context(), id, in.Add, in.Remove, r.PathValue("slug"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *handler) addNeed(w http.ResponseWriter, r *http.Request) {
	var in store.Need
	if !decode(w, r, &in) {
		return
	}
	created, err := h.s.AddNeed(r.Context(), in.From, in.To)
	if err != nil {
		fail(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, in)
}

func (h *handler) removeNeed(w http.ResponseWriter, r *http.Request) {
	var in store.Need
	if !decode(w, r, &in) {
		return
	}
	if err := h.s.RemoveNeed(r.Context(), in.From, in.To); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) repoAssignees(w http.ResponseWriter, r *http.Request) {
	users, err := h.s.RepoAssignees(r.Context(), r.PathValue("owner"), r.PathValue("repo"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// cardID reads the {id} path segment: 142, M142, m-142, c142 or an issue
// URL-escaped as one segment.
func (h *handler) cardID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := h.s.Resolve(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return 0, false
	}
	return id, true
}

// decode reads a JSON body into v, rejecting unknown fields so a typo in a
// field name is an error rather than silently ignored.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "bad JSON body: "+err.Error())
		return false
	}
	return true
}

func fail(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch store.KindOf(err) {
	case store.ErrInvalid:
		status = http.StatusBadRequest
	case store.ErrNotFound:
		status = http.StatusNotFound
	case store.ErrConflict:
		status = http.StatusConflict
	case store.ErrUpstream:
		status = http.StatusBadGateway
	default:
		log.Printf("api: %v", err)
	}
	writeError(w, status, err.Error())
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
