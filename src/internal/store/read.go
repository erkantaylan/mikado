package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mikado/internal/github"
)

// cardRow is a card as stored.
type cardRow struct {
	ID           int64
	Kind         string
	Ref          string
	RefKey       string
	Title        string
	Done         bool
	Owner        string
	WaitingOn    string
	Since        string
	SideOf       *int64
	FoundWhile   *int64
	Reason       string
	NPC          bool
	Removed      bool
	Cancelled    bool
	CancelReason string
	WorkingSince string // empty: nobody is on it
	WorkingBy    string
}

const cardCols = `id, kind, COALESCE(ref, ''), COALESCE(ref_key, ''), title, done, owner, waiting_on, since,
	side_of, found_while, reason, npc, removed_at IS NOT NULL,
	cancelled_at IS NOT NULL, cancel_reason, COALESCE(working_since, ''), working_by`

func scanCard(sc interface{ Scan(...any) error }) (*cardRow, error) {
	var c cardRow
	var sideOf, foundWhile sql.NullInt64
	if err := sc.Scan(&c.ID, &c.Kind, &c.Ref, &c.RefKey, &c.Title, &c.Done, &c.Owner, &c.WaitingOn, &c.Since,
		&sideOf, &foundWhile, &c.Reason, &c.NPC, &c.Removed,
		&c.Cancelled, &c.CancelReason, &c.WorkingSince, &c.WorkingBy); err != nil {
		return nil, err
	}
	if sideOf.Valid {
		c.SideOf = &sideOf.Int64
	}
	if foundWhile.Valid {
		c.FoundWhile = &foundWhile.Int64
	}
	return &c, nil
}

// getCard returns a card, removed or not.
func getCard(ctx context.Context, q querier, id int64) (*cardRow, error) {
	c, err := scanCard(q.QueryRowContext(ctx, `SELECT `+cardCols+` FROM cards WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, errf(ErrNotFound, "no quest %s", Key(id))
	}
	return c, err
}

// liveCard returns a card that has not been removed.
func liveCard(ctx context.Context, q querier, id int64) (*cardRow, error) {
	c, err := getCard(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if c.Removed {
		return nil, errf(ErrNotFound, "quest %s was struck", Key(id))
	}
	return c, nil
}

// journeyRow is a journey as stored. Final is nil unless it names a live card.
type journeyRow struct {
	ID         int64
	Title      string
	Final      *int64
	CreatedAt  string
	ArchivedAt string // empty: not archived
}

func (j *journeyRow) Key() string     { return JourneyKey(j.ID) }
func (j *journeyRow) ref() JourneyRef { return JourneyRef{Key: j.Key(), Title: j.Title} }

var journeyIDRe = regexp.MustCompile(`^(?i)j-?([1-9][0-9]*)$`)

// ParseJourneyID reads a journey key: J7, J-7 or j7. It reports false for
// anything else.
func ParseJourneyID(s string) (int64, bool) {
	m := journeyIDRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	return id, err == nil
}

// getJourney finds a journey by its key (J7).
func getJourney(ctx context.Context, q querier, key string) (*journeyRow, error) {
	id, ok := ParseJourneyID(key)
	if !ok {
		return nil, errf(ErrInvalid, "%q is not a journey (want J7)", key)
	}
	var r journeyRow
	var final sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT j.id, j.title, c.id, j.created_at, COALESCE(j.archived_at, '')
		FROM journeys j LEFT JOIN cards c ON c.id = j.final_card AND c.removed_at IS NULL
		WHERE j.id = ?`, id).Scan(&r.ID, &r.Title, &final, &r.CreatedAt, &r.ArchivedAt)
	if err == sql.ErrNoRows {
		return nil, errf(ErrNotFound, "no journey %s", JourneyKey(id))
	}
	if final.Valid {
		r.Final = &final.Int64
	}
	return &r, err
}

// graph is the whole live graph: cards, needs, side quests and journeys.
type graph struct {
	cards    map[int64]*cardRow
	next     map[int64][]int64 // from → cards it needs
	prev     map[int64][]int64 // to → cards that need it
	sides    map[int64][]int64 // card → its side quests
	needs    []Need
	journeys []*journeyRow
	// crowned maps each card that crowns a journey to those journeys (by id).
	crowned map[int64][]*journeyRow
}

func loadGraph(ctx context.Context, q querier) (*graph, error) {
	g := &graph{cards: map[int64]*cardRow{}, next: map[int64][]int64{}, prev: map[int64][]int64{}, sides: map[int64][]int64{}}
	rows, err := q.QueryContext(ctx, `SELECT `+cardCols+` FROM cards WHERE removed_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		g.cards[c.ID] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, c := range g.cards {
		if c.SideOf != nil {
			g.sides[*c.SideOf] = append(g.sides[*c.SideOf], c.ID)
		}
	}
	for _, s := range g.sides {
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	}
	rows, err = q.QueryContext(ctx, `SELECT from_card, to_card FROM needs ORDER BY from_card, to_card`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var n Need
		if err := rows.Scan(&n.From, &n.To); err != nil {
			rows.Close()
			return nil, err
		}
		g.needs = append(g.needs, n)
		g.next[n.From] = append(g.next[n.From], n.To)
		g.prev[n.To] = append(g.prev[n.To], n.From)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = q.QueryContext(ctx, `SELECT id, title, final_card, created_at, COALESCE(archived_at, '') FROM journeys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r journeyRow
		var final sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Title, &final, &r.CreatedAt, &r.ArchivedAt); err != nil {
			return nil, err
		}
		if final.Valid && g.cards[final.Int64] != nil {
			r.Final = &final.Int64
		}
		g.journeys = append(g.journeys, &r)
	}
	g.crowned = map[int64][]*journeyRow{}
	for _, j := range g.journeys {
		if j.Final != nil {
			g.crowned[*j.Final] = append(g.crowned[*j.Final], j)
		}
	}
	return g, rows.Err()
}

// closure returns the start cards, everything they transitively need, and
// the side quests (recursively) of all of those, ascending by id. It is what
// a card's status depends on, so it never stops at another journey.
func (g *graph) closure(start ...int64) []int64 {
	return g.reach(start, func(int64) bool { return true })
}

// reach is closure, except that only the cards expand says yes to are
// followed further (the others are included, not expanded).
func (g *graph) reach(start []int64, expand func(id int64) bool) []int64 {
	seen := map[int64]bool{}
	var stack []int64
	for _, id := range start {
		if g.cards[id] != nil && !seen[id] {
			seen[id] = true
			stack = append(stack, id)
		}
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !expand(id) {
			continue
		}
		for _, n := range g.next[id] {
			if !seen[n] {
				seen[n] = true
				stack = append(stack, n)
			}
		}
		// Side quests take no part in needs, so adding them never adds needs.
		for _, sq := range g.sides[id] {
			if !seen[sq] {
				seen[sq] = true
				stack = append(stack, sq)
			}
		}
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// members are a journey's cards: its final card, everything that transitively
// needs, and their side quests. A journey without a final has none.
//
// A card that crowns another journey stands for that whole journey: it is a
// member, but what it needs and its side quests are that journey's, so they
// are not reached through it (anything reachable by another route still is).
// The journey's own final is always followed, even when it also crowns another.
func (g *graph) members(q *journeyRow) []int64 {
	if q.Final == nil {
		return nil
	}
	return g.reach([]int64{*q.Final}, func(id int64) bool { return id == *q.Final || !g.folds(id, q) })
}

// folds reports whether card id, seen on journey q, stands for another
// journey: it crowns a journey and is not q's own final.
func (g *graph) folds(id int64, q *journeyRow) bool {
	return len(g.crowned[id]) > 0 && (q.Final == nil || *q.Final != id)
}

// membership maps each card to the journeys it is a member of.
func (g *graph) membership() map[int64][]*journeyRow {
	out := map[int64][]*journeyRow{}
	for _, j := range g.journeys {
		for _, id := range g.members(j) {
			out[id] = append(out[id], j)
		}
	}
	return out
}

func (g *graph) journey(key string) (*journeyRow, error) {
	id, ok := ParseJourneyID(key)
	if !ok {
		return nil, errf(ErrInvalid, "%q is not a journey (want J7)", key)
	}
	for _, j := range g.journeys {
		if j.ID == id {
			return j, nil
		}
	}
	return nil, errf(ErrNotFound, "no journey %s", JourneyKey(id))
}

func (g *graph) rows(ids []int64) []*cardRow {
	out := make([]*cardRow, 0, len(ids))
	for _, id := range ids {
		if c := g.cards[id]; c != nil {
			out = append(out, c)
		}
	}
	return out
}

// isFinal reports whether a card is the final of any journey.
func (g *graph) isFinal(id int64) bool { return len(g.crowned[id]) > 0 }

var cardIDRe = regexp.MustCompile(`^(?i)(?:q-?)?([1-9][0-9]*)$`)

// ParseCardID reads a quest key in any accepted form: Q142, Q-142, q142 or
// 142. It reports false for anything else (such as an issue ref).
func ParseCardID(s string) (int64, bool) {
	m := cardIDRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	return id, err == nil
}

// Resolve turns a card reference (a quest key in any form, an issue ref, an
// issue URL, or a journey key for that journey's crowning quest) into the id
// of a live card.
func (s *Store) Resolve(ctx context.Context, ref string) (int64, error) {
	if _, ok := ParseJourneyID(ref); !ok {
		return s.resolveQuest(ctx, ref)
	}
	j, err := getJourney(ctx, s.db, ref)
	if err != nil {
		return 0, err
	}
	if j.Final == nil {
		return 0, errf(ErrInvalid, "journey %s has no crowning quest yet, so it names no quest — crown one with `mikado journey crown %s Q`", j.Key(), j.Key())
	}
	return *j.Final, nil
}

// resolveQuest is Resolve for the quest forms only: a key, an issue ref or
// an issue URL. Anything else is ErrInvalid.
func (s *Store) resolveQuest(ctx context.Context, ref string) (int64, error) {
	if id, ok := ParseCardID(ref); ok {
		if _, err := liveCard(ctx, s.db, id); err != nil {
			return 0, err
		}
		return id, nil
	}
	r, err := github.ParseRef(ref)
	if err != nil {
		return 0, errf(ErrInvalid, "%q is not a quest (want Q142, owner/repo#n or a journey key like J7)", ref)
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `SELECT id FROM cards WHERE ref_key = ? AND removed_at IS NULL`, r.Key()).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, errf(ErrNotFound, "%s is not on any chart yet (add it with `mikado add %s`)", r, r)
	}
	return id, err
}

// cached is what mikado knows about an issue from GitHub.
type cached struct {
	Ref         string
	Title       string
	State       string
	StateReason string
	Assignees   []string
	URL         string
	FetchedAt   time.Time
}

func loadCache(ctx context.Context, q querier, keys []string) (map[string]cached, error) {
	out := map[string]cached{}
	if len(keys) == 0 {
		return out, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	args := make([]any, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	rows, err := q.QueryContext(ctx, `SELECT ref_key, ref, title, state, state_reason, assignees, url, fetched_at FROM github_cache WHERE ref_key IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, assignees, at string
		var c cached
		if err := rows.Scan(&key, &c.Ref, &c.Title, &c.State, &c.StateReason, &assignees, &c.URL, &at); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(assignees), &c.Assignees)
		c.FetchedAt, _ = time.Parse(time.RFC3339, at)
		out[key] = c
	}
	return out, rows.Err()
}

func (s *Store) saveCache(ctx context.Context, q querier, is github.Issue) error {
	assignees, _ := json.Marshal(is.Assignees)
	_, err := q.ExecContext(ctx, `INSERT INTO github_cache (ref_key, ref, title, state, state_reason, assignees, url, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (ref_key) DO UPDATE SET ref = excluded.ref, title = excluded.title, state = excluded.state,
			state_reason = excluded.state_reason, assignees = excluded.assignees, url = excluded.url, fetched_at = excluded.fetched_at`,
		is.Ref.Key(), is.Ref.String(), is.Title, is.State, is.StateReason, string(assignees), is.URL, s.stamp())
	return err
}

// issueData returns GitHub data for the issue cards among cards, refreshing
// stale or missing entries in one batch. When GitHub cannot be reached the
// cached data is returned along with a warning.
func (s *Store) issueData(ctx context.Context, cards []*cardRow) (map[string]cached, string, error) {
	refs := map[string]string{}
	var keys []string
	for _, c := range cards {
		if c.Kind == KindIssue && refs[c.RefKey] == "" {
			refs[c.RefKey] = c.Ref
			keys = append(keys, c.RefKey)
		}
	}
	data, err := loadCache(ctx, s.db, keys)
	if err != nil {
		return nil, "", err
	}
	var stale []github.Ref
	for _, k := range keys {
		if c, ok := data[k]; !ok || s.now().Sub(c.FetchedAt) >= CacheTTL {
			if r, err := github.ParseRef(refs[k]); err == nil {
				stale = append(stale, r)
			}
		}
	}
	if len(stale) == 0 || s.gh == nil {
		return data, "", nil
	}
	fresh, err := s.gh.Issues(ctx, stale)
	if err != nil {
		return data, fmt.Sprintf("GitHub could not be reached, showing cached data: %v", err), nil
	}
	var missing []string
	for _, r := range stale {
		is, ok := fresh[r.Key()]
		if !ok {
			missing = append(missing, r.String())
			continue
		}
		if err := s.saveCache(ctx, s.db, is); err != nil {
			return nil, "", err
		}
		data[r.Key()] = cached{Ref: is.Ref.String(), Title: is.Title, State: is.State, StateReason: is.StateReason,
			Assignees: is.Assignees, URL: is.URL, FetchedAt: s.now()}
	}
	warning := ""
	if len(missing) > 0 {
		warning = "not found on GitHub (deleted, transferred or no longer visible): " + strings.Join(missing, ", ")
	}
	return data, warning, nil
}

// buildCards turns stored cards into API cards and computes their status
// from the needs among them. Final and AlsoIn are left to the caller.
func buildCards(rows []*cardRow, needs []Need, gh map[string]cached) []Card {
	cards := make([]Card, 0, len(rows))
	for _, r := range rows {
		c := Card{
			ID: r.ID, Key: Key(r.ID), Kind: r.Kind, Title: r.Title, Done: r.Done, Assignees: []string{},
			Owner: r.Owner, SideOf: r.SideOf, FoundWhile: r.FoundWhile, Reason: r.Reason, NPC: r.NPC,
			Cancelled: r.Cancelled, CancelReason: r.CancelReason,
			Working: r.WorkingSince != "", WorkingSince: r.WorkingSince, WorkingBy: r.WorkingBy,
			AlsoIn: []JourneyRef{},
		}
		switch r.Kind {
		case KindIssue:
			c.Ref = r.Ref
			c.Done = false
			if d, ok := gh[r.RefKey]; ok {
				c.Ref, c.Title, c.State, c.URL, c.StateReason = d.Ref, d.Title, d.State, d.URL, d.StateReason
				// Closed as not planned or as a duplicate is GitHub's "won't do".
				if d.State == "closed" && closedAsCancelled(d.StateReason) {
					c.Cancelled = true
				} else {
					c.Done = d.State == "closed"
				}
				if d.Assignees != nil {
					c.Assignees = d.Assignees
				}
			} else {
				c.Title = r.Ref // nothing known yet
			}
		case KindAwaiting:
			c.WaitingOn, c.Since = r.WaitingOn, r.Since
		}
		cards = append(cards, c)
	}
	computeStatus(cards, needs)
	return cards
}

func closedAsCancelled(stateReason string) bool {
	return stateReason == "NOT_PLANNED" || stateReason == "DUPLICATE"
}

// computeStatus fills Status and OpenBefore from Done, Cancelled and the
// needs: cancelled, else done, else locked while anything it needs is neither
// done nor cancelled, else awaiting for awaiting cards and available for the
// rest. A card that is done or cancelled is never shown as being worked on.
func computeStatus(cards []Card, needs []Need) {
	settled := make(map[int64]bool, len(cards))
	for _, c := range cards {
		settled[c.ID] = c.Done || c.Cancelled
	}
	open := map[int64]int{}
	for _, n := range needs {
		if s, ok := settled[n.To]; ok && !s {
			open[n.From]++
		}
	}
	for i := range cards {
		c := &cards[i]
		c.OpenBefore = open[c.ID]
		if c.Done || c.Cancelled {
			c.Working, c.WorkingSince, c.WorkingBy = false, "", ""
		}
		switch {
		case c.Cancelled:
			c.Status = StatusCancelled
		case c.Done:
			c.Status = StatusDone
		case c.OpenBefore > 0:
			c.Status = StatusLocked
		case c.Kind == KindAwaiting:
			c.Status = StatusAwaiting
		default:
			c.Status = StatusAvailable
		}
	}
}

// needsAmong returns the needs with both ends in ids.
func (g *graph) needsAmong(ids []int64) []Need {
	in := map[int64]bool{}
	for _, id := range ids {
		in[id] = true
	}
	out := []Need{}
	for _, n := range g.needs {
		if in[n.From] && in[n.To] {
			out = append(out, n)
		}
	}
	return out
}

// view builds the cards ids (whose status needs everything they need, so
// callers pass a needs-closed set), refreshing GitHub data for them.
func (s *Store) view(ctx context.Context, g *graph, ids []int64) (map[int64]Card, string, error) {
	rows := g.rows(ids)
	gh, warning, err := s.issueData(ctx, rows)
	if err != nil {
		return nil, "", err
	}
	out := map[int64]Card{}
	for _, c := range buildCards(rows, g.needsAmong(ids), gh) {
		out[c.ID] = c
	}
	return out, warning, nil
}

// alsoIn lists the journeys of a card other than the one being viewed.
func alsoIn(journeys []*journeyRow, viewing *journeyRow) []JourneyRef {
	out := []JourneyRef{}
	for _, j := range journeys {
		if viewing == nil || j.ID != viewing.ID {
			out = append(out, j.ref())
		}
	}
	return out
}

// Journey returns the full view of one journey: its members, the needs among
// them, and its log.
func (s *Store) Journey(ctx context.Context, key string) (*JourneyView, error) {
	g, err := loadGraph(ctx, s.db)
	if err != nil {
		return nil, err
	}
	j, err := g.journey(key)
	if err != nil {
		return nil, err
	}
	ids := g.members(j)
	// A folded journey card's status (and its journey's progress) needs what lies behind it too.
	built, warning, err := s.view(ctx, g, g.closure(ids...))
	if err != nil {
		return nil, err
	}
	in := g.membership()
	v := &JourneyView{
		Journey: JourneyInfo{Key: j.Key(), Title: j.Title, FinalCardID: j.Final, State: JourneyActive, ArchivedAt: j.ArchivedAt},
		Cards:   make([]Card, 0, len(ids)),
		Needs:   g.needsAmong(ids),
		GitHub:  warning,
	}
	for _, id := range ids {
		c := built[id]
		c.Final = j.Final != nil && *j.Final == id
		c.AlsoIn = alsoIn(in[id], j)
		if c.Final {
			v.Journey.State = journeyState(&c)
		}
		if g.folds(id, j) {
			c.Crowns = g.crowns(id, j, built)
		}
		v.Cards = append(v.Cards, c)
	}
	v.Log, err = journeyLog(ctx, s.db, j.ID, ids)
	return v, err
}

// journeyLog is the journey's own events plus the events of its current
// members, oldest first.
func journeyLog(ctx context.Context, q querier, journeyID int64, members []int64) ([]Event, error) {
	query := `SELECT id, at, kind, card_id, text FROM events WHERE journey_id = ?`
	args := []any{journeyID}
	if len(members) > 0 {
		query += ` OR (journey_id IS NULL AND card_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(members)), ",") + `))`
		for _, id := range members {
			args = append(args, id)
		}
	}
	rows, err := q.QueryContext(ctx, query+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var card sql.NullInt64
		if err := rows.Scan(&e.ID, &e.At, &e.Kind, &card, &e.Text); err != nil {
			return nil, err
		}
		if card.Valid {
			e.CardID = &card.Int64
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CardView returns one live card seen globally.
func (s *Store) CardView(ctx context.Context, id int64) (*CardView, error) {
	g, err := loadGraph(ctx, s.db)
	if err != nil {
		return nil, err
	}
	if g.cards[id] == nil {
		if _, err := liveCard(ctx, s.db, id); err != nil {
			return nil, err
		}
	}
	// Everything shown, plus what those need, so every status is right.
	start := append([]int64{id}, g.prev[id]...)
	built, warning, err := s.view(ctx, g, g.closure(start...))
	if err != nil {
		return nil, err
	}
	in := g.membership()
	card := func(cid int64) Card {
		c := built[cid]
		c.Final = g.isFinal(cid)
		c.AlsoIn = alsoIn(in[cid], nil)
		return c
	}
	v := &CardView{Card: card(id), Journeys: alsoIn(in[id], nil), Needs: []Card{}, NeededBy: []Card{}, SideQuests: []Card{}, GitHub: warning}
	v.Card.Crowns = g.crowns(id, nil, built)
	for _, n := range g.next[id] {
		v.Needs = append(v.Needs, card(n))
	}
	for _, n := range g.prev[id] {
		v.NeededBy = append(v.NeededBy, card(n))
	}
	for _, n := range g.sides[id] {
		v.SideQuests = append(v.SideQuests, card(n))
	}
	return v, nil
}

// card returns one live card as the API shows it. When viewing names a
// journey (J7), Final means "final of that journey" and AlsoIn leaves that
// journey out; otherwise Final means "final of any journey" and AlsoIn lists
// every journey.
func (s *Store) card(ctx context.Context, id int64, viewing string) (*Card, error) {
	v, err := s.CardView(ctx, id)
	if err != nil {
		return nil, err
	}
	c := v.Card
	if viewing != "" {
		j, err := getJourney(ctx, s.db, viewing)
		if err != nil {
			return nil, err
		}
		c.Final = j.Final != nil && *j.Final == id
		if c.Final {
			c.Crowns = nil // the viewed journey's own crowning quest stands for no other journey
		}
		c.AlsoIn = []JourneyRef{}
		for _, r := range v.Journeys {
			if r.Key != j.Key() {
				c.AlsoIn = append(c.AlsoIn, r)
			}
		}
	}
	return &c, nil
}

// Journeys returns the atlas. The warning is non-empty when GitHub data is
// stale.
func (s *Store) Journeys(ctx context.Context) ([]JourneySummary, string, error) {
	g, err := loadGraph(ctx, s.db)
	if err != nil {
		return nil, "", err
	}
	members := map[int64][]int64{}
	var all []int64
	for _, j := range g.journeys {
		members[j.ID] = g.members(j)
		all = append(all, members[j.ID]...)
	}
	built, warning, err := s.view(ctx, g, g.closure(all...))
	if err != nil {
		return nil, "", err
	}
	last, err := lastActivity(ctx, s.db)
	if err != nil {
		return nil, "", err
	}
	out := make([]JourneySummary, 0, len(g.journeys))
	for _, j := range g.journeys {
		cards := make([]Card, 0, len(members[j.ID]))
		at := j.CreatedAt
		if t := last.journey[j.ID]; t > at {
			at = t
		}
		for _, id := range members[j.ID] {
			c := built[id]
			c.Final = j.Final != nil && *j.Final == id
			cards = append(cards, c)
			if t := last.card[id]; t > at {
				at = t
			}
		}
		out = append(out, summarize(JourneySummary{Key: j.Key(), Title: j.Title, LastActivity: at, ArchivedAt: j.ArchivedAt,
			BlockedBy: []JourneyLink{}, Blocks: []JourneyLink{}}, cards))
	}
	// A journey whose crowning quest is folded into another's chart blocks it.
	index := map[int64]int{}
	for i, j := range g.journeys {
		index[j.ID] = i
	}
	for i, j := range g.journeys {
		for _, id := range members[j.ID] {
			if !g.folds(id, j) {
				continue
			}
			for _, other := range g.crowned[id] {
				out[i].BlockedBy = append(out[i].BlockedBy, g.link(other, built))
				k := index[other.ID]
				out[k].Blocks = append(out[k].Blocks, g.link(j, built))
			}
		}
	}
	return out, warning, nil
}

// link names a journey with its state, from its final card among built.
func (g *graph) link(j *journeyRow, built map[int64]Card) JourneyLink {
	l := JourneyLink{Key: j.Key(), Title: j.Title, State: JourneyActive, ArchivedAt: j.ArchivedAt}
	if j.Final != nil {
		if c, ok := built[*j.Final]; ok {
			l.State = journeyState(&c)
		}
	}
	return l
}

// crowns describes the journey card id stands for, seen from journey viewing
// (nil: seen globally): the first journey it crowns other than viewing, with
// that journey's progress counted as the atlas counts it. built must hold the
// closure of id. Nil when id crowns no such journey.
func (g *graph) crowns(id int64, viewing *journeyRow, built map[int64]Card) *Crowns {
	var j *journeyRow
	for _, c := range g.crowned[id] {
		if viewing == nil || c.ID != viewing.ID {
			j = c
			break
		}
	}
	if j == nil {
		return nil
	}
	ids := g.members(j)
	cards := make([]Card, 0, len(ids))
	open := []OpenQuest{}
	for _, mid := range ids {
		c := built[mid]
		c.Final = mid == *j.Final
		cards = append(cards, c)
		if c.SideOf == nil && !c.Done && !c.Cancelled {
			open = append(open, OpenQuest{Key: c.Key, Title: c.Title, Status: c.Status, Working: c.Working})
		}
	}
	sum := summarize(JourneySummary{}, cards)
	return &Crowns{Key: j.Key(), Title: j.Title, State: sum.State, ArchivedAt: j.ArchivedAt,
		Done: sum.Main.Done, Total: sum.Main.Total, Working: sum.InProgress, Open: open}
}

type lastEvents struct{ journey, card map[int64]string }

func lastActivity(ctx context.Context, q querier) (lastEvents, error) {
	l := lastEvents{journey: map[int64]string{}, card: map[int64]string{}}
	rows, err := q.QueryContext(ctx, `SELECT journey_id, card_id, MAX(at) FROM events GROUP BY journey_id, card_id`)
	if err != nil {
		return l, err
	}
	defer rows.Close()
	for rows.Next() {
		var journey, card sql.NullInt64
		var at string
		if err := rows.Scan(&journey, &card, &at); err != nil {
			return l, err
		}
		switch {
		case journey.Valid:
			if at > l.journey[journey.Int64] {
				l.journey[journey.Int64] = at
			}
		case card.Valid:
			if at > l.card[card.Int64] {
				l.card[card.Int64] = at
			}
		}
	}
	return l, rows.Err()
}

// journeyState is what the final card says about the journey.
func journeyState(final *Card) string {
	switch final.Status {
	case StatusCancelled:
		return JourneyCancelled
	case StatusDone:
		return JourneyComplete
	}
	return JourneyActive
}

// summarize counts a journey's cards. Cancelled cards count only in Cancelled
// (main and side quests alike), never in progress totals.
func summarize(s JourneySummary, cards []Card) JourneySummary {
	heroes, repos := map[string]bool{}, map[string]bool{}
	s.State = JourneyActive
	for _, c := range cards {
		if c.Final {
			s.State = journeyState(&c)
		}
		if c.Ref != "" {
			if r, err := github.ParseRef(c.Ref); err == nil {
				repos[r.Repository()] = true
			}
		}
		if c.Cancelled {
			s.Cancelled++
			continue
		}
		if c.Working {
			s.InProgress++
		}
		p := &s.Main
		if c.SideOf != nil {
			p = &s.Achievements
		}
		p.Total++
		if c.Done {
			p.Done++
		}
		if c.SideOf == nil {
			switch c.Status {
			case StatusAvailable:
				s.Available++
			case StatusAwaiting:
				s.Awaiting++
			}
		}
		if !c.Done {
			for _, a := range c.Assignees {
				heroes[a] = true
			}
			if c.Owner != "" {
				heroes[c.Owner] = true
			}
		}
	}
	s.Heroes, s.Repos = sortedKeys(heroes), sortedKeys(repos)
	return s
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// RepoAssignees lists who can be assigned to issues in owner/repo.
func (s *Store) RepoAssignees(ctx context.Context, owner, repo string) ([]string, error) {
	if !github.ValidRepo(owner, repo) {
		return nil, errf(ErrInvalid, "%s/%s is not a repository name", owner, repo)
	}
	if s.gh == nil {
		return nil, errf(ErrUpstream, "GitHub is not configured")
	}
	users, err := s.gh.Assignees(ctx, owner, repo)
	if err != nil {
		return nil, errf(ErrUpstream, "%v", err)
	}
	return users, nil
}

// CheckJourney reports whether a journey exists.
func (s *Store) CheckJourney(ctx context.Context, key string) error {
	_, err := getJourney(ctx, s.db, key)
	return err
}
