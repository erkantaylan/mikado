// Package api is mikado's JSON API, mounted under /api. It is the only way in
// to the data for the CLI and the dashboard alike, so authentication can be
// added here later without touching either.
package api

import (
	"context"
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
	"sync"
	"sync/atomic"

	"mikado/internal/store"
)

// Handler returns the API handler. Paths include the /api prefix. Cards
// (quests) and needs are global; the journey-scoped card and need routes are
// aliases (they only check that the journey exists, and let "final" mean
// that journey's).
// Requests must be addressed to localhost or to an accepted host (see
// HostMatcher): one of hosts, which are fixed for this run (--allow-host,
// $MIKADO_ALLOWED_HOSTS), or one stored in the database, which can be added
// and removed while the server runs.
func Handler(s *store.Store, hosts ...string) (http.Handler, error) {
	h := &handler{s: s}
	for _, name := range hosts {
		name, err := store.CleanHost(name)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(h.fixed, name) {
			h.fixed = append(h.fixed, name)
		}
	}
	if err := h.reloadHosts(context.Background()); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/journeys", h.listJourneys)
	mux.HandleFunc("POST /api/journeys", h.createJourney)
	mux.HandleFunc("GET /api/journeys/{key}", h.getJourney)
	mux.HandleFunc("PATCH /api/journeys/{key}", h.patchJourney)

	mux.HandleFunc("POST /api/cards", h.addCard)
	mux.HandleFunc("GET /api/cards/{ref...}", h.getCard)
	mux.HandleFunc("PATCH /api/cards/{id}", h.patchCard)
	mux.HandleFunc("DELETE /api/cards/{id}", h.removeCard)
	mux.HandleFunc("POST /api/cards/{id}/assignees", h.assign)
	mux.HandleFunc("POST /api/needs", h.addNeed)
	mux.HandleFunc("DELETE /api/needs", h.removeNeed)

	// Journey-scoped aliases.
	mux.HandleFunc("POST /api/journeys/{key}/cards", h.inJourney(h.addCard))
	mux.HandleFunc("PATCH /api/journeys/{key}/cards/{id}", h.inJourney(h.patchCard))
	mux.HandleFunc("DELETE /api/journeys/{key}/cards/{id}", h.inJourney(h.removeCard))
	mux.HandleFunc("POST /api/journeys/{key}/cards/{id}/assignees", h.inJourney(h.assign))
	mux.HandleFunc("POST /api/journeys/{key}/needs", h.inJourney(h.addNeed))
	mux.HandleFunc("DELETE /api/journeys/{key}/needs", h.inJourney(h.removeNeed))

	mux.HandleFunc("GET /api/hosts", h.listHosts)
	mux.HandleFunc("POST /api/hosts", h.localOnly(h.addHost))
	mux.HandleFunc("DELETE /api/hosts/{name}", h.localOnly(h.removeHost))

	mux.HandleFunc("GET /api/search", h.search)
	mux.HandleFunc("GET /api/repos/{owner}/{repo}/assignees", h.repoAssignees)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
	})
	return guard(mux, func(host string) bool { return (*h.ours.Load())(host) }), nil
}

// HostMatcher reports whether a request's Host names this server: localhost
// or a name under it (foo.localhost: browsers resolve these to loopback
// themselves, RFC 6761), a loopback IP, or one of the accepted hosts. An accepted host is a name
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
		if isLocal(host) || strings.HasSuffix(host, ".localhost") || slices.Contains(exact, host) {
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
// rebinding: another site's page cannot make the browser send Host
// localhost or *.localhost, only a name of its own) and bodies that are not JSON (a cross-site form or no-cors fetch
// cannot send application/json without a CORS preflight, which we never
// answer).
func guard(next http.Handler, ours func(host string) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ours(r.Host) {
			writeError(w, http.StatusForbidden, "mikado does not answer requests addressed to "+normHost(r.Host)+
				": accept that host with `mikado hosts add "+normHost(r.Host)+"`")
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

// isLocal reports whether a normalised host is exactly localhost or a
// loopback IP.
func isLocal(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}

type handler struct {
	s *store.Store
	// fixed are the hosts from --allow-host / $MIKADO_ALLOWED_HOSTS; the
	// stored ones are read into ours, with them, whenever the list changes.
	fixed   []string
	ours    atomic.Pointer[func(host string) bool]
	hostsMu sync.Mutex // one change of the stored hosts (and reload) at a time
}

// reloadHosts rebuilds the host matcher from the fixed and stored hosts.
func (h *handler) reloadHosts(ctx context.Context) error {
	stored, err := h.s.Hosts(ctx)
	if err != nil {
		return err
	}
	names := slices.Clone(h.fixed)
	for _, s := range stored {
		names = append(names, s.Name)
	}
	ours := HostMatcher(names)
	h.ours.Store(&ours)
	return nil
}

// localOnly lets only requests addressed to localhost or a loopback IP
// through: accepted hosts are changed from this machine, never through a
// name that reaches it from elsewhere (a proxy, tailscale serve).
func (h *handler) localOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if host := normHost(r.Host); !isLocal(host) {
			writeError(w, http.StatusForbidden, "accepted hosts can only be changed through localhost or a loopback IP, not "+host+
				": run `mikado hosts` on the machine mikado runs on")
			return
		}
		next(w, r)
	}
}

// hostEntry is an accepted host as GET /api/hosts lists it.
type hostEntry struct {
	Name    string `json:"name"`
	Source  string `json:"source"` // "flag" (--allow-host, $MIKADO_ALLOWED_HOSTS) or "stored"
	AddedAt string `json:"addedAt,omitempty"`
}

func (h *handler) listHosts(w http.ResponseWriter, r *http.Request) {
	stored, err := h.s.Hosts(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := []hostEntry{}
	for _, name := range h.fixed {
		out = append(out, hostEntry{Name: name, Source: "flag"})
	}
	for _, s := range stored {
		if !slices.Contains(h.fixed, s.Name) {
			out = append(out, hostEntry{Name: s.Name, Source: "stored", AddedAt: s.AddedAt})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handler) addHost(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	name, err := store.CleanHost(in.Name)
	if err != nil {
		fail(w, err)
		return
	}
	if slices.Contains(h.fixed, name) {
		writeJSON(w, http.StatusOK, hostEntry{Name: name, Source: "flag"})
		return
	}
	h.hostsMu.Lock()
	defer h.hostsMu.Unlock()
	host, created, err := h.s.AddHost(r.Context(), name)
	if err == nil {
		err = h.reloadHosts(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		log.Printf("now also answering requests addressed to %s", host.Name)
	}
	writeJSON(w, status, hostEntry{Name: host.Name, Source: "stored", AddedAt: host.AddedAt})
}

func (h *handler) removeHost(w http.ResponseWriter, r *http.Request) {
	name, err := store.CleanHost(r.PathValue("name"))
	if err != nil {
		fail(w, err)
		return
	}
	if slices.Contains(h.fixed, name) {
		writeError(w, http.StatusConflict, name+" comes from --allow-host or $MIKADO_ALLOWED_HOSTS: "+
			"take it out there and restart mikado serve")
		return
	}
	h.hostsMu.Lock()
	defer h.hostsMu.Unlock()
	err = h.s.RemoveHost(r.Context(), name)
	if err == nil {
		err = h.reloadHosts(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	log.Printf("no longer answering requests addressed to %s", name)
	w.WriteHeader(http.StatusNoContent)
}

// inJourney checks the {key} of a journey-scoped alias exists before
// handing over to the global handler.
func (h *handler) inJourney(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h.s.CheckJourney(r.Context(), r.PathValue("key")); err != nil {
			fail(w, err)
			return
		}
		next(w, r)
	}
}

// cardRef is a card named in a body by id (142) or by any reference string
// ("Q142", "studio/game#7", "J7" for that journey's crowning quest).
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
		return nil, &store.Error{Kind: store.ErrInvalid, Msg: "a quest is a number or a string like \"Q142\""}
	}
	id, err := h.s.Resolve(r.Context(), ref)
	return &id, err
}

func (h *handler) listJourneys(w http.ResponseWriter, r *http.Request) {
	qs, warning, err := h.s.Journeys(r.Context())
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

func (h *handler) createJourney(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string   `json:"title"`
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
	q, err := h.s.CreateJourney(r.Context(), in.Title, final)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, q)
}

func (h *handler) getJourney(w http.ResponseWriter, r *http.Request) {
	v, err := h.s.Journey(r.Context(), r.PathValue("key"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *handler) patchJourney(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title    *string  `json:"title"`
		Final    *cardRef `json:"final"`
		Archived *bool    `json:"archived"`
	}
	if !decode(w, r, &in) {
		return
	}
	final, err := h.resolve(r, in.Final)
	if err != nil {
		fail(w, err)
		return
	}
	q, err := h.s.UpdateJourney(r.Context(), r.PathValue("key"), store.JourneyPatch{Title: in.Title, Final: final, Archived: in.Archived})
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
	c, created, err := h.s.AddCard(r.Context(), in, r.PathValue("key"))
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
	c, err := h.s.UpdateCard(r.Context(), id, in, r.PathValue("key"))
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
	c, err := h.s.Assign(r.Context(), id, in.Add, in.Remove, r.PathValue("key"))
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

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	res, err := h.s.Search(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *handler) repoAssignees(w http.ResponseWriter, r *http.Request) {
	users, err := h.s.RepoAssignees(r.Context(), r.PathValue("owner"), r.PathValue("repo"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// cardID reads the {id} path segment: 142, Q142, q-142, J7 (its crowning
// quest) or an issue URL-escaped as one segment.
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
