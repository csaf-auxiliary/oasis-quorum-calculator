// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

// Package main implements a meeting export.
package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/mattn/go-sqlite3" // Link SQLite 3 driver.

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
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

type meeting struct {
	startTime   time.Time
	stopTime    time.Time
	gathering   bool
	description string
	attendees   []int
}

func run(meetingCSV, committee, databaseURL string, useName bool) error {
	ctx := context.Background()

	url := sqlite3URL(databaseURL)
	db, err := sqlx.ConnectContext(ctx, "sqlite3", url)
	if err != nil {
		return err
	}
	defer db.Close()

	// the methods in models want a wrapped database, so FIXME for consistency
	dbdb := &database.Database{DB: db}

	meetings := []meeting{}

	// we only load users that have attended a meeting, even if they are
	// a member of the committee FIXME #103
	// an alternative could potentially be
	// overview, err := models.LoadMeetingsOverview(ctx, dbdb, committee.ID, -1)
	loadAttendeesSQL := `SELECT m.start_time, m.stop_time, m.gathering, m.description, group_concat(nickname) FROM meetings m ` +
		`LEFT JOIN attendees a ON m.id = a.meetings_id `

	queryArgs := []any{}
	if committee != "" {
		loadAttendeesSQL += `WHERE m.committees_id = (SELECT id FROM committees WHERE name = ?) `
		queryArgs = append(queryArgs, committee)
	}
	loadAttendeesSQL += `GROUP BY m.start_time ORDER BY m.start_time`
	rows, err := db.QueryContext(ctx, loadAttendeesSQL, queryArgs...)
	if err != nil {
		return fmt.Errorf("querying attendees failed: %w", err)
	}

	var users []string

	defer rows.Close()
	for rows.Next() {
		var m meeting
		var attendeesSQL sql.NullString
		var description sql.NullString
		if err := rows.Scan(&m.startTime, &m.stopTime, &m.gathering, &description, &attendeesSQL); err != nil {
			return fmt.Errorf("scanning attendees failed: %w", err)
		}
		if description.Valid {
			m.description = description.String
		}
		if attendeesSQL.Valid {
			for att := range strings.SplitSeq(attendeesSQL.String, ",") {
				idx := slices.Index(users, att)
				if idx == -1 {
					idx = len(users)
					users = append(users, att)
				}
				m.attendees = append(m.attendees, idx)
			}
		}
		meetings = append(meetings, m)
	}

	// This will hold the meeting metadata rows of the CSV
	// we fill in the first three columns
	startTimesRow := []string{"Status", "Role", "Name"}
	stopTimesRow := []string{"", "", ""}
	descriptionRow := []string{"", "", ""}
	gatheringRow := []string{"", "", ""}

	// Populate meeting rows and find maxAttendees
	for _, m := range meetings {
		startTimesRow = append(startTimesRow, m.startTime.Format("2006-01-02 15:04"))
		stopTimesRow = append(stopTimesRow, "Stop-Time: "+m.stopTime.Format("2006-01-02 15:04"))
		if m.gathering {
			gatheringRow = append(gatheringRow, "(informational)")
		} else {
			gatheringRow = append(gatheringRow, "")
		}
		descriptionRow = append(descriptionRow, "Description: "+m.description)
	}

	// This 2D slice will hold the attendee data,
	// where attendeeMatrix[i] is a row containing the (i+1)-th attendee from each meeting.
	// We pre-allocate it based on maxAttendees for rows and number of meetings for columns.
	attendeeMatrix := make([][]string, len(users))
	for i := range attendeeMatrix {
		attendeeMatrix[i] = make([]string, 3+len(meetings))
	}

	// Populate the attendeeMatrix
	for i, user := range users {
		dbUser, err := models.LoadUser(ctx, dbdb, user, nil)
		if err != nil {
			return err
		}
		/* we can find out about the current membership, but not the initial
		   status
		ms := dbUser.FindMembership(committee)
		if ms == nil {
			return fmt.Errorf("found that user %s has attended a meeting but is ot a member?!", user)
		}
		fmt.Print(ms.Status) // but we want the initial status and role
		*/
		attendeeMatrix[i][0] = "TODO"
		attendeeMatrix[i][1] = "TODO"
		if useName {
			attendeeMatrix[i][2] = *dbUser.Firstname + " " + *dbUser.Lastname
		} else {
			attendeeMatrix[i][2] = user
		}
	}
	for mIdx, m := range meetings {
		for i := range users {
			if slices.Index(m.attendees, i) >= 0 {
				attendeeMatrix[i][3+mIdx] = attendeeMatrix[i][2]
			}
		}
	}

	file, err := os.Create(meetingCSV)
	if err != nil {
		return err
	}

	writer := csv.NewWriter(file)

	writer.Write(startTimesRow)
	writer.Write(stopTimesRow)
	writer.Write(gatheringRow)
	writer.Write(descriptionRow)

	for _, row := range attendeeMatrix {
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	writer.Flush()
	err = writer.Error()
	return errors.Join(err, file.Close())
}

func main() {
	var (
		meetingCSV  string
		committee   string
		databaseURL string
		useName     bool
	)
	flag.StringVar(&meetingCSV, "meeting", "meetings.csv", "CSV file of the meetings to be exported.")
	flag.StringVar(&meetingCSV, "m", "meetings.csv", "CSV file of the meetings to be exported (shorthand).")
	flag.StringVar(&committee, "committee", "", "Committee meetings that should be exported")
	flag.StringVar(&databaseURL, "database", "oqcd.sqlite", "SQLite database")
	flag.StringVar(&databaseURL, "d", "oqcd.sqlite", "SQLite database (shorthand)")
	flag.BoolVar(&useName, "usename", true, "Export full names instead of id")
	flag.Parse()

	check(run(meetingCSV, committee, databaseURL, useName))
}
