package store

import (
	"context"
	"sort"
	"strings"
)

// SearchResult is what the dashboard's search finds: quests, then deeds.
type SearchResult struct {
	Quests []QuestInfo `json:"quests"`
	Deeds  []Card      `json:"deeds"`
	// Exact is set when the query names one deed by id or issue (M142,
	// owner/repo#n, an issue URL): that deed, also first in Deeds.
	Exact *Card `json:"exact,omitempty"`
}

// Search limits.
const (
	searchQuests = 8
	searchDeeds  = 20
)

// Search finds quests (by slug and title) and live deeds (by id, issue and
// title) whose text holds every word of q, ignoring case. It runs on every
// keystroke, so issue titles come from the GitHub cache only, never live.
// Deeds still to do come before finished ones; each deed's AlsoIn lists
// every quest it is in. An empty q lists the quests that are not archived.
func (s *Store) Search(ctx context.Context, q string) (*SearchResult, error) {
	g, err := loadGraph(ctx, s.db)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(g.cards))
	for id := range g.cards {
		ids = append(ids, id)
	}
	rows := g.rows(ids)
	var keys []string
	for _, c := range rows {
		if c.Kind == KindIssue {
			keys = append(keys, c.RefKey)
		}
	}
	gh, err := loadCache(ctx, s.db, keys)
	if err != nil {
		return nil, err
	}
	cards := map[int64]Card{}
	for _, c := range buildCards(rows, g.needsAmong(ids), gh) {
		cards[c.ID] = c
	}
	in := g.membership()
	withQuests := func(c Card) Card {
		c.Final = g.isFinal(c.ID)
		c.AlsoIn = alsoIn(in[c.ID], nil)
		return c
	}

	words := strings.Fields(fold(q))
	has := func(text string) bool {
		text = fold(text)
		for _, w := range words {
			if !strings.Contains(text, w) {
				return false
			}
		}
		return true
	}

	out := &SearchResult{Quests: []QuestInfo{}, Deeds: []Card{}}
	for _, qr := range g.quests {
		if len(words) == 0 && qr.ArchivedAt != "" {
			continue
		}
		if !has(qr.Slug + " " + qr.Title) {
			continue
		}
		info := QuestInfo{Slug: qr.Slug, Title: qr.Title, FinalCardID: qr.Final, State: QuestActive, ArchivedAt: qr.ArchivedAt}
		if qr.Final != nil {
			if c, ok := cards[*qr.Final]; ok {
				info.State = questState(&c)
			}
		}
		out.Quests = append(out.Quests, info)
	}
	// Active quests first, archived ones last, newest first within each.
	rank := func(q QuestInfo) int {
		switch {
		case q.ArchivedAt != "":
			return 2
		case q.State != QuestActive:
			return 1
		}
		return 0
	}
	sort.SliceStable(out.Quests, func(i, j int) bool { return rank(out.Quests[i]) < rank(out.Quests[j]) })
	if len(out.Quests) > searchQuests {
		out.Quests = out.Quests[:searchQuests]
	}
	if len(words) == 0 {
		return out, nil
	}

	var exact int64
	if id, err := s.Resolve(ctx, strings.TrimSpace(q)); err == nil {
		if c, ok := cards[id]; ok {
			exact = id
			ec := withQuests(c)
			out.Exact = &ec
			out.Deeds = append(out.Deeds, ec)
		}
	}
	var found []Card
	for _, c := range cards {
		if c.ID != exact && has(c.Key+" "+c.Ref+" "+c.Title) {
			found = append(found, c)
		}
	}
	finished := func(c Card) bool { return c.Status == StatusDone || c.Status == StatusCancelled }
	sort.Slice(found, func(i, j int) bool {
		if fi, fj := finished(found[i]), finished(found[j]); fi != fj {
			return !fi
		}
		return found[i].ID > found[j].ID
	})
	for _, c := range found {
		if len(out.Deeds) == searchDeeds {
			break
		}
		out.Deeds = append(out.Deeds, withQuests(c))
	}
	return out, nil
}

// fold lowercases for matching, and lets a dotted or dotless i match
// either (Turkish titles are common here).
func fold(s string) string {
	return strings.NewReplacer("i̇", "i", "ı", "i").Replace(strings.ToLower(s))
}
