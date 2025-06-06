// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package web

import (
	"encoding/csv"
	"errors"
	"fmt"
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
	members, err := models.LoadCommitteeUsers(ctx, c.db, committeeID)
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
		Attending:       len(attendees),
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

	// needed for timestamps for begin and end of meeting
	meeting, err := models.LoadMeeting(ctx, c.db, meetingID, committeeID)
	if !check(w, r, err) {
		return
	}

	// Whether to use time.Now() or not
	timer := misc.CalculateEndpoint(meeting.StartTime, meeting.StopTime)
	switch err := models.ChangeMeetingStatus(
		ctx, c.db,
		meetingID, committeeID, meetingStatus,
		timer,
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
