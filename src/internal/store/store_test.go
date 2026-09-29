package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	t   *testing.T
	s   *Store
	now time.Time
	ctx context.Context
	// keys maps the short names tests give journeys to their keys (J3), and
	// names back.
	keys  map[string]string
	names map[string]string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, now: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC), ctx: context.Background(),
		keys: map[string]string{}, names: map[string]string{}}
	s, err := Open(filepath.Join(t.TempDir(), "mikado.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.now = func() time.Time { return f.now }
	f.s = s
	return f
}

// journey creates a journey titled "Journey name", known to the test as name.
func (f *fixture) journey(name string, final *Card) {
	f.t.Helper()
	var id *int64
	if final != nil {
		id = &final.ID
	}
	j, err := f.s.CreateJourney(f.ctx, "Journey "+name, id, "")
	if err != nil {
		f.t.Fatal(err)
	}
	f.keys[name], f.names[j.Key] = j.Key, name
}

// key is the key of the journey the test calls name.
func (f *fixture) key(name string) string {
	f.t.Helper()
	k, ok := f.keys[name]
	if !ok {
		f.t.Fatalf("no journey %q in this test", name)
	}
	return k
}

func (f *fixture) add(in NewCard) *Card {
	f.t.Helper()
	c, err := f.s.AddCard(f.ctx, in, "")
	if err != nil {
		f.t.Fatalf("add %+v: %v", in, err)
	}
	return c
}

func (f *fixture) quest(title string, needs ...int64) *Card {
	return f.add(NewCard{Title: title, Needs: needs})
}

// card returns a card seen globally.
func (f *fixture) card(id int64) Card {
	f.t.Helper()
	v, err := f.s.CardView(f.ctx, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return v.Card
}

func (f *fixture) view(name string) *JourneyView {
	f.t.Helper()
	v, err := f.s.Journey(f.ctx, f.key(name))
	if err != nil {
		f.t.Fatal(err)
	}
	return v
}

// members returns the journey's card ids.
func (f *fixture) members(name string) []int64 {
	var ids []int64
	for _, c := range f.view(name).Cards {
		ids = append(ids, c.ID)
	}
	return ids
}

func (f *fixture) summary(name string) JourneySummary {
	f.t.Helper()
	js, err := f.s.Journeys(f.ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, j := range js {
		if j.Key == f.key(name) {
			return j
		}
	}
	f.t.Fatalf("journey %s missing from summaries", name)
	return JourneySummary{}
}

func (f *fixture) patch(id int64, p CardPatch) *Card {
	f.t.Helper()
	c, err := f.s.UpdateCard(f.ctx, id, p, "")
	if err != nil {
		f.t.Fatalf("patch %s: %v", Key(id), err)
	}
	return c
}

func (f *fixture) cancel(id int64, reason string) *Card {
	return f.patch(id, CardPatch{Cancelled: ptr(true), CancelReason: ptr(reason)})
}

func (f *fixture) need(from, to int64) {
	f.t.Helper()
	if _, err := f.s.AddNeed(f.ctx, from, to); err != nil {
		f.t.Fatal(err)
	}
}

func ptr[T any](v T) *T { return &v }

// namesOf lists the test's names for the journeys rs refers to.
func (f *fixture) namesOf(rs []JourneyRef) string {
	var out []string
	for _, r := range rs {
		out = append(out, f.names[r.Key])
	}
	return strings.Join(out, ",")
}

func TestParseCardID(t *testing.T) {
	for _, in := range []string{"Q142", "Q-142", "q142", "q-142", " 142 "} {
		if id, ok := ParseCardID(in); !ok || id != 142 {
			t.Errorf("ParseCardID(%q) = %d, %v", in, id, ok)
		}
	}
	for _, in := range []string{"", "Q", "Q0", "M142", "c142", "J142", "x142", "studio/game#142", "Q14a", "--142"} {
		if id, ok := ParseCardID(in); ok {
			t.Errorf("ParseCardID(%q) accepted as %d", in, id)
		}
	}
	if Key(142) != "Q142" {
		t.Errorf("Key = %s", Key(142))
	}
}

func TestParseJourneyID(t *testing.T) {
	for _, in := range []string{"J7", "J-7", "j7", "j-7", " J7 "} {
		if id, ok := ParseJourneyID(in); !ok || id != 7 {
			t.Errorf("ParseJourneyID(%q) = %d, %v", in, id, ok)
		}
	}
	for _, in := range []string{"", "J", "J0", "7", "Q7", "M7", "J7a", "winter-update"} {
		if id, ok := ParseJourneyID(in); ok {
			t.Errorf("ParseJourneyID(%q) accepted as %d", in, id)
		}
	}
	if JourneyKey(7) != "J7" {
		t.Errorf("JourneyKey = %s", JourneyKey(7))
	}
}

func TestResolve(t *testing.T) {
	f := setup(t)
	c := f.quest("t")
	for _, ref := range []string{Key(c.ID), fmt.Sprintf("q-%d", c.ID), fmt.Sprint(c.ID)} {
		if id, err := f.s.Resolve(f.ctx, ref); err != nil || id != c.ID {
			t.Errorf("Resolve(%q) = %d, %v", ref, id, err)
		}
	}
	for _, ref := range []string{"studio/game#7", "https://github.com/studio/game/issues/7", "#7"} {
		if _, err := f.s.Resolve(f.ctx, ref); KindOf(err) != ErrInvalid {
			t.Errorf("Resolve(%q): %v", ref, err)
		}
	}
	if _, err := f.s.Resolve(f.ctx, "Q999"); KindOf(err) != ErrNotFound {
		t.Errorf("unknown id: %v", err)
	}
}

func TestJourneys(t *testing.T) {
	f := setup(t)
	j, err := f.s.CreateJourney(f.ctx, "  The winter update  ", nil, "")
	if err != nil || j.Key != "J1" || j.Title != "The winter update" || j.State != JourneyActive {
		t.Fatalf("create: %+v %v", j, err)
	}
	if _, err := f.s.CreateJourney(f.ctx, " ", nil, ""); KindOf(err) != ErrInvalid {
		t.Errorf("no title: %v", err)
	}
	if v, err := f.s.Journey(f.ctx, "j-1"); err != nil || v.Journey.Key != "J1" {
		t.Errorf("lookup by j-1: %+v %v", v, err)
	}
	for _, key := range []string{"J2", "winter-update"} {
		if _, err := f.s.Journey(f.ctx, key); KindOf(err) == 0 {
			t.Errorf("Journey(%q): %v", key, err)
		}
	}
	r, err := f.s.UpdateJourney(f.ctx, "J1", JourneyPatch{Title: ptr("Winter update")})
	if err != nil || r.Key != "J1" || r.Title != "Winter update" {
		t.Fatalf("retitle: %+v %v", r, err)
	}
	if _, err := f.s.UpdateJourney(f.ctx, "J1", JourneyPatch{Title: ptr("")}); KindOf(err) != ErrInvalid {
		t.Errorf("retitle to nothing: %v", err)
	}
	f.keys["w"], f.names["J1"] = "J1", "w"
	log := f.view("w").Log
	if len(log) != 2 || log[0].Text != "journey created" || log[1].Text != "journey retitled from “The winter update”" {
		t.Errorf("events: %+v", log)
	}
}

func TestStatus(t *testing.T) {
	f := setup(t)
	adapter := f.quest("Cloud-save adapter")
	f.patch(adapter.ID, CardPatch{Done: ptr(true)})
	migration := f.add(NewCard{Title: "Save migration", Needs: []int64{adapter.ID}})
	token := f.add(NewCard{Title: "Final key art", WaitingOn: "freelance artist", Owner: "ada"})
	release := f.quest("Book feature slot")
	final := f.quest("Ship it", migration.ID, token.ID, release.ID)
	f.journey("winter", final)

	want := map[int64]string{adapter.ID: StatusDone, migration.ID: StatusAvailable, token.ID: StatusAwaiting,
		release.ID: StatusAvailable, final.ID: StatusLocked}
	v := f.view("winter")
	if len(v.Cards) != 5 {
		t.Fatalf("members: %d", len(v.Cards))
	}
	for _, c := range v.Cards {
		if c.Status != want[c.ID] {
			t.Errorf("%s status = %s, want %s", c.Key, c.Status, want[c.ID])
		}
		if c.Key != Key(c.ID) {
			t.Errorf("key %q for %d", c.Key, c.ID)
		}
	}
	if c := f.card(final.ID); c.OpenBefore != 3 || !c.Final {
		t.Errorf("final: openBefore %d final %v", c.OpenBefore, c.Final)
	}
	if v.Journey.FinalCardID == nil || *v.Journey.FinalCardID != final.ID {
		t.Errorf("finalCardId = %v", v.Journey.FinalCardID)
	}
	for _, id := range []int64{token.ID, release.ID, migration.ID} {
		f.patch(id, CardPatch{Done: ptr(true)})
	}
	if c := f.card(final.ID); c.Status != StatusAvailable || c.OpenBefore != 0 {
		t.Errorf("final after prerequisites done: %s/%d", c.Status, c.OpenBefore)
	}
}

func TestSharedQuestOneCardTwoJourneys(t *testing.T) {
	f := setup(t)
	shared := f.quest("Save migration")
	a := f.quest("ship A", shared.ID)
	b := f.quest("ship B", shared.ID)
	f.journey("qa", a)
	f.journey("qb", b)
	for name, other := range map[string]string{"qa": "qb", "qb": "qa"} {
		var found *Card
		for _, c := range f.view(name).Cards {
			if c.ID == shared.ID {
				found = &c
			}
		}
		if found == nil {
			t.Fatalf("%s: shared quest is not a member", name)
		}
		if f.namesOf(found.AlsoIn) != other {
			t.Errorf("%s: alsoIn = %v, want %s", name, found.AlsoIn, other)
		}
	}
	cv, _ := f.s.CardView(f.ctx, shared.ID)
	if f.namesOf(cv.Journeys) != "qa,qb" || f.namesOf(cv.Card.AlsoIn) != "qa,qb" || len(cv.NeededBy) != 2 {
		t.Errorf("card view: journeys %v alsoIn %v neededBy %d", cv.Journeys, cv.Card.AlsoIn, len(cv.NeededBy))
	}
	// Fulfilling it once fulfils it in both.
	f.patch(shared.ID, CardPatch{Done: ptr(true)})
	for _, name := range []string{"qa", "qb"} {
		for _, c := range f.view(name).Cards {
			if c.ID == shared.ID && c.Status != StatusDone {
				t.Errorf("%s: shared status %s", name, c.Status)
			}
			if c.Final && c.Status != StatusAvailable {
				t.Errorf("%s: final not unlocked: %s", name, c.Status)
			}
		}
		if s := f.summary(name); s.Main != (Progress{1, 2}) {
			t.Errorf("%s: main %+v", name, s.Main)
		}
	}
}

// A quest's url and mark are free: the same url on two quests is two quests,
// and neither is ever parsed.
func TestURLAndMark(t *testing.T) {
	f := setup(t)
	url := "https://example.com/recipes/42"
	a := f.add(NewCard{Title: "Bake the bread", URL: "  " + url + " ", Mark: " recipe "})
	b := f.add(NewCard{Title: "Bake it again", URL: url})
	if a.ID == b.ID || a.URL != url || a.Mark != "recipe" || b.URL != url || b.Mark != "" {
		t.Errorf("two quests with one url: %+v %+v", a, b)
	}
	if a.Kind != "" {
		t.Errorf("a plain quest has a kind: %q", a.Kind)
	}
	c := f.add(NewCard{Title: "Fix the loader", Mark: "feat(someh)"})
	if c.Mark != "feat(someh)" || c.URL != "" {
		t.Errorf("mark without url: %+v", c)
	}
	f.mustFail(func() error {
		_, err := f.s.AddCard(f.ctx, NewCard{Title: "t", Mark: "a-mark-too-long!"}, "")
		return err
	}(), "at most 15 characters")
	if got := f.add(NewCard{Title: "t", Mark: "ğüşiöçĞÜŞİÖÇ123"}); got.Mark != "ğüşiöçĞÜŞİÖÇ123" {
		t.Errorf("15 characters, more bytes: %q", got.Mark)
	}

	got := f.patch(c.ID, CardPatch{URL: ptr("https://example.com/c/234g45a"), Mark: ptr("234g45a"), Title: ptr("Fix the loader, again")})
	if got.URL != "https://example.com/c/234g45a" || got.Mark != "234g45a" || got.Title != "Fix the loader, again" {
		t.Errorf("patch url, mark and title: %+v", got)
	}
	if _, err := f.s.UpdateCard(f.ctx, c.ID, CardPatch{Mark: ptr("0123456789abcdef")}, ""); KindOf(err) != ErrInvalid {
		t.Errorf("long mark on patch: %v", err)
	}
	got = f.patch(c.ID, CardPatch{URL: ptr(""), Mark: ptr("")})
	if got.URL != "" || got.Mark != "" {
		t.Errorf("cleared: %+v", got)
	}
	var texts []string
	rows, err := f.s.db.Query(`SELECT text FROM events WHERE card_id = ? ORDER BY id`, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		rows.Scan(&s)
		texts = append(texts, s)
	}
	rows.Close()
	k := Key(c.ID)
	want := []string{k + " added", k + " renamed from “Fix the loader”", k + " link set to https://example.com/c/234g45a",
		k + " mark set to 234g45a", k + " link cleared", k + " mark cleared"}
	if strings.Join(texts, "\n") != strings.Join(want, "\n") {
		t.Errorf("events:\n%s\nwant:\n%s", strings.Join(texts, "\n"), strings.Join(want, "\n"))
	}
	// Search finds a quest by its mark and its url.
	for q, want := range map[string]int{"recipe bread": 1, "example.com/recipes": 2, "234g45a": 0} {
		res, err := f.s.Search(f.ctx, q)
		if err != nil || len(res.Quests) != want {
			t.Errorf("search %q: %+v %v", q, res, err)
		}
	}
}

// A petition is a quest awaiting a reply: setting who makes one, clearing it
// makes a plain quest again.
func TestPetition(t *testing.T) {
	f := setup(t)
	p := f.add(NewCard{Title: "Final key art", WaitingOn: "the artist"})
	if p.Kind != KindAwaiting || p.Status != StatusAwaiting || p.WaitingOn != "the artist" || p.Since != "2026-09-24" {
		t.Fatalf("petition: %+v", p)
	}
	plain := f.patch(p.ID, CardPatch{WaitingOn: ptr(" ")})
	if plain.Kind != "" || plain.Status != StatusAvailable || plain.WaitingOn != "" || plain.Since != "" {
		t.Errorf("cleared: %+v", plain)
	}
	f.now = f.now.Add(48 * time.Hour)
	again := f.patch(p.ID, CardPatch{WaitingOn: ptr("the publisher")})
	if again.Kind != KindAwaiting || again.Status != StatusAwaiting || again.WaitingOn != "the publisher" || again.Since != "2026-09-26" {
		t.Errorf("petition again: %+v", again)
	}
	if done := f.patch(p.ID, CardPatch{Done: ptr(true)}); done.Status != StatusDone || done.Kind != KindAwaiting {
		t.Errorf("fulfilled petition: %+v", done)
	}
}

func TestMembershipFollowsNeeds(t *testing.T) {
	f := setup(t)
	y := f.quest("y")
	x := f.quest("x", y.ID)
	up := f.quest("upstream")
	a := f.quest("final A", x.ID, up.ID)
	b := f.quest("final B", up.ID)
	loose := f.quest("loose")
	f.journey("qa", a)
	f.journey("qb", b)
	if got := f.members("qa"); !slices.Equal(got, []int64{y.ID, x.ID, up.ID, a.ID}) {
		t.Errorf("qa members %v", got)
	}
	if got := f.members("qb"); !slices.Equal(got, []int64{up.ID, b.ID}) {
		t.Errorf("qb members %v", got)
	}
	if cv, _ := f.s.CardView(f.ctx, loose.ID); len(cv.Journeys) != 0 {
		t.Errorf("a card with no links is in %v", cv.Journeys)
	}
	// Cancelled cards stay members.
	f.cancel(y.ID, "no")
	if !slices.Contains(f.members("qa"), y.ID) {
		t.Errorf("cancelled card left the journey")
	}
	// Cutting x→y drops y out of qa.
	if err := f.s.RemoveNeed(f.ctx, x.ID, y.ID); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(f.members("qa"), y.ID) {
		t.Errorf("unneeded card still a member")
	}
	// A journey with no final has no members and still shows its log.
	f.journey("empty", nil)
	if v := f.view("empty"); len(v.Cards) != 0 || v.Journey.State != JourneyActive || len(v.Log) != 1 {
		t.Errorf("empty journey: %+v", v)
	}
}

func TestSideQuestMembershipFollowsItsCard(t *testing.T) {
	f := setup(t)
	x := f.quest("x")
	a := f.quest("final", x.ID)
	f.journey("qa", a)
	side := f.add(NewCard{Title: "polish x", SideOf: &x.ID})
	sideOfSide := f.add(NewCard{Title: "polish the polish", SideOf: &side.ID})
	if got := f.members("qa"); !slices.Contains(got, side.ID) || !slices.Contains(got, sideOfSide.ID) {
		t.Errorf("side quests not members: %v", got)
	}
	if s := f.summary("qa"); s.Achievements != (Progress{0, 2}) || s.Main != (Progress{0, 2}) {
		t.Errorf("summary %+v %+v", s.Main, s.Achievements)
	}
	if err := f.s.RemoveNeed(f.ctx, a.ID, x.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.members("qa"); !slices.Equal(got, []int64{a.ID}) {
		t.Errorf("side quests stayed after their card left: %v", got)
	}
}

func TestCycleRejectedAcrossJourneys(t *testing.T) {
	f := setup(t)
	shared := f.quest("shared")
	a := f.quest("final A", shared.ID)
	b := f.quest("final B", shared.ID)
	f.journey("qa", a)
	f.journey("qb", b)
	f.need(a.ID, b.ID) // A needs B: fine
	_, err := f.s.AddNeed(f.ctx, shared.ID, a.ID)
	if KindOf(err) != ErrConflict || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("shared needs A: want cycle, got %v", err)
	}
	if _, err := f.s.AddNeed(f.ctx, b.ID, a.ID); KindOf(err) != ErrConflict {
		t.Fatalf("B needs A after A needs B: %v", err)
	}
	if _, err := f.s.AddNeed(f.ctx, a.ID, a.ID); KindOf(err) != ErrInvalid {
		t.Fatalf("self need: %v", err)
	}
	// A cycle closed by a new card's needs and neededBy leaves no card behind.
	before := len(f.s.mustGraph(t).cards)
	if _, err := f.s.AddCard(f.ctx, NewCard{Title: "d", Needs: []int64{a.ID}, NeededBy: []int64{shared.ID}}, ""); KindOf(err) != ErrConflict {
		t.Fatalf("new card closing a cycle: %v", err)
	}
	if after := len(f.s.mustGraph(t).cards); after != before {
		t.Fatalf("rejected add left a card behind")
	}
	if created, err := f.s.AddNeed(f.ctx, a.ID, b.ID); err != nil || created {
		t.Fatalf("duplicate need: %v %v", created, err)
	}
	if err := f.s.RemoveNeed(f.ctx, b.ID, a.ID); KindOf(err) != ErrNotFound {
		t.Fatalf("removing a missing need: %v", err)
	}
}

func (s *Store) mustGraph(t *testing.T) *graph {
	g, err := loadGraph(context.Background(), s.db)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestJourneyLogShowsCurrentMembers(t *testing.T) {
	f := setup(t)
	migration := f.quest("Save migration")
	a := f.quest("final A", migration.ID)
	f.journey("qa", a)
	b := f.quest("final B")
	f.journey("qb", b)
	found := f.add(NewCard{Title: "fix the old-save crash", FoundWhile: &migration.ID, Reason: "old saves crash the loader", NeededBy: []int64{migration.ID}})
	f.patch(found.ID, CardPatch{Working: ptr(true), WorkingBy: ptr("cyd")})

	texts := func(name string) string {
		var out []string
		for _, e := range f.view(name).Log {
			out = append(out, e.Text)
		}
		return strings.Join(out, "\n")
	}
	want := strings.Join([]string{
		Key(migration.ID) + " added",
		Key(a.ID) + " added — requires " + Key(migration.ID),
		"journey created",
		"crowning quest set to " + Key(a.ID),
		Key(found.ID) + " found on " + Key(migration.ID) + ": old saves crash the loader — opens " + Key(migration.ID),
		Key(found.ID) + " taken up by cyd",
	}, "\n")
	if got := texts("qa"); got != want {
		t.Errorf("qa log:\n%s\nwant:\n%s", got, want)
	}
	if got := texts("qb"); got != Key(b.ID)+" added\njourney created\ncrowning quest set to "+Key(b.ID) {
		t.Errorf("qb log:\n%s", got)
	}
	// Once the found card leaves qa, its events leave qa's log.
	if err := f.s.RemoveNeed(f.ctx, migration.ID, found.ID); err != nil {
		t.Fatal(err)
	}
	if got := texts("qa"); strings.Contains(got, "taken up by cyd") || !strings.Contains(got, "no longer requires "+Key(found.ID)) {
		t.Errorf("qa log after unneed:\n%s", got)
	}
}

func TestAddValidation(t *testing.T) {
	f := setup(t)
	side := f.add(NewCard{Title: "s", SideOf: ptr(f.quest("m").ID)})
	cases := map[string]struct {
		in   NewCard
		kind ErrKind
	}{
		"no title":        {NewCard{}, ErrInvalid},
		"blank title":     {NewCard{Title: "  ", WaitingOn: "x"}, ErrInvalid},
		"long mark":       {NewCard{Title: "t", Mark: "0123456789abcdef"}, ErrInvalid},
		"unknown need":    {NewCard{Title: "t", Needs: []int64{999}}, ErrNotFound},
		"unknown journey": {NewCard{Title: "t", FinalOf: "J99"}, ErrNotFound},
		"not a journey":   {NewCard{Title: "t", FinalOf: "nope"}, ErrInvalid},
		"final globally":  {NewCard{Title: "t", Final: true}, ErrInvalid},
		"need a side":     {NewCard{Title: "t", Needs: []int64{side.ID}}, ErrInvalid},
		"side with need":  {NewCard{Title: "t", SideOf: &side.ID, Needs: []int64{side.ID}}, ErrInvalid},
	}
	for name, tc := range cases {
		if _, err := f.s.AddCard(f.ctx, tc.in, ""); KindOf(err) != tc.kind {
			t.Errorf("%s: got %v (kind %d), want kind %d", name, err, KindOf(err), tc.kind)
		}
	}
}

func TestSoftRemoveCascades(t *testing.T) {
	f := setup(t)
	a := f.quest("a")
	b := f.quest("b", a.ID)
	side := f.add(NewCard{Title: "side of b", SideOf: &b.ID})
	final := f.quest("final", b.ID)
	f.journey("q", final)
	if err := f.s.RemoveCard(f.ctx, b.ID, ""); KindOf(err) != ErrInvalid {
		t.Fatalf("remove without reason: %v", err)
	}
	if err := f.s.RemoveCard(f.ctx, b.ID, "parked"); err != nil {
		t.Fatal(err)
	}
	if got := f.members("q"); !slices.Equal(got, []int64{final.ID}) {
		t.Errorf("members after remove: %v", got)
	}
	if c := f.card(final.ID); c.Status != StatusAvailable {
		t.Errorf("final should be unlocked once its need is removed: %s", c.Status)
	}
	if _, err := f.s.CardView(f.ctx, side.ID); KindOf(err) != ErrNotFound {
		t.Errorf("side quest survived its card: %v", err)
	}
	if err := f.s.RemoveCard(f.ctx, b.ID, "again"); KindOf(err) != ErrNotFound {
		t.Errorf("removing twice: %v", err)
	}
	// Removing a journey's final leaves the journey without one.
	if err := f.s.RemoveCard(f.ctx, final.ID, "whole thing dropped"); err != nil {
		t.Fatal(err)
	}
	v := f.view("q")
	if v.Journey.FinalCardID != nil || len(v.Cards) != 0 {
		t.Errorf("journey after its final was removed: %+v", v.Journey)
	}
	if last := v.Log[len(v.Log)-1]; last.Text != "crowning quest "+Key(final.ID)+" struck: whole thing dropped" {
		t.Errorf("last journey event: %+v", last)
	}
	// Remove events of b and its side quest stay in the database even though
	// b is no longer anybody's member.
	var n int
	f.s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE kind = 'remove'`).Scan(&n)
	if n != 3 {
		t.Errorf("remove events: %d", n)
	}
}

func TestSideQuestsNeverLock(t *testing.T) {
	f := setup(t)
	blocker := f.quest("blocker")
	main := f.quest("main", blocker.ID)
	f.journey("q", main)
	side := f.add(NewCard{Title: "polish", SideOf: &main.ID})
	if c := f.card(side.ID); c.Status != StatusAvailable {
		t.Errorf("side quest of a locked card: %s", c.Status)
	}
	if _, err := f.s.AddNeed(f.ctx, main.ID, side.ID); KindOf(err) != ErrInvalid {
		t.Errorf("main needs side: %v", err)
	}
	if _, err := f.s.AddNeed(f.ctx, side.ID, blocker.ID); KindOf(err) != ErrInvalid {
		t.Errorf("side needs blocker: %v", err)
	}
	if _, err := f.s.UpdateJourney(f.ctx, f.key("q"), JourneyPatch{Final: &side.ID}); KindOf(err) != ErrInvalid {
		t.Errorf("side quest made final: %v", err)
	}
	f.patch(blocker.ID, CardPatch{Done: ptr(true)})
	if c := f.card(main.ID); c.Status != StatusAvailable {
		t.Errorf("main after blocker done: %s", c.Status)
	}
	if s := f.summary("q"); s.Main != (Progress{1, 2}) || s.Achievements != (Progress{0, 1}) {
		t.Errorf("progress: main %+v achievements %+v", s.Main, s.Achievements)
	}
}

func TestFinal(t *testing.T) {
	f := setup(t)
	a := f.quest("a")
	b := f.quest("b")
	f.journey("q", a)
	if _, err := f.s.UpdateJourney(f.ctx, f.key("q"), JourneyPatch{Final: &b.ID}); err != nil {
		t.Fatal(err)
	}
	v := f.view("q")
	if *v.Journey.FinalCardID != b.ID || v.Log[len(v.Log)-1].Text != fmt.Sprintf("crowning quest changed from %s to %s", Key(a.ID), Key(b.ID)) {
		t.Errorf("final change: %v %+v", *v.Journey.FinalCardID, v.Log)
	}
	// Journey-scoped patch sets the final of that journey; the global one refuses.
	if _, err := f.s.UpdateCard(f.ctx, a.ID, CardPatch{Final: ptr(true)}, ""); KindOf(err) != ErrInvalid {
		t.Errorf("global final patch: %v", err)
	}
	c, err := f.s.UpdateCard(f.ctx, a.ID, CardPatch{Final: ptr(true)}, f.key("q"))
	if err != nil || !c.Final {
		t.Fatalf("journey-scoped final: %+v %v", c, err)
	}
	c, err = f.s.UpdateCard(f.ctx, a.ID, CardPatch{Final: ptr(false)}, f.key("q"))
	if err != nil || c.Final || f.view("q").Journey.FinalCardID != nil {
		t.Fatalf("unset final: %+v %v", c, err)
	}
	// Journey-scoped add with final: true.
	d, err := f.s.AddCard(f.ctx, NewCard{Title: "d", Final: true}, f.key("q"))
	if err != nil || !d.Final || *f.view("q").Journey.FinalCardID != d.ID {
		t.Fatalf("journey-scoped add final: %+v %v", d, err)
	}
}

func TestCancelledUnblocks(t *testing.T) {
	f := setup(t)
	a := f.quest("a")
	b := f.quest("b")
	top := f.quest("top", a.ID, b.ID)
	f.patch(a.ID, CardPatch{Done: ptr(true)})
	if c := f.card(top.ID); c.Status != StatusLocked || c.OpenBefore != 1 {
		t.Fatalf("before cancel: %s/%d", c.Status, c.OpenBefore)
	}
	got := f.cancel(b.ID, "vendor dropped the feature")
	if got.Status != StatusCancelled || !got.Cancelled || got.CancelReason != "vendor dropped the feature" {
		t.Errorf("cancelled card: %+v", got)
	}
	if c := f.card(top.ID); c.Status != StatusAvailable || c.OpenBefore != 0 {
		t.Errorf("a cancelled prerequisite still blocks: %s/%d", c.Status, c.OpenBefore)
	}
	if c := f.cancel(a.ID, "moot"); c.Status != StatusCancelled {
		t.Errorf("done+cancelled status = %s", c.Status)
	}
	if _, err := f.s.UpdateCard(f.ctx, b.ID, CardPatch{Cancelled: ptr(true)}, ""); err != nil {
		t.Errorf("re-cancelling is a no-op, got %v", err)
	}
	if _, err := f.s.UpdateCard(f.ctx, top.ID, CardPatch{Cancelled: ptr(true)}, ""); KindOf(err) != ErrInvalid {
		t.Errorf("cancel without reason: %v", err)
	}
	f.patch(b.ID, CardPatch{Cancelled: ptr(false)})
	if c := f.card(top.ID); c.Status != StatusLocked {
		t.Errorf("after uncancel: %s", c.Status)
	}
}

func TestCancelledExcludedFromTotals(t *testing.T) {
	f := setup(t)
	a := f.quest("a")
	b := f.quest("b")
	c := f.quest("c")
	final := f.quest("final", a.ID, b.ID, c.ID)
	f.journey("q", final)
	sideA := f.add(NewCard{Title: "polish a", SideOf: &a.ID})
	f.add(NewCard{Title: "polish b", SideOf: &b.ID})
	f.patch(a.ID, CardPatch{Done: ptr(true)})
	f.cancel(b.ID, "not needed") // cascades to "polish b"
	f.patch(sideA.ID, CardPatch{Done: ptr(true)})
	q := f.summary("q")
	if q.Main != (Progress{1, 3}) || q.Achievements != (Progress{1, 1}) || q.Cancelled != 2 || q.Available != 1 {
		t.Errorf("summary: main %+v achievements %+v cancelled %d available %d", q.Main, q.Achievements, q.Cancelled, q.Available)
	}
}

func TestCancelCascades(t *testing.T) {
	f := setup(t)
	main := f.quest("main")
	side := f.add(NewCard{Title: "side", SideOf: &main.ID})
	sideOfSide := f.add(NewCard{Title: "side of side", SideOf: &side.ID})
	other := f.quest("other")
	otherSide := f.add(NewCard{Title: "other side", SideOf: &other.ID})
	f.cancel(main.ID, "out of scope")
	for _, id := range []int64{main.ID, side.ID, sideOfSide.ID} {
		if c := f.card(id); c.Status != StatusCancelled || c.CancelReason != "out of scope" {
			t.Errorf("%s after cancel cascade: %s %q", Key(id), c.Status, c.CancelReason)
		}
	}
	if f.card(otherSide.ID).Cancelled {
		t.Errorf("an unrelated side quest was cancelled")
	}
	var n int
	f.s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE kind = 'cancel' AND text LIKE '%abandoned with its quest ` + Key(main.ID) + `: out of scope'`).Scan(&n)
	if n != 2 {
		t.Errorf("cascade cancel events: %d", n)
	}
}

func TestJourneyState(t *testing.T) {
	f := setup(t)
	final := f.quest("ship")
	f.journey("q", final)
	if f.view("q").Journey.State != JourneyActive || f.summary("q").State != JourneyActive {
		t.Errorf("new journey not active")
	}
	f.patch(final.ID, CardPatch{Done: ptr(true)})
	if f.view("q").Journey.State != JourneyComplete || f.summary("q").State != JourneyComplete {
		t.Errorf("final done: not complete")
	}
	f.cancel(final.ID, "whole goal dropped")
	if v, s := f.view("q").Journey.State, f.summary("q").State; v != JourneyCancelled || s != JourneyCancelled {
		t.Errorf("final cancelled: view %s summary %s", v, s)
	}
}

func TestWorking(t *testing.T) {
	f := setup(t)
	a := f.quest("a")
	b := f.quest("b")
	f.journey("q", f.quest("final", a.ID, b.ID))
	got := f.patch(a.ID, CardPatch{Working: ptr(true), WorkingBy: ptr("cyd")})
	if !got.Working || got.WorkingBy != "cyd" || got.WorkingSince == "" {
		t.Fatalf("start: %+v", got)
	}
	f.patch(b.ID, CardPatch{Working: ptr(true)})
	if q := f.summary("q"); q.InProgress != 2 {
		t.Errorf("inProgress = %d", q.InProgress)
	}
	if got := f.patch(b.ID, CardPatch{Working: ptr(false)}); got.Working || got.WorkingSince != "" {
		t.Errorf("stop: %+v", got)
	}
	if got := f.patch(a.ID, CardPatch{Done: ptr(true)}); got.Working {
		t.Errorf("done did not clear working: %+v", got)
	}
	if got := f.patch(a.ID, CardPatch{Done: ptr(false)}); got.Working {
		t.Errorf("working came back after undone: %+v", got)
	}
	f.patch(b.ID, CardPatch{Working: ptr(true), WorkingBy: ptr("bo")})
	if got := f.cancel(b.ID, "no"); got.Working {
		t.Errorf("cancel did not clear working: %+v", got)
	}
	if _, err := f.s.UpdateCard(f.ctx, b.ID, CardPatch{Working: ptr(true)}, ""); KindOf(err) != ErrInvalid {
		t.Errorf("start on a cancelled card: %v", err)
	}
	if _, err := f.s.UpdateCard(f.ctx, a.ID, CardPatch{WorkingBy: ptr("x")}, ""); KindOf(err) != ErrInvalid {
		t.Errorf("workingBy without working: %v", err)
	}
	if q := f.summary("q"); q.InProgress != 0 {
		t.Errorf("inProgress after all stopped = %d", q.InProgress)
	}
}

func TestArchive(t *testing.T) {
	f := setup(t)
	final := f.quest("ship")
	f.journey("q", final)
	archive := func(on bool) {
		t.Helper()
		if _, err := f.s.UpdateJourney(f.ctx, strings.ToLower(f.key("q")), JourneyPatch{Archived: ptr(on)}); err != nil {
			t.Fatal(err)
		}
	}
	archive(true)
	archive(true) // already archived: no second event
	if f.summary("q").ArchivedAt == "" || f.view("q").Journey.ArchivedAt == "" {
		t.Errorf("not archived")
	}
	if got := len(f.members("q")); got != 1 {
		t.Errorf("archiving changed the journey's quests: %d members", got)
	}
	archive(false)
	if f.summary("q").ArchivedAt != "" || f.view("q").Journey.ArchivedAt != "" {
		t.Errorf("still archived")
	}
	var texts []string
	for _, e := range f.view("q").Log {
		if e.Kind == "archive" {
			texts = append(texts, e.Text)
		}
	}
	if strings.Join(texts, "; ") != "journey archived; journey brought back from the archive" {
		t.Errorf("chronicle: %q", texts)
	}
}

func TestSearch(t *testing.T) {
	f := setup(t)
	final := f.quest("Ship the winter update")
	f.journey("winter", final)
	crash := f.add(NewCard{Title: "Old saves crash the loader", Mark: "#88", NeededBy: []int64{final.ID}})
	notes := f.quest("old notes")
	f.patch(notes.ID, CardPatch{Done: ptr(true)})
	f.need(final.ID, notes.ID)
	tr := f.quest("İzin ekranı")

	search := func(q string) *SearchResult {
		t.Helper()
		r, err := f.s.Search(f.ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	keys := func(cs []Card) string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Key)
		}
		return strings.Join(out, " ")
	}

	if r := search("OLD"); keys(r.Quests) != crash.Key+" "+notes.Key {
		t.Errorf("OLD: %s, want the open quest before the fulfilled one", keys(r.Quests))
	}
	if r := search("saves loader"); keys(r.Quests) != crash.Key || len(r.Quests[0].AlsoIn) != 1 || f.names[r.Quests[0].AlsoIn[0].Key] != "winter" {
		t.Errorf("saves loader: %+v", r.Quests)
	}
	if r := search("izin"); keys(r.Quests) != tr.Key {
		t.Errorf("izin: %s, want the dotted İ to match", keys(r.Quests))
	}
	if r := search("#88"); keys(r.Quests) != crash.Key || r.Exact != nil {
		t.Errorf("#88: %+v, want the quest marked so, found but not exact", r)
	}
	for _, q := range []string{crash.Key, "q-" + strconv.FormatInt(crash.ID, 10)} {
		if r := search(q); r.Exact == nil || r.Exact.ID != crash.ID || r.Quests[0].ID != crash.ID {
			t.Errorf("%s: exact %+v", q, r.Exact)
		}
	}
	if r := search("winter"); len(r.Journeys) != 1 || r.Journeys[0].Key != f.key("winter") || keys(r.Quests) != final.Key || !r.Quests[0].Final {
		t.Errorf("winter: journeys %+v quests %s", r.Journeys, keys(r.Quests))
	}
	if r := search(f.key("winter")); len(r.Journeys) != 1 || r.Exact != nil {
		t.Errorf("by journey key: journeys %+v exact %+v", r.Journeys, r.Exact)
	}
	if _, err := f.s.UpdateJourney(f.ctx, f.key("winter"), JourneyPatch{Archived: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if r := search(""); len(r.Journeys) != 0 || len(r.Quests) != 0 {
		t.Errorf("empty query lists archived journeys or quests: %+v", r)
	}
	if r := search("winter"); len(r.Journeys) != 1 {
		t.Errorf("a query still finds an archived journey: %+v", r.Journeys)
	}
}

func TestHosts(t *testing.T) {
	f := setup(t)
	for _, bad := range []string{"", " ", "http://mikado.home", "mikado.home:8080", "mikado.home/x", "*", "*.", "a.*.b", "**.ts.net", "mi kado", ".home", "a..b"} {
		if _, _, err := f.s.AddHost(f.ctx, bad); KindOf(err) != ErrInvalid {
			t.Errorf("AddHost(%q): %v, want invalid", bad, err)
		}
	}
	h, created, err := f.s.AddHost(f.ctx, " Mikado.Home. ")
	if err != nil || !created || h.Name != "mikado.home" || h.AddedAt != "2026-09-24T10:00:00Z" {
		t.Fatalf("AddHost: %+v %v %v", h, created, err)
	}
	f.now = f.now.Add(time.Hour)
	if h, created, err := f.s.AddHost(f.ctx, "mikado.home"); err != nil || created || h.AddedAt != "2026-09-24T10:00:00Z" {
		t.Errorf("adding it again: %+v %v %v, want the first one unchanged", h, created, err)
	}
	if _, _, err := f.s.AddHost(f.ctx, "*.TS.net"); err != nil {
		t.Fatal(err)
	}
	names := func() string {
		hs, err := f.s.Hosts(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, h := range hs {
			out = append(out, h.Name)
		}
		return strings.Join(out, " ")
	}
	if got := names(); got != "*.ts.net mikado.home" {
		t.Errorf("Hosts: %q", got)
	}
	if err := f.s.RemoveHost(f.ctx, "MIKADO.home"); err != nil {
		t.Fatal(err)
	}
	if err := f.s.RemoveHost(f.ctx, "mikado.home"); KindOf(err) != ErrNotFound {
		t.Errorf("removing it again: %v, want not found", err)
	}
	if err := f.s.RemoveHost(f.ctx, "x:1"); KindOf(err) != ErrInvalid {
		t.Errorf("removing a bad name: %v, want invalid", err)
	}
	if got := names(); got != "*.ts.net" {
		t.Errorf("Hosts after remove: %q", got)
	}
}

// crownsOf returns the Crowns of card id on journey name's chart (nil if the
// card is not there or stands for no other journey).
func (f *fixture) crownsOf(name string, id int64) *Crowns {
	f.t.Helper()
	for _, c := range f.view(name).Cards {
		if c.ID == id {
			return c.Crowns
		}
	}
	f.t.Fatalf("%s is not on %s's chart", Key(id), name)
	return nil
}

func (f *fixture) linkNames(ls []JourneyLink) string {
	var out []string
	for _, l := range ls {
		out = append(out, f.names[l.Key])
	}
	return strings.Join(out, ",")
}

func TestJourneyFoldsIntoOneCard(t *testing.T) {
	f := setup(t)
	// Quest qb: bFinal requires b1 and shared; a side quest hangs on bFinal.
	b1 := f.quest("b1")
	shared := f.quest("shared")
	bFinal := f.quest("final B", b1.ID, shared.ID)
	bSide := f.add(NewCard{Title: "polish B", SideOf: &bFinal.ID})
	f.journey("qb", bFinal)
	// Journey qa: aFinal requires x and shared directly; x requires the whole of qb.
	x := f.quest("x", bFinal.ID)
	aFinal := f.quest("final A", x.ID, shared.ID)
	f.journey("qa", aFinal)

	// qb's own quests stay out of qa, except shared, which qa reaches directly.
	if got, want := f.members("qa"), []int64{shared.ID, bFinal.ID, x.ID, aFinal.ID}; !slices.Equal(got, want) {
		t.Errorf("qa members %v, want %v", got, want)
	}
	if got, want := f.members("qb"), []int64{b1.ID, shared.ID, bFinal.ID, bSide.ID}; !slices.Equal(got, want) {
		t.Errorf("qb members %v, want %v", got, want)
	}
	for _, c := range []struct {
		id   int64
		want string
	}{{b1.ID, "qb"}, {bSide.ID, "qb"}, {shared.ID, "qb,qa"}, {bFinal.ID, "qb,qa"}, {x.ID, "qa"}} {
		cv, err := f.s.CardView(f.ctx, c.id)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.namesOf(cv.Journeys); got != c.want {
			t.Errorf("%s journeys %q, want %q", Key(c.id), got, c.want)
		}
	}
	// The journey card counts as one quest, and is sealed while qb has work left.
	if s := f.summary("qa"); s.Main != (Progress{0, 4}) || s.Achievements != (Progress{0, 0}) {
		t.Errorf("qa summary %+v %+v", s.Main, s.Achievements)
	}
	v := f.view("qa")
	for _, c := range v.Cards {
		switch c.ID {
		case bFinal.ID:
			if c.Status != StatusLocked || c.OpenBefore != 2 || c.Crowns == nil {
				t.Errorf("journey card: %s %d %+v", c.Status, c.OpenBefore, c.Crowns)
			}
		case aFinal.ID:
			if c.Crowns != nil {
				t.Errorf("own crowning quest has crowns %+v", c.Crowns)
			}
		}
	}
	// Only needs between members: bFinal→shared stays (both are on qa's chart), bFinal→b1 does not.
	if !slices.Contains(v.Needs, Need{bFinal.ID, shared.ID}) || slices.Contains(v.Needs, Need{bFinal.ID, b1.ID}) {
		t.Errorf("qa needs %v", v.Needs)
	}
	cr := f.crownsOf("qa", bFinal.ID)
	if cr.Key != f.key("qb") || cr.Title != "Journey qb" || cr.State != JourneyActive || cr.Done != 0 || cr.Total != 3 || len(cr.Open) != 3 {
		t.Errorf("crowns %+v", cr)
	}

	f.patch(b1.ID, CardPatch{Done: ptr(true)})
	f.patch(shared.ID, CardPatch{Working: ptr(true)})
	cr = f.crownsOf("qa", bFinal.ID)
	if cr.Done != 1 || cr.Total != 3 || cr.Working != 1 || len(cr.Open) != 2 || cr.Open[0].Key != Key(shared.ID) || !cr.Open[0].Working || cr.Open[0].Status != StatusAvailable {
		t.Errorf("crowns after progress %+v", cr)
	}
	f.patch(shared.ID, CardPatch{Done: ptr(true)})
	f.patch(bFinal.ID, CardPatch{Done: ptr(true)})
	if cr = f.crownsOf("qa", bFinal.ID); cr.State != JourneyComplete || cr.Done != 3 || len(cr.Open) != 0 {
		t.Errorf("fulfilled journey card %+v", cr)
	}
	if s := f.summary("qa"); s.Main != (Progress{2, 4}) {
		t.Errorf("qa summary after qb fulfilled %+v", s.Main)
	}
	// qb's chronicle stays qb's: b1's events are not in qa's.
	for _, e := range f.view("qa").Log {
		if e.CardID != nil && *e.CardID == b1.ID {
			t.Errorf("qa log has b1's event %q", e.Text)
		}
	}
	// Globally, the card names the journey it crowns.
	if cv, _ := f.s.CardView(f.ctx, bFinal.ID); cv.Card.Crowns == nil || cv.Card.Crowns.Key != f.key("qb") {
		t.Errorf("card view crowns %+v", cv.Card.Crowns)
	}
	// Seen from qb itself (journey-scoped route), its own crowning quest stands for nothing.
	if c, err := f.s.card(f.ctx, bFinal.ID, f.key("qb")); err != nil || c.Crowns != nil {
		t.Errorf("qb's crowning quest seen from qb: %+v %v", c.Crowns, err)
	}
	// The atlas: qa is blocked by qb, and qb blocks qa.
	if a, b := f.summary("qa"), f.summary("qb"); f.linkNames(a.BlockedBy) != "qb" || len(a.Blocks) != 0 ||
		f.linkNames(b.Blocks) != "qa" || len(b.BlockedBy) != 0 || a.BlockedBy[0].State != JourneyComplete {
		t.Errorf("atlas links: qa %+v %+v, qb %+v %+v", a.BlockedBy, a.Blocks, b.BlockedBy, b.Blocks)
	}
}

func TestNestedJourneysFoldOneLevel(t *testing.T) {
	f := setup(t)
	c1 := f.quest("c1")
	cFinal := f.quest("final C", c1.ID)
	f.journey("qc", cFinal)
	b1 := f.quest("b1")
	bFinal := f.quest("final B", b1.ID, cFinal.ID)
	f.journey("qb", bFinal)
	aFinal := f.quest("final A", bFinal.ID)
	f.journey("qa", aFinal)

	if got, want := f.members("qa"), []int64{bFinal.ID, aFinal.ID}; !slices.Equal(got, want) {
		t.Errorf("qa members %v, want %v", got, want)
	}
	if got, want := f.members("qb"), []int64{cFinal.ID, b1.ID, bFinal.ID}; !slices.Equal(got, want) {
		t.Errorf("qb members %v, want %v", got, want)
	}
	// qc counts as one quest inside qb, so qb's card on qa says 0/3.
	if cr := f.crownsOf("qa", bFinal.ID); cr.Total != 3 || cr.Done != 0 {
		t.Errorf("qb on qa: %+v", cr)
	}
	if cr := f.crownsOf("qb", cFinal.ID); cr.Key != f.key("qc") || cr.Total != 2 {
		t.Errorf("qc on qb: %+v", cr)
	}
	if cv, _ := f.s.CardView(f.ctx, c1.ID); f.namesOf(cv.Journeys) != "qc" {
		t.Errorf("c1 journeys %q", f.namesOf(cv.Journeys))
	}
	a, b, c := f.summary("qa"), f.summary("qb"), f.summary("qc")
	if f.linkNames(a.BlockedBy) != "qb" || f.linkNames(b.BlockedBy) != "qc" || f.linkNames(b.Blocks) != "qa" ||
		f.linkNames(c.Blocks) != "qb" || len(c.BlockedBy) != 0 || len(a.Blocks) != 0 {
		t.Errorf("atlas links: qa %v/%v qb %v/%v qc %v/%v", a.BlockedBy, a.Blocks, b.BlockedBy, b.Blocks, c.BlockedBy, c.Blocks)
	}
	if a.Main != (Progress{0, 2}) || b.Main != (Progress{0, 3}) {
		t.Errorf("totals qa %+v qb %+v", a.Main, b.Main)
	}
}

func TestSharedCrowningQuestIsNotFolded(t *testing.T) {
	f := setup(t)
	x := f.quest("x")
	final := f.quest("final", x.ID)
	f.journey("qa", final)
	f.journey("qd", final)
	for _, name := range []string{"qa", "qd"} {
		if got := f.members(name); !slices.Equal(got, []int64{x.ID, final.ID}) {
			t.Errorf("%s members %v", name, got)
		}
		if cr := f.crownsOf(name, final.ID); cr != nil {
			t.Errorf("%s: own crowning quest shown as journey %+v", name, cr)
		}
		if s := f.summary(name); len(s.BlockedBy)+len(s.Blocks) != 0 {
			t.Errorf("%s atlas links %+v %+v", name, s.BlockedBy, s.Blocks)
		}
	}
	// A third journey requiring that quest folds it, naming the first journey it crowns.
	top := f.quest("top", final.ID)
	f.journey("qe", top)
	if got := f.members("qe"); !slices.Equal(got, []int64{final.ID, top.ID}) {
		t.Errorf("qe members %v", got)
	}
	if cr := f.crownsOf("qe", final.ID); cr == nil || cr.Key != f.key("qa") || cr.Total != 2 {
		t.Errorf("qe's journey card %+v", cr)
	}
	if s := f.summary("qe"); f.linkNames(s.BlockedBy) != "qa,qd" {
		t.Errorf("qe blocked by %v", s.BlockedBy)
	}
}

func TestResolveJourneyKey(t *testing.T) {
	f := setup(t)
	final := f.quest("final")
	f.journey("controller-support", final)
	f.journey("no-crown", nil)
	key := f.key("controller-support")
	for _, ref := range []string{key, strings.ToLower(key), " " + key + " "} {
		if id, err := f.s.Resolve(f.ctx, ref); err != nil || id != final.ID {
			t.Errorf("Resolve(%q) = %d, %v", ref, id, err)
		}
	}
	if _, err := f.s.Resolve(f.ctx, f.key("no-crown")); KindOf(err) != ErrInvalid || !strings.Contains(err.Error(), "no crowning quest") {
		t.Errorf("journey without a crowning quest: %v", err)
	}
	if _, err := f.s.Resolve(f.ctx, "J99"); KindOf(err) != ErrNotFound {
		t.Errorf("unknown journey: %v", err)
	}
	for _, ref := range []string{"controller-support", "not a key!"} {
		if _, err := f.s.Resolve(f.ctx, ref); KindOf(err) != ErrInvalid {
			t.Errorf("Resolve(%q): %v", ref, err)
		}
	}
	// Search names quests exactly by key only; the journey itself is found as a journey.
	res, err := f.s.Search(f.ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if res.Exact != nil || len(res.Journeys) != 1 {
		t.Errorf("search by journey key: exact %+v, journeys %+v", res.Exact, res.Journeys)
	}
}

// A database made before journeys (schema 3: quests with slugs) keeps its
// quests as journeys, same ids, and its chronicle.
func TestMigrateQuestsToJourneys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikado.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_init.sql", "002_archive.sql", "003_hosts.sql"} {
		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, q := range []string{
		`CREATE TABLE schema_version (version INTEGER NOT NULL)`,
		`INSERT INTO schema_version VALUES (3)`,
		`INSERT INTO cards (id, kind, title, created_at) VALUES (1, 'errand', 'ship it', '2026-01-01T00:00:00Z')`,
		`INSERT INTO quests (id, slug, title, final_card, created_at, archived_at) VALUES
			(4, 'winter', 'Winter update', 1, '2026-01-01T00:00:00Z', NULL),
			(9, 'old', 'Old goal', NULL, '2026-01-02T00:00:00Z', '2026-02-01T00:00:00Z')`,
		`INSERT INTO events (quest_id, card_id, at, kind, text) VALUES
			(4, NULL, '2026-01-01T00:00:00Z', 'create', 'quest created'),
			(NULL, 1, '2026-01-01T00:00:01Z', 'add', 'M1 added'),
			(4, 1, '2026-01-01T00:00:02Z', 'final', 'crowning deed set to M1')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v, err := s.Journey(context.Background(), "J4")
	if err != nil {
		t.Fatal(err)
	}
	if v.Journey.Title != "Winter update" || v.Journey.FinalCardID == nil || *v.Journey.FinalCardID != 1 || len(v.Cards) != 1 || len(v.Log) != 3 {
		t.Errorf("migrated journey: %+v, %d cards, log %+v", v.Journey, len(v.Cards), v.Log)
	}
	js, err := s.Journeys(context.Background())
	if err != nil || len(js) != 2 || js[1].Key != "J9" || js[1].ArchivedAt == "" {
		t.Errorf("migrated atlas: %+v %v", js, err)
	}
	// New journeys and events go on from there.
	j, err := s.CreateJourney(context.Background(), "Next", nil, "")
	if err != nil || j.Key != "J10" {
		t.Errorf("journey after migration: %+v %v", j, err)
	}
}

func (f *fixture) mustFail(err error, want string) {
	f.t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		f.t.Fatalf("got error %v, want one mentioning %q", err, want)
	}
}

func TestRewire(t *testing.T) {
	f := setup(t)
	a, b, c := f.quest("a"), f.quest("b"), f.quest("c")
	top := f.quest("top", a.ID)
	f.journey("q", top)
	f.need(top.ID, b.ID)
	r, err := f.s.Rewire(f.ctx, top.ID, a.ID, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := f.members("q"), []int64{b.ID, c.ID, top.ID}; !slices.Equal(got, want) {
		t.Errorf("members %v, want %v", got, want)
	}
	if !slices.Equal(r.Orphaned, []string{a.Key}) {
		t.Errorf("orphaned %v, want %s", r.Orphaned, a.Key)
	}
	_, err = f.s.Rewire(f.ctx, top.ID, a.ID, c.ID)
	f.mustFail(err, "does not require")
	// A cycle is refused and nothing changes.
	_, err = f.s.Rewire(f.ctx, c.ID, a.ID, top.ID)
	f.mustFail(err, "does not require")
	f.need(c.ID, a.ID)
	_, err = f.s.Rewire(f.ctx, c.ID, a.ID, top.ID)
	f.mustFail(err, "cycle")
	if got := f.card(c.ID); got.OpenBefore != 1 {
		t.Errorf("c should still require a after a refused rewire, openBefore %d", got.OpenBefore)
	}
}

func TestExtract(t *testing.T) {
	f := setup(t)
	// top requires x and y; x requires x1; y requires x1 too; z is left alone.
	x1 := f.quest("x1")
	x := f.quest("x", x1.ID)
	y := f.quest("y")
	z := f.quest("z")
	side := f.add(NewCard{Title: "x polish", SideOf: &x.ID})
	top := f.quest("top", x.ID, y.ID, z.ID)
	f.journey("q", top)

	_, err := f.s.Extract(f.ctx, f.key("q"), []int64{top.ID}, "all")
	f.mustFail(err, "crowns")
	_, err = f.s.Extract(f.ctx, f.key("q"), []int64{side.ID}, "s")
	f.mustFail(err, "side quest")

	out, err := f.s.Extract(f.ctx, f.key("q"), []int64{x.ID, x1.ID, y.ID}, "Loader")
	if err != nil {
		t.Fatal(err)
	}
	crown, _ := ParseCardID(out.Crown)
	if got, want := f.members("q"), []int64{z.ID, top.ID, crown}; !slices.Equal(got, want) {
		t.Errorf("old journey members %v, want %v", got, want)
	}
	f.keys["n"], f.names[out.Journey.Key] = out.Journey.Key, "n"
	if got, want := f.members("n"), []int64{x1.ID, x.ID, y.ID, side.ID, crown}; !slices.Equal(got, want) {
		t.Errorf("new journey members %v, want %v", got, want)
	}
	// The crown requires only the tops: x1 is reached through x.
	if v, _ := f.s.CardView(f.ctx, crown); len(v.Needs) != 2 {
		t.Errorf("crown requires %d quests, want 2 (x, y)", len(v.Needs))
	}
	if len(out.StillIn) != 0 {
		t.Errorf("stillIn %v, want none", out.StillIn)
	}
	if s := f.summary("q"); len(s.BlockedBy) != 1 || s.BlockedBy[0].Key != out.Journey.Key {
		t.Errorf("q blockedBy %+v", s.BlockedBy)
	}
}

func TestDeleteQuestFromJourney(t *testing.T) {
	f := setup(t)
	pre := f.quest("pre")
	q := f.quest("q", pre.ID)
	sq := f.add(NewCard{Title: "side", SideOf: &q.ID})
	other := f.quest("other")
	top := f.quest("top", q.ID, other.ID)
	f.journey("a", top)
	top2 := f.quest("top2", q.ID)
	f.journey("b", top2)

	// Its prerequisite and side quest would leave a with it.
	_, err := f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{Journey: f.key("a")})
	f.mustFail(err, "--branch")
	_, err = f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{})
	f.mustFail(err, "--journey")
	_, err = f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{Journey: f.key("a"), Force: true, Branch: true})
	f.mustFail(err, "still in")

	out, err := f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{Journey: f.key("a"), Branch: true})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Branch, []string{pre.Key, sq.Key}) || len(out.Deleted) != 0 {
		t.Errorf("out %+v", out)
	}
	if got, want := f.members("a"), []int64{other.ID, top.ID}; !slices.Equal(got, want) {
		t.Errorf("a members %v, want %v", got, want)
	}
	if got, want := f.members("b"), []int64{pre.ID, q.ID, sq.ID, top2.ID}; !slices.Equal(got, want) {
		t.Errorf("b members %v, want %v", got, want)
	}
	// b is its last journey now.
	_, err = f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{Journey: f.key("b"), Rewire: &top2.ID})
	f.mustFail(err, "--force")
	out, err = f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{Journey: f.key("b"), Force: true, Rewire: &top2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Deleted, []string{q.Key}) {
		t.Errorf("deleted %v", out.Deleted)
	}
	if got, want := f.members("b"), []int64{pre.ID, sq.ID, top2.ID}; !slices.Equal(got, want) {
		t.Errorf("b members %v, want %v", got, want)
	}
	if c := f.card(sq.ID); c.SideOf == nil || *c.SideOf != top2.ID {
		t.Errorf("side quest should hang on top2 now, sideOf %v", c.SideOf)
	}
	var n int
	f.s.db.QueryRow(`SELECT COUNT(*) FROM cards WHERE id = ?`, q.ID).Scan(&n)
	if n != 0 {
		t.Error("q is still in the database")
	}
	// The chronicle keeps a line for it.
	found := false
	for _, e := range f.view("b").Log {
		found = found || strings.Contains(e.Text, q.Key+" “q” deleted")
	}
	if !found {
		t.Error("b's chronicle has no line for the deleted quest")
	}
}

func TestDeleteQuestEverywhere(t *testing.T) {
	f := setup(t)
	shared := f.quest("shared")
	pre := f.quest("pre", shared.ID)
	q := f.quest("q", pre.ID)
	top := f.quest("top", q.ID)
	f.journey("a", top)
	top2 := f.quest("top2", shared.ID)
	f.journey("b", top2)

	_, err := f.s.DeleteQuest(f.ctx, top.ID, QuestDelete{Force: true})
	f.mustFail(err, "crowns")
	_, err = f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{Force: true})
	f.mustFail(err, "--branch")
	out, err := f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{Force: true, Branch: true})
	if err != nil {
		t.Fatal(err)
	}
	// shared leaves a but stays in b, so it is not deleted.
	if !slices.Equal(out.Deleted, []string{q.Key, pre.Key}) || !slices.Equal(out.Branch, []string{shared.Key, pre.Key}) {
		t.Errorf("out %+v", out)
	}
	if got, want := f.members("a"), []int64{top.ID}; !slices.Equal(got, want) {
		t.Errorf("a members %v, want %v", got, want)
	}
	if got, want := f.members("b"), []int64{shared.ID, top2.ID}; !slices.Equal(got, want) {
		t.Errorf("b members %v, want %v", got, want)
	}
}

func TestDeleteJourney(t *testing.T) {
	f := setup(t)
	shared := f.quest("shared")
	own := f.quest("own")
	top := f.quest("top", shared.ID, own.ID)
	f.journey("a", top)
	top2 := f.quest("top2", shared.ID)
	f.journey("b", top2)
	outer := f.quest("outer", top2.ID)
	f.journey("c", outer)

	_, err := f.s.DeleteJourney(f.ctx, f.key("b"), true)
	f.mustFail(err, "war table")
	_, err = f.s.DeleteJourney(f.ctx, f.key("a"), false)
	f.mustFail(err, "--force")
	out, err := f.s.DeleteJourney(f.ctx, f.key("a"), true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Deleted, []string{own.Key, top.Key}) || !slices.Equal(out.Kept, []string{shared.Key}) {
		t.Errorf("out %+v", out)
	}
	if _, err := f.s.Journey(f.ctx, f.key("a")); KindOf(err) != ErrNotFound {
		t.Errorf("journey a still there: %v", err)
	}
}

func TestDeleteRefusalNamesEveryMissingFlag(t *testing.T) {
	f := setup(t)
	pre := f.quest("pre")
	q := f.quest("q", pre.ID)
	top := f.quest("top", q.ID)
	f.journey("a", top)
	_, err := f.s.DeleteQuest(f.ctx, q.ID, QuestDelete{Journey: f.key("a")})
	f.mustFail(err, "add --force")
	f.mustFail(err, "--branch")
	if got, want := f.members("a"), []int64{pre.ID, q.ID, top.ID}; !slices.Equal(got, want) {
		t.Errorf("a refused delete changed the journey: %v, want %v", got, want)
	}
}

func TestRegions(t *testing.T) {
	f := setup(t)
	rs, err := f.s.Regions(f.ctx)
	if err != nil || len(rs) != 1 || rs[0].Key != "R1" || rs[0].Name != "Personal" {
		t.Fatalf("a new database has R1 Personal: %+v %v", rs, err)
	}
	work, err := f.s.CreateRegion(f.ctx, "  Work ")
	if err != nil || work.Key != "R2" || work.Name != "Work" {
		t.Fatalf("create: %+v %v", work, err)
	}
	_, err = f.s.CreateRegion(f.ctx, "work")
	f.mustFail(err, "already called")
	_, err = f.s.CreateRegion(f.ctx, "R7")
	f.mustFail(err, "reads as a region key")

	// A journey goes to the default region unless one is named, by key or name.
	a := f.quest("a")
	f.journey("home", a)
	if s := f.summary("home"); s.Region.Key != "R1" || s.Region.Name != "Personal" {
		t.Errorf("default region %+v", s.Region)
	}
	j, err := f.s.CreateJourney(f.ctx, "Ship", nil, "work")
	if err != nil || j.Region.Key != "R2" {
		t.Fatalf("journey in work: %+v %v", j, err)
	}
	f.keys["ship"], f.names[j.Key] = j.Key, "ship"
	if v := f.view("ship"); v.Journey.Region.Key != "R2" {
		t.Errorf("journey view region %+v", v.Journey.Region)
	}
	_, err = f.s.CreateJourney(f.ctx, "Nowhere", nil, "R9")
	f.mustFail(err, "no region R9")

	// Nothing links across the border: not a requirement, a crown, a quest
	// opening both, nor a journey waiting on another.
	b := f.quest("b")
	_, err = f.s.UpdateJourney(f.ctx, f.key("ship"), JourneyPatch{Final: &a.ID})
	f.mustFail(err, "stays inside one region")
	if _, err := f.s.UpdateJourney(f.ctx, f.key("ship"), JourneyPatch{Final: &b.ID}); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.AddNeed(f.ctx, b.ID, a.ID)
	f.mustFail(err, "stays inside one region")
	_, err = f.s.AddNeed(f.ctx, a.ID, b.ID)
	f.mustFail(err, "stays inside one region")
	// A new quest opening journeys on both sides.
	_, err = f.s.AddCard(f.ctx, NewCard{Title: "Crash on load", NeededBy: []int64{a.ID, b.ID}}, "")
	f.mustFail(err, "stays inside one region")
	// The refused change left nothing behind.
	if got := f.members("ship"); !slices.Equal(got, []int64{b.ID}) {
		t.Errorf("ship members after refusals %v", got)
	}

	// Inside one region links work as ever, and extract keeps the region.
	c := f.quest("c")
	if _, err := f.s.AddNeed(f.ctx, b.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	out, err := f.s.Extract(f.ctx, f.key("ship"), []int64{c.ID}, "C part")
	if err != nil || out.Journey.Region.Key != "R2" {
		t.Fatalf("extract: %+v %v", out, err)
	}

	// A journey moves only when nothing on it stays behind on another.
	_, err = f.s.UpdateJourney(f.ctx, f.key("ship"), JourneyPatch{Region: ptr("Personal")})
	f.mustFail(err, "move the journeys that share it together")
	// Together they can: ship waits on the journey extracted from it.
	both, err := f.s.MoveJourneys(f.ctx, "R1", []string{f.key("ship"), out.Journey.Key})
	if err != nil || len(both) != 2 || both[0].Region.Key != "R1" || both[1].Region.Key != "R1" {
		t.Fatalf("move together: %+v %v", both, err)
	}
	if _, err := f.s.MoveJourneys(f.ctx, "R2", []string{f.key("ship"), out.Journey.Key}); err != nil {
		t.Fatal(err)
	}
	lone, err := f.s.CreateJourney(f.ctx, "Lone", nil, "R2")
	if err != nil {
		t.Fatal(err)
	}
	moved, err := f.s.UpdateJourney(f.ctx, lone.Key, JourneyPatch{Region: ptr("R1")})
	if err != nil || moved.Region.Key != "R1" {
		t.Fatalf("move: %+v %v", moved, err)
	}

	// Rename keeps the key; delete takes only an empty region, never the last.
	if r, err := f.s.UpdateRegion(f.ctx, "R2", RegionPatch{Name: ptr("Alternet")}); err != nil || r.Name != "Alternet" || r.Key != "R2" {
		t.Fatalf("rename: %+v %v", r, err)
	}
	_, err = f.s.DeleteRegion(f.ctx, "Alternet")
	f.mustFail(err, "still holds")
	empty, err := f.s.CreateRegion(f.ctx, "Empty")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.DeleteRegion(f.ctx, empty.Key); err != nil {
		t.Fatal(err)
	}
	if rs, _ := f.s.Regions(f.ctx); len(rs) != 2 || rs[1].Journeys != 2 {
		t.Errorf("regions after delete: %+v", rs)
	}
	// Search finds a journey by its region's name.
	if res, err := f.s.Search(f.ctx, "alternet"); err != nil || len(res.Journeys) != 2 || res.Journeys[0].Region.Key != "R2" {
		t.Errorf("search by region: %+v %v", res, err)
	}
}

// oldDB makes a database at path as mikado left it at schema version n,
// from the migrations up to n.
func oldDB(t *testing.T, path string, n int) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if v, _ := strconv.Atoi(strings.SplitN(filepath.Base(name), "_", 2)[0]); v > n {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE schema_version (version INTEGER NOT NULL); INSERT INTO schema_version VALUES (?)`, n); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateRegions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikado.db")
	db := oldDB(t, path, 4)
	if _, err := db.Exec(`INSERT INTO journeys (title, created_at) VALUES ('Old', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	js, err := s.Journeys(context.Background())
	if err != nil || len(js) != 1 || js[0].Region != (RegionRef{Key: "R1", Name: "Personal"}) {
		t.Errorf("migrated journey region: %+v %v", js, err)
	}
}

// A database from before quests had one kind (schema 5): issue quests
// become plain quests with the same ids, their last known title and state,
// and all of mikado's own state; GitHub's data is gone.
func TestMigrateToOneKind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mikado.db")
	db := oldDB(t, path, 5)
	const at = "2026-01-01T00:00:00Z"
	for _, q := range []string{
		// 1 open issue, cached, with a hero, underway, an NPC.
		`INSERT INTO cards (id, kind, ref, ref_key, owner, npc, working_since, working_by, created_at)
			VALUES (1, 'issue', 'Studio/Game#7', 'studio/game#7', 'ada', 1, '` + at + `', 'cyd', '` + at + `')`,
		// 2 an issue never fetched: no title but its ref.
		`INSERT INTO cards (id, kind, ref, ref_key, created_at) VALUES (2, 'issue', 'studio/game#8', 'studio/game#8', '` + at + `')`,
		// 3 closed as completed on GitHub, 4 closed as not planned, 5 as not
		// planned but abandoned here first, with its own reason.
		`INSERT INTO cards (id, kind, ref, ref_key, created_at) VALUES (3, 'issue', 'studio/game#9', 'studio/game#9', '` + at + `')`,
		`INSERT INTO cards (id, kind, ref, ref_key, created_at) VALUES (4, 'issue', 'studio/game#10', 'studio/game#10', '` + at + `')`,
		`INSERT INTO cards (id, kind, ref, ref_key, cancelled_at, cancel_reason, created_at)
			VALUES (5, 'issue', 'studio/game#11', 'studio/game#11', '` + at + `', 'out of scope', '` + at + `')`,
		// 6 an errand requiring 1 and 2, crowning a journey; 7 a petition; 8 a
		// fulfilled errand, a side quest on 6 found on 1; 9 a struck issue.
		`INSERT INTO cards (id, kind, title, created_at) VALUES (6, 'errand', 'Ship it', '` + at + `')`,
		`INSERT INTO cards (id, kind, title, waiting_on, since, created_at) VALUES (7, 'awaiting', 'Key art', 'the artist', '2026-01-01', '` + at + `')`,
		`INSERT INTO cards (id, kind, title, done, side_of, found_while, reason, created_at)
			VALUES (8, 'errand', 'Polish', 1, 6, 1, 'looked rough', '` + at + `')`,
		`INSERT INTO cards (id, kind, ref, ref_key, removed_at, removed_reason, created_at)
			VALUES (9, 'issue', 'studio/game#12', 'studio/game#12', '` + at + `', 'moved', '` + at + `')`,
		`INSERT INTO needs VALUES (6, 1), (6, 2), (6, 7), (1, 3), (1, 4), (1, 5)`,
		`INSERT INTO journeys (id, title, final_card, created_at) VALUES (3, 'Winter', 6, '` + at + `')`,
		`INSERT INTO events (journey_id, card_id, at, kind, text) VALUES (NULL, 1, '` + at + `', 'assign', 'Q1 assigned to @cyd')`,
		`INSERT INTO github_cache (ref_key, ref, title, state, state_reason, assignees, url, fetched_at) VALUES
			('studio/game#7', 'Studio/Game#7', 'Crash on load', 'open', '', '["cyd"]', 'https://github.com/Studio/Game/issues/7', '` + at + `'),
			('studio/game#9', 'studio/game#9', 'Done upstream', 'closed', 'COMPLETED', '[]', 'u', '` + at + `'),
			('studio/game#10', 'studio/game#10', 'Not planned', 'closed', 'NOT_PLANNED', '[]', 'u', '2026-02-01T00:00:00Z'),
			('studio/game#11', 'studio/game#11', 'Dropped', 'closed', 'NOT_PLANNED', '[]', 'u', '` + at + `'),
			('studio/game#12', 'studio/game#12', 'Gone', 'open', '', '[]', 'u', '` + at + `')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	db.Close()

	for i := range int64(2) { // opening it again changes nothing
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		v, err := s.Journey(ctx, "J3")
		if err != nil {
			t.Fatal(err)
		}
		got := map[int64]Card{}
		for _, c := range v.Cards {
			got[c.ID] = c
		}
		for id, want := range map[int64]struct{ title, status string }{
			1: {"Crash on load", StatusAvailable}, 2: {"studio/game#8", StatusAvailable},
			3: {"Done upstream", StatusDone}, 4: {"Not planned", StatusCancelled}, 5: {"Dropped", StatusCancelled},
			6: {"Ship it", StatusLocked}, 7: {"Key art", StatusAwaiting}, 8: {"Polish", StatusDone},
		} {
			c, ok := got[id]
			if !ok || c.Title != want.title || c.Status != want.status || c.URL != "" || c.Mark != "" {
				t.Errorf("%s: %+v, want %q %s", Key(id), c, want.title, want.status)
			}
		}
		if c := got[1]; c.Kind != "" || c.Owner != "ada" || !c.NPC || !c.Working || c.WorkingBy != "cyd" {
			t.Errorf("Q1 lost its state: %+v", c)
		}
		if c := got[4]; c.CancelReason != "closed on GitHub as not planned" {
			t.Errorf("Q4: %+v", c)
		}
		if c := got[5]; c.CancelReason != "out of scope" {
			t.Errorf("Q5 lost its reason: %+v", c)
		}
		if c := got[7]; c.Kind != KindAwaiting || c.WaitingOn != "the artist" || c.Since != "2026-01-01" {
			t.Errorf("Q7 petition: %+v", c)
		}
		if c := got[8]; c.SideOf == nil || *c.SideOf != 6 || c.FoundWhile == nil || *c.FoundWhile != 1 || c.Reason != "looked rough" {
			t.Errorf("Q8 side quest: %+v", c)
		}
		if len(v.Needs) != 6 || len(v.Log) != 1 || v.Log[0].Text != "Q1 assigned to @cyd" {
			t.Errorf("needs %+v log %+v", v.Needs, v.Log)
		}
		var title, reason string
		if err := s.db.QueryRow(`SELECT title, removed_reason FROM cards WHERE id = 9 AND removed_at IS NOT NULL`).Scan(&title, &reason); err != nil || title != "Gone" || reason != "moved" {
			t.Errorf("struck Q9: %q %q %v", title, reason, err)
		}
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'github_cache' OR sql LIKE '%ref_key%'`).Scan(&n); err != nil || n != 0 {
			t.Errorf("GitHub left in the schema: %d %v", n, err)
		}
		// New quests go on from the highest id.
		c, err := s.AddCard(ctx, NewCard{Title: "next"}, "")
		if err != nil || c.ID != 10+i {
			t.Errorf("quest after migration: %+v %v", c, err)
		}
		s.Close()
	}
}
