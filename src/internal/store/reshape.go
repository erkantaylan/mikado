package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// Reshaping the map: moving a requirement (Rewire), moving quests into a
// journey of their own (Extract), taking a quest out of a journey or out of
// the database (DeleteQuest) and deleting a journey (DeleteJourney). Each runs
// in one transaction and changes nothing when it refuses; a refusal names the
// flag or command that would do what was meant, since an AI reads it and
// nobody is asked yes or no.

// Rewired is the outcome of a rewire.
type Rewired struct {
	From int64 `json:"from"`
	Old  int64 `json:"old"`
	New  int64 `json:"new"`
	// Orphaned are the quests the rewire left in no journey: Old and what
	// only it held there.
	Orphaned []string `json:"orphaned"`
}

// Extracted is the outcome of an extract.
type Extracted struct {
	Journey JourneySummary `json:"journey"` // the new journey
	Crown   string         `json:"crown"`   // its crowning quest, a new errand
	// StillIn are extracted quests the old journey still reaches by another
	// route, so they stay on its war table too.
	StillIn []string `json:"stillIn"`
}

// QuestDelete says how to delete a quest.
type QuestDelete struct {
	// Journey takes the quest out of this journey only. Empty: out of every
	// journey, which needs Force.
	Journey string `json:"journey"`
	// Force deletes the quest from the database. With Journey it is needed
	// when that journey is the quest's last.
	Force bool `json:"force"`
	// Branch takes along what would leave with it: its side quests and the
	// prerequisites only it held there (deleted too when left in no journey).
	Branch bool `json:"branch"`
	// Rewire hands those to this quest instead: it requires the prerequisites
	// and gets the side quests.
	Rewire *int64 `json:"rewire"`
}

// QuestDeleted is the outcome of a quest delete.
type QuestDeleted struct {
	Key      string   `json:"key"`
	Journeys []string `json:"journeys"` // the journeys it left
	Branch   []string `json:"branch"`   // the quests that left with it
	Deleted  []string `json:"deleted"`  // the quests deleted from the database
}

// JourneyDeleted is the outcome of a journey delete.
type JourneyDeleted struct {
	Key     string   `json:"key"`
	Deleted []string `json:"deleted"` // its quests deleted from the database
	Kept    []string `json:"kept"`    // its quests still in other journeys
}

// placement maps each card to the ids of the journeys it is a member of.
func (g *graph) placement() map[int64]map[int64]bool {
	out := map[int64]map[int64]bool{}
	for id, js := range g.membership() {
		out[id] = map[int64]bool{}
		for _, j := range js {
			out[id][j.ID] = true
		}
	}
	return out
}

// sortedIDs returns the keys of a set, ascending.
func sortedIDs(set map[int64]bool) []int64 {
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func keyList(ids []int64) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = Key(id)
	}
	return out
}

// label names a card in a log line that must outlive it: Q12 “title”, or
// Q12 owner/repo#7 for an issue.
func label(c *cardRow) string {
	if c.Kind == KindIssue {
		return Key(c.ID) + " " + c.Ref
	}
	return Key(c.ID) + " " + quoted(c.Title)
}

// expands reports whether the requirements of card id count on journey j:
// they do unless the card stands for another journey there.
func (g *graph) expands(id int64, j *journeyRow) bool {
	return (j.Final != nil && *j.Final == id) || !g.folds(id, j)
}

// Rewire makes from require new instead of old, in one step, so nothing
// drops out of a journey on the way. What old alone held in a journey leaves
// it; the quests left in no journey at all are listed.
func (s *Store) Rewire(ctx context.Context, from, old, new int64) (*Rewired, error) {
	if old == new {
		return nil, errf(ErrInvalid, "%s is already what %s requires; name another quest to rewire it to", Key(new), Key(from))
	}
	out := &Rewired{From: from, Old: old, New: new, Orphaned: []string{}}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := mainCards(ctx, tx, []int64{from, old, new}); err != nil {
			return err
		}
		g, err := loadGraph(ctx, tx)
		if err != nil {
			return err
		}
		before := g.placement()
		res, err := tx.ExecContext(ctx, `DELETE FROM needs WHERE from_card = ? AND to_card = ?`, from, old)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errf(ErrNotFound, "%s does not require %s, so there is nothing to rewire (add it with `mikado require %s %s`)",
				Key(from), Key(old), Key(from), Key(new))
		}
		if _, err := addNeed(ctx, tx, from, new); err != nil {
			return err
		}
		if g, err = loadGraph(ctx, tx); err != nil {
			return err
		}
		after := g.placement()
		orphaned := map[int64]bool{}
		for id, js := range before {
			if len(js) > 0 && len(after[id]) == 0 {
				orphaned[id] = true
			}
		}
		out.Orphaned = keyList(sortedIDs(orphaned))
		return s.cardEvent(ctx, tx, from, "need", fmt.Sprintf("%s now requires %s instead of %s", Key(from), Key(new), Key(old)))
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Extract moves quests of a journey into a new journey of their own, crowned
// by a new errand titled like it. The errand requires the chosen quests (those
// no other chosen quest already leads to), and every quest of the old journey
// that required one of them requires the errand instead, so the old journey
// shows the new one as a single journey card where they were.
func (s *Store) Extract(ctx context.Context, key string, ids []int64, title string) (*Extracted, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errf(ErrInvalid, "the new journey needs a title")
	}
	if len(ids) == 0 {
		return nil, errf(ErrInvalid, "name the quests to extract")
	}
	var newID, crown int64
	out := &Extracted{StillIn: []string{}}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		g, err := loadGraph(ctx, tx)
		if err != nil {
			return err
		}
		j, err := g.journey(key)
		if err != nil {
			return err
		}
		members := map[int64]bool{}
		for _, id := range g.members(j) {
			members[id] = true
		}
		chosen := map[int64]bool{}
		for _, id := range ids {
			c := g.cards[id]
			switch {
			case c == nil:
				_, err := liveCard(ctx, tx, id)
				return err
			case !members[id]:
				return errf(ErrInvalid, "%s is not on %s's war table", Key(id), j.Key())
			case j.Final != nil && *j.Final == id:
				return errf(ErrInvalid, "%s crowns %s; extracting it would move the whole journey (retitle it with `mikado journey set %s --title T` instead)",
					Key(id), j.Key(), j.Key())
			case c.SideOf != nil:
				return errf(ErrInvalid, "%s is a side quest on %s; extract %s and it comes along", Key(id), Key(*c.SideOf), Key(*c.SideOf))
			}
			chosen[id] = true
		}
		// inside: what the new journey will hold besides its crown. below:
		// what a chosen quest leads to, so a chosen quest in it is no top.
		plain := func(id int64) bool { return len(g.crowned[id]) == 0 }
		inside := map[int64]bool{}
		for _, id := range g.reach(sortedIDs(chosen), plain) {
			inside[id] = true
		}
		var children []int64
		for id := range chosen {
			if plain(id) {
				children = append(children, g.next[id]...)
			}
		}
		below := map[int64]bool{}
		for _, id := range g.reach(children, plain) {
			below[id] = true
		}

		stamp := s.stamp()
		if err := tx.QueryRowContext(ctx, `INSERT INTO cards (kind, title, created_at) VALUES (?, ?, ?) RETURNING id`,
			KindErrand, title, stamp).Scan(&crown); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO journeys (title, created_at) VALUES (?, ?) RETURNING id`,
			title, stamp).Scan(&newID); err != nil {
			return err
		}
		var tops []int64
		for _, id := range sortedIDs(chosen) {
			if !below[id] {
				tops = append(tops, id)
				if _, err := addNeed(ctx, tx, crown, id); err != nil {
					return err
				}
			}
		}
		// Every quest of the old journey that required a chosen one, and is
		// not itself moving, now requires the new journey instead.
		var rewired []int64
		for _, p := range g.members(j) {
			if inside[p] || !g.expands(p, j) {
				continue
			}
			var was []int64
			for _, to := range g.next[p] {
				if chosen[to] {
					was = append(was, to)
				}
			}
			if len(was) == 0 {
				continue
			}
			for _, to := range was {
				if _, err := tx.ExecContext(ctx, `DELETE FROM needs WHERE from_card = ? AND to_card = ?`, p, to); err != nil {
					return err
				}
			}
			if _, err := addNeed(ctx, tx, p, crown); err != nil {
				return err
			}
			if err := s.cardEvent(ctx, tx, p, "need", fmt.Sprintf("%s now requires %s (journey %s) instead of %s",
				Key(p), Key(crown), JourneyKey(newID), keys(was))); err != nil {
				return err
			}
			rewired = append(rewired, p)
		}
		if err := s.event(ctx, tx, &newID, nil, "create", fmt.Sprintf("journey extracted from %s with %s", j.Key(), keys(sortedIDs(chosen)))); err != nil {
			return err
		}
		if err := s.cardEvent(ctx, tx, crown, "add", fmt.Sprintf("%s errand added to crown %s — requires %s", Key(crown), JourneyKey(newID), keys(tops))); err != nil {
			return err
		}
		if err := s.setFinal(ctx, tx, &journeyRow{ID: newID}, crown); err != nil {
			return err
		}
		if err := s.event(ctx, tx, &j.ID, nil, "extract", fmt.Sprintf("%s extracted into journey %s %s", keys(sortedIDs(chosen)), JourneyKey(newID), quoted(title))); err != nil {
			return err
		}
		if g, err = loadGraph(ctx, tx); err != nil {
			return err
		}
		still := map[int64]bool{}
		for _, id := range g.members(j) {
			if inside[id] {
				still[id] = true
			}
		}
		out.StillIn = keyList(sortedIDs(still))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sum, err := s.summary(ctx, newID)
	if err != nil {
		return nil, err
	}
	out.Journey, out.Crown = *sum, Key(crown)
	return out, nil
}

// DeleteQuest takes a quest out of one journey, or deletes it from the
// database (Force). Nothing is left hanging: what would leave a journey with
// it (its side quests, the prerequisites only it held there) must go along
// (Branch) or be handed to another quest (Rewire), and a quest is only
// deleted from the database with Force.
func (s *Store) DeleteQuest(ctx context.Context, id int64, in QuestDelete) (*QuestDeleted, error) {
	if in.Journey == "" && !in.Force {
		return nil, errf(ErrInvalid, "say which journey to take %s out of (--journey J), or add --force to delete it from every journey and the database", Key(id))
	}
	if in.Branch && in.Rewire != nil {
		return nil, errf(ErrInvalid, "--branch takes what hangs on %s along, --rewire hands it to another quest: pick one", Key(id))
	}
	out := &QuestDeleted{Key: Key(id), Journeys: []string{}, Branch: []string{}, Deleted: []string{}}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := liveCard(ctx, tx, id); err != nil {
			return err
		}
		g, err := loadGraph(ctx, tx)
		if err != nil {
			return err
		}
		q := g.cards[id]
		before := g.placement()
		var j *journeyRow
		if in.Journey != "" {
			if j, err = g.journey(in.Journey); err != nil {
				return err
			}
			if !before[id][j.ID] {
				return errf(ErrInvalid, "%s is not on %s's war table", Key(id), j.Key())
			}
		}
		for _, cj := range g.crowned[id] {
			if j == nil || cj.ID == j.ID {
				return errf(ErrConflict, "%s crowns %s; crown another quest first (`mikado journey crown %s Q`) or delete the journey (`mikado journey delete %s`)",
					Key(id), cj.Key(), cj.Key(), cj.Key())
			}
		}
		exec := func(query string, args ...any) error {
			_, err := tx.ExecContext(ctx, query, args...)
			return err
		}

		// Unlink the quest: from the journey's quests that hold it, or from everything.
		if j != nil {
			if q.SideOf != nil {
				if err := exec(`UPDATE cards SET side_of = NULL WHERE id = ?`, id); err != nil {
					return err
				}
			}
			for _, p := range g.prev[id] {
				if before[p][j.ID] && g.expands(p, j) {
					if err := exec(`DELETE FROM needs WHERE from_card = ? AND to_card = ?`, p, id); err != nil {
						return err
					}
				}
			}
		} else {
			if err := exec(`DELETE FROM needs WHERE from_card = ? OR to_card = ?`, id, id); err != nil {
				return err
			}
		}
		// Hand what hangs on it to another quest.
		var heir *cardRow
		if in.Rewire != nil {
			if *in.Rewire == id {
				return errf(ErrInvalid, "%s cannot take over its own quests; name another quest for --rewire", Key(id))
			}
			if err := mainCards(ctx, tx, []int64{*in.Rewire}); err != nil {
				return err
			}
			heir = g.cards[*in.Rewire]
			for _, r := range g.next[id] {
				if r == heir.ID {
					continue
				}
				created, err := addNeed(ctx, tx, heir.ID, r)
				if err != nil {
					return err
				}
				if created {
					if err := s.cardEvent(ctx, tx, heir.ID, "need", fmt.Sprintf("%s now requires %s, taken over from %s", Key(heir.ID), Key(r), Key(id))); err != nil {
						return err
					}
				}
			}
			for _, sq := range g.sides[id] {
				if err := exec(`UPDATE cards SET side_of = ? WHERE id = ?`, heir.ID, sq); err != nil {
					return err
				}
				if err := s.cardEvent(ctx, tx, sq, "side", fmt.Sprintf("%s is now a side quest on %s, taken over from %s", Key(sq), Key(heir.ID), Key(id))); err != nil {
					return err
				}
			}
		}
		// A quest deleted everywhere lets go of its side quests.
		if j == nil {
			if err := exec(`UPDATE cards SET side_of = NULL WHERE side_of = ? AND removed_at IS NULL`, id); err != nil {
				return err
			}
		}

		g2, err := loadGraph(ctx, tx)
		if err != nil {
			return err
		}
		after := g2.placement()
		left := func(cid int64) []int64 {
			var out []int64
			for jid := range before[cid] {
				if !after[cid][jid] {
					out = append(out, jid)
				}
			}
			sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
			return out
		}
		jkeys := func(ids []int64) []string {
			out := make([]string, len(ids))
			for i, jid := range ids {
				out[i] = JourneyKey(jid)
			}
			return out
		}
		qLeft := left(id)
		// Taken out of one journey only: it must not leave any other.
		if j != nil {
			for _, jid := range qLeft {
				if jid != j.ID {
					return errf(ErrConflict, "taking %s out of %s would take it out of %s too: the quests holding it there are also on %s's war table (`mikado show %s` lists them)",
						Key(id), j.Key(), JourneyKey(jid), JourneyKey(jid), Key(id))
				}
			}
		}
		if j != nil && in.Force && len(after[id]) > 0 {
			return errf(ErrConflict, "%s is still in %s once out of %s; --force deletes it only from its last journey — drop --force to take it out of %s, or drop --journey to delete it everywhere",
				Key(id), strings.Join(jkeys(sortedIDs(after[id])), ", "), j.Key(), j.Key())
		}
		// Out of its last journey, it leaves the database, which takes --force.
		deleting := j == nil || len(after[id]) == 0
		// What would leave a journey with it.
		branch := map[int64]bool{}
		var hanging []string
		for cid := range before {
			if cid == id {
				continue
			}
			lost := left(cid)
			if len(lost) == 0 {
				continue
			}
			branch[cid] = true
			what := Key(cid)
			if c := g.cards[cid]; c.SideOf != nil {
				what += " (side quest)"
			}
			hanging = append(hanging, what+" from "+strings.Join(jkeys(lost), ", "))
		}
		// A quest leaving the database also lets go of what hangs on it in
		// no journey at all.
		if deleting {
			for _, cid := range g.closure(id) {
				if cid != id && !branch[cid] && len(after[cid]) == 0 {
					branch[cid] = true
					hanging = append(hanging, Key(cid)+" (in no journey)")
				}
			}
		}
		sort.Strings(hanging)
		var orphans []int64
		for _, cid := range sortedIDs(branch) {
			if len(after[cid]) == 0 {
				orphans = append(orphans, cid)
			}
		}
		// Every flag that is missing, in one refusal.
		var problems []string
		if deleting && !in.Force {
			problems = append(problems, fmt.Sprintf("%s is %s's last journey, so %s would leave the database: add --force",
				j.Key(), Key(id), Key(id)))
		}
		switch {
		case len(branch) > 0 && heir != nil:
			problems = append(problems, fmt.Sprintf("even with %s taking over, these would leave with %s: %s — rewire to a quest on the same war table, or use --branch",
				Key(heir.ID), Key(id), strings.Join(hanging, "; ")))
		case len(branch) > 0 && !in.Branch:
			problems = append(problems, fmt.Sprintf("these would leave with %s: %s — take them along with --branch, or hand them to another quest with --rewire Q",
				Key(id), strings.Join(hanging, "; ")))
		case len(orphans) > 0 && !deleting:
			problems = append(problems, fmt.Sprintf("%s would be left in no journey: add --force to delete them from the database", keys(orphans)))
		}
		if len(problems) > 0 {
			return errf(ErrConflict, "%s", strings.Join(problems, "; and "))
		}

		// The chronicle: one line in each journey the quest leaves.
		text := label(q) + " taken out of the journey"
		if deleting {
			text = label(q) + " deleted"
		}
		if len(branch) > 0 {
			text += " with " + keys(sortedIDs(branch))
		}
		if heir != nil && (len(g.next[id]) > 0 || len(g.sides[id]) > 0) {
			text += "; " + Key(heir.ID) + " took over what hung on it"
		}
		for _, jid := range qLeft {
			var cardID *int64
			if !deleting {
				cardID = &id
			}
			if err := s.event(ctx, tx, &jid, cardID, "delete", text); err != nil {
				return err
			}
		}
		out.Journeys = jkeys(qLeft)
		out.Branch = keyList(sortedIDs(branch))
		if deleting {
			gone := append([]int64{id}, orphans...)
			if err := hardDelete(ctx, tx, gone); err != nil {
				return err
			}
			out.Deleted = keyList(gone)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteJourney deletes a journey and its own chronicle lines. Its quests in
// other journeys stay; those it alone held are deleted from the database
// with force, and without it the delete is refused. A journey drawn on
// another's war table cannot go: that one would suddenly show all its quests.
func (s *Store) DeleteJourney(ctx context.Context, key string, force bool) (*JourneyDeleted, error) {
	out := &JourneyDeleted{Deleted: []string{}, Kept: []string{}}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		g, err := loadGraph(ctx, tx)
		if err != nil {
			return err
		}
		j, err := g.journey(key)
		if err != nil {
			return err
		}
		out.Key = j.Key()
		if j.Final != nil {
			for _, other := range g.journeys {
				if other.ID == j.ID || !g.folds(*j.Final, other) {
					continue
				}
				for _, m := range g.members(other) {
					if m == *j.Final {
						return errf(ErrConflict, "%s is on %s's war table as a journey card; take it off first (`mikado show %s` lists the quests that require it; `mikado unrequire Q %s`)",
							j.Key(), other.Key(), j.Key(), j.Key())
					}
				}
			}
		}
		members := g.members(j)
		if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE journey_id = ?`, j.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM journeys WHERE id = ?`, j.ID); err != nil {
			return err
		}
		if g, err = loadGraph(ctx, tx); err != nil {
			return err
		}
		after := g.placement()
		var orphans, kept []int64
		for _, id := range members {
			if len(after[id]) == 0 {
				orphans = append(orphans, id)
			} else {
				kept = append(kept, id)
			}
		}
		if len(orphans) > 0 && !force {
			return errf(ErrConflict, "%s would be left in no journey; add --force to delete them from the database too, or link them into another journey first",
				keys(orphans))
		}
		if err := hardDelete(ctx, tx, orphans); err != nil {
			return err
		}
		out.Deleted, out.Kept = keyList(orphans), keyList(kept)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// hardDelete removes cards from the database. Journey-level chronicle lines
// that name one keep their text and lose the link; the cards' own lines go.
func hardDelete(ctx context.Context, tx *sql.Tx, ids []int64) error {
	for _, id := range ids {
		for _, q := range []string{
			`UPDATE events SET card_id = NULL WHERE card_id = ? AND journey_id IS NOT NULL`,
			`DELETE FROM events WHERE card_id = ?`,
			`DELETE FROM needs WHERE from_card = ? OR to_card = ?`,
			`UPDATE cards SET side_of = NULL WHERE side_of = ?`,
			`UPDATE cards SET found_while = NULL WHERE found_while = ?`,
			`UPDATE journeys SET final_card = NULL WHERE final_card = ?`,
		} {
			args := []any{id}
			if strings.Count(q, "?") == 2 {
				args = append(args, id)
			}
			if _, err := tx.ExecContext(ctx, q, args...); err != nil {
				return err
			}
		}
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM cards WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return nil
}
