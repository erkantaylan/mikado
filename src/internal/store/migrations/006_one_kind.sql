-- mikado no longer knows about GitHub, and quests have one kind: a title and
-- a status of their own, set by hand, with an optional link (url) and a short
-- mark. A petition is a quest awaiting a reply from someone (waiting_on).
--
-- Issue quests become plain quests with the same id. Each keeps its title:
-- its own, else the last one GitHub gave (it lived only in github_cache), else
-- its ref. Nothing else from GitHub survives: its state, assignees and the
-- cache go, and every flag (fulfilled, abandoned, underway) is mikado's own as
-- it was, so an issue closed on GitHub but never fulfilled here is open again.
-- Errands become plain quests; petitions stay petitions.
--
-- SQLite cannot drop constrained columns, so cards is rebuilt (the migration
-- runs with foreign keys off, and checks them before it commits).
CREATE TABLE cards_new (
    id             INTEGER PRIMARY KEY,
    title          TEXT NOT NULL,
    url            TEXT NOT NULL DEFAULT '',  -- any link, never parsed
    mark           TEXT NOT NULL DEFAULT '',  -- free text, at most 15 characters
    done           INTEGER NOT NULL DEFAULT 0,
    owner          TEXT NOT NULL DEFAULT '',  -- the hero
    waiting_on     TEXT NOT NULL DEFAULT '',  -- set: a petition, awaiting a reply from them
    since          TEXT NOT NULL DEFAULT '',  -- petition: the date it was made
    side_of        INTEGER REFERENCES cards(id),
    found_while    INTEGER REFERENCES cards(id),
    reason         TEXT NOT NULL DEFAULT '',
    npc            INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    removed_at     TEXT,                      -- struck: gone (kept for the log)
    removed_reason TEXT NOT NULL DEFAULT '',
    cancelled_at   TEXT,                      -- abandoned: won't do, stays on the map
    cancel_reason  TEXT NOT NULL DEFAULT '',
    working_since  TEXT,                      -- someone is on it right now
    working_by     TEXT NOT NULL DEFAULT ''
);

INSERT INTO cards_new (id, title, done, owner, waiting_on, since, side_of, found_while, reason, npc,
        created_at, removed_at, removed_reason, cancelled_at, cancel_reason, working_since, working_by)
SELECT c.id,
    COALESCE(NULLIF(TRIM(c.title), ''), NULLIF(TRIM(g.title), ''), c.ref, ''),
    c.done,
    c.owner,
    CASE WHEN c.kind = 'awaiting' THEN c.waiting_on ELSE '' END,
    CASE WHEN c.kind = 'awaiting' THEN c.since ELSE '' END,
    c.side_of, c.found_while, c.reason, c.npc, c.created_at, c.removed_at, c.removed_reason,
    c.cancelled_at, c.cancel_reason, c.working_since, c.working_by
FROM cards c
LEFT JOIN github_cache g ON c.kind = 'issue' AND g.ref_key = c.ref_key;

DROP TABLE cards;
ALTER TABLE cards_new RENAME TO cards;
CREATE INDEX cards_side_of ON cards(side_of);
DROP TABLE github_cache;
