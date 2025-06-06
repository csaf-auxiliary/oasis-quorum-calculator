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
	"slices"
	"time"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
)

var (
	// ErrAlreadyRunning is returned if there is a meeting running.
	ErrAlreadyRunning = errors.New("already running")
	// ErrNewerConcluded is returned if there is a newer meeting
	// that is already concluded.
	ErrNewerConcluded = errors.New("newer concluded")
)

// ChangeMeetingStatus changes the status of a given meeting in
// a given committee to a given status.
// It checks if all conditions are met and does further adjustments
// after the status change has happened.
func ChangeMeetingStatus(
	ctx context.Context,
	db *database.Database,
	meetingID, committeeID int64,
	meetingStatus MeetingStatus,
	timer time.Time,
) error {

	// Extra checks before we try to change the status.
	precondition := func(ctx context.Context, tx *sql.Tx) error {
		switch meetingStatus {
		case MeetingRunning:
			// We should not start a meeting if one is already running.
			switch has, err := HasCommitteeRunningMeetingTx(ctx, tx, committeeID); {
			case err != nil:
				return err
			case has:
				return ErrAlreadyRunning
			}
		case MeetingConcluded:
			// To ensure the correct time order of conclusions
			// prevent that we conclude a meeting if a newer
			// one already has been concluded.
			switch has, err := HasConcludedMeetingNewerThanTx(ctx, tx, meetingID); {
			case err != nil:
				return err
			case has:
				return ErrNewerConcluded
			}
		}
		return nil
	}

	// This is only called if the update was successful.
	onSuccess := func(ctx context.Context, tx *sql.Tx) error {
		if meetingStatus != MeetingConcluded {
			return nil
		}
		gathering, err := IsGatheringMeetingTx(ctx, tx, meetingID)
		if err != nil {
			return err
		}
		// Gatherings have no influence on voting.
		if gathering {
			return nil
		}
		prevMeetingID, hasPrev, err := PreviousMeetingTx(ctx, tx, meetingID)
		if err != nil {
			return err
		}
		if !hasPrev { // We need two meetings.
			return nil
		}
		prevAttendees, err := MeetingAttendeesTx(ctx, tx, prevMeetingID)
		if err != nil {
			return err
		}
		currAttendees, err := MeetingAttendeesTx(ctx, tx, meetingID)
		if err != nil {
			return err
		}
		users, err := LoadCommitteeUsersTx(ctx, tx, committeeID)
		if err != nil {
			return err
		}

		// Lazy previous loading as we don't need this in all cases.
		var prevMeeting *Meeting
		loadPrevMeeting := func() error {
			if prevMeeting != nil {
				return nil
			}
			var err error
			prevMeeting, err = LoadMeetingTx(ctx, tx, meetingID, committeeID)
			if err != nil {
				err = fmt.Errorf("loading previous meeting failed: %w", err)
			}
			return err
		}

		// Lists of users to upgrade and downgrade.
		var upgrades, downgrades []string

		crit := MembershipByID(committeeID)
		for _, user := range users {
			ms := user.FindMembershipCriterion(crit)
			if ms == nil || ms.Status == NoneVoting {
				continue
			}
			votingCurr, wasInCurr := currAttendees[user.Nickname]
			votingPrev, wasInPrev := prevAttendees[user.Nickname]

			if !wasInCurr { // user was absent in current meeting.
				if ms.Status == Voting { // currently a voting member
					if !wasInPrev { // was absent in previous meeting.
						// There could be three reasons:
						// 1. User was not in the committee at end of the previous meeting.
						// 2. User was not a voting member at this time.
						// 3. User was a voting member but absent.
						if err := loadPrevMeeting(); err != nil {
							return err
						}
						memberStatus, wasMemberPrev, err := UserMemberStatusSinceTx(
							ctx, tx,
							user.Nickname, committeeID,
							prevMeeting.StopTime)
						if err != nil {
							return err
						}
						isExcused, err := IsUserExcusedFromMeetingTx(ctx, tx, user.Nickname, committeeID, prevMeeting.StopTime)
						if err != nil {
							return err
						}
						switch {
						case isExcused:
							// user had approved absent
						case !wasMemberPrev:
							// user was not member so that is his/her first strike.
						case memberStatus != Voting:
							// user was a member but at not a voter -> first strike.
						default:
							// second strike
							downgrades = append(downgrades, user.Nickname)
						}
					}
				}
				continue
			}
			// User was in current meeting
			if !votingCurr && ms.Status == Member { // Currently a none voting member
				if wasInPrev { // Was in previous too
					if votingPrev { // We know user was a downgraded voter -> no upgrade.
						continue
					}
					// To be upgrade the user needs to be a member at the
					// time of the previous time.
					if err := loadPrevMeeting(); err != nil {
						return err
					}
					memberStatus, wasMemberPrev, err := UserMemberStatusSinceTx(
						ctx, tx,
						user.Nickname, committeeID,
						prevMeeting.StopTime)
					if err != nil {
						return err
					}
					if wasMemberPrev && memberStatus == Member {
						upgrades = append(upgrades, user.Nickname)
					}
				}
			}
		} // all committee users.

		// Store the changes.
		if len(upgrades) > 0 || len(downgrades) > 0 {
			if err := UpdateUserCommitteeStatusTx(
				ctx, tx,
				misc.Join2(
					misc.Attribute(slices.Values(upgrades), Voting),
					misc.Attribute(slices.Values(downgrades), Member)),
				committeeID,
				timer,
			); err != nil {
				return fmt.Errorf("upgrading / downgrading members failed: %w", err)
			}
		}
		return nil
	}
	return UpdateMeetingStatus(
		ctx, db,
		meetingID, committeeID, meetingStatus,
		precondition,
		onSuccess,
	)
}

// UpdateMeetingStatus updates the status of the meeting identified by its id.
func UpdateMeetingStatus(
	ctx context.Context, db *database.Database,
	meetingID, committeeID int64,
	meetingStatus MeetingStatus,
	precondition, onSuccess func(context.Context, *sql.Tx) error,
) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if precondition != nil {
		if err := precondition(ctx, tx); err != nil {
			return err
		}
	}

	const updateSQL = `UPDATE meetings SET status = ? ` +
		`WHERE id = ? AND committees_id = ? ` +
		`AND status <> 2` // Don't update concluded meetings.

	result, err := tx.ExecContext(ctx, updateSQL,
		meetingStatus,
		meetingID,
		committeeID,
	)
	if err != nil {
		return fmt.Errorf("updating meeting status failed: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("cannot determine meeting status change: %w", err)
	}
	if n == 1 && onSuccess != nil {
		if err := onSuccess(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}
