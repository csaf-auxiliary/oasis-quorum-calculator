// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
)

// MeetingStatus represents the current status of a meeting.
type MeetingStatus int

const (
	// MeetingOnHold represents a meeting that is currently not running.
	MeetingOnHold MeetingStatus = iota
	// MeetingRunning represents a meeting that is currently running.
	MeetingRunning
	// MeetingConcluded represents a finished meeting.
	MeetingConcluded
)

// Meeting holds the informations about a meeting.
type Meeting struct {
	ID          int64
	CommitteeID int64
	Gathering   bool
	Status      MeetingStatus
	StartTime   time.Time
	StopTime    time.Time
	Description *string
}

// Quorum is the quorum of this meeting.
type Quorum struct {
	Total           int
	Voting          int
	AttendingVoting int
	NonVoting       int
	Member          int
}

// Attendees is a map from nicknames to (attended, voting rights).
type Attendees map[string]bool

// MeetingData captures the main data of a meeting.
type MeetingData struct {
	Meeting   *Meeting
	Attendees Attendees
	Quorum    *Quorum
}

// MeetingsOverview the an overview over a list of meetings.
type MeetingsOverview struct {
	Data           []*MeetingData
	UsersHistories UsersHistories
	Users          []*User // Only basic user data, no memberships.
}

// Number is the number of voting members to reach the quorum.
func (q *Quorum) Number() int {
	return 1 + q.Voting/2
}

// Reached indicates that the quorum is reached.
func (q *Quorum) Reached() bool {
	return q.AttendingVoting >= q.Number()
}

// Meetings is a slice of meetings.
type Meetings []*Meeting

// Attended checks if a given user attended.
func (a Attendees) Attended(nickname string) bool {
	_, ok := a[nickname]
	return ok
}

// Voting checks if a given has voting rights.
func (a Attendees) Voting(nickname string) bool {
	return a[nickname]
}

// String implements [fmt.Stringer].
func (m MeetingStatus) String() string {
	switch m {
	case MeetingOnHold:
		return "onhold"
	case MeetingRunning:
		return "running"
	case MeetingConcluded:
		return "concluded"
	default:
		return fmt.Sprintf("unknown meeting status (%d)", m)
	}
}

// ParseMeetingStatus parse a given string to a meeting status.
func ParseMeetingStatus(s string) (MeetingStatus, error) {
	switch strings.ToLower(s) {
	case "onhold":
		return MeetingOnHold, nil
	case "running":
		return MeetingRunning, nil
	case "concluded":
		return MeetingConcluded, nil
	default:
		return 0, fmt.Errorf("unknown meeting status %q", s)
	}
}

// MeetingFilter defines a filter mechanism for meetings.
type MeetingFilter func(*Meeting) bool

// And return a filter which is 'and'ing two filters.
func (mf MeetingFilter) And(other MeetingFilter) MeetingFilter {
	return func(m *Meeting) bool {
		return mf(m) && other(m)
	}
}

// MeetingCommitteeIDsFilter filters meetings by their committee ids.
func MeetingCommitteeIDsFilter(seq iter.Seq[*Committee]) MeetingFilter {
	ids := maps.Collect(misc.Attribute(misc.Map(seq, (*Committee).GetID), true))
	return func(m *Meeting) bool { return ids[m.CommitteeID] }
}

// CommitteeIDFilter creates a filter condition which looks for
// meetings with the given committee id.
func CommitteeIDFilter(id int64) MeetingFilter {
	return func(m *Meeting) bool {
		return m.CommitteeID == id
	}
}

// RunningFilter helps return running meetings.
func RunningFilter(m *Meeting) bool {
	return m.Status == MeetingRunning
}

// OverlapFilter creates a filter which checks if a meeting overlaps
// a given interval.
func OverlapFilter(start, stop time.Time, exceptions ...int64) func(m *Meeting) bool {
	return func(m *Meeting) bool {
		return !(m.StopTime.Before(start) || stop.Before(m.StartTime)) &&
			!slices.Contains(exceptions, m.ID)
	}
}

// Duration returns duration of the meeting.
func (m *Meeting) Duration() time.Duration {
	return m.StopTime.Sub(m.StartTime)
}

// Filter returns a sequence of meetings which fulfill the given condition.
func (ms Meetings) Filter(cond func(m *Meeting) bool) iter.Seq[*Meeting] {
	return misc.Filter(slices.Values(ms), cond)
}

// Contains checks if there is a meeting fulfilling the given condition.
func (ms Meetings) Contains(cond func(m *Meeting) bool) bool {
	return slices.ContainsFunc(ms, cond)
}

// LoadMeeting loads a meeting by its id.
func LoadMeeting(
	ctx context.Context, db *database.Database,
	meetingID, committeeID int64,
) (*Meeting, error) {
	tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return LoadMeetingTx(ctx, tx, meetingID, committeeID)
}

// LoadMeetingTx loads a meeting by its id.
func LoadMeetingTx(
	ctx context.Context, tx *sql.Tx,
	meetingID, committeeID int64,
) (*Meeting, error) {
	meeting := Meeting{
		ID:          meetingID,
		CommitteeID: committeeID,
	}
	const loadSQL = `SELECT status, gathering, start_time, stop_time, description ` +
		`FROM meetings ` +
		`WHERE id = ? AND committees_id = ?`
	switch err := tx.QueryRowContext(ctx, loadSQL, meetingID, committeeID).Scan(
		&meeting.Status,
		&meeting.Gathering,
		&meeting.StartTime,
		&meeting.StopTime,
		&meeting.Description,
	); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("loading meeting failed: %w", err)
	}
	return &meeting, nil
}

// LoadMeetings loads meetings for a sequence of committees.
func LoadMeetings(
	ctx context.Context,
	db *database.Database,
	committees iter.Seq[int64],
) (Meetings, error) {
	tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	const loadSQL = `SELECT id, status, gathering, start_time, stop_time, description ` +
		`FROM meetings ` +
		`WHERE committees_id = ? ` +
		`ORDER BY unixepoch(start_time)`
	stmt, err := tx.PrepareContext(ctx, loadSQL)
	if err != nil {
		return nil, fmt.Errorf("preparing loadind meetings failed: %w", err)
	}
	defer stmt.Close()
	var meetings Meetings
	for committee := range committees {
		rows, err := stmt.QueryContext(ctx, committee)
		if err != nil {
			return nil, fmt.Errorf("querying meetings failed: %w", err)
		}
		if err := func() error {
			defer rows.Close()
			for rows.Next() {
				meeting := Meeting{CommitteeID: committee}
				if err := rows.Scan(
					&meeting.ID,
					&meeting.Status,
					&meeting.Gathering,
					&meeting.StartTime,
					&meeting.StopTime,
					&meeting.Description,
				); err != nil {
					return nil
				}
				meetings = append(meetings, &meeting)
			}
			return rows.Err()
		}(); err != nil {
			return nil, fmt.Errorf("scanning meetings failed: %w", err)
		}
	}
	return meetings, nil
}

// LoadLastNMeetingsTx loads the last n meetings.
// If n < 0 all meetings are loaded.
// The returned meetings are sorted lastest first.
func LoadLastNMeetingsTx(
	ctx context.Context,
	tx *sql.Tx,
	committeeID int64,
	limit int64,
) (Meetings, error) {
	const loadSQL = `SELECT id, status, gathering, start_time, stop_time, description ` +
		`FROM meetings ` +
		`WHERE committees_id = ? ` +
		`ORDER BY unixepoch(start_time) DESC `
	var query string
	if limit >= 0 {
		query = query + " LIMIT " + strconv.FormatInt(limit, 10)
	} else {
		query = loadSQL
	}
	rows, err := tx.QueryContext(ctx, query, committeeID)
	if err != nil {
		return nil, fmt.Errorf("querying last n meetings failed: %w", err)
	}
	defer rows.Close()
	var meetings Meetings
	for rows.Next() {
		var meeting Meeting
		if err := rows.Scan(
			&meeting.ID,
			&meeting.Status,
			&meeting.Gathering,
			&meeting.StartTime,
			&meeting.StopTime,
			&meeting.Description,
		); err != nil {
			return nil, fmt.Errorf("scanning n last meetings failed: %w", err)
		}
		meetings = append(meetings, &meeting)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("querying last n meetings failed: %w", err)
	}
	return meetings, nil
}

// DeleteMeetingsByID removes meetings the database identified by their id.
func DeleteMeetingsByID(
	ctx context.Context,
	db *database.Database,
	committeeID int64,
	meetingsIDs iter.Seq[int64],
) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const deleteSQL = `DELETE FROM meetings ` +
		`WHERE id = ? AND committees_id = ? AND status <> 2` // MeetingConcluded
	stmt, err := tx.PrepareContext(ctx, deleteSQL)
	if err != nil {
		return fmt.Errorf("preparing delete meetings failed: %w", err)
	}
	defer stmt.Close()
	for meetingID := range meetingsIDs {
		if _, err := stmt.ExecContext(ctx, meetingID, committeeID); err != nil {
			return fmt.Errorf("deleting meeting failed: %w", err)
		}
	}
	return tx.Commit()
}

// StoreNew stores a new meeting into the database.
func (m *Meeting) StoreNew(ctx context.Context, db *database.Database) error {
	const insertSQL = `INSERT INTO meetings ` +
		`(gathering, committees_id, start_time, stop_time, description) ` +
		`VALUES (?, ?, ?, ?, ?) ` +
		`RETURNING id`
	if err := db.DB.QueryRowContext(ctx, insertSQL,
		m.Gathering,
		m.CommitteeID,
		m.StartTime,
		m.StopTime,
		m.Description,
	).Scan(&m.ID); err != nil {
		return fmt.Errorf("inserting meeting into database failed: %w", err)
	}
	return nil
}

// Store updates a meeting in the database.
func (m *Meeting) Store(ctx context.Context, db *database.Database) error {
	const updateSQL = `UPDATE meetings SET ` +
		`gathering = ?, ` +
		`start_time = ?,` +
		`stop_time = ?,` +
		`description = ? ` +
		`WHERE id = ? AND committees_id = ?`
	if _, err := db.DB.ExecContext(ctx, updateSQL,
		m.Gathering,
		m.StartTime,
		m.StopTime,
		m.Description,
		m.ID, m.CommitteeID); err != nil {
		return fmt.Errorf("updating meeting failed: %w", err)
	}
	return nil
}

// Attendees loads the nicknames from the database which attend this meeting.
func (m *Meeting) Attendees(ctx context.Context, db *database.Database) (Attendees, error) {
	const loadAttendeesSQL = `SELECT nickname FROM attendees ` +
		`WHERE meetings_id = ?`
	attendees := make(Attendees)
	rows, err := db.DB.QueryContext(ctx, loadAttendeesSQL, m.ID)
	if err != nil {
		return nil, fmt.Errorf("querying attendees failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var attendee string
		if err := rows.Scan(&attendee); err != nil {
			return nil, fmt.Errorf("scanning attendees failed: %w", err)
		}
		attendees[attendee] = true
	}
	return attendees, nil
}

// Unattend removes the attendees from a given list from a meeting.
func Unattend(
	ctx context.Context, db *database.Database,
	meetingID int64,
	seq iter.Seq2[string, bool],
	accept time.Time,
) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const (
		checkSQL = `SELECT time FROM attendees_changes ` +
			`WHERE meetings_id = ? AND nickname = ?`
		deleteSQL = `DELETE FROM attendees ` +
			`WHERE meetings_id = ? AND nickname = ?`
	)
	deleteStmt, err := tx.PrepareContext(ctx, deleteSQL)
	if err != nil {
		return fmt.Errorf("preparing unattend failed: %w", err)
	}
	defer deleteStmt.Close()
	checkStmt, err := tx.PrepareContext(ctx, checkSQL)
	if err != nil {
		return fmt.Errorf("preparing unattend check failed: %w", err)
	}
	defer checkStmt.Close()

	for nickname := range seq {
		var t time.Time
		switch err := checkStmt.QueryRowContext(ctx, meetingID, nickname).Scan(&t); {
		case errors.Is(err, sql.ErrNoRows):
			// It's okay.
		case err != nil:
			return fmt.Errorf("checking unattend failed: %w", err)
		default:
			if t.After(accept) {
				slog.DebugContext(ctx, "race in unattend detected", "nickname", nickname)
				continue
			}
		}
		if _, err := deleteStmt.ExecContext(ctx, meetingID, nickname); err != nil {
			return fmt.Errorf("unattend failed: %w", err)
		}
	}
	return tx.Commit()
}

// Attend sets the attendees of a meeting to a given list.
func Attend(
	ctx context.Context, db *database.Database,
	meetingID int64,
	seq iter.Seq2[string, bool],
	accept time.Time,
) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const (
		checkSQL = `SELECT time FROM attendees_changes ` +
			`WHERE meetings_id = ? AND nickname = ?`
		insertSQL = `INSERT INTO attendees ` +
			`(meetings_id, nickname, voting_allowed) ` +
			`VALUES (?, ?, ?) ` +
			`ON CONFLICT DO UPDATE SET voting_allowed = ?`
	)
	insertStmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("preparing attend failed: %w", err)
	}
	defer insertStmt.Close()
	checkStmt, err := tx.PrepareContext(ctx, checkSQL)
	if err != nil {
		return fmt.Errorf("preparing attend check failed: %w", err)
	}
	defer checkStmt.Close()

	for nickname, voting := range seq {
		var t time.Time
		switch err := checkStmt.QueryRowContext(ctx, meetingID, nickname).Scan(&t); {
		case errors.Is(err, sql.ErrNoRows):
			// It's okay.
		case err != nil:
			return fmt.Errorf("checking attend failed: %w", err)
		default:
			if t.After(accept) {
				slog.DebugContext(ctx, "race in attend detected", "nickname", nickname)
				continue
			}
		}
		if _, err := insertStmt.ExecContext(ctx, meetingID, nickname, voting, voting); err != nil {
			return fmt.Errorf("attend failed: %w", err)
		}
	}
	return tx.Commit()
}

// UpdateAttendee updates a given attendee for given meeting.
func UpdateAttendee(
	ctx context.Context, db *database.Database,
	meetingID int64,
	nickname string,
	attend, voting bool,
) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const (
		insertSQL = `INSERT INTO attendees (meetings_id, nickname, voting_allowed) ` +
			`VALUES (?, ?, ?) ` +
			`ON CONFLICT DO UPDATE SET voting_allowed = ?`
		deleteSQL = `DELETE FROM attendees WHERE meetings_id = ? AND nickname = ?`
	)
	if attend {
		_, err = tx.ExecContext(ctx, insertSQL, meetingID, nickname, voting, voting)
	} else {
		_, err = tx.ExecContext(ctx, deleteSQL, meetingID, nickname)
	}
	if err != nil {
		return fmt.Errorf("updating attendee failed: %w", err)
	}
	return tx.Commit()
}

// AttendedMeetings returns a set of ids of meetings the given user attended.
func AttendedMeetings(
	ctx context.Context,
	db *database.Database,
	nickname string,
) (map[int64]bool, error) {
	const attendedSQL = `SELECT meetings_id FROM attendees WHERE nickname = ?`
	rows, err := db.DB.QueryContext(ctx, attendedSQL, nickname)
	if err != nil {
		return nil, fmt.Errorf("querying attended meetings failed: %w", err)
	}
	defer rows.Close()
	meetings := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning attended meetings failed: %w", err)
		}
		meetings[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("querying attended meetings failed: %w", err)
	}
	return meetings, nil
}

// MeetingAttendeesTx loads the attendees of a meeting
// and their voting rights.
func MeetingAttendeesTx(
	ctx context.Context,
	tx *sql.Tx,
	meetingID int64,
) (Attendees, error) {
	const attendeesSQL = `SELECT nickname, voting_allowed FROM attendees ` +
		`WHERE meetings_id = ?`
	rows, err := tx.QueryContext(ctx, attendeesSQL, meetingID)
	if err != nil {
		return nil, fmt.Errorf("loading meeting attendees failed: %w", err)
	}
	defer rows.Close()
	attendees := Attendees{}
	for rows.Next() {
		var (
			nickname string
			voting   bool
		)
		if err := rows.Scan(&nickname, &voting); err != nil {
			return nil, fmt.Errorf("scanning meeting attendees failed: %w", err)
		}
		attendees[nickname] = voting
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lading meeting attendees failed: %w", err)
	}
	return attendees, nil
}

// PreviousMeetingTx the id of the meeting before the given meeting.
// Returns false as the second value if there isn't any.
func PreviousMeetingTx(
	ctx context.Context,
	tx *sql.Tx,
	meetingID int64,
) (int64, bool, error) {
	const prevSQL = `SELECT m2.id FROM meetings m1, meetings m2 ` +
		`WHERE m1.id = ? ` +
		`AND m1.committees_id = m2.committees_id ` +
		`AND NOT m2.gathering ` +
		`AND m2.status = 2 ` + // MeetingConcluded
		`AND unixepoch(m2.start_time) < unixepoch(m1.start_time) ` +
		`ORDER by unixepoch(m2.start_time) DESC LIMIT 1`
	var prevID int64
	switch err := tx.QueryRowContext(ctx, prevSQL, meetingID).Scan(&prevID); {
	case errors.Is(err, sql.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, fmt.Errorf("find last meeting failed: %w", err)
	}
	return prevID, true, nil
}

// HasCommitteeRunningMeeting checks if a committee has a running meeting.
func HasCommitteeRunningMeeting(
	ctx context.Context,
	db *database.Database,
	committeeID int64,
) (bool, error) {
	tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	return HasCommitteeRunningMeetingTx(ctx, tx, committeeID)
}

// HasCommitteeRunningMeetingTx checks if a committee has a running meeting.
func HasCommitteeRunningMeetingTx(
	ctx context.Context,
	tx *sql.Tx,
	committeeID int64,
) (bool, error) {
	const existsSQL = `SELECT EXISTS(SELECT 1 FROM meetings ` +
		`WHERE committees_id = ? AND status = 1)` // MeetingRunning
	var exists bool
	if err := tx.QueryRowContext(ctx, existsSQL, committeeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query running meeting exists failed: %w", err)
	}
	return exists, nil
}

// HasConcludedMeetingNewerThanTx checks if there is a meeting
// in the same committee that is newer and concluded.
func HasConcludedMeetingNewerThanTx(
	ctx context.Context,
	tx *sql.Tx,
	meetingID int64,
) (bool, error) {
	const existsSQL = `SELECT EXISTS (SELECT 1 FROM meetings m1, meetings m2 ` +
		`WHERE m1.id = ? ` +
		`AND m1.committees_id = m2.committees_id ` +
		`AND m1.id <> m2.id ` +
		`AND m2.status = 2 ` + // MeetingConcluded
		`AND unixepoch(m2.start_time) > unixepoch(m1.start_time))`
	var exists bool
	if err := tx.QueryRowContext(ctx, existsSQL, meetingID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query newer concluded meeting exists failed: %w", err)
	}
	return exists, nil
}

// IsGatheringMeetingTx checks if a given meeting is a gathering.
func IsGatheringMeetingTx(
	ctx context.Context,
	tx *sql.Tx,
	meetingID int64,
) (bool, error) {
	const gatheringSQL = `SELECT gathering FROM meetings WHERE id = ?`
	var gathering bool
	if err := tx.QueryRowContext(ctx, gatheringSQL, meetingID).Scan(&gathering); err != nil {
		return false, fmt.Errorf("query gathering failed: %w", err)
	}
	return gathering, nil
}

// LoadMeetingsOverview loads the last meetings and gathers infos about them.
func LoadMeetingsOverview(
	ctx context.Context,
	db *database.Database,
	committeeID int64,
	limit int64,
) (*MeetingsOverview, error) {
	tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	meetings, err := LoadLastNMeetingsTx(ctx, tx, committeeID, limit)
	if err != nil {
		return nil, err
	}

	histories, err := LoadUsersHistoriesTx(ctx, tx, committeeID)
	if err != nil {
		return nil, err
	}

	data := make([]*MeetingData, 0, len(meetings))

	neededUsers := map[string]bool{}
	for _, meeting := range meetings {
		for nickname, history := range histories {
			if history.Status(meeting.StopTime) != NoMember {
				neededUsers[nickname] = true
			}
		}
		attendees, err := MeetingAttendeesTx(ctx, tx, meeting.ID)
		if err != nil {
			return nil, err
		}
		for nickname := range attendees {
			neededUsers[nickname] = true
		}

		data = append(data, &MeetingData{
			Meeting:   meeting,
			Attendees: attendees,
		})
	}

	users := make([]*User, 0, len(neededUsers))
	for nickname := range neededUsers {
		user, err := loadBasicUserTx(ctx, tx, nickname)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	// Calculate the quora
	for _, d := range data {
		meeting := d.Meeting
		if meeting.Gathering {
			continue
		}
		var voting, attending int
		for nickname := range neededUsers {
			history := histories[nickname]
			if history.Status(meeting.StopTime) == Voting {
				voting++
				if d.Attendees.Attended(nickname) {
					attending++
				}
			}
		}
		d.Quorum = &Quorum{
			Voting:          voting,
			AttendingVoting: attending,
		}
	}

	// Sort user by firstname, lastname and nickname.
	slices.SortFunc(users, (*User).Compare)
	overview := &MeetingsOverview{
		Data:           data,
		Users:          users,
		UsersHistories: histories,
	}
	return overview, nil
}
