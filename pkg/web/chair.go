// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package web

import (
	"errors"
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
	meetings, err := models.LoadMeetings(
		ctx, c.db,
		misc.Map(user.Committees(), (*models.Committee).GetID))
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Session":  auth.SessionFromContext(ctx),
		"User":     user,
		"Meetings": meetings,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "chair.tmpl", data))
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
		gathering   = r.FormValue("gathering") != ""
		s, errS     = time.ParseInLocation("2006-01-02T15:04", startTime, time.UTC)
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
		gathering         = r.FormValue("gathering") != ""
		s, errS           = time.ParseInLocation("2006-01-02T15:04", startTime, time.UTC)
		d, errD           = parseDuration(duration)
		ctx               = r.Context()
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

func (c *Controller) meetingStatusError(
	w http.ResponseWriter,
	r *http.Request,
	errMsg string,
) {
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
	members, err := models.LoadCommitteeUsers(ctx, c.db, committeeID)
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
		NonVoting:       numNonVoters,
	}

	slices.SortFunc(members, (*models.User).Compare)

	data := templateData{
		"Session":        auth.SessionFromContext(ctx),
		"User":           auth.UserFromContext(ctx),
		"Meeting":        meeting,
		"Members":        members,
		"Attendees":      attendees,
		"Quorum":         &quorum,
		"Committee":      committee,
		"AlreadyRunning": alreadyRunning,
	}
	if errMsg != "" {
		data.error(errMsg)
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "meeting_status.tmpl", data))
}

func (c *Controller) meetingStatusStore(w http.ResponseWriter, r *http.Request) {
	var (
		meetingID, err1     = misc.Atoi64(r.FormValue("meeting"))
		committeeID, err2   = misc.Atoi64(r.FormValue("committee"))
		meetingStatus, err3 = models.ParseMeetingStatus(r.FormValue("status"))
		ctx                 = r.Context()
	)
	if !checkParam(w, err1, err2, err3) {
		return
	}
	switch err := models.ChangeMeetingStatus(
		ctx, c.db,
		meetingID, committeeID, meetingStatus,
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
		ctx               = r.Context()
	)
	if !checkParam(w, err1, err2) {
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
	users, err := models.LoadCommitteeUsers(ctx, c.db, committeeID)
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
	if !check(w, r, models.UpdateAttendees(ctx, c.db, meetingID, seq)) {
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
