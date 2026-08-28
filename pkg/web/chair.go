// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package web

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/auth"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
)

func (c *Controller) chair(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)
	committees := misc.Map(user.Committees(), (*models.Committee).GetID)
	meetings, err := models.LoadMeetings(
		ctx, c.db,
		misc.Map(user.Committees(), (*models.Committee).GetID))
	if !check(w, r, err) {
		return
	}
	committeesMembers := make(map[int64][]*models.User)
	for co := range committees {
		users, err := models.LoadCommitteeUsers(ctx, c.db, co, nil)
		if !check(w, r, err) {
			return
		}
		committeesMembers[co] = users
	}

	data := templateData{
		"Session":           auth.SessionFromContext(ctx),
		"User":              user,
		"Meetings":          meetings,
		"CommitteesMembers": committeesMembers,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "chair.tmpl", data))
}

func (c *Controller) committeeMemberDetails(w http.ResponseWriter, r *http.Request) {
	var (
		committeeID, err = misc.Atoi64(r.FormValue("committee"))
		nickname         = r.FormValue("nickname")
		ctx              = r.Context()
	)
	if !checkParam(w, err) {
		return
	}

	committee, err := models.LoadCommittee(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	member, err := models.LoadUser(ctx, c.db, nickname, nil)
	if !check(w, r, err) {
		return
	}
	histories, err := models.LoadUsersHistories(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}

	data := templateData{
		"Session":   auth.SessionFromContext(ctx),
		"Committee": committee,
		"Member":    member,
		"History":   histories[nickname],
		"User":      auth.UserFromContext(ctx),
	}
	check(w, r, c.htmxTmpls.ExecuteTemplate(w, "committee_member_details.tmpl", data))
}

func (c *Controller) committeeMemberOverview(w http.ResponseWriter, r *http.Request) {
	var (
		committeeID, err = misc.Atoi64(r.FormValue("committee"))
		ctx              = r.Context()
	)
	if !checkParam(w, err) {
		return
	}
	user := auth.UserFromContext(ctx)
	committee, err := models.LoadCommittee(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	members, err := models.LoadCommitteeUsers(ctx, c.db, committeeID, nil)
	if !check(w, r, err) {
		return
	}

	data := templateData{
		"Session":   auth.SessionFromContext(ctx),
		"Committee": committee,
		"Members":   members,
		"User":      user,
	}
	check(w, r, c.htmxTmpls.ExecuteTemplate(w, "committee_member_overview.tmpl", data))
}

func (c *Controller) absentOverview(w http.ResponseWriter, r *http.Request) {
	var (
		committeeID, err = misc.Atoi64(r.FormValue("committee"))
		ctx              = r.Context()
	)
	if !checkParam(w, err) {
		return
	}
	user := auth.UserFromContext(ctx)
	memberAbsent, err := models.LoadAbsent(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	committee, err := models.LoadCommittee(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	members, err := models.LoadCommitteeUsers(ctx, c.db, committeeID, nil)
	if !check(w, r, err) {
		return
	}

	data := templateData{
		"Session":      auth.SessionFromContext(ctx),
		"User":         user,
		"Committee":    committee,
		"Members":      members,
		"MemberAbsent": memberAbsent,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "absent_overview.tmpl", data))
}

func (c *Controller) absentStore(w http.ResponseWriter, r *http.Request) {
	committeeID, err := misc.Atoi64(r.FormValue("committee"))
	if !checkParam(w, err) {
		return
	}
	ctx := r.Context()
	if r.FormValue("delete") != "" {
		parseAbsentEntries := func(s string) (string, time.Time, error) {
			split := strings.Split(s, ";")
			if len(split) != 2 {
				return "", time.Time{}, errors.New("invalid entry length")
			}
			t, err := time.Parse("2006-01-02T15:04:05Z07:00", split[1])
			if err != nil {
				return "", time.Time{}, err
			}
			return split[0], t, nil
		}
		ids := misc.ParseSeq2(slices.Values(r.Form["entries"]), parseAbsentEntries)
		if !check(w, r, models.DeleteAbsentEntries(ctx, c.db, committeeID, ids)) {
			return
		}
	}
	c.absentOverview(w, r)
}

func (c *Controller) absentCreateStore(w http.ResponseWriter, r *http.Request) {
	committeeID, err := misc.Atoi64(r.FormValue("committee"))
	if !checkParam(w, err) {
		return
	}
	var (
		nickname  = r.FormValue("nickname")
		startTime = r.FormValue("start_time")
		stopTime  = r.FormValue("stop_time")
		timezone  = r.FormValue("timezone")
		ctx       = r.Context()
	)

	committee, err := models.LoadCommittee(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Session":   auth.SessionFromContext(ctx),
		"User":      auth.UserFromContext(ctx),
		"Committee": committee,
	}

	location, errL := time.LoadLocation(timezone)
	if errL != nil {
		data.error("Invalid timezone.")
		location = time.UTC
	}
	start, errStart := time.ParseInLocation("2006-01-02T15:04", startTime, location)
	if errStart == nil {
		start = start.UTC()
	}

	stop, errStop := time.ParseInLocation("2006-01-02T15:04", stopTime, location)
	if errStop == nil {
		stop = stop.UTC()
	}

	switch {
	case errStart != nil && errStop != nil:
		data.error("Start time and stop time are invalid.")
	case errStart != nil:
		data.error("Start time is invalid.")
	case errStop != nil:
		data.error("Stop time is invalid.")
	}

	var m models.MemberAbsent
	m.Name = nickname
	m.StartTime = start
	m.StopTime = stop
	if data.hasError() {
		check(w, r, c.tmpls.ExecuteTemplate(w, "absent_overview.tmpl", data))
		return
	}
	memberAbsent, err := models.LoadAbsent(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	data["MemberAbsent"] = memberAbsent
	if memberAbsent.Contains(models.MemberAbsentOverlapFilter(m.Name, m.StartTime, m.StopTime)) {
		data.error("Time range collides with another excused absent in this committee.")
		check(w, r, c.tmpls.ExecuteTemplate(w, "absent_overview.tmpl", data))
		return
	}
	if !memberAbsent.CheckMaximumAbsentTime(time.Hour*24*40, m.Name) {
		data.error("Maximum absent time is too large.")
		check(w, r, c.tmpls.ExecuteTemplate(w, "absent_overview.tmpl", data))
		return
	}
	if !check(w, r, m.StoreNew(ctx, c.db, committeeID)) {
		return
	}
	c.absentOverview(w, r)
}

func (c *Controller) meetingsStore(w http.ResponseWriter, r *http.Request) {
	committeeID, err := misc.Atoi64(r.FormValue("committee"))
	if !checkParam(w, err) {
		return
	}
	ctx := r.Context()
	if r.FormValue("delete") != "" {
		ids := misc.ParseSeq(slices.Values(r.Form["meetings"]), misc.Atoi64)
		if !check(w, r, models.DeleteMeetingsByID(ctx, c.db, committeeID, ids)) {
			return
		}
	}
	user := auth.UserFromContext(ctx)
	remaining, err := models.LoadMeetings(ctx, c.db,
		misc.Map(user.Committees(), (*models.Committee).GetID))
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Session":  auth.SessionFromContext(ctx),
		"User":     user,
		"Meetings": remaining,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "chair.tmpl", data))
}

func (c *Controller) meetingCreate(w http.ResponseWriter, r *http.Request) {
	committee, err := misc.Atoi64(r.FormValue("committee"))
	if !checkParam(w, err) {
		return
	}
	ctx := r.Context()
	now := time.Now()
	data := templateData{
		"Session": auth.SessionFromContext(ctx),
		"User":    auth.UserFromContext(ctx),
		"Meeting": &models.Meeting{
			StartTime: now,
			StopTime:  now.Add(time.Hour),
		},
		"Committee": committee,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "meeting_create.tmpl", data))
}

func (c *Controller) meetingCreateStore(w http.ResponseWriter, r *http.Request) {
	committee, err := misc.Atoi64(r.FormValue("committee"))
	if !checkParam(w, err) {
		return
	}
	var (
		description = misc.NilString(strings.TrimSpace(r.FormValue("description")))
		startTime   = r.FormValue("start_time")
		duration    = r.FormValue("duration")
		timezone    = r.FormValue("timezone")
		gathering   = r.FormValue("gathering") != ""
		d, errD     = parseDuration(duration)
		ctx         = r.Context()
	)
	meeting := models.Meeting{
		CommitteeID: committee,
		Gathering:   gathering,
		Description: description,
	}
	data := templateData{
		"Session":   auth.SessionFromContext(ctx),
		"User":      auth.UserFromContext(ctx),
		"Meeting":   &meeting,
		"Committee": committee,
	}

	location, errL := time.LoadLocation(timezone)
	if errL != nil {
		data.error("Invalid timezone.")
		location = time.UTC
	}
	s, errS := time.ParseInLocation("2006-01-02T15:04", startTime, location)
	if errS == nil {
		s = s.UTC()
	}

	switch {
	case errS != nil && errD != nil:
		data.error("Start time and duration are invalid.")
		s, d = time.Now(), time.Hour
	case errS != nil:
		data.error("Start time is invalid.")
		s = time.Now()
	case errD != nil:
		data.error("Duration is invalid.")
		d = time.Hour
	}

	meeting.StartTime = s
	meeting.StopTime = s.Add(d)
	if data.hasError() {
		check(w, r, c.tmpls.ExecuteTemplate(w, "meeting_create.tmpl", data))
		return
	}
	meetings, err := models.LoadMeetings(ctx, c.db, misc.Values(committee))
	if !check(w, r, err) {
		return
	}
	if meetings.Contains(models.OverlapFilter(meeting.StartTime, meeting.StopTime)) {
		data.error("Time range collides with another meeting in this committee.")
		check(w, r, c.tmpls.ExecuteTemplate(w, "meeting_create.tmpl", data))
		return
	}
	if !check(w, r, meeting.StoreNew(ctx, c.db)) {
		return
	}
	c.chair(w, r)
}

func (c *Controller) meetingEdit(w http.ResponseWriter, r *http.Request) {
	var (
		meetingID, err1   = misc.Atoi64(r.FormValue("meeting"))
		committeeID, err2 = misc.Atoi64(r.FormValue("committee"))
	)
	if !checkParam(w, err1, err2) {
		return
	}
	ctx := r.Context()
	meeting, err := models.LoadMeeting(ctx, c.db, meetingID, committeeID)
	if !check(w, r, err) {
		return
	}
	if meeting == nil {
		c.chair(w, r)
		return
	}
	data := templateData{
		"Session":   auth.SessionFromContext(ctx),
		"User":      auth.UserFromContext(ctx),
		"Meeting":   meeting,
		"Committee": committeeID,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "meeting_edit.tmpl", data))
}

func (c *Controller) meetingEditStore(w http.ResponseWriter, r *http.Request) {
	var (
		meetingID, err1   = misc.Atoi64(r.FormValue("meeting"))
		committeeID, err2 = misc.Atoi64(r.FormValue("committee"))
		description       = misc.NilString(strings.TrimSpace(r.FormValue("description")))
		startTime         = r.FormValue("start_time")
		duration          = r.FormValue("duration")
		timezone          = r.FormValue("timezone")
		gathering         = r.FormValue("gathering") != ""
		d, errD           = parseDuration(duration)
		ctx               = r.Context()
		s                 time.Time
		errS              error
	)
	if !checkParam(w, err1, err2) {
		return
	}
	meeting, err := models.LoadMeeting(ctx, c.db, meetingID, committeeID)
	if !check(w, r, err) {
		return
	}
	if meeting == nil || meeting.Status == models.MeetingConcluded {
		c.chair(w, r)
		return
	}
	meeting.Description = description
	data := templateData{
		"Session":   auth.SessionFromContext(ctx),
		"User":      auth.UserFromContext(ctx),
		"Meeting":   meeting,
		"Committee": committeeID,
	}

	location, errL := time.LoadLocation(timezone)
	if errL != nil {
		data.error("Invalid timezone.")
		location = time.UTC
	}
	if s, errS = time.ParseInLocation("2006-01-02T15:04", startTime, location); errS != nil {
		s = s.UTC()
	}

	switch {
	case errS != nil && errD != nil:
		data.error("Start time and duration are invalid.")
		s, d = time.Now(), time.Hour
	case errS != nil:
		data.error("Start time is invalid.")
		s = time.Now()
	case errD != nil:
		data.error("Duration is invalid.")
		d = time.Hour
	}

	meeting.StartTime = s
	meeting.StopTime = s.Add(d)
	if data.hasError() {
		check(w, r, c.tmpls.ExecuteTemplate(w, "meeting_edit.tmpl", data))
		return
	}
	meetings, err := models.LoadMeetings(ctx, c.db, misc.Values(committeeID))
	if !check(w, r, err) {
		return
	}
	if meetings.Contains(
		models.OverlapFilter(meeting.StartTime, meeting.StopTime, meetingID)) {
		data.error("Time range collides with another meeting in this committee.")
		check(w, r, c.tmpls.ExecuteTemplate(w, "meeting_edit.tmpl", data))
		return
	}
	meeting.Gathering = gathering
	if !check(w, r, meeting.Store(ctx, c.db)) {
		return
	}
	c.chair(w, r)
}

func (c *Controller) meetingStatus(w http.ResponseWriter, r *http.Request) {
	c.meetingStatusError(w, r, "")
}

func getPrevNewMemberStatus(
	history models.UserHistory,
	status models.MeetingStatus,
) (models.MemberStatus, models.MemberStatus) {
	var (
		prevStatus models.MemberStatus
		newStatus  models.MemberStatus
	)
	if len(history) == 1 {
		entry := *history[len(history)-1]
		prevStatus = entry.Status
		newStatus = entry.Status
	} else if len(history) > 1 {
		prevEntry := *history[len(history)-2]
		latestEntry := *history[len(history)-1]
		if status == models.MeetingInReview && !latestEntry.Pending {
			// It might be that no new (pending) entry was made by OQC itself yet
			prevStatus = latestEntry.Status
		} else {
			prevStatus = prevEntry.Status
		}
		newStatus = latestEntry.Status
	}
	return prevStatus, newStatus
}

func (c *Controller) meetingStatusError(
	w http.ResponseWriter,
	r *http.Request,
	errMsg string,
) {
	var (
		meetingID, err1   = misc.Atoi64(r.FormValue("meeting"))
		committeeID, err2 = misc.Atoi64(r.FormValue("committee"))
		meetingStatus, _  = models.ParseMeetingStatus(r.FormValue("status"))
		ctx               = r.Context()
	)
	if !checkParam(w, err1, err2) {
		return
	}
	meeting, err := models.LoadMeeting(ctx, c.db, meetingID, committeeID)
	if !check(w, r, err) {
		return
	}
	if meeting == nil {
		c.chair(w, r)
		return
	}
	attendees, err := meeting.Attendees(ctx, c.db)
	if !check(w, r, err) {
		return
	}
	committee, err := models.LoadCommittee(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	alreadyRunning, err := models.HasCommitteeRunningMeeting(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}

	// Number of all members, number of voting members, number of voters attending the meeting,
	// number of permanent non-voters, number of members with no voting rights.
	var numTotal, numVoters, attendingVoters, numNonVoters, numMembers int

	allUsers, err := models.LoadAllUsers(ctx, c.db)

	tx, err := c.db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return
	}
	defer tx.Rollback() // Rollback on error or if commit is not reached

	// Load all user histories for the given committee
	allUsersHistories, err := models.LoadUsersHistoriesTx(ctx, tx, committeeID)
	if err != nil {
		return
	}

	var historicalUsers []*models.HistoricalUser

	// Go over all users to include those that have left the committee since
	for _, user := range allUsers {
		// Define realStatus so it can be used later in scope
		realStatus := models.NoMember
		// Get explicit user History
		userHistory, found := allUsersHistories[user.Nickname]

		// if the User was part of the committee at meeting start, get Status they had at the time
		if found {
			realStatus = userHistory.Status(meeting.StartTime)
		}

		if realStatus != models.NoMember {
			numTotal++

			member := &models.HistoricalUser{
				User:   user,
				Status: realStatus,
			}

			historicalUsers = append(historicalUsers, member)
			switch realStatus {
			case models.Voting:
				numVoters++
				if attendees[user.Nickname] {
					attendingVoters++
				}
			case models.NoneVoting:
				numNonVoters++
			case models.Member:
				numMembers++
			default:
				return
			}
		}
	}
	quorum := models.Quorum{
		Total:           numTotal,
		Member:          numMembers,
		Voting:          numVoters,
		AttendingVoting: attendingVoters,
		Attending:       len(attendees),
		NonVoting:       numNonVoters,
	}

	slices.SortFunc(historicalUsers, func(a, b *models.HistoricalUser) int {
		return a.Compare(b.User)
	})

	if !check(w, r, err) {
		return
	}
	histories := map[string]models.MemberStatus{}
	prevStatus := map[string]models.MemberStatus{}
	newStatus := map[string]models.MemberStatus{}

	type StatusChange struct {
		Nickname string
		Status   models.MemberStatus
	}
	statusChanges := make([]StatusChange, 0, numMembers)
	for _, member := range historicalUsers {
		nickname := member.Nickname
		history := allUsersHistories[nickname]
		status, _, err := models.UserMemberStatusSince(ctx, c.db, nickname, meeting.CommitteeID, meeting.StopTime)
		if !check(w, r, err) {
			return
		}
		histories[nickname] = status
		if p, n := getPrevNewMemberStatus(history, meetingStatus); p != n || meetingStatus == models.MeetingInReview {
			prevStatus[nickname] = p
			newStatus[nickname] = n
		}
		for _, entry := range history {
			if entry.DecisionReason != nil && *entry.DecisionReason == meetingID {
				change := StatusChange{
					Nickname: nickname,
					Status:   entry.Status,
				}
				statusChanges = append(statusChanges, change)
			}
		}
	}

	data := templateData{
		"Session":        auth.SessionFromContext(ctx),
		"User":           auth.UserFromContext(ctx),
		"Meeting":        meeting,
		"Members":        historicalUsers,
		"Attendees":      attendees,
		"Quorum":         &quorum,
		"Committee":      committee,
		"AlreadyRunning": alreadyRunning,
		"PrevStatus":     prevStatus,
		"NewStatus":      newStatus,
		"Histories":      histories,
		"StatusChanges":  statusChanges,
	}
	if errMsg != "" {
		data.error(errMsg)
	}

	if meetingStatus == models.MeetingInReview {
		check(w, r, c.htmxTmpls.ExecuteTemplate(w, "meeting_review.tmpl", data))
	} else {
		check(w, r, c.tmpls.ExecuteTemplate(w, "meeting_status.tmpl", data))
	}
}

func (c *Controller) meetingReview(w http.ResponseWriter, r *http.Request) {
	var (
		meetingID, err1   = misc.Atoi64(r.FormValue("meeting"))
		committeeID, err2 = misc.Atoi64(r.FormValue("committee"))
		ctx               = r.Context()
	)
	if !checkParam(w, err1, err2) {
		return
	}
	meeting, err := models.LoadMeeting(ctx, c.db, meetingID, committeeID)
	if !check(w, r, err) {
		return
	}
	if meeting == nil {
		c.chair(w, r)
		return
	}
	members, err := models.LoadCommitteeUsers(ctx, c.db, committeeID, &meeting.StartTime)
	if !check(w, r, err) {
		return
	}
	attendees, err := meeting.Attendees(ctx, c.db)
	if !check(w, r, err) {
		return
	}
	committee, err := models.LoadCommittee(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	alreadyRunning, err := models.HasCommitteeRunningMeeting(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}

	var numVoters, attendingVoters, numNonVoters, numMembers int
	for _, member := range members {
		if ms := member.FindMembership(committee.Name); ms != nil &&
			ms.HasRole(models.MemberRole) {
			switch ms.Status {
			case models.Voting:
				numVoters++
				if attendees[member.Nickname] {
					attendingVoters++
				}
			case models.NoneVoting:
				numNonVoters++
			case models.Member:
				numMembers++
			}
		}
	}

	quorum := models.Quorum{
		Total:           len(members),
		Member:          numMembers,
		Voting:          numVoters,
		AttendingVoting: attendingVoters,
		Attending:       len(attendees),
		NonVoting:       numNonVoters,
	}

	slices.SortFunc(members, (*models.User).Compare)

	histories, err := models.LoadUsersHistories(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}

	prevStatus := map[string]models.MemberStatus{}
	newStatus := map[string]models.MemberStatus{}
	for _, member := range members {
		nickname := member.Nickname
		history := histories[nickname]
		p, n := getPrevNewMemberStatus(history, models.MeetingInReview)
		prevStatus[nickname] = p
		newStatus[nickname] = n
	}

	data := templateData{
		"Session":        auth.SessionFromContext(ctx),
		"User":           auth.UserFromContext(ctx),
		"Meeting":        meeting,
		"Members":        members,
		"Attendees":      attendees,
		"Quorum":         &quorum,
		"Committee":      committee,
		"PrevStatus":     prevStatus,
		"NewStatus":      newStatus,
		"AlreadyRunning": alreadyRunning,
	}

	slices.SortFunc(members, (*models.User).Compare)
	check(w, r, c.htmxTmpls.ExecuteTemplate(w, "meeting_review.tmpl", data))
}

func (c *Controller) meetingFinish(w http.ResponseWriter, r *http.Request) {
	var (
		committeeID, err = misc.Atoi64(r.FormValue("committee"))
		meetingID, err2  = misc.Atoi64(r.FormValue("meeting"))
		ctx              = r.Context()
		user             = auth.UserFromContext(ctx)
	)
	if !checkParam(w, err, err2) {
		return
	}
	db := c.db
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	meeting, err := models.LoadMeeting(ctx, c.db, meetingID, committeeID)
	if !check(w, r, err) {
		return
	}
	if meeting == nil {
		c.chair(w, r)
		return
	}
	members, err := models.LoadCommitteeUsers(ctx, c.db, committeeID, &meeting.StartTime)
	if !check(w, r, err) {
		return
	}
	histories, err := models.LoadUsersHistories(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}

	for _, member := range members {
		userHistory := histories[member.Nickname]
		historyLength := len(userHistory)
		if historyLength == 0 {
			continue
		}
		lastHistoryEntry := userHistory[historyLength-1]
		if lastHistoryEntry.Pending {
			if historyLength == 1 || lastHistoryEntry.Status != userHistory[historyLength-2].Status {
				// If there is only one entry or if the status did change we want to persist the new status
				err = models.UpdateUserHistoryEntryTx(ctx, tx, lastHistoryEntry.Status, member.Nickname, lastHistoryEntry.Since, false)
				if err != nil {
					return
				}
			} else if lastHistoryEntry.Status == userHistory[historyLength-2].Status {
				// If the status didn't change we have to remove the latest entry if it's pending
				err = models.DeleteUserHistoryEntryTx(ctx, tx, committeeID, member.Nickname, lastHistoryEntry.Since)
				if err != nil {
					return
				}
			}
		}
	}
	err = models.UpdateMeetingStatusTx(
		ctx,
		tx,
		meetingID,
		committeeID,
		models.MeetingConcluded,
		models.ChangeMeetingStatusPrecondition,
		nil,
		time.Now(),
		user,
	)
	if err != nil {
		log.Printf("changing meeting status failed: %v", err)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("finishing meeting failed: %v", err)
		return
	}
	c.meetingStatus(w, r)
}

func (c *Controller) meetingStatusStore(w http.ResponseWriter, r *http.Request) {
	var (
		meetingID, err1     = misc.Atoi64(r.FormValue("meeting"))
		committeeID, err2   = misc.Atoi64(r.FormValue("committee"))
		meetingStatus, err3 = models.ParseMeetingStatus(r.FormValue("status"))
		ctx                 = r.Context()
		user                = auth.UserFromContext(ctx)
	)
	if !checkParam(w, err1, err2, err3) {
		return
	}

	// needed for timestamps for begin and end of meeting
	meeting, err := models.LoadMeeting(ctx, c.db, meetingID, committeeID)
	if !check(w, r, err) {
		return
	}

	// Whether to use time.Now() or not
	timer := misc.CalculateEndpoint(meeting.StartTime, meeting.StopTime)
	switch err := models.UpdateMeetingStatus(
		ctx,
		c.db,
		meetingID,
		committeeID,
		meetingStatus,
		models.ApplyUpDowngrades,
		timer,
		user,
	); {
	case errors.Is(err, models.ErrAlreadyRunning):
		c.meetingStatusError(w, r, "Already have a running meeting in this committee.")
		return
	case errors.Is(err, models.ErrNewerConcluded):
		c.meetingStatusError(w, r, "Already have a concluded meeting that is newer.")
		return
	case !check(w, r, err):
		return
	}
	c.meetingStatus(w, r)
}

func (c *Controller) meetingAttendStore(w http.ResponseWriter, r *http.Request) {
	var (
		meetingID, err1   = misc.Atoi64(r.FormValue("meeting"))
		committeeID, err2 = misc.Atoi64(r.FormValue("committee"))
		attend            = !strings.Contains(strings.ToLower(r.FormValue("action")), "not attending")
		rendered, err3    = misc.Atoi64(r.FormValue("rendered"))
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
		c.meetingStatus(w, r)
		return
	}
	users, err := models.LoadCommitteeUsers(ctx, c.db, committeeID, &meeting.StartTime)
	if !check(w, r, err) {
		return
	}
	seq := func(yield func(string, bool) bool) {
		crit := models.MembershipByID(committeeID)
		for _, nickname := range r.Form["attend"] {
			// Check if the given nickname is really in the members of this committee.
			idx := slices.IndexFunc(users, func(u *models.User) bool {
				return u.Nickname == nickname
			})
			if idx == -1 {
				continue
			}
			if ms := users[idx].FindMembershipCriterion(crit); ms != nil {
				// Remember if voting is allowed at the moment.
				// This may change in the future.
				voting := ms.Status == models.Voting && ms.HasRole(models.MemberRole)
				if !yield(nickname, voting) {
					return
				}
			}
		}
	}
	action := models.Attend
	if !attend {
		action = models.Unattend
	}
	if !check(w, r, action(ctx, c.db, meetingID, seq, time.UnixMicro(rendered).UTC())) {
		return
	}
	c.meetingStatus(w, r)
}

func (c *Controller) meetingsOverview(w http.ResponseWriter, r *http.Request) {
	var (
		committeeID, err = misc.Atoi64(r.FormValue("committee"))
		ctx              = r.Context()
	)
	if !checkParam(w, err) {
		return
	}
	committee, err := models.LoadCommittee(ctx, c.db, committeeID)
	if !check(w, r, err) {
		return
	}
	// Number of meetings to load.
	const limit = -1
	overview, err := models.LoadMeetingsOverview(ctx, c.db, committeeID, limit)
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Session":   auth.SessionFromContext(ctx),
		"User":      auth.UserFromContext(ctx),
		"Committee": committee,
		"Overview":  overview,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "meetings_overview.tmpl", data))
}

func (c *Controller) meetingsExport(w http.ResponseWriter, r *http.Request) {
	var (
		committeeID, err = misc.Atoi64(r.FormValue("committee"))
		ctx              = r.Context()
	)
	if !checkParam(w, err) {
		return
	}
	const limit = -1
	overview, err := models.LoadMeetingsOverview(ctx, c.db, committeeID, limit)
	if !check(w, r, err) {
		return
	}

	// Set headers for CSV download
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=meetings_%d.csv", committeeID))

	// Create CSV writer
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write CSV header
	header := []string{
		"Meeting ID",
		"Start Time",
		"Stop Time",
		"Status",
		"Gathering",
		"Description",
		"Quorum Reached",
		"Quorum Percent",
		"Attending Voting",
		"Total Voters",
		"Attendees",
		"Non-Attendees",
	}
	if err := writer.Write(header); err != nil {
		check(w, r, err)
		return
	}

	// Write meeting data
	for _, meetingData := range overview.Data {
		meeting := meetingData.Meeting
		quorum := meetingData.Quorum
		if quorum == nil {
			quorum = &models.Quorum{}
		}

		// Convert Status to string
		var status string
		switch meeting.Status {
		case models.MeetingOnHold:
			status = "On Hold"
		case models.MeetingRunning:
			status = "Running"
		case models.MeetingConcluded:
			status = "Concluded"
		default:
			status = "Could not load Status"
		}
		// Get description
		description := ""
		if meeting.Description != nil {
			description = *meeting.Description
		}

		var attendeesList []string
		for nickname, voting := range meetingData.Attendees {
			status := "non-voting"
			if voting {
				status = "voting"
			}
			attendeesList = append(attendeesList, fmt.Sprintf("%s:%s", nickname, status))
		}
		// Convert to String to write to CSV
		attendeesString := strings.Join(attendeesList, ",")

		// All users except those who attended to get a list of all non-Attendees
		var nonAttendeesList []string
		for _, user := range overview.Users {
			if _, attended := meetingData.Attendees[user.Nickname]; !attended {
				nonAttendeesList = append(nonAttendeesList, user.Nickname)
			}
		}
		// Convert to String to write to CSV
		nonAttendeesString := strings.Join(nonAttendeesList, ",")

		// Gather all data
		data := []string{
			fmt.Sprintf("%d", meeting.ID),
			meeting.StartTime.Format("2006-01-02 15:04:05"),
			meeting.StopTime.Format("2006-01-02 15:04:05"),
			status,
			fmt.Sprintf("%t", meeting.Gathering),
			description,
			fmt.Sprintf("%t", quorum.Reached()),
			fmt.Sprintf("%.2f", quorum.Percent()),
			fmt.Sprintf("%d", quorum.AttendingVoting),
			fmt.Sprintf("%d", quorum.Voting),
			attendeesString,
			nonAttendeesString,
		}
		// and write it to a file
		if err := writer.Write(data); err != nil {
			check(w, r, err)
			return
		}
	}
}
