package store

import (
	"context"
	"errors"
	"fmt"

	"mikado/internal/github"
)

// Card kinds.
const (
	KindIssue    = "issue"
	KindErrand   = "errand"
	KindAwaiting = "awaiting"
)

// Computed card statuses.
const (
	StatusCancelled = "cancelled"
	StatusDone      = "done"
	StatusLocked    = "locked"
	StatusAwaiting  = "awaiting"
	StatusAvailable = "available"
)

// GitHub is what the store needs from GitHub; *github.Client implements it and
// tests use a fake.
type GitHub interface {
	// Issues returns the issues that exist, keyed by Ref.Key(); missing ones
	// are absent from the map.
	Issues(ctx context.Context, refs []github.Ref) (map[string]github.Issue, error)
	Assign(ctx context.Context, ref github.Ref, add, remove []string) error
	Assignees(ctx context.Context, owner, repo string) ([]string, error)
}

// Card is a card as the API shows it: stored fields, what GitHub says about
// it (issues), and its computed status.
type Card struct {
	ID         int64    `json:"id"`
	Key        string   `json:"key"` // "Q142": how people and the AI name the card
	Kind       string   `json:"kind"`
	Ref        string   `json:"ref,omitempty"`
	URL        string   `json:"url,omitempty"`
	Title      string   `json:"title"`
	Done       bool     `json:"done"`
	State      string   `json:"state,omitempty"`
	Assignees  []string `json:"assignees"`
	Owner      string   `json:"owner,omitempty"`
	WaitingOn  string   `json:"waitingOn,omitempty"`
	Since      string   `json:"since,omitempty"`
	Final      bool     `json:"final"`
	SideOf     *int64   `json:"sideOf,omitempty"`
	FoundWhile *int64   `json:"foundWhile,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	NPC        bool     `json:"npc"`
	// Cancelled: won't do — cancelled here, or closed on GitHub as not
	// planned / duplicate. A cancelled card blocks nothing and counts in no total.
	Cancelled    bool   `json:"cancelled"`
	CancelReason string `json:"cancelReason,omitempty"`
	StateReason  string `json:"stateReason,omitempty"` // issues: GitHub's raw stateReason
	// Working: someone is on it right now. Never set on a done or cancelled card.
	Working      bool   `json:"working"`
	WorkingSince string `json:"workingSince,omitempty"`
	WorkingBy    string `json:"workingBy,omitempty"`
	Status       string `json:"status"`
	OpenBefore   int    `json:"openBefore"`
	// AlsoIn lists the journeys the card is a member of, other than the one
	// being viewed (all of them when no journey is being viewed).
	AlsoIn []JourneyRef `json:"alsoIn"`
	// Crowns is set on a card that crowns another journey. On a journey's
	// chart (GET /api/journeys/{key}) such a card stands for that whole
	// journey, drawn as one journey card; seen globally (GET /api/cards) it
	// names the journey it crowns. Never set on the viewed journey's own
	// crowning quest.
	Crowns *Crowns `json:"crowns,omitempty"`
}

// Crowns is the journey a card crowns, as its journey card shows it: the
// journey's state and its main-quest progress, counted as the atlas counts it
// (a journey folded into it counts as one quest), and what is still to do in it.
type Crowns struct {
	Key        string      `json:"key"`
	Title      string      `json:"title"`
	State      string      `json:"state"`
	ArchivedAt string      `json:"archivedAt,omitempty"`
	Done       int         `json:"done"`
	Total      int         `json:"total"`
	Working    int         `json:"working"` // quests underway in it
	Open       []OpenQuest `json:"open"`    // its main quests neither fulfilled nor abandoned
}

// OpenQuest is a quest still to do in a journey, as a journey card lists it.
type OpenQuest struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Working bool   `json:"working"`
}

// JourneyLink names a journey with its state, as the atlas links it.
type JourneyLink struct {
	Key        string `json:"key"`
	Title      string `json:"title"`
	State      string `json:"state"`
	ArchivedAt string `json:"archivedAt,omitempty"`
}

// JourneyRef names a journey.
type JourneyRef struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

// CardView is one card seen globally: the journeys it is in, the cards it
// needs, the cards that need it and its side quests.
type CardView struct {
	Card       Card         `json:"card"`
	Journeys   []JourneyRef `json:"journeys"`
	Needs      []Card       `json:"needs"`
	NeededBy   []Card       `json:"neededBy"`
	SideQuests []Card       `json:"sideQuests"`
	GitHub     string       `json:"github,omitempty"`
}

// Need says From needs To done first.
type Need struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// Event is one line of the journey log (the chronicle).
type Event struct {
	ID     int64  `json:"id"`
	At     string `json:"at"`
	Kind   string `json:"kind"`
	CardID *int64 `json:"cardId,omitempty"`
	Text   string `json:"text"`
}

// JourneyInfo is the journey's own fields.
type JourneyInfo struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	FinalCardID *int64 `json:"finalCardId"`
	State       string `json:"state"`                // active, complete or cancelled
	ArchivedAt  string `json:"archivedAt,omitempty"` // set while the journey is archived
}

// Journey states, from the final card.
const (
	JourneyActive    = "active"
	JourneyComplete  = "complete"
	JourneyCancelled = "cancelled"
)

// JourneyView is everything the war table needs. GitHub is set when live
// data could not be fetched and cached data is shown instead.
type JourneyView struct {
	Journey JourneyInfo `json:"journey"`
	Cards   []Card      `json:"cards"`
	Needs   []Need      `json:"needs"`
	Log     []Event     `json:"log"`
	GitHub  string      `json:"github,omitempty"`
}

// Progress counts done cards out of a total.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// JourneySummary is one journey on the atlas.
type JourneySummary struct {
	Key          string   `json:"key"`
	Title        string   `json:"title"`
	Main         Progress `json:"main"`
	Achievements Progress `json:"achievements"`
	State        string   `json:"state"`
	Available    int      `json:"available"`
	Awaiting     int      `json:"awaiting"`
	Cancelled    int      `json:"cancelled"`
	InProgress   int      `json:"inProgress"`
	Heroes       []string `json:"heroes"`
	Repos        []string `json:"repos"`
	LastActivity string   `json:"lastActivity"`
	// ArchivedAt is set while the journey is archived: put away, off the
	// atlas's shelves, otherwise unchanged.
	ArchivedAt string `json:"archivedAt,omitempty"`
	// BlockedBy are the journeys whose crowning quests are on this journey's
	// chart as journey cards; Blocks are the journeys with this one's on theirs.
	BlockedBy []JourneyLink `json:"blockedBy"`
	Blocks    []JourneyLink `json:"blocks"`
}

// NewCard is a request to add a card. Cards are global; a card is in a journey
// once it is linked into it (FinalOf, NeededBy a member, SideOf a member).
type NewCard struct {
	Kind       string  `json:"kind"`
	Ref        string  `json:"ref"`
	Title      string  `json:"title"`
	FinalOf    string  `json:"finalOf"` // make it the final card of this journey (J7)
	Final      bool    `json:"final"`   // journey-scoped route only: final of that journey
	SideOf     *int64  `json:"sideOf"`
	FoundWhile *int64  `json:"foundWhile"`
	Reason     string  `json:"reason"`
	Needs      []int64 `json:"needs"`
	NeededBy   []int64 `json:"neededBy"`
	WaitingOn  string  `json:"waitingOn"`
	Owner      string  `json:"owner"`
	NPC        bool    `json:"npc"`
}

// CardPatch changes a card; nil fields are left alone.
type CardPatch struct {
	Done  *bool   `json:"done"`
	Owner *string `json:"owner"`
	NPC   *bool   `json:"npc"`
	Title *string `json:"title"`
	// Final (journey-scoped route only) makes the card that journey's final,
	// or clears it if it was.
	Final *bool `json:"final"`
	// Cancelled cancels (with CancelReason, required) or uncancels.
	Cancelled    *bool   `json:"cancelled"`
	CancelReason *string `json:"cancelReason"`
	// Working starts (optionally WorkingBy) or stops work on the card.
	Working   *bool   `json:"working"`
	WorkingBy *string `json:"workingBy"`
}

// ErrKind classifies store errors so the API can pick a status code.
type ErrKind int

const (
	ErrInvalid  ErrKind = iota + 1 // the request makes no sense (400)
	ErrNotFound                    // no such journey/card/need (404)
	ErrConflict                    // clashes with existing data (409)
	ErrUpstream                    // GitHub (gh) failed (502)
)

// Error is a store error with a message meant for the person using mikado.
type Error struct {
	Kind ErrKind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func errf(kind ErrKind, format string, args ...any) error {
	return &Error{Kind: kind, Msg: fmt.Sprintf(format, args...)}
}

// KindOf returns the ErrKind of err, or 0 for an unexpected error.
func KindOf(err error) ErrKind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return 0
}

// JourneyPatch changes a journey; nil fields are left alone.
type JourneyPatch struct {
	Title *string `json:"title"`
	Final *int64  `json:"final"`
	// Archived puts the journey away (true) or brings it back (false).
	Archived *bool `json:"archived"`
}

// Key is how a card id is shown: Q142. A card is a quest.
func Key(id int64) string { return fmt.Sprintf("Q%d", id) }

// JourneyKey is how a journey id is shown: J7.
func JourneyKey(id int64) string { return fmt.Sprintf("J%d", id) }

// kindNoun is how a sentence names a kind of quest: "an issue", "an errand",
// "a petition" (stored as awaiting).
func kindNoun(kind string) string {
	switch kind {
	case KindIssue:
		return "an issue"
	case KindErrand:
		return "an errand"
	case KindAwaiting:
		return "a petition"
	}
	return kind
}
