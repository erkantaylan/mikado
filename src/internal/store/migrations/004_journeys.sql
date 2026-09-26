-- Quests become journeys, named by their key (J7) alone: the slug is gone.
-- SQLite cannot drop a UNIQUE column, so the table is rebuilt; events, the
-- only table pointing at it, is rebuilt with it (quest_id → journey_id).
CREATE TABLE journeys (
    id          INTEGER PRIMARY KEY,
    title       TEXT NOT NULL,
    final_card  INTEGER REFERENCES cards(id), -- NULL until one is set
    created_at  TEXT NOT NULL,
    archived_at TEXT                          -- NULL: not archived
);
INSERT INTO journeys (id, title, final_card, created_at, archived_at)
    SELECT id, title, final_card, created_at, archived_at FROM quests;

-- A journey-level event (journey created, final set) has journey_id; a card
-- event has journey_id NULL and shows in the log of every journey the card is
-- currently a member of. text is written at the time of the change.
CREATE TABLE events_new (
    id         INTEGER PRIMARY KEY,
    journey_id INTEGER REFERENCES journeys(id),
    card_id    INTEGER REFERENCES cards(id),
    at         TEXT NOT NULL,
    kind       TEXT NOT NULL,
    text       TEXT NOT NULL
);
INSERT INTO events_new (id, journey_id, card_id, at, kind, text)
    SELECT id, quest_id, card_id, at, kind, text FROM events;
DROP TABLE events;
DROP TABLE quests;
ALTER TABLE events_new RENAME TO events;
CREATE INDEX events_journey ON events(journey_id);
CREATE INDEX events_card ON events(card_id);
