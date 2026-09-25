package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"mikado/internal/github"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// stopWords are left out of derived slugs.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "with": true, "in": true, "on": true, "for": true, "of": true,
	"to": true, "and": true, "or": true, "at": true, "by": true, "from": true, "into": true, "is": true,
	"are": true, "be": true, "its": true, "it": true, "as": true, "via": true,
}

// maxSlug caps derived slugs.
const maxSlug = 32

// Slugify derives a short slug from a title: its first four significant
// words (stop-words dropped; a hyphenated word counts as one), lowercase
// ASCII letters and digits joined by dashes, at most 32 characters.
func Slugify(title string) string {
	var words []string
	for _, field := range strings.Fields(strings.ToLower(title)) {
		var parts []string
		for _, p := range strings.FieldsFunc(field, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') }) {
			parts = append(parts, p)
		}
		w := strings.Join(parts, "-")
		if w == "" || stopWords[w] {
			continue
		}
		words = append(words, w)
		if len(words) == 4 {
			break
		}
	}
	s := strings.Join(words, "-")
	for len(s) > maxSlug {
		i := strings.LastIndexByte(s, '-')
		if i <= 0 {
			s = s[:maxSlug]
			break
		}
		s = s[:i]
	}
	return strings.Trim(s, "-")
}

func checkSlug(slug string) error {
	if !slugRe.MatchString(slug) {
		return errf(ErrInvalid, "slug %q: use lowercase letters, digits and single dashes", slug)
	}
	return nil
}

func (s *Store) event(ctx context.Context, q querier, questID, cardID *int64, kind, text string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO events (quest_id, card_id, at, kind, text) VALUES (?, ?, ?, ?, ?)`,
		questID, cardID, s.stamp(), kind, text)
	return err
}

// cardEvent logs a change to a card; it shows in the log of every quest the
// card is a member of.
func (s *Store) cardEvent(ctx context.Context, q querier, id int64, kind, text string) error {
	return s.event(ctx, q, nil, &id, kind, text)
}

func keys(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = Key(id)
	}
	return strings.Join(parts, ", ")
}

func quoted(t string) string {
	if utf8.RuneCountInString(t) > 40 {
		t = string([]rune(t)[:39]) + "…"
	}
	return "“" + t + "”"
}

// CreateQuest creates a quest, optionally with its final card. With no slug
// one is derived from the title (suffixed -2, -3… if taken); an explicit
// slug that is taken is a conflict.
func (s *Store) CreateQuest(ctx context.Context, title, slug string, final *int64) (*QuestSummary, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errf(ErrInvalid, "a quest needs a title")
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	explicit := slug != ""
	if !explicit {
		if slug = Slugify(title); slug == "" {
			slug = "quest"
		}
	} else if err := checkSlug(slug); err != nil {
		return nil, err
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		base := slug
		for n := 2; ; n++ {
			var exists int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM quests WHERE slug = ?`, slug).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				break
			}
			if explicit {
				return errf(ErrConflict, "a quest with slug %q already exists", slug)
			}
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		var id int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO quests (slug, title, created_at) VALUES (?, ?, ?) RETURNING id`,
			slug, title, s.stamp()).Scan(&id); err != nil {
			return err
		}
		if err := s.event(ctx, tx, &id, nil, "create", "quest created"); err != nil {
			return err
		}
		if final != nil {
			return s.setFinal(ctx, tx, &questRow{ID: id, Slug: slug}, *final)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.summary(ctx, slug)
}

func (s *Store) summary(ctx context.Context, slug string) (*QuestSummary, error) {
	qs, _, err := s.Quests(ctx)
	if err != nil {
		return nil, err
	}
	for i := range qs {
		if qs[i].Slug == slug {
			return &qs[i], nil
		}
	}
	return nil, errf(ErrNotFound, "no quest %q", slug)
}

// UpdateQuest renames a quest (slug and/or title), archives or unarchives it,
// or sets its final card.
func (s *Store) UpdateQuest(ctx context.Context, slug string, p QuestPatch) (*QuestSummary, error) {
	q, err := getQuest(ctx, s.db, slug)
	if err != nil {
		return nil, err
	}
	newSlug := q.Slug
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if p.Slug != nil {
			ns := strings.ToLower(strings.TrimSpace(*p.Slug))
			if err := checkSlug(ns); err != nil {
				return err
			}
			if ns != q.Slug {
				var n int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM quests WHERE slug = ?`, ns).Scan(&n); err != nil {
					return err
				}
				if n > 0 {
					return errf(ErrConflict, "a quest with slug %q already exists", ns)
				}
				if _, err := tx.ExecContext(ctx, `UPDATE quests SET slug = ? WHERE id = ?`, ns, q.ID); err != nil {
					return err
				}
				if err := s.event(ctx, tx, &q.ID, nil, "edit", "quest renamed from "+q.Slug); err != nil {
					return err
				}
				newSlug = ns
			}
		}
		if p.Title != nil {
			t := strings.TrimSpace(*p.Title)
			if t == "" {
				return errf(ErrInvalid, "a quest needs a title")
			}
			if t != q.Title {
				if _, err := tx.ExecContext(ctx, `UPDATE quests SET title = ? WHERE id = ?`, t, q.ID); err != nil {
					return err
				}
				if err := s.event(ctx, tx, &q.ID, nil, "edit", "quest retitled from "+quoted(q.Title)); err != nil {
					return err
				}
			}
		}
		if p.Archived != nil && *p.Archived != (q.ArchivedAt != "") {
			var at any // NULL: brought back
			text := "quest brought back from the archive"
			if *p.Archived {
				at, text = s.stamp(), "quest archived"
			}
			if _, err := tx.ExecContext(ctx, `UPDATE quests SET archived_at = ? WHERE id = ?`, at, q.ID); err != nil {
				return err
			}
			if err := s.event(ctx, tx, &q.ID, nil, "archive", text); err != nil {
				return err
			}
		}
		if p.Final != nil {
			return s.setFinal(ctx, tx, q, *p.Final)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.summary(ctx, newSlug)
}

// setFinal makes a live, non-side card the quest's final card.
func (s *Store) setFinal(ctx context.Context, tx *sql.Tx, q *questRow, id int64) error {
	c, err := liveCard(ctx, tx, id)
	if err != nil {
		return err
	}
	if c.SideOf != nil {
		return errf(ErrInvalid, "%s is a side quest; a side quest cannot be a quest's crowning deed", Key(id))
	}
	var old sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT q.final_card FROM quests q JOIN cards c ON c.id = q.final_card AND c.removed_at IS NULL WHERE q.id = ?`, q.ID).Scan(&old); err != nil && err != sql.ErrNoRows {
		return err
	}
	if old.Valid && old.Int64 == id {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE quests SET final_card = ? WHERE id = ?`, id, q.ID); err != nil {
		return err
	}
	text := "crowning deed set to " + Key(id)
	if old.Valid {
		text = fmt.Sprintf("crowning deed changed from %s to %s", Key(old.Int64), Key(id))
	}
	return s.event(ctx, tx, &q.ID, &id, "final", text)
}

// clearFinal leaves the quest without a final card.
func (s *Store) clearFinal(ctx context.Context, tx *sql.Tx, q *questRow, text string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE quests SET final_card = NULL WHERE id = ?`, q.ID); err != nil {
		return err
	}
	return s.event(ctx, tx, &q.ID, q.Final, "final", text)
}

// AddCard adds a card to the global graph and links it as asked. An issue
// that already has a live card is not added again: that card is returned
// (created=false) with the requested links applied to it. viewing (a slug,
// or "") only shapes the returned card; see card.
func (s *Store) AddCard(ctx context.Context, in NewCard, viewing string) (*Card, bool, error) {
	in.Title, in.Owner, in.WaitingOn, in.Reason = strings.TrimSpace(in.Title), strings.TrimSpace(in.Owner), strings.TrimSpace(in.WaitingOn), strings.TrimSpace(in.Reason)
	if in.Final {
		if viewing == "" {
			return nil, false, errf(ErrInvalid, "a crowning deed needs a quest: use finalOf")
		}
		in.FinalOf = viewing
	}
	var finalOf *questRow
	if in.FinalOf != "" {
		q, err := getQuest(ctx, s.db, in.FinalOf)
		if err != nil {
			return nil, false, err
		}
		finalOf = q
	}
	if in.SideOf != nil && (finalOf != nil || len(in.Needs) > 0 || len(in.NeededBy) > 0) {
		return nil, false, errf(ErrInvalid, "a side quest never blocks anything: it cannot crown a quest, require or open a deed")
	}

	var ref github.Ref
	var err error
	switch in.Kind {
	case KindIssue:
		if ref, err = github.ParseRef(in.Ref); err != nil {
			return nil, false, errf(ErrInvalid, "%v", err)
		}
		if in.Title != "" || in.WaitingOn != "" {
			return nil, false, errf(ErrInvalid, "an issue takes its title from GitHub; title and waitingOn are for errands and petitions")
		}
		var existing int64
		err := s.db.QueryRowContext(ctx, `SELECT id FROM cards WHERE ref_key = ? AND removed_at IS NULL`, ref.Key()).Scan(&existing)
		if err == nil {
			if err := s.inTx(ctx, func(tx *sql.Tx) error { return s.link(ctx, tx, existing, in, finalOf) }); err != nil {
				return nil, false, err
			}
			c, err := s.card(ctx, existing, viewing)
			return c, false, err
		} else if err != sql.ErrNoRows {
			return nil, false, err
		}
	case KindErrand, KindAwaiting:
		if in.Ref != "" {
			return nil, false, errf(ErrInvalid, "only issues have a ref")
		}
		if in.Title == "" {
			return nil, false, errf(ErrInvalid, "%s needs a title", kindNoun(in.Kind))
		}
		if in.Kind == KindAwaiting && in.WaitingOn == "" {
			return nil, false, errf(ErrInvalid, "a petition needs waitingOn: who it awaits a reply from")
		}
		if in.Kind == KindErrand && in.WaitingOn != "" {
			return nil, false, errf(ErrInvalid, "waitingOn is for petitions")
		}
	default:
		return nil, false, errf(ErrInvalid, "kind must be issue, errand or awaiting (a petition), not %q", in.Kind)
	}

	// Check the issue on GitHub before opening the transaction.
	if in.Kind == KindIssue {
		if s.gh == nil {
			return nil, false, errf(ErrUpstream, "GitHub is not configured")
		}
		found, err := s.gh.Issues(ctx, []github.Ref{ref})
		if err != nil {
			return nil, false, errf(ErrUpstream, "cannot check %s on GitHub: %v", ref, err)
		}
		is, ok := found[ref.Key()]
		switch {
		case !ok:
			return nil, false, errf(ErrInvalid, "%s does not exist on GitHub (or gh cannot see it)", ref)
		case is.IsPR:
			return nil, false, errf(ErrInvalid, "%s is a pull request; mikado tracks issues — add the issue it resolves instead", ref)
		}
		if err := s.saveCache(ctx, s.db, is); err != nil {
			return nil, false, err
		}
		ref = is.Ref // GitHub's casing
	}

	var id int64
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		if in.SideOf != nil {
			if _, err = liveCard(ctx, tx, *in.SideOf); err != nil {
				return err
			}
		}
		if in.FoundWhile != nil {
			if _, err = liveCard(ctx, tx, *in.FoundWhile); err != nil {
				return err
			}
		}
		if err := mainCards(ctx, tx, append(append([]int64{}, in.Needs...), in.NeededBy...)); err != nil {
			return err
		}
		var refStr, refKey any
		since := ""
		if in.Kind == KindIssue {
			refStr, refKey = ref.String(), ref.Key()
		}
		if in.Kind == KindAwaiting {
			since = s.now().Format("2006-01-02")
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO cards (kind, ref, ref_key, title, owner, waiting_on, since,
				side_of, found_while, reason, npc, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			in.Kind, refStr, refKey, in.Title, in.Owner, in.WaitingOn, since,
			in.SideOf, in.FoundWhile, in.Reason, b2i(in.NPC), s.stamp()).Scan(&id)
		if err != nil {
			return err
		}
		for _, n := range in.Needs {
			if _, err := addNeed(ctx, tx, id, n); err != nil {
				return err
			}
		}
		for _, n := range in.NeededBy {
			if _, err := addNeed(ctx, tx, n, id); err != nil {
				return err
			}
		}

		// "M9 errand added as a side quest on M4 — opens M3", or for a deed found
		// on the way, "M9 unearthed while on M3: old saves crash the loader — opens M3".
		head := Key(id)
		switch in.Kind {
		case KindErrand:
			head += " errand"
		case KindAwaiting:
			head += " petition"
		}
		var details []string
		if in.FoundWhile != nil {
			head += " unearthed while on " + Key(*in.FoundWhile)
			if in.Reason != "" {
				head += ": " + in.Reason
			}
			if in.SideOf != nil {
				details = append(details, "a side quest on "+Key(*in.SideOf))
			}
		} else {
			head += " added"
			if in.SideOf != nil {
				head += " as a side quest on " + Key(*in.SideOf)
			}
			if in.Reason != "" {
				details = append(details, in.Reason)
			}
		}
		if in.Kind == KindAwaiting {
			details = append(details, "awaiting reply from "+in.WaitingOn)
		}
		if len(in.NeededBy) > 0 {
			details = append(details, "opens "+keys(in.NeededBy))
		}
		if len(in.Needs) > 0 {
			details = append(details, "requires "+keys(in.Needs))
		}
		text := head
		if len(details) > 0 {
			text += " — " + strings.Join(details, "; ")
		}
		if err := s.cardEvent(ctx, tx, id, "add", text); err != nil {
			return err
		}
		if finalOf != nil {
			return s.setFinal(ctx, tx, finalOf, id)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	c, err := s.card(ctx, id, viewing)
	return c, true, err
}

// link applies the link requests of an add to an existing card: needs,
// neededBy, sideOf and finalOf. Each new link is logged; existing ones are
// left alone.
func (s *Store) link(ctx context.Context, tx *sql.Tx, id int64, in NewCard, finalOf *questRow) error {
	c, err := liveCard(ctx, tx, id)
	if err != nil {
		return err
	}
	if len(in.Needs) > 0 || len(in.NeededBy) > 0 {
		if err := mainCards(ctx, tx, append(append([]int64{id}, in.Needs...), in.NeededBy...)); err != nil {
			return err
		}
	}
	for _, n := range in.Needs {
		created, err := addNeed(ctx, tx, id, n)
		if err != nil {
			return err
		}
		if created {
			if err := s.cardEvent(ctx, tx, id, "need", Key(id)+" now requires "+Key(n)); err != nil {
				return err
			}
		}
	}
	for _, n := range in.NeededBy {
		created, err := addNeed(ctx, tx, n, id)
		if err != nil {
			return err
		}
		if created {
			if err := s.cardEvent(ctx, tx, n, "need", Key(n)+" now requires "+Key(id)); err != nil {
				return err
			}
		}
	}
	if in.SideOf != nil && (c.SideOf == nil || *c.SideOf != *in.SideOf) {
		if err := s.makeSide(ctx, tx, c, *in.SideOf); err != nil {
			return err
		}
	}
	if finalOf != nil {
		return s.setFinal(ctx, tx, finalOf, id)
	}
	return nil
}

// makeSide turns an existing card into a side quest of parent.
func (s *Store) makeSide(ctx context.Context, tx *sql.Tx, c *cardRow, parent int64) error {
	if c.SideOf != nil {
		return errf(ErrConflict, "%s is already a side quest on %s", Key(c.ID), Key(*c.SideOf))
	}
	if _, err := liveCard(ctx, tx, parent); err != nil {
		return err
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM needs WHERE from_card = ? OR to_card = ?`, c.ID, c.ID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errf(ErrConflict, "%s requires or opens other deeds; a side quest never blocks anything (unrequire it first)", Key(c.ID))
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM quests WHERE final_card = ?`, c.ID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errf(ErrConflict, "%s is a quest's crowning deed; a side quest cannot be", Key(c.ID))
	}
	// parent must not be c or one of c's own side quests.
	for p := &parent; p != nil; {
		if *p == c.ID {
			return errf(ErrConflict, "%s cannot be a side quest of its own side quest %s", Key(c.ID), Key(parent))
		}
		pc, err := getCard(ctx, tx, *p)
		if err != nil {
			return err
		}
		p = pc.SideOf
	}
	if _, err := tx.ExecContext(ctx, `UPDATE cards SET side_of = ? WHERE id = ?`, parent, c.ID); err != nil {
		return err
	}
	return s.cardEvent(ctx, tx, c.ID, "side", Key(c.ID)+" is now a side quest on "+Key(parent))
}

// mainCards checks the cards are live and not side quests, for use in needs.
func mainCards(ctx context.Context, q querier, ids []int64) error {
	for _, id := range ids {
		c, err := liveCard(ctx, q, id)
		if err != nil {
			return err
		}
		if c.SideOf != nil {
			return errf(ErrInvalid, "%s is a side quest; side quests neither require nor open other deeds", Key(id))
		}
	}
	return nil
}

// addNeed inserts from→to unless it already exists (created=false) or would
// close a cycle anywhere in the graph.
func addNeed(ctx context.Context, q querier, from, to int64) (bool, error) {
	if from == to {
		return false, errf(ErrInvalid, "a deed cannot require itself")
	}
	rows, err := q.QueryContext(ctx, `SELECT from_card, to_card FROM needs`)
	if err != nil {
		return false, err
	}
	next := map[int64][]int64{}
	for rows.Next() {
		var f, t int64
		if err := rows.Scan(&f, &t); err != nil {
			rows.Close()
			return false, err
		}
		if f == from && t == to {
			rows.Close()
			return false, nil
		}
		next[f] = append(next[f], t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}
	// A cycle appears if `from` is already reachable from `to`.
	if path := findPath(next, to, from); path != nil {
		steps := make([]string, len(path))
		for i, id := range path {
			steps[i] = Key(id)
		}
		return false, errf(ErrConflict, "%s cannot require %s: that would make a cycle, since %s already requires %s (%s)",
			Key(from), Key(to), Key(to), Key(from), strings.Join(steps, " → "))
	}
	_, err = q.ExecContext(ctx, `INSERT INTO needs (from_card, to_card) VALUES (?, ?)`, from, to)
	return err == nil, err
}

// findPath returns a path start→…→goal through next, or nil.
func findPath(next map[int64][]int64, start, goal int64) []int64 {
	prev := map[int64]int64{start: start}
	queue := []int64{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == goal {
			var path []int64
			for n := goal; n != start; n = prev[n] {
				path = append([]int64{n}, path...)
			}
			return append([]int64{start}, path...)
		}
		for _, n := range next[cur] {
			if _, seen := prev[n]; !seen {
				prev[n] = cur
				queue = append(queue, n)
			}
		}
	}
	return nil
}

// AddNeed records that card `from` needs card `to` done first. It returns
// created=false if the need already existed.
func (s *Store) AddNeed(ctx context.Context, from, to int64) (bool, error) {
	created := false
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := mainCards(ctx, tx, []int64{from, to}); err != nil {
			return err
		}
		var err error
		if created, err = addNeed(ctx, tx, from, to); err != nil || !created {
			return err
		}
		return s.cardEvent(ctx, tx, from, "need", Key(from)+" now requires "+Key(to))
	})
	return created, err
}

// RemoveNeed drops the need from→to. This is how a card leaves a quest.
func (s *Store) RemoveNeed(ctx context.Context, from, to int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM needs WHERE from_card = ? AND to_card = ?`, from, to)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errf(ErrNotFound, "%s does not require %s", Key(from), Key(to))
		}
		return s.cardEvent(ctx, tx, from, "unneed", Key(from)+" no longer requires "+Key(to))
	})
}

// sideQuestsOf returns the live side quests hanging off card id, and theirs
// in turn, parents before children.
func sideQuestsOf(ctx context.Context, q querier, id int64) ([]*cardRow, error) {
	var out []*cardRow
	parents := []int64{id}
	seen := map[int64]bool{id: true}
	for len(parents) > 0 {
		p := parents[0]
		parents = parents[1:]
		rows, err := q.QueryContext(ctx, `SELECT `+cardCols+` FROM cards WHERE side_of = ? AND removed_at IS NULL ORDER BY id`, p)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			c, err := scanCard(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			if !seen[c.ID] {
				seen[c.ID] = true
				out = append(out, c)
				parents = append(parents, c.ID)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// RemoveCard soft-deletes a card: it leaves the graph (and every need that
// touches it) but stays in the log. Its side quests go with it, each with
// its own event; a quest whose final card goes is left without one.
func (s *Store) RemoveCard(ctx context.Context, id int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errf(ErrInvalid, "say why the deed is struck (reason)")
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := liveCard(ctx, tx, id); err != nil {
			return err
		}
		sides, err := sideQuestsOf(ctx, tx, id)
		if err != nil {
			return err
		}
		remove := func(cid int64, text string) error {
			if _, err := tx.ExecContext(ctx, `UPDATE cards SET removed_at = ?, removed_reason = ? WHERE id = ?`, s.stamp(), reason, cid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM needs WHERE from_card = ? OR to_card = ?`, cid, cid); err != nil {
				return err
			}
			if err := s.cardEvent(ctx, tx, cid, "remove", text); err != nil {
				return err
			}
			rows, err := tx.QueryContext(ctx, `SELECT id, slug FROM quests WHERE final_card = ?`, cid)
			if err != nil {
				return err
			}
			var qs []*questRow
			for rows.Next() {
				q := &questRow{Final: &cid}
				if err := rows.Scan(&q.ID, &q.Slug); err != nil {
					rows.Close()
					return err
				}
				qs = append(qs, q)
			}
			rows.Close()
			for _, q := range qs {
				if err := s.clearFinal(ctx, tx, q, "crowning deed "+Key(cid)+" struck: "+reason); err != nil {
					return err
				}
			}
			return nil
		}
		if err := remove(id, Key(id)+" struck: "+reason); err != nil {
			return err
		}
		for _, sq := range sides {
			if err := remove(sq.ID, fmt.Sprintf("%s struck with its deed %s: %s", Key(sq.ID), Key(id), reason)); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateCard applies a patch. done and title apply to errands and awaitings
// only: an issue is done when it closes on GitHub. Cancelling (any kind,
// reason required) cascades to the card's side quests; marking a card done
// or cancelled stops work on it. Final needs a quest: viewing names it.
func (s *Store) UpdateCard(ctx context.Context, id int64, p CardPatch, viewing string) (*Card, error) {
	var q *questRow
	if viewing != "" {
		var err error
		if q, err = getQuest(ctx, s.db, viewing); err != nil {
			return nil, err
		}
	} else if p.Final != nil {
		return nil, errf(ErrInvalid, "a crowning deed belongs to a quest: set it with PATCH /api/quests/{slug} {final} (mikado quest crown)")
	}
	// An issue's done and abandoned states live on GitHub, which the local
	// guard below cannot see: check them before taking an issue up.
	if p.Working != nil && *p.Working {
		v, err := s.CardView(ctx, id)
		if err != nil {
			return nil, err
		}
		if c := v.Card; c.Kind == KindIssue {
			switch {
			case c.Cancelled && !c.Done && c.State == "closed":
				return nil, errf(ErrInvalid, "%s is abandoned: %s was closed on GitHub as not planned; reopen it there before taking it up", c.Key, c.Ref)
			case c.Done:
				return nil, errf(ErrInvalid, "%s is fulfilled: %s is closed on GitHub; reopen it there before taking it up", c.Key, c.Ref)
			}
		}
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		c, err := liveCard(ctx, tx, id)
		if err != nil {
			return err
		}
		k := Key(id)
		if c.Kind == KindIssue && (p.Done != nil || p.Title != nil) {
			return errf(ErrInvalid, "%s is an issue: it is fulfilled when %s closes on GitHub, and its title comes from there", k, c.Ref)
		}
		if p.CancelReason != nil && (p.Cancelled == nil || !*p.Cancelled) {
			return errf(ErrInvalid, "cancelReason goes with cancelled: true")
		}
		if p.WorkingBy != nil && (p.Working == nil || !*p.Working) {
			return errf(ErrInvalid, "workingBy goes with working: true")
		}
		exec := func(query string, args ...any) error {
			_, err := tx.ExecContext(ctx, query, args...)
			return err
		}
		set := func(col string, v any, kind, text string) error {
			if err := exec(`UPDATE cards SET `+col+` = ? WHERE id = ?`, v, id); err != nil {
				return err
			}
			return s.cardEvent(ctx, tx, id, kind, text)
		}
		working := c.WorkingSince != ""
		stopWork := func() error {
			working = false
			return exec(`UPDATE cards SET working_since = NULL, working_by = '' WHERE id = ?`, id)
		}
		if p.Title != nil {
			t := strings.TrimSpace(*p.Title)
			if t == "" {
				return errf(ErrInvalid, "title cannot be empty")
			}
			if t != c.Title {
				if err := set("title", t, "edit", k+" renamed from "+quoted(c.Title)); err != nil {
					return err
				}
			}
		}
		if p.Done != nil && *p.Done != c.Done {
			kind, text := "done", k+" fulfilled"
			if !*p.Done {
				kind, text = "undone", k+" unfulfilled"
			}
			if err := set("done", b2i(*p.Done), kind, text); err != nil {
				return err
			}
			if *p.Done && working {
				if err := stopWork(); err != nil {
					return err
				}
			}
			c.Done = *p.Done
		}
		if p.Owner != nil {
			o := strings.TrimSpace(*p.Owner)
			if o != c.Owner {
				text := k + " hero cleared"
				if o != "" {
					text = k + " hero set to " + o
				}
				if err := set("owner", o, "edit", text); err != nil {
					return err
				}
			}
		}
		if p.NPC != nil && *p.NPC != c.NPC {
			text := k + " marked as an NPC"
			if !*p.NPC {
				text = k + " no longer an NPC"
			}
			if err := set("npc", b2i(*p.NPC), "edit", text); err != nil {
				return err
			}
		}
		if p.Cancelled != nil && *p.Cancelled != c.Cancelled {
			if *p.Cancelled {
				reason := ""
				if p.CancelReason != nil {
					reason = strings.TrimSpace(*p.CancelReason)
				}
				if reason == "" {
					return errf(ErrInvalid, "say why the deed is abandoned (cancelReason)")
				}
				sides, err := sideQuestsOf(ctx, tx, id)
				if err != nil {
					return err
				}
				cancel := func(cid int64, text string) error {
					if err := exec(`UPDATE cards SET cancelled_at = ?, cancel_reason = ?, working_since = NULL, working_by = '' WHERE id = ?`,
						s.stamp(), reason, cid); err != nil {
						return err
					}
					return s.cardEvent(ctx, tx, cid, "cancel", text)
				}
				if err := cancel(id, k+" abandoned: "+reason); err != nil {
					return err
				}
				for _, sq := range sides {
					if !sq.Cancelled {
						if err := cancel(sq.ID, fmt.Sprintf("%s abandoned with its deed %s: %s", Key(sq.ID), k, reason)); err != nil {
							return err
						}
					}
				}
				working = false
			} else {
				if err := exec(`UPDATE cards SET cancelled_at = NULL, cancel_reason = '' WHERE id = ?`, id); err != nil {
					return err
				}
				if err := s.cardEvent(ctx, tx, id, "uncancel", k+" no longer abandoned"); err != nil {
					return err
				}
			}
			c.Cancelled = *p.Cancelled
		}
		if p.Working != nil {
			by := ""
			if p.WorkingBy != nil {
				by = strings.TrimSpace(*p.WorkingBy)
			}
			switch {
			case *p.Working && c.Cancelled:
				return errf(ErrInvalid, "%s is abandoned; unabandon it before taking it up", k)
			case *p.Working && c.Done:
				return errf(ErrInvalid, "%s is fulfilled; unfulfil it before taking it up", k)
			case *p.Working && (!working || by != c.WorkingBy):
				text := k + " taken up"
				if by != "" {
					text += " by " + by
				}
				since := c.WorkingSince
				if !working {
					since = s.stamp()
				}
				if err := exec(`UPDATE cards SET working_since = ?, working_by = ? WHERE id = ?`, since, by, id); err != nil {
					return err
				}
				if err := s.cardEvent(ctx, tx, id, "start", text); err != nil {
					return err
				}
			case !*p.Working && working:
				if err := stopWork(); err != nil {
					return err
				}
				if err := s.cardEvent(ctx, tx, id, "stop", k+" set down"); err != nil {
					return err
				}
			}
		}
		if p.Final != nil {
			isFinal := q.Final != nil && *q.Final == id
			switch {
			case *p.Final && !isFinal:
				return s.setFinal(ctx, tx, q, id)
			case !*p.Final && isFinal:
				return s.clearFinal(ctx, tx, q, "crowning deed "+k+" unset; the quest has no crowning deed")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.card(ctx, id, viewing)
}

var loginRe = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)

func cleanLogins(in []string) ([]string, error) {
	var out []string
	for _, l := range in {
		l = strings.TrimPrefix(strings.TrimSpace(l), "@")
		if !loginRe.MatchString(l) {
			return nil, errf(ErrInvalid, "%q is not a GitHub login", l)
		}
		out = append(out, l)
	}
	return out, nil
}

// Assign adds and removes assignees of an issue card on GitHub.
func (s *Store) Assign(ctx context.Context, id int64, add, remove []string, viewing string) (*Card, error) {
	add, err := cleanLogins(add)
	if err != nil {
		return nil, err
	}
	remove, err = cleanLogins(remove)
	if err != nil {
		return nil, err
	}
	if len(add) == 0 && len(remove) == 0 {
		return nil, errf(ErrInvalid, "nobody to assign or unassign")
	}
	c, err := liveCard(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	if c.Kind != KindIssue {
		return nil, errf(ErrInvalid, "%s is %s; only issues have assignees on GitHub (give it a hero instead)", Key(id), kindNoun(c.Kind))
	}
	ref, err := github.ParseRef(c.Ref)
	if err != nil {
		return nil, err
	}
	if s.gh == nil {
		return nil, errf(ErrUpstream, "GitHub is not configured")
	}
	ghErr := s.gh.Assign(ctx, ref, add, remove)
	// Whatever happened, what we cached may now be wrong.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM github_cache WHERE ref_key = ?`, ref.Key()); err != nil {
		return nil, err
	}
	if ghErr != nil {
		return nil, errf(ErrUpstream, "%v", ghErr)
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		at := func(ls []string) string { return "@" + strings.Join(ls, ", @") }
		if len(add) > 0 {
			if err := s.cardEvent(ctx, tx, id, "assign", Key(id)+" assigned to "+at(add)); err != nil {
				return err
			}
		}
		if len(remove) > 0 {
			return s.cardEvent(ctx, tx, id, "unassign", Key(id)+" unassigned "+at(remove))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.card(ctx, id, viewing)
}
