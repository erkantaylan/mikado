-- A region groups journeys: every journey lives in exactly one, and a quest
-- stays inside one region (the store refuses a link that would put it on
-- journeys of two). The journeys already there go to R1, "Personal".
CREATE TABLE regions (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX regions_name ON regions(lower(name));
INSERT INTO regions (id, name, created_at) VALUES (1, 'Personal', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

-- SQLite cannot add a REFERENCES column with a non-NULL default while foreign
-- keys are on, so the store keeps region_id pointing at a region itself.
ALTER TABLE journeys ADD COLUMN region_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX journeys_region ON journeys(region_id);
