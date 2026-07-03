package db

// Migrate runs the base schema DDL and any incremental column additions.
// It is safe to call on every startup; all statements are idempotent.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Column additions — errors are intentionally swallowed because SQLite
	// returns an error when the column already exists (no IF NOT EXISTS on
	// ALTER TABLE ADD COLUMN in SQLite < 3.37.0).
	s.runMigrations()
	return nil
}
