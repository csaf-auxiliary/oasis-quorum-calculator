// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

// Package web implements the endpoints of the web server.
package web

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"time"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/auth"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/config"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
)

// Controller binds the endpoints to the internal logic.
type Controller struct {
	cfg   *config.Config
	db    *database.Database
	tmpls *template.Template
}

type templateData map[string]any

func (td templateData) error(msg string) {
	if v, ok := td["error"]; ok {
		if m, ok := v.(string); ok {
			msg = m + " " + msg
		}
	}
	td["Error"] = msg
}

func (td templateData) hasError() bool {
	_, ok := td["Error"]
	return ok
}

// templateFuncs are the functions usable in the templates.
var templateFuncs = template.FuncMap{
	"Role":                      models.ParseRole,
	"MemberStatus":              models.ParseMemberStatus,
	"MeetingStatus":             models.ParseMeetingStatus,
	"Shorten":                   misc.Shorten,
	"Args":                      args,
	"CommitteeIDFilter":         models.CommitteeIDFilter,
	"RunningFilter":             func() models.MeetingFilter { return models.RunningFilter },
	"MeetingCommitteeIDsFilter": models.MeetingCommitteeIDsFilter,
	"DatetimeHoursMinutes":      datetimeHoursMinutes,
	"HoursMinutes":              hoursMinutes,
	"Now":                       func() time.Time { return time.Now().UTC() },
}

// NewController returns a new Controller.
func NewController(
	cfg *config.Config,
	db *database.Database,
) (*Controller, error) {
	path := filepath.Join(cfg.Web.Root, "templates", "*.tmpl")

	tmpls, err := template.New("index").Funcs(templateFuncs).ParseGlob(path)
	if err != nil {
		return nil, fmt.Errorf("loading templates failed: %w", err)
	}

	return &Controller{
		cfg:   cfg,
		db:    db,
		tmpls: tmpls,
	}, nil
}

func (c *Controller) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)
	session := auth.SessionFromContext(ctx)
	if user == nil || session == nil {
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	var isMember, isChair, isSecretary bool
	for _, i := range user.Memberships {
		isChair = isChair || slices.Contains(i.Roles, models.ChairRole)
		isSecretary = isSecretary || slices.Contains(i.Roles, models.SecretaryRole)
		isMember = isMember || slices.Contains(i.Roles, models.MemberRole)
	}

	redirectURI := "/user"
	switch {
	case user.IsAdmin:
		redirectURI = "/users"
	case isChair || isSecretary:
		redirectURI = "/chair"
	case isMember:
		redirectURI = "/member"
	}

	http.Redirect(w, r, redirectURI+"?SESSIONID="+url.QueryEscape(session.ID()), http.StatusFound)
}

// Bind return a http handler to be used in a web server.
func (c *Controller) Bind() http.Handler {
	router := http.NewServeMux()
	mw := auth.NewMiddleware(c.cfg, c.db, "/auth")

	for _, route := range []struct {
		pattern string
		handler http.HandlerFunc
	}{
		// Auth
		{"/auth", c.auth},
		{"/login", c.login},
		{"/logout", mw.LoggedIn(c.logout)},
		{"/", mw.User(c.home)},
		// User
		{"/user", mw.User(c.user)},
		{"/user_store", mw.User(c.userStore)},
		{"/user_create", mw.Admin(c.userCreate)},
		{"/user_edit", mw.Admin(c.userEdit)},
		{"/user_edit_store", mw.Admin(c.userEditStore)},
		{"/user_create_store", mw.Admin(c.userCreateStore)},
		{"/user_committees_store", mw.Admin(c.userCommitteesStore)},
		{"/users", mw.Admin(c.users)},
		{"/users_store", mw.Admin(c.usersStore)},
		// Committees
		{"/committee_edit", mw.Admin(c.committeeEdit)},
		{"/committee_edit_store", mw.Admin(c.committeeEditStore)},
		{"/committees", mw.Admin(c.committees)},
		{"/committees_store", mw.Admin(c.committeesStore)},
		{"/committee_create", mw.Admin(c.committeeCreate)},
		{"/committee_store", mw.Admin(c.committeeStore)},
		// Chair and Secretary
		{"/chair", mw.Roles(c.chair, models.ChairRole, models.SecretaryRole)},
		{"/meetings_overview", mw.CommitteeRoles(c.meetingsOverview, models.ChairRole, models.MemberRole, models.SecretaryRole)},
		{"/meetings_store", mw.CommitteeRoles(c.meetingsStore, models.ChairRole, models.SecretaryRole)},
		{"/meeting_create", mw.CommitteeRoles(c.meetingCreate, models.ChairRole, models.SecretaryRole)},
		{"/meeting_create_store", mw.CommitteeRoles(c.meetingCreateStore, models.ChairRole, models.SecretaryRole)},
		{"/meeting_edit", mw.CommitteeRoles(c.meetingEdit, models.ChairRole, models.SecretaryRole)},
		{"/meeting_edit_store", mw.CommitteeRoles(c.meetingEditStore, models.ChairRole, models.SecretaryRole)},
		{"/meeting_status", mw.CommitteeRoles(c.meetingStatus, models.ChairRole, models.MemberRole, models.SecretaryRole)},
		{"/meeting_status_store", mw.CommitteeRoles(c.meetingStatusStore, models.ChairRole, models.SecretaryRole)},
		{"/meeting_attend_store", mw.CommitteeRoles(c.meetingAttendStore, models.ChairRole, models.SecretaryRole)},
		{"/meetings_export", mw.CommitteeRoles(c.meetingsExport, models.ChairRole, models.SecretaryRole)},
		// Member
		{"/member", mw.Roles(c.member, models.MemberRole)},
		{"/member_attend", mw.CommitteeRoles(c.memberAttend, models.MemberRole)},
	} {
		router.HandleFunc(route.pattern, route.handler)
	}

	static := http.FileServer(http.Dir(c.cfg.Web.Root))
	router.Handle("/static/", static)

	return router
}
