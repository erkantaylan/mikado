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
	Key        string   `json:"key"` // "M142": how people and the AI name the card
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
	// AlsoIn lists the quests the card is a member of, other than the one
	// being viewed (all of them when no quest is being viewed).
	AlsoIn []QuestRef `json:"alsoIn"`
	// Crowns is set on a card that crowns another quest. On a quest's chart
	// (GET /api/quests/{slug}) such a card stands for that whole quest, drawn
	// as one quest card; seen globally (GET /api/cards) it names the quest it
	// crowns. Never set on the viewed quest's own crowning deed.
	Crowns *Crowns `json:"crowns,omitempty"`
}

// Crowns is the quest a card crowns, as its quest card shows it: the
// quest's state and its main-quest progress, counted as the board counts it
// (a quest folded into it counts as one deed), and what is still to do in it.
type Crowns struct {
	Slug       string     `json:"slug"`
	Title      string     `json:"title"`
	State      string     `json:"state"`
	ArchivedAt string     `json:"archivedAt,omitempty"`
	Done       int        `json:"done"`
	Total      int        `json:"total"`
	Working    int        `json:"working"` // deeds underway in it
	Open       []OpenDeed `json:"open"`    // its main-quest deeds neither fulfilled nor abandoned
}

// OpenDeed is a deed still to do in a quest, as a quest card lists it.
type OpenDeed struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Working bool   `json:"working"`
}

// QuestLink names a quest with its state, as the board links it.
type QuestLink struct {
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	State      string `json:"state"`
	ArchivedAt string `json:"archivedAt,omitempty"`
}

// QuestRef names a quest.
type QuestRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// CardView is one card seen globally: the quests it is in, the cards it
// needs, the cards that need it and its side quests.
type CardView struct {
	Card       Card       `json:"card"`
	Quests     []QuestRef `json:"quests"`
	Needs      []Card     `json:"needs"`
	NeededBy   []Card     `json:"neededBy"`
	SideQuests []Card     `json:"sideQuests"`
	GitHub     string     `json:"github,omitempty"`
}

// Need says From needs To done first.
type Need struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// Event is one line of the quest log.
type Event struct {
	ID     int64  `json:"id"`
	At     string `json:"at"`
	Kind   string `json:"kind"`
	CardID *int64 `json:"cardId,omitempty"`
	Text   string `json:"text"`
}

// QuestInfo is the quest's own fields.
type QuestInfo struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	FinalCardID *int64 `json:"finalCardId"`
	State       string `json:"state"`                // active, complete or cancelled
	ArchivedAt  string `json:"archivedAt,omitempty"` // set while the quest is archived
}

// Quest states, from the final card.
const (
	QuestActive    = "active"
	QuestComplete  = "complete"
	QuestCancelled = "cancelled"
)

// QuestView is everything the quest map needs. GitHub is set when live data
// could not be fetched and cached data is shown instead.
type QuestView struct {
	Quest  QuestInfo `json:"quest"`
	Cards  []Card    `json:"cards"`
	Needs  []Need    `json:"needs"`
	Log    []Event   `json:"log"`
	GitHub string    `json:"github,omitempty"`
}

// Progress counts done cards out of a total.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// QuestSummary is one row of the quest board.
type QuestSummary struct {
	Slug         string   `json:"slug"`
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
	// ArchivedAt is set while the quest is archived: put away, off the
	// board's shelves, otherwise unchanged.
	ArchivedAt string `json:"archivedAt,omitempty"`
	// BlockedBy are the quests whose crowning deeds are on this quest's
	// chart as quest cards; Blocks are the quests with this one's on theirs.
	BlockedBy []QuestLink `json:"blockedBy"`
	Blocks    []QuestLink `json:"blocks"`
}

// NewCard is a request to add a card. Cards are global; a card is in a quest
// once it is linked into it (FinalOf, NeededBy a member, SideOf a member).
type NewCard struct {
	Kind       string  `json:"kind"`
	Ref        string  `json:"ref"`
	Title      string  `json:"title"`
	FinalOf    string  `json:"finalOf"` // make it the final card of this quest (slug)
	Final      bool    `json:"final"`   // quest-scoped route only: final of that quest
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
	// Final (quest-scoped route only) makes the card that quest's final, or
	// clears it if it was.
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
	ErrNotFound                    // no such quest/card/need (404)
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

// QuestPatch changes a quest; nil fields are left alone.
type QuestPatch struct {
	Slug  *string `json:"slug"`
	Title *string `json:"title"`
	Final *int64  `json:"final"`
	// Archived puts the quest away (true) or brings it back (false).
	Archived *bool `json:"archived"`
}

// Key is how a card id is shown: M142.
func Key(id int64) string { return fmt.Sprintf("M%d", id) }

// kindNoun is how a sentence names a kind of deed: "an issue", "an errand",
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
