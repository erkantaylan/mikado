-- An archived quest is put away: off the Quest Board's shelves, untouched
-- otherwise (its deeds, chart and chronicle stay). NULL: not archived.
ALTER TABLE quests ADD COLUMN archived_at TEXT;
