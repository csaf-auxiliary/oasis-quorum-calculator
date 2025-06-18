// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2024 Intevation GmbH <https://intevation.de>

// Package database implements the handling of the database.
package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/config"
	"github.com/jmoiron/sqlx"

	_ "github.com/mattn/go-sqlite3" // Link SQLite 3 driver.
)

// ErrTerminateMigration is returned by NewDatabase if a migration
// was done and the configuration forces a termination.
var ErrTerminateMigration = errors.New("terminate migration")

// Database implements the handling with the database connection pool.
type Database struct {
	DB *sqlx.DB
}

func sqlite3URL(url string) string {
	if !strings.ContainsRune(url, '?') {
		return url + "?_journal=WAL&_timeout=5000&_fk=true"
	}
	return url
}

// NewDatabase creates a new connection pool.
func NewDatabase(ctx context.Context, cfg *config.Database) (*Database, error) {

	if cfg.Driver != "sqlite3" {
		return nil, fmt.Errorf("database driver %q is not supported", cfg.Driver)
	}

	create, err := needsCreation(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if !cfg.Migrate && create {
		return nil, errors.New("setup migration needed")
	}

	url := sqlite3URL(cfg.DatabaseURL)

	db, err := sqlx.ConnectContext(ctx, "sqlite3", url)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database %q: %w", url, err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdletime)

	migs, err := listMigrations()
	if err != nil {
		return nil, err
	}

	if create {
		if err := createDatabase(ctx, cfg, db, migs); err != nil {
			return nil, fmt.Errorf("creating database %q failed: %w", url, err)
		}
		if cfg.TerminateAfterMigration {
			return nil, ErrTerminateMigration
		}
		return &Database{DB: db}, nil
	}

	database := &Database{DB: db}

	if err := database.applyMigrations(ctx, cfg, migs); err != nil {
		return nil, err
	}

	if cfg.Migrate && cfg.TerminateAfterMigration {
		return nil, ErrTerminateMigration
	}

	return database, nil
}

// Close closes the connection pool.
func (db *Database) Close(context.Context) {
	// Currently not needed.
}
