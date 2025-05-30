// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

// Package main implements a bulk user creation.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
	"github.com/jmoiron/sqlx"
)

// CSV layout
// nickname,first name,last name,committee,chair,member,status
// "anton","Anton","Amann","false","asaf","false","true","voting"

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

var memberStatus = map[string]int{
	"member":     0,
	"voting":     1,
	"nonevoting": 2,
	"nomember":   3,
}

func run(usersCSV, passwordCSV, databaseURL string) error {
	ctx := context.Background()
	f, err := os.Open(usersCSV)
	if err != nil {
		return err
	}
	defer f.Close()

	passwords, err := os.Create(passwordCSV)
	if err != nil {
		return err
	}

	closePWs := func(err error) error {
		return errors.Join(err, passwords.Close())
	}

	url := sqlite3URL(databaseURL)
	db, err := sqlx.ConnectContext(ctx, "sqlite3", url)
	if err != nil {
		return closePWs(err)
	}
	defer db.Close()

	r := csv.NewReader(f)
next:
	for lineNo := 1; ; lineNo++ {
		record, err := r.Read()
		switch {
		case errors.Is(err, io.EOF):
			break next
		case err != nil:
			return closePWs(err)
		}
		if len(record) < 8 {
			log.Printf("line %d has not enough columns\n", lineNo)
			continue
		}
		var (
			nickname  = record[0]
			firstname = misc.NilString(strings.TrimSpace(record[1]))
			lastname  = misc.NilString(strings.TrimSpace(record[2]))
			admin     = record[3] == "true"
			committee = record[4]
			chair     *bool
			member    *bool
			status    *int
		)
		if record[5] != "" {
			x := record[5] == "true"
			chair = &x
		}
		if record[6] != "" {
			x := record[6] == "true"
			member = &x
		}
		if record[7] != "" {
			st, ok := memberStatus[record[7]]
			if !ok {
				log.Printf("line %d: status column (8) is invalid.\n", lineNo)
				continue
			}
			status = &st
		}

		var exists bool
		const existsSQL = `SELECT EXISTS(SELECT 1 FROM users WHERE nickname = ?)`
		if err := db.QueryRowContext(ctx, existsSQL, nickname).Scan(&exists); err != nil {
			return closePWs(err)
		}

		if !exists {
			nuser := models.User{
				Nickname:  nickname,
				Firstname: firstname,
				Lastname:  lastname,
				IsAdmin:   admin,
			}
			password := misc.RandomString(12)
			success, err := nuser.StoreNew(ctx, &database.Database{DB: db}, password)
			if err != nil {
				return closePWs(err)
			}
			if !success {
				log.Printf("line %d: adding user failed.\n", lineNo)
				continue
			}
			fmt.Fprintf(passwords, "%q,%q\n", nickname, password)
		}

		// TODO: Implement me!
		_ = committee
		_ = chair
		_ = member
		_ = status
	}

	return nil
}

func main() {
	var (
		usersCSV    string
		passwordCSV string
		databaseURL string
	)
	flag.StringVar(&usersCSV, "users", "users.csv", "CSV file of the users to be created.")
	flag.StringVar(&usersCSV, "u", "users.csv", "CSV file of the users to be created (shorthand).")
	flag.StringVar(&passwordCSV, "passwords", "passwords.csv", "CSV file of the user passwords to be created.")
	flag.StringVar(&passwordCSV, "p", "passwords.csv", "CSV file of the user passwords to be created (shorthand).")
	flag.StringVar(&databaseURL, "database", "oqcd.sqlite", "SQLite database")
	flag.StringVar(&databaseURL, "d", "oqcd.sqlite", "SQLite database (shorthand)")
	flag.Parse()

	check(run(usersCSV, passwordCSV, databaseURL))
}
