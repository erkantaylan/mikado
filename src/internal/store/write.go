package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

func (s *Store) event(ctx context.Context, q querier, journeyID, cardID *int64, kind, text string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO events (journey_id, card_id, at, kind, text) VALUES (?, ?, ?, ?, ?)`,
		journeyID, cardID, s.stamp(), kind, text)
	return err
}

// cardEvent logs a change to a card; it shows in the log of every journey the
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

// CreateJourney creates a journey in a region (by key or name; empty: the
// default one), optionally with its final card.
func (s *Store) CreateJourney(ctx context.Context, title string, final *int64, region string) (*JourneySummary, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errf(ErrInvalid, "a journey needs a title")
	}
	var id int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		rid, err := regionFor(ctx, tx, region)
		if err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO journeys (title, created_at, region_id) VALUES (?, ?, ?) RETURNING id`,
			title, s.stamp(), rid).Scan(&id); err != nil {
			return err
		}
		if err := s.event(ctx, tx, &id, nil, "create", "journey created"); err != nil {
			return err
		}
		if final != nil {
			return s.setFinal(ctx, tx, &journeyRow{ID: id}, *final)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.summary(ctx, id)
}

func (s *Store) summary(ctx context.Context, id int64) (*JourneySummary, error) {
	js, err := s.Journeys(ctx)
	if err != nil {
		return nil, err
	}
	for i := range js {
		if js[i].Key == JourneyKey(id) {
			return &js[i], nil
		}
	}
	return nil, errf(ErrNotFound, "no journey %s", JourneyKey(id))
}

// UpdateJourney retitles a journey, archives or unarchives it, moves it to
// another region, or sets its final card.
func (s *Store) UpdateJourney(ctx context.Context, key string, p JourneyPatch) (*JourneySummary, error) {
	j, err := getJourney(ctx, s.db, key)
	if err != nil {
		return nil, err
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if p.Title != nil {
			t := strings.TrimSpace(*p.Title)
			if t == "" {
				return errf(ErrInvalid, "a journey needs a title")
			}
			if t != j.Title {
				if _, err := tx.ExecContext(ctx, `UPDATE journeys SET title = ? WHERE id = ?`, t, j.ID); err != nil {
					return err
				}
				if err := s.event(ctx, tx, &j.ID, nil, "edit", "journey retitled from "+quoted(j.Title)); err != nil {
					return err
				}
			}
		}
		if p.Archived != nil && *p.Archived != (j.ArchivedAt != "") {
			var at any // NULL: brought back
			text := "journey brought back from the archive"
			if *p.Archived {
				at, text = s.stamp(), "journey archived"
			}
			if _, err := tx.ExecContext(ctx, `UPDATE journeys SET archived_at = ? WHERE id = ?`, at, j.ID); err != nil {
				return err
			}
			if err := s.event(ctx, tx, &j.ID, nil, "archive", text); err != nil {
				return err
			}
		}
		if p.Region != nil {
			if err := s.moveJourney(ctx, tx, j, *p.Region); err != nil {
				return err
			}
		}
		if p.Final != nil {
			return s.setFinal(ctx, tx, j, *p.Final)
		}
		return nil
	})
	if err != nil {
		if p.Region != nil {
			err = s.movedTogether(ctx, err)
		}
		return nil, err
	}
	return s.summary(ctx, j.ID)
}

// setFinal makes a live, non-side card the journey's final card.
func (s *Store) setFinal(ctx context.Context, tx *sql.Tx, j *journeyRow, id int64) error {
	c, err := liveCard(ctx, tx, id)
	if err != nil {
		return err
	}
	if c.SideOf != nil {
		return errf(ErrInvalid, "%s is a side quest; a side quest cannot be a journey's crowning quest", Key(id))
	}
	var old sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT j.final_card FROM journeys j JOIN cards c ON c.id = j.final_card AND c.removed_at IS NULL WHERE j.id = ?`, j.ID).Scan(&old); err != nil && err != sql.ErrNoRows {
		return err
	}
	if old.Valid && old.Int64 == id {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE journeys SET final_card = ? WHERE id = ?`, id, j.ID); err != nil {
		return err
	}
	text := "crowning quest set to " + Key(id)
	if old.Valid {
		text = fmt.Sprintf("crowning quest changed from %s to %s", Key(old.Int64), Key(id))
	}
	return s.event(ctx, tx, &j.ID, &id, "final", text)
}

// clearFinal leaves the journey without a final card.
func (s *Store) clearFinal(ctx context.Context, tx *sql.Tx, j *journeyRow, text string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE journeys SET final_card = NULL WHERE id = ?`, j.ID); err != nil {
		return err
	}
	return s.event(ctx, tx, &j.ID, j.Final, "final", text)
}

// cleanMark trims a mark and checks its length.
func cleanMark(m string) (string, error) {
	m = strings.TrimSpace(m)
	if n := utf8.RuneCountInString(m); n > MaxMark {
		return "", errf(ErrInvalid, "a mark is at most %d characters, and %q has %d: keep it short (#13, 234g45a, PROJ-88) and put the rest in the title or url", MaxMark, m, n)
	}
	return m, nil
}

// AddCard adds a card to the global graph and links it as asked; WaitingOn
// makes it a petition. viewing (a journey key, or "") only shapes the
// returned card; see card.
func (s *Store) AddCard(ctx context.Context, in NewCard, viewing string) (*Card, error) {
	in.Title, in.Owner, in.WaitingOn, in.Reason = strings.TrimSpace(in.Title), strings.TrimSpace(in.Owner), strings.TrimSpace(in.WaitingOn), strings.TrimSpace(in.Reason)
	in.URL = strings.TrimSpace(in.URL)
	var err error
	if in.Mark, err = cleanMark(in.Mark); err != nil {
		return nil, err
	}
	if in.Title == "" {
		return nil, errf(ErrInvalid, "a quest needs a title")
	}
	if in.Final {
		if viewing == "" {
			return nil, errf(ErrInvalid, "a crowning quest needs a journey: use finalOf")
		}
		in.FinalOf = viewing
	}
	var finalOf *journeyRow
	if in.FinalOf != "" {
		q, err := getJourney(ctx, s.db, in.FinalOf)
		if err != nil {
			return nil, err
		}
		finalOf = q
	}
	if in.SideOf != nil && (finalOf != nil || len(in.Needs) > 0 || len(in.NeededBy) > 0) {
		return nil, errf(ErrInvalid, "a side quest never blocks anything: it cannot crown a journey, require or open a quest")
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
		since := ""
		if in.WaitingOn != "" {
			since = s.now().Format("2006-01-02")
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO cards (title, url, mark, owner, waiting_on, since,
				side_of, found_while, reason, npc, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			in.Title, in.URL, in.Mark, in.Owner, in.WaitingOn, since,
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

		// "Q9 added as a side quest on Q4 — opens Q3", or for a quest found
		// on the way, "Q9 found on Q3: old saves crash the loader — opens Q3".
		head := Key(id)
		if in.WaitingOn != "" {
			head += " petition"
		}
		var details []string
		if in.FoundWhile != nil {
			head += " found on " + Key(*in.FoundWhile)
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
		if in.WaitingOn != "" {
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
		return nil, err
	}
	return s.card(ctx, id, viewing)
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
		return errf(ErrConflict, "%s requires or opens other quests; a side quest never blocks anything (unrequire it first)", Key(c.ID))
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM journeys WHERE final_card = ?`, c.ID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errf(ErrConflict, "%s is a journey's crowning quest; a side quest cannot be", Key(c.ID))
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
			return errf(ErrInvalid, "%s is a side quest; side quests neither require nor open other quests", Key(id))
		}
	}
	return nil
}

// addNeed inserts from→to unless it already exists (created=false) or would
// close a cycle anywhere in the graph.
func addNeed(ctx context.Context, q querier, from, to int64) (bool, error) {
	if from == to {
		return false, errf(ErrInvalid, "a quest cannot require itself")
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

// RemoveNeed drops the need from→to. This is how a card leaves a journey.
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
// its own event; a journey whose final card goes is left without one.
func (s *Store) RemoveCard(ctx context.Context, id int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errf(ErrInvalid, "say why the quest is struck (reason)")
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
			rows, err := tx.QueryContext(ctx, `SELECT id FROM journeys WHERE final_card = ?`, cid)
			if err != nil {
				return err
			}
			var qs []*journeyRow
			for rows.Next() {
				q := &journeyRow{Final: &cid}
				if err := rows.Scan(&q.ID); err != nil {
					rows.Close()
					return err
				}
				qs = append(qs, q)
			}
			rows.Close()
			for _, q := range qs {
				if err := s.clearFinal(ctx, tx, q, "crowning quest "+Key(cid)+" struck: "+reason); err != nil {
					return err
				}
			}
			return nil
		}
		if err := remove(id, Key(id)+" struck: "+reason); err != nil {
			return err
		}
		for _, sq := range sides {
			if err := remove(sq.ID, fmt.Sprintf("%s struck with its quest %s: %s", Key(sq.ID), Key(id), reason)); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateCard applies a patch. Cancelling (reason required) cascades to the
// card's side quests; marking a card done or cancelled stops work on it.
// Setting WaitingOn makes the quest a petition, clearing it a plain quest.
// Final needs a journey: viewing names it.
func (s *Store) UpdateCard(ctx context.Context, id int64, p CardPatch, viewing string) (*Card, error) {
	var q *journeyRow
	if viewing != "" {
		var err error
		if q, err = getJourney(ctx, s.db, viewing); err != nil {
			return nil, err
		}
	} else if p.Final != nil {
		return nil, errf(ErrInvalid, "a crowning quest belongs to a journey: set it with PATCH /api/journeys/{key} {final} (mikado journey crown)")
	}
	var mark string
	if p.Mark != nil {
		var err error
		if mark, err = cleanMark(*p.Mark); err != nil {
			return nil, err
		}
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		c, err := liveCard(ctx, tx, id)
		if err != nil {
			return err
		}
		k := Key(id)
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
		if p.URL != nil {
			u := strings.TrimSpace(*p.URL)
			if u != c.URL {
				text := k + " link cleared"
				if u != "" {
					text = k + " link set to " + u
				}
				if err := set("url", u, "edit", text); err != nil {
					return err
				}
			}
		}
		if p.Mark != nil && mark != c.Mark {
			text := k + " mark cleared"
			if mark != "" {
				text = k + " mark set to " + mark
			}
			if err := set("mark", mark, "edit", text); err != nil {
				return err
			}
		}
		if p.WaitingOn != nil {
			who := strings.TrimSpace(*p.WaitingOn)
			if who != c.WaitingOn {
				since, text := "", k+" no longer awaits a reply: a plain quest now"
				if who != "" {
					since, text = s.now().Format("2006-01-02"), k+" now awaits a reply from "+who
				}
				if err := exec(`UPDATE cards SET waiting_on = ?, since = ? WHERE id = ?`, who, since, id); err != nil {
					return err
				}
				if err := s.cardEvent(ctx, tx, id, "edit", text); err != nil {
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
					return errf(ErrInvalid, "say why the quest is abandoned (cancelReason)")
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
						if err := cancel(sq.ID, fmt.Sprintf("%s abandoned with its quest %s: %s", Key(sq.ID), k, reason)); err != nil {
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
				return s.clearFinal(ctx, tx, q, "crowning quest "+k+" unset; the journey has no crowning quest")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.card(ctx, id, viewing)
}
