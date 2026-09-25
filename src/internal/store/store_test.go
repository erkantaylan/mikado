package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"mikado/internal/github"
)

// fakeGitHub is an in-memory GitHub keyed by Ref.Key().
type fakeGitHub struct {
	issues map[string]github.Issue
	fail   error
	calls  int // Issues calls
}

func newFake() *fakeGitHub { return &fakeGitHub{issues: map[string]github.Issue{}} }

func (f *fakeGitHub) put(ref, title, state string, assignees ...string) {
	f.putReason(ref, title, state, "", assignees...)
}

func (f *fakeGitHub) putReason(ref, title, state, reason string, assignees ...string) {
	r, err := github.ParseRef(ref)
	if err != nil {
		panic(err)
	}
	if assignees == nil {
		assignees = []string{}
	}
	f.issues[r.Key()] = github.Issue{Ref: r, Title: title, State: state, StateReason: reason, Assignees: assignees,
		URL: "https://github.com/" + r.Repository() + "/issues/1"}
}

func (f *fakeGitHub) Issues(_ context.Context, refs []github.Ref) (map[string]github.Issue, error) {
	f.calls++
	if f.fail != nil {
		return nil, f.fail
	}
	out := map[string]github.Issue{}
	for _, r := range refs {
		if is, ok := f.issues[r.Key()]; ok {
			out[r.Key()] = is
		}
	}
	return out, nil
}

func (f *fakeGitHub) Assign(_ context.Context, ref github.Ref, add, remove []string) error {
	if f.fail != nil {
		return f.fail
	}
	is, ok := f.issues[ref.Key()]
	if !ok {
		return errors.New("no such issue")
	}
	keep := []string{}
	for _, a := range is.Assignees {
		if !slices.Contains(remove, a) {
			keep = append(keep, a)
		}
	}
	is.Assignees = append(keep, add...)
	f.issues[ref.Key()] = is
	return nil
}

func (f *fakeGitHub) Assignees(context.Context, string, string) ([]string, error) {
	return []string{"bo", "cyd"}, nil
}

type fixture struct {
	t   *testing.T
	s   *Store
	gh  *fakeGitHub
	now time.Time
	ctx context.Context
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, gh: newFake(), now: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC), ctx: context.Background()}
	s, err := Open(filepath.Join(t.TempDir(), "mikado.db"), f.gh)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.now = func() time.Time { return f.now }
	f.s = s
	return f
}

func (f *fixture) quest(slug string, final *Card) {
	f.t.Helper()
	var id *int64
	if final != nil {
		id = &final.ID
	}
	if _, err := f.s.CreateQuest(f.ctx, "Quest "+slug, slug, id); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) add(in NewCard) *Card {
	f.t.Helper()
	c, _, err := f.s.AddCard(f.ctx, in, "")
	if err != nil {
		f.t.Fatalf("add %+v: %v", in, err)
	}
	return c
}

func (f *fixture) errand(title string, needs ...int64) *Card {
	return f.add(NewCard{Kind: KindErrand, Title: title, Needs: needs})
}

func (f *fixture) issue(ref string, links NewCard) *Card {
	links.Kind, links.Ref = KindIssue, ref
	return f.add(links)
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

func (f *fixture) view(slug string) *QuestView {
	f.t.Helper()
	v, err := f.s.Quest(f.ctx, slug)
	if err != nil {
		f.t.Fatal(err)
	}
	return v
}

// members returns the quest's card ids.
func (f *fixture) members(slug string) []int64 {
	var ids []int64
	for _, c := range f.view(slug).Cards {
		ids = append(ids, c.ID)
	}
	return ids
}

func (f *fixture) summary(slug string) QuestSummary {
	f.t.Helper()
	qs, _, err := f.s.Quests(f.ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, q := range qs {
		if q.Slug == slug {
			return q
		}
	}
	f.t.Fatalf("quest %s missing from summaries", slug)
	return QuestSummary{}
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

func slugsOf(rs []QuestRef) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Slug)
	}
	return strings.Join(out, ",")
}

func TestParseCardID(t *testing.T) {
	for _, in := range []string{"M142", "M-142", "m142", "m-142", "c142", "C142", " 142 "} {
		if id, ok := ParseCardID(in); !ok || id != 142 {
			t.Errorf("ParseCardID(%q) = %d, %v", in, id, ok)
		}
	}
	for _, in := range []string{"", "M", "M0", "x142", "studio/game#142", "M14a", "--142"} {
		if id, ok := ParseCardID(in); ok {
			t.Errorf("ParseCardID(%q) accepted as %d", in, id)
		}
	}
	if Key(142) != "M142" {
		t.Errorf("Key = %s", Key(142))
	}
}

func TestResolve(t *testing.T) {
	f := setup(t)
	f.gh.put("Studio/Game#7", "t", "open")
	c := f.issue("studio/game#7", NewCard{})
	for _, ref := range []string{Key(c.ID), fmt.Sprintf("m-%d", c.ID), "STUDIO/game#7", "https://github.com/studio/game/issues/7"} {
		if id, err := f.s.Resolve(f.ctx, ref); err != nil || id != c.ID {
			t.Errorf("Resolve(%q) = %d, %v", ref, id, err)
		}
	}
	if _, err := f.s.Resolve(f.ctx, "studio/game#8"); KindOf(err) != ErrNotFound {
		t.Errorf("unknown issue: %v", err)
	}
	if _, err := f.s.Resolve(f.ctx, "M999"); KindOf(err) != ErrNotFound {
		t.Errorf("unknown id: %v", err)
	}
}

func TestSlugs(t *testing.T) {
	f := setup(t)
	cases := map[string]string{
		"The winter update ships to every player":             "winter-update-ships-every",
		"Split-screen co-op for the winter update":            "split-screen-co-op-winter-update",
		"The level editor goes public for everyone":           "level-editor-goes-public",
		"Controller support on every platform":                "controller-support-every",
		"Supercalifragilistic expialidocious extraordinarily": "supercalifragilistic",
		"The": "",
	}
	for title, want := range cases {
		if got := Slugify(title); got != want || len(got) > maxSlug {
			t.Errorf("Slugify(%q) = %q, want %q", title, got, want)
		}
	}
	q, err := f.s.CreateQuest(f.ctx, "The winter update ships to every player", "", nil)
	if err != nil || q.Slug != "winter-update-ships-every" || q.State != QuestActive {
		t.Fatalf("create: %+v %v", q, err)
	}
	if q2, _ := f.s.CreateQuest(f.ctx, "The winter update ships to every player", "", nil); q2.Slug != q.Slug+"-2" {
		t.Errorf("clash suffix: %s", q2.Slug)
	}
	if _, err := f.s.CreateQuest(f.ctx, "x", q.Slug, nil); KindOf(err) != ErrConflict {
		t.Errorf("explicit taken slug: %v", err)
	}
	if _, err := f.s.CreateQuest(f.ctx, "x", "Bad Slug", nil); KindOf(err) != ErrInvalid {
		t.Errorf("bad explicit slug: %v", err)
	}
	// Slugs are matched case-insensitively.
	if v := f.view("WINTER-Update-Ships-Every"); v.Quest.Slug != q.Slug {
		t.Errorf("case-insensitive lookup: %s", v.Quest.Slug)
	}
	// Rename.
	if _, err := f.s.UpdateQuest(f.ctx, q.Slug, QuestPatch{Slug: ptr("winter-update!")}); KindOf(err) != ErrInvalid {
		t.Errorf("rename to bad slug: %v", err)
	}
	if _, err := f.s.UpdateQuest(f.ctx, q.Slug, QuestPatch{Slug: ptr(q.Slug + "-2")}); KindOf(err) != ErrConflict {
		t.Errorf("rename onto a taken slug: %v", err)
	}
	r, err := f.s.UpdateQuest(f.ctx, q.Slug, QuestPatch{Slug: ptr("Winter-Update"), Title: ptr("Winter update")})
	if err != nil || r.Slug != "winter-update" || r.Title != "Winter update" {
		t.Fatalf("rename: %+v %v", r, err)
	}
	log := f.view("winter-update").Log
	if log[1].Text != "quest renamed from winter-update-ships-every" || !strings.HasPrefix(log[2].Text, "quest retitled from") {
		t.Errorf("rename events: %+v", log)
	}
	if _, err := f.s.Quest(f.ctx, q.Slug); KindOf(err) != ErrNotFound {
		t.Errorf("old slug still resolves: %v", err)
	}
}

func TestStatus(t *testing.T) {
	f := setup(t)
	f.gh.put("studio/saves#93", "Cloud-save adapter", "closed")
	f.gh.put("studio/saves#88", "Save migration", "open", "cyd")

	adapter := f.issue("studio/saves#93", NewCard{})
	migration := f.issue("studio/saves#88", NewCard{Needs: []int64{adapter.ID}})
	token := f.add(NewCard{Kind: KindAwaiting, Title: "Final key art", WaitingOn: "freelance artist", Owner: "ada"})
	release := f.errand("Book feature slot")
	final := f.errand("Ship it", migration.ID, token.ID, release.ID)
	f.quest("winter", final)

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
	if c := f.card(migration.ID); c.Title != "Save migration" || c.State != "open" || len(c.Assignees) != 1 {
		t.Errorf("issue data not filled from GitHub: %+v", c)
	}
	if v.Quest.FinalCardID == nil || *v.Quest.FinalCardID != final.ID {
		t.Errorf("finalCardId = %v", v.Quest.FinalCardID)
	}
	f.patch(token.ID, CardPatch{Done: ptr(true)})
	f.patch(release.ID, CardPatch{Done: ptr(true)})
	f.gh.put("studio/saves#88", "Save migration", "closed", "cyd")
	f.now = f.now.Add(CacheTTL)
	if c := f.card(final.ID); c.Status != StatusAvailable || c.OpenBefore != 0 {
		t.Errorf("final after prerequisites done: %s/%d", c.Status, c.OpenBefore)
	}
	if _, err := f.s.UpdateCard(f.ctx, migration.ID, CardPatch{Done: ptr(true)}, ""); KindOf(err) != ErrInvalid {
		t.Errorf("done on an issue: %v", err)
	}
}

func TestSharedIssueOneCardTwoQuests(t *testing.T) {
	f := setup(t)
	f.gh.put("studio/saves#88", "Save migration", "open")
	shared := f.issue("studio/saves#88", NewCard{})
	a := f.errand("ship A", shared.ID)
	b := f.errand("ship B", shared.ID)
	f.quest("qa", a)
	f.quest("qb", b)
	for slug, other := range map[string]string{"qa": "qb", "qb": "qa"} {
		var found *Card
		for _, c := range f.view(slug).Cards {
			if c.ID == shared.ID {
				found = &c
			}
		}
		if found == nil {
			t.Fatalf("%s: shared issue is not a member", slug)
		}
		if slugsOf(found.AlsoIn) != other {
			t.Errorf("%s: alsoIn = %v, want %s", slug, found.AlsoIn, other)
		}
	}
	cv, _ := f.s.CardView(f.ctx, shared.ID)
	if slugsOf(cv.Quests) != "qa,qb" || slugsOf(cv.Card.AlsoIn) != "qa,qb" || len(cv.NeededBy) != 2 {
		t.Errorf("card view: quests %v alsoIn %v neededBy %d", cv.Quests, cv.Card.AlsoIn, len(cv.NeededBy))
	}
	// Closing it on GitHub completes it in both.
	f.gh.put("studio/saves#88", "Save migration", "closed")
	f.now = f.now.Add(CacheTTL)
	for _, slug := range []string{"qa", "qb"} {
		for _, c := range f.view(slug).Cards {
			if c.ID == shared.ID && c.Status != StatusDone {
				t.Errorf("%s: shared status %s", slug, c.Status)
			}
			if c.Final && c.Status != StatusAvailable {
				t.Errorf("%s: final not unlocked: %s", slug, c.Status)
			}
		}
		if s := f.summary(slug); s.Main != (Progress{1, 2}) {
			t.Errorf("%s: main %+v", slug, s.Main)
		}
	}
}

func TestIdempotentAddAppliesLinks(t *testing.T) {
	f := setup(t)
	f.gh.put("Studio/Game#140", "Ship the update", "open")
	first, created, err := f.s.AddCard(f.ctx, NewCard{Kind: KindIssue, Ref: "studio/game#140"}, "")
	if err != nil || !created || first.Ref != "Studio/Game#140" || len(first.AlsoIn) != 0 {
		t.Fatalf("first add: %+v %v %v", first, created, err)
	}
	f.quest("qa", nil)
	finalB := f.errand("ship B")
	f.quest("qb", finalB)
	calls := f.gh.calls
	again, created, err := f.s.AddCard(f.ctx, NewCard{Kind: KindIssue, Ref: "STUDIO/game#140", FinalOf: "QA", NeededBy: []int64{finalB.ID}}, "")
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("second add: created=%v card=%+v err=%v", created, again, err)
	}
	if f.gh.calls != calls {
		t.Errorf("a duplicate add asked GitHub again")
	}
	if slugsOf(again.AlsoIn) != "qa,qb" || !again.Final {
		t.Errorf("links not applied: alsoIn %v final %v", again.AlsoIn, again.Final)
	}
	// Once more with the same links: nothing new.
	if _, _, err := f.s.AddCard(f.ctx, NewCard{Kind: KindIssue, Ref: "studio/game#140", FinalOf: "qa", NeededBy: []int64{finalB.ID}}, ""); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, e := range f.view("qb").Log {
		texts = append(texts, e.Text)
	}
	want := []string{
		fmt.Sprintf("%s added", Key(first.ID)),
		fmt.Sprintf("%s errand added", Key(finalB.ID)),
		"quest created",
		fmt.Sprintf("crowning deed set to %s", Key(finalB.ID)),
		fmt.Sprintf("%s now requires %s", Key(finalB.ID), Key(first.ID)),
	}
	if strings.Join(texts, "\n") != strings.Join(want, "\n") {
		t.Errorf("qb log:\n%s\nwant:\n%s", strings.Join(texts, "\n"), strings.Join(want, "\n"))
	}
	// Side-of on an existing card that takes part in needs is refused.
	if _, _, err := f.s.AddCard(f.ctx, NewCard{Kind: KindIssue, Ref: "studio/game#140", SideOf: &finalB.ID}, ""); KindOf(err) != ErrConflict {
		t.Errorf("side-of on a needed card: %v", err)
	}
}

func TestMembershipFollowsNeeds(t *testing.T) {
	f := setup(t)
	y := f.errand("y")
	x := f.errand("x", y.ID)
	up := f.errand("upstream")
	a := f.errand("final A", x.ID, up.ID)
	b := f.errand("final B", up.ID)
	loose := f.errand("loose")
	f.quest("qa", a)
	f.quest("qb", b)
	if got := f.members("qa"); !slices.Equal(got, []int64{y.ID, x.ID, up.ID, a.ID}) {
		t.Errorf("qa members %v", got)
	}
	if got := f.members("qb"); !slices.Equal(got, []int64{up.ID, b.ID}) {
		t.Errorf("qb members %v", got)
	}
	if cv, _ := f.s.CardView(f.ctx, loose.ID); len(cv.Quests) != 0 {
		t.Errorf("a card with no links is in %v", cv.Quests)
	}
	// Cancelled cards stay members.
	f.cancel(y.ID, "no")
	if !slices.Contains(f.members("qa"), y.ID) {
		t.Errorf("cancelled card left the quest")
	}
	// Cutting x→y drops y out of qa.
	if err := f.s.RemoveNeed(f.ctx, x.ID, y.ID); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(f.members("qa"), y.ID) {
		t.Errorf("unneeded card still a member")
	}
	// A quest with no final has no members and still shows its log.
	f.quest("empty", nil)
	if v := f.view("empty"); len(v.Cards) != 0 || v.Quest.State != QuestActive || len(v.Log) != 1 {
		t.Errorf("empty quest: %+v", v)
	}
}

func TestSideQuestMembershipFollowsItsCard(t *testing.T) {
	f := setup(t)
	x := f.errand("x")
	a := f.errand("final", x.ID)
	f.quest("qa", a)
	side := f.add(NewCard{Kind: KindErrand, Title: "polish x", SideOf: &x.ID})
	sideOfSide := f.add(NewCard{Kind: KindErrand, Title: "polish the polish", SideOf: &side.ID})
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

func TestCycleRejectedAcrossQuests(t *testing.T) {
	f := setup(t)
	shared := f.errand("shared")
	a := f.errand("final A", shared.ID)
	b := f.errand("final B", shared.ID)
	f.quest("qa", a)
	f.quest("qb", b)
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
	if _, _, err := f.s.AddCard(f.ctx, NewCard{Kind: KindErrand, Title: "d", Needs: []int64{a.ID}, NeededBy: []int64{shared.ID}}, ""); KindOf(err) != ErrConflict {
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

func TestQuestLogShowsCurrentMembers(t *testing.T) {
	f := setup(t)
	f.gh.put("studio/saves#88", "Save migration", "open")
	migration := f.issue("studio/saves#88", NewCard{})
	a := f.errand("final A", migration.ID)
	f.quest("qa", a)
	b := f.errand("final B")
	f.quest("qb", b)
	found := f.add(NewCard{Kind: KindErrand, Title: "fix the old-save crash", FoundWhile: &migration.ID, Reason: "old saves crash the loader", NeededBy: []int64{migration.ID}})
	f.patch(found.ID, CardPatch{Working: ptr(true), WorkingBy: ptr("cyd")})

	texts := func(slug string) string {
		var out []string
		for _, e := range f.view(slug).Log {
			out = append(out, e.Text)
		}
		return strings.Join(out, "\n")
	}
	want := strings.Join([]string{
		Key(migration.ID) + " added",
		Key(a.ID) + " errand added — requires " + Key(migration.ID),
		"quest created",
		"crowning deed set to " + Key(a.ID),
		Key(found.ID) + " errand unearthed while on " + Key(migration.ID) + ": old saves crash the loader — opens " + Key(migration.ID),
		Key(found.ID) + " taken up by cyd",
	}, "\n")
	if got := texts("qa"); got != want {
		t.Errorf("qa log:\n%s\nwant:\n%s", got, want)
	}
	if got := texts("qb"); got != Key(b.ID)+" errand added\nquest created\ncrowning deed set to "+Key(b.ID) {
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
	f.gh.issues["studio/game#7"] = github.Issue{Ref: github.Ref{Owner: "studio", Repo: "game", Number: 7}, IsPR: true}
	side := f.add(NewCard{Kind: KindErrand, Title: "s", SideOf: ptr(f.errand("m").ID)})
	cases := map[string]struct {
		in   NewCard
		kind ErrKind
	}{
		"missing issue":  {NewCard{Kind: KindIssue, Ref: "studio/game#999"}, ErrInvalid},
		"pull request":   {NewCard{Kind: KindIssue, Ref: "studio/game#7"}, ErrInvalid},
		"bad ref":        {NewCard{Kind: KindIssue, Ref: "game#7"}, ErrInvalid},
		"no title":       {NewCard{Kind: KindErrand}, ErrInvalid},
		"no waitingOn":   {NewCard{Kind: KindAwaiting, Title: "t"}, ErrInvalid},
		"bad kind":       {NewCard{Kind: "task", Title: "t"}, ErrInvalid},
		"unknown need":   {NewCard{Kind: KindErrand, Title: "t", Needs: []int64{999}}, ErrNotFound},
		"unknown quest":  {NewCard{Kind: KindErrand, Title: "t", FinalOf: "nope"}, ErrNotFound},
		"final globally": {NewCard{Kind: KindErrand, Title: "t", Final: true}, ErrInvalid},
		"need a side":    {NewCard{Kind: KindErrand, Title: "t", Needs: []int64{side.ID}}, ErrInvalid},
		"side with need": {NewCard{Kind: KindErrand, Title: "t", SideOf: &side.ID, Needs: []int64{side.ID}}, ErrInvalid},
	}
	for name, tc := range cases {
		if _, _, err := f.s.AddCard(f.ctx, tc.in, ""); KindOf(err) != tc.kind {
			t.Errorf("%s: got %v (kind %d), want kind %d", name, err, KindOf(err), tc.kind)
		}
	}
	f.gh.fail = errors.New("gh: network down")
	if _, _, err := f.s.AddCard(f.ctx, NewCard{Kind: KindIssue, Ref: "studio/game#1"}, ""); KindOf(err) != ErrUpstream {
		t.Errorf("gh failing on add: %v", err)
	}
}

func TestSoftRemoveCascades(t *testing.T) {
	f := setup(t)
	a := f.errand("a")
	b := f.errand("b", a.ID)
	side := f.add(NewCard{Kind: KindErrand, Title: "side of b", SideOf: &b.ID})
	final := f.errand("final", b.ID)
	f.quest("q", final)
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
	// Removing a quest's final leaves the quest without one.
	if err := f.s.RemoveCard(f.ctx, final.ID, "whole thing dropped"); err != nil {
		t.Fatal(err)
	}
	v := f.view("q")
	if v.Quest.FinalCardID != nil || len(v.Cards) != 0 {
		t.Errorf("quest after its final was removed: %+v", v.Quest)
	}
	if last := v.Log[len(v.Log)-1]; last.Text != "crowning deed "+Key(final.ID)+" struck: whole thing dropped" {
		t.Errorf("last quest event: %+v", last)
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
	blocker := f.errand("blocker")
	main := f.errand("main", blocker.ID)
	f.quest("q", main)
	side := f.add(NewCard{Kind: KindErrand, Title: "polish", SideOf: &main.ID})
	if c := f.card(side.ID); c.Status != StatusAvailable {
		t.Errorf("side quest of a locked card: %s", c.Status)
	}
	if _, err := f.s.AddNeed(f.ctx, main.ID, side.ID); KindOf(err) != ErrInvalid {
		t.Errorf("main needs side: %v", err)
	}
	if _, err := f.s.AddNeed(f.ctx, side.ID, blocker.ID); KindOf(err) != ErrInvalid {
		t.Errorf("side needs blocker: %v", err)
	}
	if _, err := f.s.UpdateQuest(f.ctx, "q", QuestPatch{Final: &side.ID}); KindOf(err) != ErrInvalid {
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
	a := f.errand("a")
	b := f.errand("b")
	f.quest("q", a)
	if _, err := f.s.UpdateQuest(f.ctx, "q", QuestPatch{Final: &b.ID}); err != nil {
		t.Fatal(err)
	}
	v := f.view("q")
	if *v.Quest.FinalCardID != b.ID || v.Log[len(v.Log)-1].Text != fmt.Sprintf("crowning deed changed from %s to %s", Key(a.ID), Key(b.ID)) {
		t.Errorf("final change: %v %+v", *v.Quest.FinalCardID, v.Log)
	}
	// Quest-scoped patch sets the final of that quest; the global one refuses.
	if _, err := f.s.UpdateCard(f.ctx, a.ID, CardPatch{Final: ptr(true)}, ""); KindOf(err) != ErrInvalid {
		t.Errorf("global final patch: %v", err)
	}
	c, err := f.s.UpdateCard(f.ctx, a.ID, CardPatch{Final: ptr(true)}, "q")
	if err != nil || !c.Final {
		t.Fatalf("quest-scoped final: %+v %v", c, err)
	}
	c, err = f.s.UpdateCard(f.ctx, a.ID, CardPatch{Final: ptr(false)}, "q")
	if err != nil || c.Final || f.view("q").Quest.FinalCardID != nil {
		t.Fatalf("unset final: %+v %v", c, err)
	}
	// Quest-scoped add with final: true.
	d, _, err := f.s.AddCard(f.ctx, NewCard{Kind: KindErrand, Title: "d", Final: true}, "q")
	if err != nil || !d.Final || *f.view("q").Quest.FinalCardID != d.ID {
		t.Fatalf("quest-scoped add final: %+v %v", d, err)
	}
}

func TestCancelledUnblocks(t *testing.T) {
	f := setup(t)
	a := f.errand("a")
	b := f.errand("b")
	top := f.errand("top", a.ID, b.ID)
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
	a := f.errand("a")
	b := f.errand("b")
	c := f.errand("c")
	final := f.errand("final", a.ID, b.ID, c.ID)
	f.quest("q", final)
	sideA := f.add(NewCard{Kind: KindErrand, Title: "polish a", SideOf: &a.ID})
	f.add(NewCard{Kind: KindErrand, Title: "polish b", SideOf: &b.ID})
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
	main := f.errand("main")
	side := f.add(NewCard{Kind: KindErrand, Title: "side", SideOf: &main.ID})
	sideOfSide := f.add(NewCard{Kind: KindErrand, Title: "side of side", SideOf: &side.ID})
	other := f.errand("other")
	otherSide := f.add(NewCard{Kind: KindErrand, Title: "other side", SideOf: &other.ID})
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
	f.s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE kind = 'cancel' AND text LIKE '%abandoned with its deed ` + Key(main.ID) + `: out of scope'`).Scan(&n)
	if n != 2 {
		t.Errorf("cascade cancel events: %d", n)
	}
}

func TestGitHubUnplannedIsCancelled(t *testing.T) {
	f := setup(t)
	f.gh.putReason("studio/a#1", "t", "closed", "NOT_PLANNED")
	f.gh.putReason("studio/a#2", "t", "closed", "DUPLICATE")
	f.gh.putReason("studio/a#3", "t", "closed", "COMPLETED")
	unplanned := f.issue("studio/a#1", NewCard{})
	dup := f.issue("studio/a#2", NewCard{})
	completed := f.issue("studio/a#3", NewCard{})
	top := f.errand("top", unplanned.ID, dup.ID)
	for _, id := range []int64{unplanned.ID, dup.ID} {
		if c := f.card(id); c.Status != StatusCancelled || !c.Cancelled || c.Done || c.StateReason == "" {
			t.Errorf("%s: %+v", Key(id), c)
		}
	}
	if c := f.card(completed.ID); c.Status != StatusDone || c.Cancelled || c.StateReason != "COMPLETED" {
		t.Errorf("completed issue: %+v", c)
	}
	if c := f.card(top.ID); c.Status != StatusAvailable {
		t.Errorf("GitHub-cancelled needs still block: %s", c.Status)
	}
}

func TestLocalCancelOfOpenIssue(t *testing.T) {
	f := setup(t)
	f.gh.put("studio/a#5", "still open upstream", "open")
	c := f.issue("studio/a#5", NewCard{})
	got := f.cancel(c.ID, "not part of this plan anymore")
	if got.Status != StatusCancelled || got.State != "open" || got.CancelReason == "" {
		t.Errorf("locally cancelled open issue: %+v", got)
	}
}

func TestQuestState(t *testing.T) {
	f := setup(t)
	final := f.errand("ship")
	f.quest("q", final)
	if f.view("q").Quest.State != QuestActive || f.summary("q").State != QuestActive {
		t.Errorf("new quest not active")
	}
	f.patch(final.ID, CardPatch{Done: ptr(true)})
	if f.view("q").Quest.State != QuestComplete || f.summary("q").State != QuestComplete {
		t.Errorf("final done: not complete")
	}
	f.cancel(final.ID, "whole goal dropped")
	if v, s := f.view("q").Quest.State, f.summary("q").State; v != QuestCancelled || s != QuestCancelled {
		t.Errorf("final cancelled: view %s summary %s", v, s)
	}
}

func TestWorking(t *testing.T) {
	f := setup(t)
	a := f.errand("a")
	b := f.errand("b")
	f.quest("q", f.errand("final", a.ID, b.ID))
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

// A closed issue cannot be taken up: its state lives on GitHub, not in the
// local done flag.
func TestTakeUpClosedIssue(t *testing.T) {
	f := setup(t)
	f.gh.put("studio/saves#93", "Cloud-save adapter", "closed")
	f.gh.putReason("studio/saves#95", "Split-screen co-op", "closed", "NOT_PLANNED")
	f.gh.put("studio/saves#88", "Save migration", "open")
	done := f.issue("studio/saves#93", NewCard{})
	dropped := f.issue("studio/saves#95", NewCard{})
	open := f.issue("studio/saves#88", NewCard{})
	for _, c := range []*Card{done, dropped} {
		_, err := f.s.UpdateCard(f.ctx, c.ID, CardPatch{Working: ptr(true)}, "")
		if KindOf(err) != ErrInvalid || !strings.Contains(err.Error(), "reopen it there") {
			t.Errorf("take up closed %s: %v", c.Key, err)
		}
	}
	if got := f.patch(open.ID, CardPatch{Working: ptr(true)}); !got.Working {
		t.Errorf("take up an open issue: %+v", got)
	}
}

func TestAssign(t *testing.T) {
	f := setup(t)
	f.gh.put("studio/saves#88", "Save migration", "open")
	migration := f.issue("studio/saves#88", NewCard{})
	c, err := f.s.Assign(f.ctx, migration.ID, []string{"@cyd"}, nil, "")
	if err != nil || strings.Join(c.Assignees, ",") != "cyd" {
		t.Fatalf("assign: %+v %v", c, err)
	}
	if c, _ = f.s.Assign(f.ctx, migration.ID, nil, []string{"cyd"}, ""); len(c.Assignees) != 0 {
		t.Errorf("unassign: %v", c.Assignees)
	}
	if _, err := f.s.Assign(f.ctx, f.errand("e").ID, []string{"cyd"}, nil, ""); KindOf(err) != ErrInvalid {
		t.Errorf("assign on an errand: %v", err)
	}
	if _, err := f.s.Assign(f.ctx, migration.ID, []string{"not a login"}, nil, ""); KindOf(err) != ErrInvalid {
		t.Errorf("bad login: %v", err)
	}
}

func TestGitHubDownServesCache(t *testing.T) {
	f := setup(t)
	f.gh.put("studio/game#1", "Cached title", "open")
	c := f.issue("studio/game#1", NewCard{})
	f.quest("q", c)
	f.now = f.now.Add(2 * CacheTTL)
	f.gh.fail = errors.New("gh: offline")
	v := f.view("q")
	if v.GitHub == "" || v.Cards[0].Title != "Cached title" {
		t.Errorf("gh down: warning %q card %+v", v.GitHub, v.Cards[0])
	}
	f.gh.fail = nil
	f.view("q")
	calls := f.gh.calls
	f.view("q")
	if f.gh.calls != calls {
		t.Errorf("fresh cache refetched")
	}
}

func TestArchive(t *testing.T) {
	f := setup(t)
	final := f.errand("ship")
	f.quest("q", final)
	archive := func(on bool) {
		t.Helper()
		if _, err := f.s.UpdateQuest(f.ctx, "Q", QuestPatch{Archived: ptr(on)}); err != nil {
			t.Fatal(err)
		}
	}
	archive(true)
	archive(true) // already archived: no second event
	if f.summary("q").ArchivedAt == "" || f.view("q").Quest.ArchivedAt == "" {
		t.Errorf("not archived")
	}
	if got := len(f.members("q")); got != 1 {
		t.Errorf("archiving changed the quest's deeds: %d members", got)
	}
	archive(false)
	if f.summary("q").ArchivedAt != "" || f.view("q").Quest.ArchivedAt != "" {
		t.Errorf("still archived")
	}
	var texts []string
	for _, e := range f.view("q").Log {
		if e.Kind == "archive" {
			texts = append(texts, e.Text)
		}
	}
	if strings.Join(texts, "; ") != "quest archived; quest brought back from the archive" {
		t.Errorf("chronicle: %q", texts)
	}
}

func TestSearch(t *testing.T) {
	f := setup(t)
	f.gh.put("studio/saves#88", "Old saves crash the loader", "OPEN")
	final := f.errand("Ship the winter update")
	f.quest("winter", final)
	issue := f.issue("studio/saves#88", NewCard{NeededBy: []int64{final.ID}})
	notes := f.errand("old notes")
	f.patch(notes.ID, CardPatch{Done: ptr(true)})
	f.need(final.ID, notes.ID)
	tr := f.errand("İzin ekranı")
	calls := f.gh.calls

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

	if r := search("OLD"); keys(r.Deeds) != issue.Key+" "+notes.Key {
		t.Errorf("OLD: %s, want the open issue before the fulfilled errand", keys(r.Deeds))
	}
	if r := search("saves loader"); keys(r.Deeds) != issue.Key || len(r.Deeds[0].AlsoIn) != 1 || r.Deeds[0].AlsoIn[0].Slug != "winter" {
		t.Errorf("saves loader: %+v", r.Deeds)
	}
	if r := search("izin"); keys(r.Deeds) != tr.Key {
		t.Errorf("izin: %s, want the dotted İ to match", keys(r.Deeds))
	}
	for _, q := range []string{issue.Key, "m-" + strconv.FormatInt(issue.ID, 10), "studio/saves#88"} {
		if r := search(q); r.Exact == nil || r.Exact.ID != issue.ID || r.Deeds[0].ID != issue.ID {
			t.Errorf("%s: exact %+v", q, r.Exact)
		}
	}
	if r := search("winter"); len(r.Quests) != 1 || r.Quests[0].Slug != "winter" || keys(r.Deeds) != final.Key || !r.Deeds[0].Final {
		t.Errorf("winter: quests %+v deeds %s", r.Quests, keys(r.Deeds))
	}
	if _, err := f.s.UpdateQuest(f.ctx, "winter", QuestPatch{Archived: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if r := search(""); len(r.Quests) != 0 || len(r.Deeds) != 0 {
		t.Errorf("empty query lists archived quests or deeds: %+v", r)
	}
	if r := search("winter"); len(r.Quests) != 1 {
		t.Errorf("a query still finds an archived quest: %+v", r.Quests)
	}
	if f.gh.calls != calls {
		t.Errorf("search called GitHub %d times; it reads the cache only", f.gh.calls-calls)
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
