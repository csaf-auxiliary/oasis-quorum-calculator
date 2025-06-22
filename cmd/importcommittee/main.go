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
	"slices"
	"strings"
	"time"

	"encoding/csv"
	"flag"
	"log"
	"os"

	"github.com/jmoiron/sqlx"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/config"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
)

type user struct {
	name          string
	initialRole   models.Role
	initialStatus models.MemberStatus
}

type meeting struct {
	startTime time.Time
	gathering bool
	attendees []string
}

type data struct {
	users       []*user
	meetings    []*meeting
	appearances map[string]time.Time
	absences    map[string][]time.Time
}

func (d *data) findUser(name string) *user {
	if idx := slices.IndexFunc(d.users, func(u *user) bool {
		return u.name == name
	}); idx >= 0 {
		return d.users[idx]
	}
	return nil
}

func (d *data) storeAbsences(
	ctx context.Context,
	db *sqlx.DB,
	startTime, stopTime time.Time,
	committeeID int64,
) error {
	const insertSQL = `INSERT INTO member_absent ` +
		`(nickname, start_time, stop_time, committee_id) ` +
		`VALUES (?, ?, ?, ?)`
	var (
		from = startTime.Add(-time.Second).UTC()
		to   = stopTime.Add(time.Second).UTC()
	)
	for name, absences := range d.absences {
		for _, t := range absences {
			if !t.Equal(startTime) {
				continue
			}
			if _, err := db.ExecContext(
				ctx, insertSQL,
				name, from, to, committeeID,
			); err != nil {
				return fmt.Errorf("inserting absent failed: %w", err)
			}
		}
	}
	return nil
}

func (d *data) storeNewMembers(
	ctx context.Context,
	db *database.Database,
	startTime time.Time,
	attendees []string,
	committee *models.Committee,
) error {
	for _, att := range attendees {
		if d.appearances[att].Equal(startTime) {
			user := d.findUser(att)
			if user == nil {
				return fmt.Errorf("could not find appearing user: %q", att)
			}
			ms := &models.Membership{
				Committee: committee,
				Status:    user.initialStatus,
				Roles:     []models.Role{user.initialRole},
			}
			if err := models.UpdateMemberships(ctx, db, user.name, misc.Values(ms)); err != nil {
				return fmt.Errorf("updating membership failed: %w", err)
			}
		}
	}
	return nil
}

func fuzzyMatchUser(name string) func(*models.User) bool {
	username := strings.ToLower(name)
	return func(user *models.User) bool {
		firstname := strings.ToLower(misc.EmptyString(user.Firstname))
		lastname := strings.ToLower(misc.EmptyString(user.Lastname))
		if firstname == "" && lastname == "" {
			return false
		}
		return strings.Contains(username, firstname) &&
			strings.Contains(username, lastname)
	}
}

func (d *data) replaceNamesByNicknames(users []*models.User) error {

	replace := func(name *string) error {
		// Check if username exists
		idx := slices.IndexFunc(users, func(u *models.User) bool {
			return u.Nickname == *name
		})
		// Username not found trying firstname and lastname
		if idx < 0 {
			if idx = slices.IndexFunc(users, fuzzyMatchUser(*name)); idx < 0 {
				return fmt.Errorf("no nickname found for user %q", *name)
			}
			// Set username if a good match was found
			*name = users[idx].Nickname
		}
		return nil
	}

	for _, user := range d.users {
		if err := replace(&user.name); err != nil {
			return err
		}
	}

	for _, m := range d.meetings {
		for attendeeIdx := range m.attendees {
			if err := replace(&m.attendees[attendeeIdx]); err != nil {
				return err
			}
		}
	}

	appearances := make(map[string]time.Time, len(d.appearances))
	for name, first := range d.appearances {
		if err := replace(&name); err != nil {
			return err
		}
		appearances[name] = first
	}
	d.appearances = appearances

	absences := make(map[string][]time.Time, len(d.absences))
	for name, abs := range d.absences {
		if err := replace(&name); err != nil {
			return err
		}
		absences[name] = abs
	}
	d.absences = absences
	return nil
}

func extractMeetings(records [][]string) (
	[]*meeting,
	map[string]time.Time,
	map[string][]time.Time,
	error,
) {
	// Transpose rows to columns
	numCols := len(records[0])
	columns := make([][]string, numCols)
	for i := range numCols {
		for _, row := range records {
			if i < len(row) {
				columns[i] = append(columns[i], row[i])
			}
		}
	}

	// Meeting columns start after the initial user status list
	if len(columns) <= 3 {
		return nil, nil, nil, errors.New("not enough columns")
	}
	columns = columns[3:]
	var meetings []*meeting

	// When does a user first appear in the committee?
	appearances := map[string]time.Time{}
	absences := map[string][]time.Time{}

	for _, m := range columns {
		if len(m) < 1 || m[0] == "" {
			continue
		}
		t, err := time.Parse("2006-01-02", m[0])
		if err != nil {
			return nil, nil, nil, err
		}

		attendees := []string{}
		gathering := false
		for _, a := range m[1:] {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			if a == "(informational)" {
				gathering = true
				continue
			}
			if strings.HasSuffix(a, "(Leave of absence)") {
				a = strings.TrimSpace(a[:len(a)-len("(Leave of absence)")])
				absences[a] = append(absences[a], t)
				continue
			}
			if first, ok := appearances[a]; !ok || first.After(t) {
				appearances[a] = t
			}
			attendees = append(attendees, a)
		}
		meetings = append(meetings, &meeting{
			startTime: t,
			attendees: attendees,
			gathering: gathering,
		})
	}

	// Meetings need to be sorted in ascending order
	slices.SortFunc(meetings, func(a, b *meeting) int {
		return a.startTime.Compare(b.startTime)
	})
	return meetings, appearances, absences, nil
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
		case "secretary":
			initialRole = models.SecretaryRole
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

	meetings, appearances, absences, err := extractMeetings(records)
	if err != nil {
		return nil, fmt.Errorf("extracting meetings failed: %w", err)
	}

	return &data{
		users:       users,
		meetings:    meetings,
		appearances: appearances,
		absences:    absences,
	}, nil
}

func deleteOldMeetings(
	ctx context.Context,
	db *sqlx.DB,
	committeeID int64,
) error {
	const deleteSQL = `DELETE FROM meetings WHERE committees_id = ?`
	_, err := db.ExecContext(ctx, deleteSQL, committeeID)
	return err
}

func deleteMembership(
	ctx context.Context,
	db *sqlx.DB,
	committeeID int64,
) error {
	const deleteSQL = `DELETE FROM member_history WHERE committee_id = ?`
	_, err := db.ExecContext(ctx, deleteSQL, committeeID)
	return err
}

func deleteAbsenses(
	ctx context.Context,
	db *sqlx.DB,
	committeeID int64,
) error {
	const deleteSQL = `DELETE FROM member_absent WHERE committee_id = ?`
	_, err := db.ExecContext(ctx, deleteSQL, committeeID)
	return err
}

func findCommittee(committees []*models.Committee, name string) *models.Committee {
	if idx := slices.IndexFunc(committees, func(c *models.Committee) bool {
		return c.Name == name
	}); idx >= 0 {
		return committees[idx]
	}
	return nil
}

func run(committee, csv, databaseURL string) error {
	ctx := context.Background()

	table, err := loadCSV(csv)
	if err != nil {
		return fmt.Errorf("loading CSV failed: %w", err)
	}

	db, err := database.NewDatabase(ctx, &config.Database{
		Driver:      "sqlite3",
		DatabaseURL: databaseURL,
	})
	if err != nil {
		return err
	}
	defer db.Close(ctx)
	committees, err := models.LoadCommittees(ctx, db)
	if err != nil {
		return err
	}

	committeeModel := findCommittee(committees, committee)
	if committeeModel == nil {
		return fmt.Errorf("committee %q not found", committee)
	}

	// Load and check if the username is correct and try to guess the username
	// based on firstname and lastname if the specified name does not exist
	users, err := models.LoadAllUsers(ctx, db)
	if err != nil {
		return fmt.Errorf("loading users failed: %w", err)
	}

	// The nickname is the primary key to the database user,
	// so try to find it by looking it up in the loaded users and replace it.
	if err := table.replaceNamesByNicknames(users); err != nil {
		return err
	}

	if err := deleteOldMeetings(ctx, db.DB, committeeModel.ID); err != nil {
		return fmt.Errorf("deleting old meetings failed: %w", err)
	}

	if err := deleteMembership(ctx, db.DB, committeeModel.ID); err != nil {
		return fmt.Errorf("deleting membership failed: %w", err)
	}

	if err := deleteAbsenses(ctx, db.DB, committeeModel.ID); err != nil {
		return fmt.Errorf("deleting absences failed: %w", err)
	}

	for _, m := range table.meetings {
		var (
			from = m.startTime
			to   = m.startTime.Add(1 * time.Hour) // TODO: Don't guess stop time
		)
		// We add users right before their first meeting to the committee.
		if err := table.storeNewMembers(ctx, db, from, m.attendees, committeeModel); err != nil {
			return err
		}
		// Store the leaves of absence.
		if err := table.storeAbsences(ctx, db.DB, from, to, committeeModel.ID); err != nil {
			return err
		}

		meeting := models.Meeting{
			CommitteeID: committeeModel.ID,
			Gathering:   m.gathering,
			StartTime:   from,
			StopTime:    to,
			Description: nil,
		}
		if err = meeting.StoreNew(ctx, db); err != nil {
			return err
		}

		if err = models.Attend(
			ctx, db,
			meeting.ID,
			misc.Attribute(slices.Values(m.attendees), true),
			from,
		); err != nil {
			return err
		}

		if err = models.ChangeMeetingStatus(
			ctx, db,
			meeting.ID,
			committeeModel.ID,
			models.MeetingConcluded,
			to,
		); err != nil {
			return err
		}
	}

	return nil
}

func check(err error) {
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}
}

func main() {
	var (
		committee   string
		databaseURL string
		csvFile     string
	)
	flag.StringVar(&committee, "committee", "", "Committee to be imported")
	flag.StringVar(&csvFile, "csv", "committee.csv", "CSV with a committee time table to import")
	flag.StringVar(&databaseURL, "database", "oqcd.sqlite", "SQLite database")
	flag.StringVar(&databaseURL, "d", "oqcd.sqlite", "SQLite database (shorthand)")
	flag.Parse()
	if committee == "" {
		log.Fatalln("missing committee name")
	}
	if csvFile == "" {
		log.Fatalln("missing CSV filename")
	}
	check(run(committee, csvFile, databaseURL))
}
