// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package web

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/auth"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
)

func (c *Controller) member(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)
	meetings, err := models.LoadMeetings(
		ctx, c.db,
		misc.Map(user.Committees(), (*models.Committee).GetID))
	if !check(w, r, err) {
		return
	}
	attended, err := models.AttendedMeetings(ctx, c.db, user.Nickname)
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Session":  auth.SessionFromContext(ctx),
		"User":     user,
		"Meetings": meetings,
		"Attended": attended,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "member.tmpl", data))
}

func (c *Controller) memberAttend(w http.ResponseWriter, r *http.Request) {
	var (
		meetingID, err1   = misc.Atoi64(r.FormValue("meeting"))
		committeeID, err2 = misc.Atoi64(r.FormValue("committee"))
		attend, err3      = strconv.ParseBool(r.FormValue("attend"))
		ctx               = r.Context()
	)
	if !checkParam(w, err1, err2, err3) {
		return
	}
	meeting, err := models.LoadMeeting(ctx, c.db, meetingID, committeeID)
	if !check(w, r, err) {
		return
	}
	if meeting == nil || meeting.Status != models.MeetingRunning {
		c.member(w, r)
		return
	}
	user := auth.UserFromContext(ctx)
	ms := user.FindMembershipCriterion(models.MembershipByID(committeeID))
	voting := ms.Status == models.Voting
	if !check(w, r, models.UpdateAttendee(ctx, c.db, meetingID, user.Nickname, attend, voting)) {
		return
	}
	// new parameter where to redirect
	redirect := r.FormValue("redirect")

	switch redirect {
	case "meeting_status":
		sessionID := r.FormValue("SESSIONID")
		target := fmt.Sprintf("/meeting_status?SESSIONID=%s&meeting=%d&committee=%d", sessionID, meetingID, committeeID)
		http.Redirect(w, r, target, http.StatusSeeOther)
	default:
		c.member(w, r)
	}
}

func (c *Controller) memberStatusEdit(w http.ResponseWriter, r *http.Request) {
	var (
		committeeID, err1 = misc.Atoi64(r.FormValue("committee"))
		formMeetingID     = r.FormValue("meeting")
		nickname          = r.FormValue("nickname")
		status            = r.FormValue("status")
	)
	if !checkParam(w, err1) {
		return
	}
	if nickname == "" {
		return
	}
	if status == "" {
		return
	}
	var meetingID int64 = -1
	if formMeetingID != "" {
		meetingID, err1 = misc.Atoi64(formMeetingID)
		if !checkParam(w, err1) {
			return
		}
	}
	data := templateData{
		"CommitteeID": committeeID,
		"MeetingID":   meetingID,
		"Nickname":    nickname,
		"Status":      status,
	}
	check(w, r, c.htmxTmpls.ExecuteTemplate(w, "member_status_edit.tmpl", data))
}

func (c *Controller) memberStatusStore(w http.ResponseWriter, r *http.Request) {
	var (
		committeeID, err1 = misc.Atoi64(r.FormValue("committee"))
		formMeetingID     = r.FormValue("meeting")
		nickname          = r.FormValue("nickname")
		status            = r.FormValue("status")
		ctx               = r.Context()
	)
	user := auth.UserFromContext(ctx)
	if !checkParam(w, err1) {
		return
	}
	if nickname == "" || status == "" {
		return
	}

	membershipStatus, err := models.ParseMemberStatus(status)
	if err != nil {
		return
	}
	db := c.db
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	var meetingID int64 = -1
	defer tx.Rollback()
	// If formMeetingID is not empty the change happens as part of a meeting. If not the status
	// is updated from somewhere else e.g. the member overview.
	if formMeetingID != "" {
		meetingID, err = misc.Atoi64(formMeetingID)
		if !checkParam(w, err) {
			return
		}

		var allUserHistories models.UsersHistories
		allUserHistories, err = models.LoadUsersHistoriesTx(ctx, tx, committeeID)
		if !check(w, r, err) {
			return
		}
		userHistory := allUserHistories[nickname]
		length := len(userHistory)
		if length > 0 && userHistory[len(userHistory)-1].Pending {
			since := userHistory[len(userHistory)-1].Since
			err = models.UpdateUserHistoryEntryTx(ctx, tx, membershipStatus, nickname, since, true)
		} else {
			var meeting *models.Meeting
			meeting, err = models.LoadMeeting(ctx, c.db, meetingID, committeeID)
			if !check(w, r, err) {
				return
			}
			updateTime := misc.CalculateEndpoint(meeting.StartTime, meeting.StopTime)
			err = models.AddUserHistoryEntryTx(
				ctx,
				tx,
				committeeID,
				membershipStatus,
				updateTime,
				nickname,
				&meetingID,
				user.Nickname,
			)
		}
	} else {
		err = models.UpdateUserCommitteeStatusTx(
			ctx,
			tx,
			misc.Pair(nickname, membershipStatus),
			committeeID,
			time.Now(),
			false,
			nil,
			&user.Nickname,
		)
	}
	if !check(w, r, err) {
		log.Printf("updating membership status failed: %v", err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("updating temporary member status failed: %v", err)
	}

	data := templateData{
		"CommitteeID": committeeID,
		"MeetingID":   meetingID,
		"Nickname":    nickname,
		"Status":      membershipStatus,
	}
	check(w, r, c.htmxTmpls.ExecuteTemplate(w, "member_status_display.tmpl", data))
}
