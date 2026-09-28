package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Region is a group of journeys, as the atlas bands them and `mikado region
// list` lists them. Every journey lives in exactly one region, and a quest
// stays inside one: no link may put it on journeys of two.
type Region struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	Journeys  int    `json:"journeys"` // how many journeys live in it, archived ones included
}

// RegionRef names a region.
type RegionRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// RegionPatch changes a region; nil fields are left alone.
type RegionPatch struct {
	Name *string `json:"name"`
}

// RegionDeleted says which region was deleted.
type RegionDeleted struct {
	Region RegionRef `json:"region"`
}

// RegionKey is how a region id is shown: R2.
func RegionKey(id int64) string { return fmt.Sprintf("R%d", id) }

var regionIDRe = regexp.MustCompile(`^(?i)r-?([1-9][0-9]*)$`)

// ParseRegionID reads a region key: R2, R-2 or r2. It reports false for
// anything else.
func ParseRegionID(s string) (int64, bool) {
	m := regionIDRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	return id, err == nil
}

// regionRow is a region as stored.
type regionRow struct {
	ID        int64
	Name      string
	CreatedAt string
}

func (r *regionRow) Key() string    { return RegionKey(r.ID) }
func (r *regionRow) ref() RegionRef { return RegionRef{Key: r.Key(), Name: r.Name} }

// label names a region in a sentence: R2 “Alternet”.
func (r *regionRow) label() string { return r.Key() + " " + quoted(r.Name) }

func loadRegions(ctx context.Context, q querier) (map[int64]*regionRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name, created_at FROM regions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]*regionRow{}
	for rows.Next() {
		var r regionRow
		if err := rows.Scan(&r.ID, &r.Name, &r.CreatedAt); err != nil {
			return nil, err
		}
		out[r.ID] = &r
	}
	return out, rows.Err()
}

// getRegion finds a region by its key (R2) or, ignoring case, its name.
func getRegion(ctx context.Context, q querier, ref string) (*regionRow, error) {
	var r regionRow
	var err error
	if id, ok := ParseRegionID(ref); ok {
		err = q.QueryRowContext(ctx, `SELECT id, name, created_at FROM regions WHERE id = ?`, id).Scan(&r.ID, &r.Name, &r.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, errf(ErrNotFound, "no region %s", RegionKey(id))
		}
		return &r, err
	}
	name := strings.TrimSpace(ref)
	if name == "" {
		return nil, errf(ErrInvalid, "name a region by its key (R2) or its name")
	}
	err = q.QueryRowContext(ctx, `SELECT id, name, created_at FROM regions WHERE lower(name) = lower(?)`, name).Scan(&r.ID, &r.Name, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, errf(ErrNotFound, "no region named %q (see `mikado region list`)", name)
	}
	return &r, err
}

// defaultRegion is where a journey goes when none is named: the oldest region.
func defaultRegion(ctx context.Context, q querier) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM regions ORDER BY id LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, errf(ErrConflict, "there is no region to put the journey in: create one with `mikado region new \"name\"`")
	}
	return id, err
}

// regionFor picks the region a new journey goes to: the one named, else the
// default.
func regionFor(ctx context.Context, q querier, ref string) (int64, error) {
	if strings.TrimSpace(ref) == "" {
		return defaultRegion(ctx, q)
	}
	r, err := getRegion(ctx, q, ref)
	if err != nil {
		return 0, err
	}
	return r.ID, nil
}

// checkRegions refuses a graph in which a quest is on journeys of two
// regions: a link (require, side quest, crown, a journey waiting on another)
// never crosses a region's border.
func (g *graph) checkRegions() error {
	m := g.membership()
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	for _, id := range ids {
		js := m[id]
		for _, j := range js[1:] {
			if j.Region != js[0].Region {
				return errf(ErrConflict, "%s would be on %s in region %s and on %s in region %s; a quest stays inside one region",
					Key(id), js[0].Key(), g.regionLabel(js[0].Region), j.Key(), g.regionLabel(j.Region))
			}
		}
	}
	return nil
}

func (g *graph) regionLabel(id int64) string {
	if r := g.regions[id]; r != nil {
		return r.label()
	}
	return RegionKey(id)
}

// Regions lists every region, oldest first, with how many journeys each holds.
func (s *Store) Regions(ctx context.Context) ([]Region, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id, r.name, r.created_at, COUNT(j.id)
		FROM regions r LEFT JOIN journeys j ON j.region_id = r.id
		GROUP BY r.id ORDER BY r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Region{}
	for rows.Next() {
		var r regionRow
		var n int
		if err := rows.Scan(&r.ID, &r.Name, &r.CreatedAt, &n); err != nil {
			return nil, err
		}
		out = append(out, Region{Key: r.Key(), Name: r.Name, CreatedAt: r.CreatedAt, Journeys: n})
	}
	return out, rows.Err()
}

func (s *Store) region(ctx context.Context, id int64) (*Region, error) {
	rs, err := s.Regions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rs {
		if rs[i].Key == RegionKey(id) {
			return &rs[i], nil
		}
	}
	return nil, errf(ErrNotFound, "no region %s", RegionKey(id))
}

// cleanRegionName trims a region name and refuses one that is empty, reads
// as a region key, or is taken by another region.
func cleanRegionName(ctx context.Context, q querier, name string, self int64) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errf(ErrInvalid, "a region needs a name")
	}
	if _, ok := ParseRegionID(name); ok {
		return "", errf(ErrInvalid, "%q reads as a region key; pick a name that does not", name)
	}
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM regions WHERE lower(name) = lower(?) AND id <> ?`, name, self).Scan(&id)
	if err == nil {
		return "", errf(ErrConflict, "region %s is already called %q", RegionKey(id), name)
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	return name, nil
}

// CreateRegion creates an empty region.
func (s *Store) CreateRegion(ctx context.Context, name string) (*Region, error) {
	var id int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		name, err := cleanRegionName(ctx, tx, name, 0)
		if err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `INSERT INTO regions (name, created_at) VALUES (?, ?) RETURNING id`, name, s.stamp()).Scan(&id)
	})
	if err != nil {
		return nil, err
	}
	return s.region(ctx, id)
}

// UpdateRegion renames a region; its key stays.
func (s *Store) UpdateRegion(ctx context.Context, ref string, p RegionPatch) (*Region, error) {
	var id int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		r, err := getRegion(ctx, tx, ref)
		if err != nil {
			return err
		}
		id = r.ID
		if p.Name == nil {
			return nil
		}
		name, err := cleanRegionName(ctx, tx, *p.Name, r.ID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE regions SET name = ? WHERE id = ?`, name, r.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.region(ctx, id)
}

// DeleteRegion deletes an empty region. The last region stays, so a new
// journey always has somewhere to go.
func (s *Store) DeleteRegion(ctx context.Context, ref string) (*RegionDeleted, error) {
	var out RegionDeleted
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		r, err := getRegion(ctx, tx, ref)
		if err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT id FROM journeys WHERE region_id = ? ORDER BY id`, r.ID)
		if err != nil {
			return err
		}
		var held []string
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			held = append(held, JourneyKey(id))
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(held) > 0 {
			return errf(ErrConflict, "region %s still holds %s: move or delete them first", r.label(), strings.Join(held, ", "))
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM regions`).Scan(&n); err != nil {
			return err
		}
		if n == 1 {
			return errf(ErrConflict, "%s is the last region; a journey always needs one to live in", r.label())
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM regions WHERE id = ?`, r.ID); err != nil {
			return err
		}
		out.Region = r.ref()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// MoveJourneys puts journeys in a region, all in one step, so journeys that
// share quests or wait on each other can move together. It is refused when a
// quest on them is also on a journey that stays behind.
func (s *Store) MoveJourneys(ctx context.Context, region string, keys []string) ([]JourneySummary, error) {
	if len(keys) == 0 {
		return nil, errf(ErrInvalid, "name the journeys to move")
	}
	var ids []int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, key := range keys {
			j, err := getJourney(ctx, tx, key)
			if err != nil {
				return err
			}
			if err := s.moveJourney(ctx, tx, j, region); err != nil {
				return err
			}
			ids = append(ids, j.ID)
		}
		return nil
	})
	if err != nil {
		return nil, s.movedTogether(ctx, err)
	}
	out := make([]JourneySummary, 0, len(ids))
	for _, id := range ids {
		sum, err := s.summary(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *sum)
	}
	return out, nil
}

// movedTogether adds to a refused move how to make it: move the journeys
// together.
func (s *Store) movedTogether(ctx context.Context, err error) error {
	var e *Error
	if !errors.As(err, &e) || e.Kind != ErrConflict || !strings.Contains(e.Msg, "stays inside one region") {
		return err
	}
	return errf(ErrConflict, "%s: move the journeys that share it together (`mikado region move R J...`)", e.Msg)
}

// moveJourney puts a journey in another region. The move is refused (by
// inTx's check) when a quest on it is also on a journey that stays behind.
func (s *Store) moveJourney(ctx context.Context, tx *sql.Tx, j *journeyRow, ref string) error {
	to, err := getRegion(ctx, tx, ref)
	if err != nil {
		return err
	}
	if to.ID == j.Region {
		return nil
	}
	regions, err := loadRegions(ctx, tx)
	if err != nil {
		return err
	}
	from := RegionKey(j.Region)
	if r := regions[j.Region]; r != nil {
		from = r.label()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE journeys SET region_id = ? WHERE id = ?`, to.ID, j.ID); err != nil {
		return err
	}
	return s.event(ctx, tx, &j.ID, nil, "region", fmt.Sprintf("journey moved from region %s to %s", from, to.label()))
}

// regionRef names region id as the graph knows it.
func (g *graph) regionRef(id int64) RegionRef {
	if r := g.regions[id]; r != nil {
		return r.ref()
	}
	return RegionRef{Key: RegionKey(id)}
}
