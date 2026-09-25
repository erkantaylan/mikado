package store

import (
	"context"
	"database/sql"
	"strings"
)

// Host is a host name the server answers besides localhost, stored so it
// survives restarts.
type Host struct {
	Name    string `json:"name"`
	AddedAt string `json:"addedAt"`
}

// CleanHost checks a host name to accept and returns it in the form it is
// stored and compared in: a name ("mikado.home") or a wildcard for its
// subdomains ("*.ts.net"), lowercase, without a trailing dot. A scheme, port
// or path is refused.
func CleanHost(name string) (string, error) {
	h := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	rest := strings.TrimPrefix(h, "*.")
	ok := rest != "" && !strings.HasPrefix(rest, ".") && !strings.Contains(rest, "..")
	for _, r := range rest {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_') {
			ok = false
		}
	}
	if !ok {
		return "", errf(ErrInvalid, "%q: give a host name like mikado.home or *.ts.net (no scheme, port or path)", name)
	}
	return h, nil
}

// Hosts lists the stored hosts by name.
func (s *Store) Hosts(ctx context.Context) ([]Host, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, added_at FROM hosts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Host{}
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.Name, &h.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// AddHost stores a host (see CleanHost). Adding one that is already stored
// changes nothing and reports created false.
func (s *Store) AddHost(ctx context.Context, name string) (h Host, created bool, err error) {
	if h.Name, err = CleanHost(name); err != nil {
		return h, false, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO hosts (name, added_at) VALUES (?, ?) ON CONFLICT (name) DO NOTHING`, h.Name, s.stamp())
	if err != nil {
		return h, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return h, false, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT added_at FROM hosts WHERE name = ?`, h.Name).Scan(&h.AddedAt)
	return h, n > 0, err
}

// RemoveHost forgets a stored host.
func (s *Store) RemoveHost(ctx context.Context, name string) error {
	h, err := CleanHost(name)
	if err != nil {
		return err
	}
	var gone string
	err = s.db.QueryRowContext(ctx, `DELETE FROM hosts WHERE name = ? RETURNING name`, h).Scan(&gone)
	if err == sql.ErrNoRows {
		return errf(ErrNotFound, "%s is not an accepted host (see `mikado hosts`)", h)
	}
	return err
}
