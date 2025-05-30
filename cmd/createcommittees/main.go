// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

// Package main implements a bulk committee creation.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"

	_ "github.com/mattn/go-sqlite3" // Link SQLite 3 driver.
)

func check(err error) {
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}
}

func sqlite3URL(url string) string {
	if !strings.ContainsRune(url, '?') {
		return url + "?_journal=WAL&_timeout=5000&_fk=true"
	}
	return url
}

func run(committeesCSV, databaseURL string) error {
	ctx := context.Background()
	f, err := os.Open(committeesCSV)
	if err != nil {
		return err
	}
	defer f.Close()

	url := sqlite3URL(databaseURL)
	db, err := sqlx.ConnectContext(ctx, "sqlite3", url)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	r := csv.NewReader(f)
next:
	for lineNo := 1; ; lineNo++ {
		record, err := r.Read()
		switch {
		case errors.Is(err, io.EOF):
			break next
		case err != nil:
			return err
		}
		if len(record) < 2 {
			log.Printf("line %d has not enough columns\n", lineNo)
			continue
		}
		user := record[0]
		var desc *string
		if s := strings.TrimSpace(record[1]); len(s) > 1 {
			desc = &s
		}
		const insertSQL = `INSERT INTO committees (name, description) VALUES (?, ?)` +
			`ON CONFLICT DO UPDATE SET description = ?`

		if _, err := tx.ExecContext(ctx, insertSQL, user, desc, desc); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func main() {
	var (
		committeesCSV string
		databaseURL   string
	)
	flag.StringVar(&committeesCSV, "committees", "committees.csv", "CSV file of the committees to be created.")
	flag.StringVar(&committeesCSV, "c", "committees.csv", "CSV file of the committees to be created (shorthand).")
	flag.StringVar(&databaseURL, "database", "oqcd.sqlite", "SQLite database")
	flag.StringVar(&databaseURL, "d", "oqcd.sqlite", "SQLite database (shorthand)")
	flag.Parse()

	check(run(committeesCSV, databaseURL))
}
