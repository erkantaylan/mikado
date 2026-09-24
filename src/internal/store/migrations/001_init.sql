-- One global graph of cards and needs; a quest is a view onto it (its final
-- card, everything that card transitively needs, and the side quests of any
-- of those). Membership is computed, never stored. Plain SQL: booleans are
-- 0/1, timestamps RFC 3339 UTC text.

-- A card exists once. An issue card's title/state/assignees come from
-- github_cache, never from here. Status is computed.
CREATE TABLE cards (
    id             INTEGER PRIMARY KEY,
    kind           TEXT NOT NULL CHECK (kind IN ('issue', 'errand', 'awaiting')),
    ref            TEXT,                      -- owner/repo#n in GitHub's casing (issue only)
    ref_key        TEXT,                      -- lower(ref)
    title          TEXT NOT NULL DEFAULT '',  -- errand/awaiting only
    done           INTEGER NOT NULL DEFAULT 0, -- errand/awaiting only
    owner          TEXT NOT NULL DEFAULT '',
    waiting_on     TEXT NOT NULL DEFAULT '',  -- awaiting only
    since          TEXT NOT NULL DEFAULT '',  -- awaiting only: date added
    side_of        INTEGER REFERENCES cards(id),
    found_while    INTEGER REFERENCES cards(id),
    reason         TEXT NOT NULL DEFAULT '',
    npc            INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    removed_at     TEXT,                      -- remove: gone (kept for the log)
    removed_reason TEXT NOT NULL DEFAULT '',
    cancelled_at   TEXT,                      -- cancel: won't do, stays on the map
    cancel_reason  TEXT NOT NULL DEFAULT '',
    working_since  TEXT,                      -- someone is on it right now
    working_by     TEXT NOT NULL DEFAULT '',
    CHECK ((kind = 'issue') = (ref_key IS NOT NULL))
);
-- At most one live card per GitHub issue, globally.
CREATE UNIQUE INDEX cards_ref ON cards(ref_key) WHERE removed_at IS NULL AND ref_key IS NOT NULL;
CREATE INDEX cards_side_of ON cards(side_of);

CREATE TABLE quests (
    id         INTEGER PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    title      TEXT NOT NULL,
    final_card INTEGER REFERENCES cards(id), -- NULL until one is set
    created_at TEXT NOT NULL
);

-- from_card needs to_card done first. Global and acyclic; enforced by the store.
CREATE TABLE needs (
    from_card INTEGER NOT NULL REFERENCES cards(id),
    to_card   INTEGER NOT NULL REFERENCES cards(id),
    PRIMARY KEY (from_card, to_card),
    CHECK (from_card <> to_card)
);
CREATE INDEX needs_to ON needs(to_card);

-- The log. A quest-level event (quest created, final set) has quest_id; a
-- card event has quest_id NULL and shows in the log of every quest the card
-- is currently a member of. text is written at the time of the change.
CREATE TABLE events (
    id       INTEGER PRIMARY KEY,
    quest_id INTEGER REFERENCES quests(id),
    card_id  INTEGER REFERENCES cards(id),
    at       TEXT NOT NULL,
    kind     TEXT NOT NULL,
    text     TEXT NOT NULL
);
CREATE INDEX events_quest ON events(quest_id);
CREATE INDEX events_card ON events(card_id);

-- What GitHub last said about an issue, keyed by lower(owner/repo#n).
CREATE TABLE github_cache (
    ref_key      TEXT PRIMARY KEY,
    ref          TEXT NOT NULL,
    title        TEXT NOT NULL,
    state        TEXT NOT NULL,
    state_reason TEXT NOT NULL DEFAULT '', -- COMPLETED, NOT_PLANNED, DUPLICATE, REOPENED
    assignees    TEXT NOT NULL,            -- JSON array of logins
    url          TEXT NOT NULL,
    fetched_at   TEXT NOT NULL
);
