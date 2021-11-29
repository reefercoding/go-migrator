// Package gomigrator provides a very basic and simple
// migration package for MySQL based applications.
package gomigrator

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var (
	// In MySQL table used to keep track of migration versions.
	GomigratorTable = "gomigrator_version"
	// InfoLogger is a logger function which by default uses the Go log package,
	// this variable may be stubbed with your own logger function.
	InfoLogger = DefaultInfoLogger
	// DB used to prevent passing connection to every function in package.
	db *sql.DB
)

// Migration object, containing the version, name and path.
type migration struct {
	version int
	name    string
	path    string
}

// Migrate starts the migrate process by using a given
// *sql.DB (must contain valid connection to SQL instance),
// and a migrationsDir which must be an existing directory.
func Migrate(d *sql.DB, migrationsDir string) error {
	db = d
	exists := gomigratorTableExists()
	if !exists {
		createMigratorTable()
	}
	lastVersion, err := checkLastMigration()
	if err != nil {
		return err
	}
	versions, foundMigrations, err := scanMigrationsDir(migrationsDir)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		InfoLogger("no migrations found")
		return nil
	}
	if lastVersion == versions[len(versions)-1] {
		InfoLogger("migrations up-to-date (last version: %d)", lastVersion)
		return nil
	}
	for _, version := range versions {
		if version <= lastVersion {
			continue
		}
		err = executeMigration(foundMigrations[version])
		if err != nil {
			return err
		}
	}
	return nil
}

// Executes a migration and rolls back on failure,
// keep in mind that some MySQL actions cannot be rolled back (e.g. creating a table).
func executeMigration(m migration) error {
	mgFile, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("could not read migration file %s, error: %w", m.name, err)
	}
	queries := strings.Split(string(mgFile), ";")
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("could not initiate transaction for migration %s, error: %w", m.name, err)
	}
	for _, q := range queries {
		if q == "" {
			continue
		}
		_, err := tx.Exec(q)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error during migration %s, rolled back, cause: %w", m.name, err)
		}
	}
	// Update migrator table, insert new migration.
	_, err = tx.Exec(fmt.Sprintf("INSERT INTO %s (version, title) VALUES (?, ?)", GomigratorTable), m.version, m.name)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error during execution of %s table: %w", GomigratorTable, err)
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error during commit migration %s, rolled back, cause: %w", m.name, err)
	}
	InfoLogger("successfully migrated: %s", m.name)
	return nil
}

// Checks if the gomigrator table exists.
func gomigratorTableExists() bool {
	result, err := db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 1", GomigratorTable))
	if err != nil {
		return false
	}
	result.Close()
	return true
}

// Create gomigrator table if not exists\.
func createMigratorTable() error {
	_, err := db.Exec(
		fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s (version INT NOT NULL, title VARCHAR(255) NOT NULL, 
				executed_at DATETIME NOT NULL DEFAULT NOW(), UNIQUE(version))`,
			GomigratorTable,
		),
	)
	if err != nil {
		return fmt.Errorf("could not create migrator table: %w", err)
	}
	return nil
}

// Get last migration version.
func checkLastMigration() (int, error) {
	var lastVersion int
	row := db.QueryRow(
		fmt.Sprintf(
			"SELECT version FROM %s ORDER BY version DESC LIMIT 1",
			GomigratorTable,
		),
	)
	if err := row.Scan(&lastVersion); err != nil {
		if err == sql.ErrNoRows {
			InfoLogger("no previous migration versions detected")
			return 0, nil
		}
		return 0, fmt.Errorf("error checking version: %w", err)
	}
	InfoLogger("last migration version: %d", lastVersion)
	return lastVersion, nil
}

// Scans migration directory, returns a slice of integers containing the versions sorted,
// a map[int]migration, containing the migration objects mapped by version or an error.
func scanMigrationsDir(migrationsDir string) ([]int, map[int]migration, error) {
	items, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open migrations directory: %w", err)
	}
	var foundMigrations map[int]migration = make(map[int]migration)
	var versions []int
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		if !strings.Contains(item.Name(), ".sql") {
			return nil, nil, fmt.Errorf("file is not sql file: %s", item.Name())
		}
		filename := strings.ReplaceAll(item.Name(), ".sql", "")
		parts := strings.Split(filename, "_")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf(
				`illegal migration filename %s, can only contain _ to divide version and name like 1_create-user-table.sql`,
				item.Name(),
			)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, nil, fmt.Errorf(
				"illegal version in filename %s, version can only be a single integer like 1_create-user-table.sql",
				item.Name(),
			)
		}
		versions = append(versions, version)
		foundMigrations[version] = migration{
			name:    parts[1],
			path:    filepath.Join(migrationsDir, item.Name()),
			version: version,
		}
	}
	sort.Ints(versions)
	return versions, foundMigrations, nil
}

// Default info logger used in this package.
// Stub the InfoLogger variable to replace this by your own logger.
func DefaultInfoLogger(msg string, v ...interface{}) {
	if len(v) == 0 {
		log.Println(msg)
		return
	}
	log.Printf(msg, v...)
}
