package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

func (s *Store) migrate(ctx context.Context) error {
	return s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		if _, err := tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY)"); err != nil {
			return err
		}
		var current int
		if err := tx.conn.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&current); err != nil {
			return err
		}
		names, err := fs.Glob(migrations, "migrations/*.sql")
		if err != nil {
			return err
		}
		if current > len(names) {
			return fmt.Errorf("ledger schema %d is newer than supported schema %d", current, len(names))
		}
		for i, name := range names {
			version, err := strconv.Atoi(strings.Split(strings.TrimPrefix(name, "migrations/"), "_")[0])
			if err != nil || version != i+1 {
				return fmt.Errorf("invalid migration name %q", name)
			}
			if version <= current {
				continue
			}
			body, err := migrations.ReadFile(name)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, string(body)); err != nil {
				return fmt.Errorf("migration %d: %w", version, err)
			}
			if _, err = tx.Exec(ctx, "INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
				return err
			}
		}
		return nil
	})
}
