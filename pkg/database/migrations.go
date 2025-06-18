// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package database

import (
	"bytes"
	"cmp"
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/config"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
	"github.com/jmoiron/sqlx"
)

//go:embed migrations
var migrations embed.FS

// migration stores the meta information extracted from the
// embedded SQL migration files and their names.
type migration struct {
	version     int64
	description string
	path        string
}

func needsCreation(url string) (bool, error) {
	idx := strings.IndexRune(url, '?')
	if idx != -1 {
		url = url[:idx]
	}
	switch _, err := os.Stat(url); {
	case errors.Is(err, os.ErrNotExist):
		return true, nil
	case err != nil:
		return false, fmt.Errorf("unable to examine database %q: %w", url, err)
	}
	return false, nil
}

func sqlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func createFuncMap() template.FuncMap {
	passwords := map[string]string{}
	return template.FuncMap{
		"sqlQuote": sqlQuote,
		"generatePassword": func(user string) string {
			if s := passwords[user]; s != "" {
				return s
			}
			password := misc.RandomString(12)
			encoded := misc.EncodePassword(password)
			passwords[user] = encoded
			slog.Info("Generated new password. Note it down to log in",
				"user", user,
				"password", password)
			return encoded
		},
	}
}

func (m *migration) load(cfg *config.Database, funcs template.FuncMap) (string, error) {
	data, err := migrations.ReadFile(m.path)
	if err != nil {
		return "", fmt.Errorf("loading migration %q failed: %w", m.path, err)
	}
	tmpl, err := template.New("sql").Funcs(funcs).Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("parsing migration %q failed: %w", m.path, err)
	}
	var script bytes.Buffer
	if err := tmpl.Execute(&script, cfg); err != nil {
		return "", fmt.Errorf("templating migration %q failed: %w", m.path, err)
	}
	return script.String(), nil
}

func (db *Database) applyMigrations(ctx context.Context, cfg *config.Database, migs []migration) error {
	slog.InfoContext(ctx, "Applying migrations", "num", len(migs)-1)
	var version int64
	if err := db.DB.QueryRowContext(
		ctx, "SELECT max(version) FROM VERSIONS").Scan(&version); err != nil {
		return fmt.Errorf("current migration version not found: %w", err)
	}
	slog.DebugContext(ctx, "current migration version", "version", version)
	funcMap := createFuncMap()
	for i := range migs {
		mig := &migs[i]
		if mig.version <= version {
			continue
		}
		if !cfg.Migrate {
			return errors.New("needing migrations but migration flag is not set")
		}
		script, err := mig.load(cfg, funcMap)
		if err != nil {
			return fmt.Errorf("loading migration %q failed: %w", mig.path, err)
		}
		slog.DebugContext(ctx, "applying migration", "path", mig.path)
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("cannot start migrations: %w", err)
		}
		if _, err := tx.ExecContext(ctx, script); err != nil {
			tx.Rollback()
			return fmt.Errorf("applying migration %q failed: %w", mig.path, err)
		}
		if _, err := tx.ExecContext(
			ctx, "INSERT INTO versions (version, description) VALUES (?, ?)",
			mig.version, mig.description,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf(
				"inserting version/description of migration %q failed: %w", mig.path, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf(
				"commiting transaction of migration %q failed: %w", mig.path, err)
		}
	}
	slog.InfoContext(ctx, "All migrations applied")
	return nil
}

func createDatabase(ctx context.Context, cfg *config.Database, db *sqlx.DB, migs []migration) error {
	slog.InfoContext(ctx, "Creating database", "url", cfg.DatabaseURL)
	script, err := migs[0].load(cfg, createFuncMap())
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, script); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO versions (version, description) VALUES (?, ?)",
		migs[len(migs)-1].version,
		migs[len(migs)-1].description,
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	slog.InfoContext(ctx, "Creating database done", "url", cfg.DatabaseURL)
	return nil
}

func listMigrations() ([]migration, error) {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	migReg, err := regexp.Compile(`^(\d+)-([^.]+)\.sql$`)
	if err != nil {
		return nil, err
	}
	var migs []migration
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		m := migReg.FindStringSubmatch(filepath.Base(entry.Name()))
		if m == nil {
			continue
		}
		version, err := misc.Atoi64(m[1])
		if err != nil {
			return nil, err
		}
		description := m[2]
		path := "migrations/" + entry.Name()
		migs = append(migs, migration{
			version:     version,
			description: description,
			path:        path,
		})
	}
	slices.SortFunc(migs, func(a, b migration) int {
		return cmp.Compare(a.version, b.version)
	})
	return migs, nil
}
