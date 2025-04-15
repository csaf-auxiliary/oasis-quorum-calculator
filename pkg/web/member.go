// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package web

import (
	"net/http"
	"strconv"

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
	c.member(w, r)
}
