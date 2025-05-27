// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

// Package main implements committee import.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/config"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"iter"
	"strings"
	"time"

	"encoding/csv"
	"flag"
	"log"
	"os"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
)

type user struct {
	name          string
	initialRole   models.Role
	initialStatus models.MemberStatus
}

type meeting struct {
	startTime time.Time
	attendees []string
}

type data struct {
	users    []*user
	meetings []*meeting
}

func check(err error) {
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}
}

func extractMeetings(records [][]string) ([]*meeting, error) {
	var meetings []*meeting

	// Transpose rows to columns
	numCols := len(records[0])
	columns := make([][]string, numCols)
	for i := 0; i < numCols; i++ {
		for _, row := range records {
			if i < len(row) {
				columns[i] = append(columns[i], row[i])
			}
		}
	}

	// Meeting columns start after the initial user status list
	if len(columns) <= 4 {
		return nil, errors.New("not enough columns")
	}
	columns = columns[4:]

	for _, m := range columns {
		if len(m) < 1 {
			continue
		}
		t, err := time.Parse("2006-01-02", m[0])
		if err != nil {
			return nil, err
		}

		attendees := []string{}
		for _, a := range m[1:] {
			if a != "" {
				attendees = append(attendees, a)
			}
		}
		meetings = append(meetings, &meeting{
			startTime: t,
			attendees: attendees,
		})
	}
	return meetings, nil
}

func extractUsers(records [][]string) ([]*user, error) {
	var users []*user

	if len(records) < 2 {
		return nil, errors.New("no users")
	}

	for _, row := range records[1:] {
		if len(row) < 3 {
			return nil, errors.New("not enough user infos")
		}
		status, role, name := row[0], row[1], row[2]
		status = strings.TrimSpace(status)
		role = strings.TrimSpace(role)
		name = strings.TrimSpace(name)
		// Ignore incomplete lines
		if status == "" || role == "" || name == "" {
			continue
		}
		// Parse status
		var initialStatus models.MemberStatus
		switch strings.ToLower(status) {
		case "voter":
			initialStatus = models.Voting
		case "non-voter":
			initialStatus = models.NoneVoting
		default:
			return nil, fmt.Errorf("unknown status %q for user %q", status, name)
		}
		// Parse role
		var initialRole models.Role
		switch strings.ToLower(role) {
		case "voting member":
			initialRole = models.MemberRole
		case "member":
			initialRole = models.MemberRole
			initialStatus = models.NoneVoting
		case "chair":
			initialRole = models.ChairRole
		default:
			return nil, fmt.Errorf("unknown role %q for user %q", role, name)
		}
		users = append(users, &user{
			name:          name,
			initialStatus: initialStatus,
			initialRole:   initialRole,
		})
	}

	return users, nil
}

func loadCSV(filename string) (*data, error) {

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)

	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	users, err := extractUsers(records)
	if err != nil {
		return nil, fmt.Errorf("extracting users failed: %w", err)
	}

	meetings, err := extractMeetings(records)
	if err != nil {
		return nil, fmt.Errorf("extracting meetings failed: %w", err)
	}

	return &data{
		users:    users,
		meetings: meetings,
	}, nil
}

func run(committee, csv, databaseURL string) error {
	ctx := context.Background()

	table, err := loadCSV(csv)
	if err != nil {
		return fmt.Errorf("loading CSV failed: %w", err)
	}

	_ = table
	_ = committee

	db, err := database.NewDatabase(ctx, &config.Database{
		DatabaseURL: databaseURL,
	})
	if err != nil {
		return err
	}
	defer db.Close(ctx)
	committees, err := models.LoadCommittees(ctx, db)

	var committeeModel *models.Committee
	for _, c := range committees {
		if c.Name == committee {
			committeeModel = c
		}
	}
	if committeeModel == nil {
		return fmt.Errorf("committee %q not found", committee)
	}
	for _, user := range table.users {
		ms := []models.Membership{{
			Committee: committeeModel,
			Status:    user.initialStatus,
			// TODO avoid role overwrite
			Roles: nil,
		}}
		msIter := func() iter.Seq[*models.Membership] {
			return func(yield func(ms *models.Membership) bool) {
				for _, m := range ms {
					if !yield(&m) {
						return
					}
				}
			}
		}
		if err := models.UpdateMemberships(ctx, db, user.name, msIter()); err != nil {
			return err
		}
	}

	for _, m := range table.meetings {
		meeting := models.Meeting{
			CommitteeID: committeeModel.ID,
			Gathering:   false,
			StartTime:   m.startTime,
			// TODO: Don't guess stop time
			StopTime:    m.startTime.Add(1 * time.Hour),
			Description: nil,
		}
		if err = meeting.StoreNew(ctx, db); err != nil {
			return err
		}

		if err = models.ChangeMeetingStatus(ctx, db, meeting.ID, committeeModel.ID, models.MeetingConcluded); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	var (
		committee   string
		databaseURL string
		csv         string
	)
	flag.StringVar(&committee, "committee", "", "Committee to be imported")
	flag.StringVar(&csv, "csv", "committee.csv", "CSV with a committee time table to import")
	flag.StringVar(&databaseURL, "database", "oqcd.sqlite", "SQLite database")
	flag.StringVar(&databaseURL, "d", "oqcd.sqlite", "SQLite database (shorthand)")
	flag.Parse()
	if committee == "" {
		log.Fatalln("missing committee name")
	}
	if csv == "" {
		log.Fatalln("missing CSV filename")
	}
	check(run(committee, csv, databaseURL))
}
