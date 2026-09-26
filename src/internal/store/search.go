package store

import (
	"context"
	"sort"
	"strings"
)

// SearchResult is what the dashboard's search finds: journeys, then quests.
type SearchResult struct {
	Journeys []JourneyInfo `json:"journeys"`
	Quests   []Card        `json:"quests"`
	// Exact is set when the query names one quest by key or issue (Q142,
	// owner/repo#n, an issue URL): that quest, also first in Quests.
	Exact *Card `json:"exact,omitempty"`
}

// Search limits.
const (
	searchJourneys = 8
	searchQuests   = 20
)

// Search finds journeys (by key and title) and live quests (by key, issue
// and title) whose text holds every word of q, ignoring case. It runs on
// every keystroke, so issue titles come from the GitHub cache only, never
// live. Quests still to do come before finished ones; each quest's AlsoIn
// lists every journey it is in. An empty q lists the journeys that are not
// archived.
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
	withJourneys := func(c Card) Card {
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

	out := &SearchResult{Journeys: []JourneyInfo{}, Quests: []Card{}}
	for _, jr := range g.journeys {
		if len(words) == 0 && jr.ArchivedAt != "" {
			continue
		}
		if !has(jr.Key() + " " + jr.Title) {
			continue
		}
		info := JourneyInfo{Key: jr.Key(), Title: jr.Title, FinalCardID: jr.Final, State: JourneyActive, ArchivedAt: jr.ArchivedAt}
		if jr.Final != nil {
			if c, ok := cards[*jr.Final]; ok {
				info.State = journeyState(&c)
			}
		}
		out.Journeys = append(out.Journeys, info)
	}
	// Active journeys first, archived ones last, newest first within each.
	rank := func(j JourneyInfo) int {
		switch {
		case j.ArchivedAt != "":
			return 2
		case j.State != JourneyActive:
			return 1
		}
		return 0
	}
	sort.SliceStable(out.Journeys, func(i, j int) bool { return rank(out.Journeys[i]) < rank(out.Journeys[j]) })
	if len(out.Journeys) > searchJourneys {
		out.Journeys = out.Journeys[:searchJourneys]
	}
	if len(words) == 0 {
		return out, nil
	}

	var exact int64
	if id, err := s.resolveQuest(ctx, strings.TrimSpace(q)); err == nil {
		if c, ok := cards[id]; ok {
			exact = id
			ec := withJourneys(c)
			out.Exact = &ec
			out.Quests = append(out.Quests, ec)
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
		if len(out.Quests) == searchQuests {
			break
		}
		out.Quests = append(out.Quests, withJourneys(c))
	}
	return out, nil
}

// fold lowercases for matching, and lets a dotted or dotless i match
// either (Turkish titles are common here).
func fold(s string) string {
	return strings.NewReplacer("i̇", "i", "ı", "i").Replace(strings.ToLower(s))
}
