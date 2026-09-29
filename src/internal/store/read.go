package store

import (
	"context"
	"database/sql"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// cardRow is a card as stored.
type cardRow struct {
	ID           int64
	Title        string
	URL          string
	Mark         string
	Done         bool
	Owner        string
	WaitingOn    string // set: a petition
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

const cardCols = `id, title, url, mark, done, owner, waiting_on, since,
	side_of, found_while, reason, npc, removed_at IS NOT NULL,
	cancelled_at IS NOT NULL, cancel_reason, COALESCE(working_since, ''), working_by`

func scanCard(sc interface{ Scan(...any) error }) (*cardRow, error) {
	var c cardRow
	var sideOf, foundWhile sql.NullInt64
	if err := sc.Scan(&c.ID, &c.Title, &c.URL, &c.Mark, &c.Done, &c.Owner, &c.WaitingOn, &c.Since,
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
	Region     int64
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
	err := q.QueryRowContext(ctx, `SELECT j.id, j.title, c.id, j.created_at, COALESCE(j.archived_at, ''), j.region_id
		FROM journeys j LEFT JOIN cards c ON c.id = j.final_card AND c.removed_at IS NULL
		WHERE j.id = ?`, id).Scan(&r.ID, &r.Title, &final, &r.CreatedAt, &r.ArchivedAt, &r.Region)
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
	regions  map[int64]*regionRow
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
	if g.regions, err = loadRegions(ctx, q); err != nil {
		return nil, err
	}
	rows, err = q.QueryContext(ctx, `SELECT id, title, final_card, created_at, COALESCE(archived_at, ''), region_id FROM journeys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r journeyRow
		var final sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Title, &final, &r.CreatedAt, &r.ArchivedAt, &r.Region); err != nil {
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
// 142. It reports false for anything else.
func ParseCardID(s string) (int64, bool) {
	m := cardIDRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	return id, err == nil
}

// Resolve turns a card reference (a quest key in any form, or a journey key
// for that journey's crowning quest) into the id of a live card.
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

// resolveQuest is Resolve for a quest key alone. Anything else is ErrInvalid.
func (s *Store) resolveQuest(ctx context.Context, ref string) (int64, error) {
	id, ok := ParseCardID(ref)
	if !ok {
		return 0, errf(ErrInvalid, "%q is not a quest (want Q142, or a journey key like J7)", ref)
	}
	if _, err := liveCard(ctx, s.db, id); err != nil {
		return 0, err
	}
	return id, nil
}

// buildCards turns stored cards into API cards and computes their status
// from the needs among them. Final and AlsoIn are left to the caller.
func buildCards(rows []*cardRow, needs []Need) []Card {
	cards := make([]Card, 0, len(rows))
	for _, r := range rows {
		c := Card{
			ID: r.ID, Key: Key(r.ID), Title: r.Title, URL: r.URL, Mark: r.Mark, Done: r.Done,
			Owner: r.Owner, SideOf: r.SideOf, FoundWhile: r.FoundWhile, Reason: r.Reason, NPC: r.NPC,
			Cancelled: r.Cancelled, CancelReason: r.CancelReason,
			Working: r.WorkingSince != "", WorkingSince: r.WorkingSince, WorkingBy: r.WorkingBy,
			AlsoIn: []JourneyRef{},
		}
		if r.WaitingOn != "" {
			c.Kind, c.WaitingOn, c.Since = KindAwaiting, r.WaitingOn, r.Since
		}
		cards = append(cards, c)
	}
	computeStatus(cards, needs)
	return cards
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
// callers pass a needs-closed set).
func (g *graph) view(ids []int64) map[int64]Card {
	out := map[int64]Card{}
	for _, c := range buildCards(g.rows(ids), g.needsAmong(ids)) {
		out[c.ID] = c
	}
	return out
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
	built := g.view(g.closure(ids...))
	in := g.membership()
	v := &JourneyView{
		Journey: JourneyInfo{Key: j.Key(), Title: j.Title, FinalCardID: j.Final, State: JourneyActive, ArchivedAt: j.ArchivedAt, Region: g.regionRef(j.Region)},
		Cards:   make([]Card, 0, len(ids)),
		Needs:   g.needsAmong(ids),
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
	built := g.view(g.closure(start...))
	in := g.membership()
	card := func(cid int64) Card {
		c := built[cid]
		c.Final = g.isFinal(cid)
		c.AlsoIn = alsoIn(in[cid], nil)
		return c
	}
	v := &CardView{Card: card(id), Journeys: alsoIn(in[id], nil), Needs: []Card{}, NeededBy: []Card{}, SideQuests: []Card{}}
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

// Journeys returns the atlas.
func (s *Store) Journeys(ctx context.Context) ([]JourneySummary, error) {
	g, err := loadGraph(ctx, s.db)
	if err != nil {
		return nil, err
	}
	members := map[int64][]int64{}
	var all []int64
	for _, j := range g.journeys {
		members[j.ID] = g.members(j)
		all = append(all, members[j.ID]...)
	}
	built := g.view(g.closure(all...))
	last, err := lastActivity(ctx, s.db)
	if err != nil {
		return nil, err
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
		out = append(out, summarize(JourneySummary{Key: j.Key(), Title: j.Title, Region: g.regionRef(j.Region), LastActivity: at, ArchivedAt: j.ArchivedAt,
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
	return out, nil
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
	heroes := map[string]bool{}
	s.State = JourneyActive
	for _, c := range cards {
		if c.Final {
			s.State = journeyState(&c)
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
		if !c.Done && c.Owner != "" {
			heroes[c.Owner] = true
		}
	}
	s.Heroes = sortedKeys(heroes)
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

// CheckJourney reports whether a journey exists.
func (s *Store) CheckJourney(ctx context.Context, key string) error {
	_, err := getJourney(ctx, s.db, key)
	return err
}
