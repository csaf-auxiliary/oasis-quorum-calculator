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
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/auth"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
)

func (c *Controller) users(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	users, err := models.LoadAllUsers(ctx, c.db)
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Users":   users,
		"Session": auth.SessionFromContext(ctx),
		"User":    auth.UserFromContext(ctx),
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "users.tmpl", data))
}

func (c *Controller) user(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := templateData{
		"Session": auth.SessionFromContext(ctx),
		"User":    auth.UserFromContext(ctx),
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "user.tmpl", data))
}

func (c *Controller) userStore(w http.ResponseWriter, r *http.Request) {
	var (
		firstname       = strings.TrimSpace(r.FormValue("firstname"))
		lastname        = strings.TrimSpace(r.FormValue("lastname"))
		password        = strings.TrimSpace(r.FormValue("password"))
		passwordConfirm = strings.TrimSpace(r.FormValue("password2"))
		changed         = false
		ctx             = r.Context()
		user            = auth.UserFromContext(ctx)
	)
	misc.NilChanger(&changed, &user.Firstname, firstname)
	misc.NilChanger(&changed, &user.Lastname, lastname)

	data := templateData{
		"Session": auth.SessionFromContext(ctx),
		"User":    user,
	}
	switch {
	case password != "" && password != passwordConfirm:
		data.error("Password and confirmation do not match.")
	case password != "" && utf8.RuneCountInString(password) < 8:
		data.error("Password too short (need at least 8 characters)")
	case password != "":
		misc.NilChanger(&changed, &user.Password, password)
	}
	if changed && !check(w, r, user.Store(ctx, c.db)) {
		return
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "user.tmpl", data))
}

func (c *Controller) usersStore(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("delete") != "" {
		me := auth.SessionFromContext(r.Context()).Nickname()
		filter := misc.Filter(slices.Values(r.Form["users"]), func(nickname string) bool {
			return nickname != "admin" && nickname != me
		})
		if !check(w, r, models.DeleteUsersByNickname(r.Context(), c.db, filter)) {
			return
		}
	}
	c.users(w, r)
}

func (c *Controller) userCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := templateData{
		"Session": auth.SessionFromContext(ctx),
		"User":    auth.UserFromContext(ctx),
		"NewUser": &models.User{},
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "user_create.tmpl", data))
}

func (c *Controller) userCreateStore(w http.ResponseWriter, r *http.Request) {
	nuser := models.User{
		Nickname:  strings.TrimSpace(r.FormValue("nickname")),
		Firstname: misc.NilString(strings.TrimSpace(r.FormValue("firstname"))),
		Lastname:  misc.NilString(strings.TrimSpace(r.FormValue("lastname"))),
		IsAdmin:   r.FormValue("admin") == "admin",
	}
	ctx := r.Context()
	committees, err := models.LoadCommittees(ctx, c.db)
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Session":    auth.SessionFromContext(ctx),
		"User":       auth.UserFromContext(ctx),
		"NewUser":    &nuser,
		"Committees": committees,
	}
	if nuser.Nickname == "" {
		data.error("Login name is missing.")
	} else {
		password := misc.RandomString(12)
		switch success, err := nuser.StoreNew(ctx, c.db, password); {
		case !check(w, r, err):
			return
		case !success:
			data.error(fmt.Sprintf("User %q already exists.", nuser.Nickname))
		default:
			data["Password"] = password
			check(w, r, c.tmpls.ExecuteTemplate(w, "user_created.tmpl", data))
			return
		}
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "user_create.tmpl", data))
}

func (c *Controller) userEdit(w http.ResponseWriter, r *http.Request) {
	nickname := r.FormValue("nickname")
	ctx := r.Context()
	user, err := models.LoadUser(ctx, c.db, nickname)
	if !check(w, r, err) {
		return
	}
	if user == nil {
		c.users(w, r)
		return
	}
	committees, err := models.LoadCommittees(ctx, c.db)
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Session":    auth.SessionFromContext(ctx),
		"User":       auth.UserFromContext(ctx),
		"NewUser":    user,
		"Committees": committees,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "user_edit.tmpl", data))
}

func (c *Controller) userEditStore(w http.ResponseWriter, r *http.Request) {
	nickname := r.FormValue("nickname")
	ctx := r.Context()
	user, err := models.LoadUser(ctx, c.db, nickname)
	if !check(w, r, err) {
		return
	}
	if user == nil {
		c.users(w, r)
		return
	}
	var (
		firstname       = strings.TrimSpace(r.FormValue("firstname"))
		lastname        = strings.TrimSpace(r.FormValue("lastname"))
		password        = strings.TrimSpace(r.FormValue("password"))
		passwordConfirm = strings.TrimSpace(r.FormValue("password2"))
		changed         = false
	)

	misc.NilChanger(&changed, &user.Firstname, firstname)
	misc.NilChanger(&changed, &user.Lastname, lastname)

	committees, err := models.LoadCommittees(ctx, c.db)
	if !check(w, r, err) {
		return
	}

	data := templateData{
		"Session":    auth.SessionFromContext(ctx),
		"User":       auth.UserFromContext(ctx),
		"NewUser":    user,
		"Committees": committees,
	}
	switch {
	case password != "" && password != passwordConfirm:
		data.error("Password and confirmation do not match.")
	case password != "" && utf8.RuneCountInString(password) < 8:
		data.error("Password too short (need at least 8 characters)")
	case password != "":
		misc.NilChanger(&changed, &user.Password, password)
	}
	if changed && !check(w, r, user.Store(ctx, c.db)) {
		return
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "user_edit.tmpl", data))
}

var roleCommitteeRe = regexp.MustCompile(`(member|chair)(\d+)`)

func (c *Controller) userCommitteesStore(w http.ResponseWriter, r *http.Request) {
	roleCommittees := r.Form["role_committee"]
	memberships := map[int64]*models.Membership{}
	for _, rc := range roleCommittees {
		m := roleCommitteeRe.FindStringSubmatch(rc)
		if m == nil {
			continue
		}
		var (
			role, err2 = models.ParseRole(m[1])
			id, err1   = misc.Atoi64(m[2])
		)
		if err1 != nil || err2 != nil {
			// Should not happen.
			continue
		}
		ms := memberships[id]
		if ms == nil {
			ms = &models.Membership{
				Committee: &models.Committee{ID: id},
				Status:    models.Member,
			}
			memberships[id] = ms
		}
		ms.Roles = append(ms.Roles, role)
	}
	// Collect the status values
	for _, ms := range memberships {
		if v := r.FormValue(fmt.Sprintf("status%d", ms.Committee.ID)); v != "" {
			status, err := models.ParseMemberStatus(v)
			if !checkParam(w, err) {
				return
			}
			ms.Status = status
		}
	}

	nickname := r.FormValue("nickname")
	ctx := r.Context()
	if !check(w, r, models.UpdateMemberships(
		ctx, c.db, nickname, maps.Values(memberships))) {
		return
	}
	user, err := models.LoadUser(ctx, c.db, nickname)
	if !check(w, r, err) {
		return
	}
	committees, err := models.LoadCommittees(ctx, c.db)
	if !check(w, r, err) {
		return
	}
	data := templateData{
		"Session":    auth.SessionFromContext(ctx),
		"User":       auth.UserFromContext(ctx),
		"NewUser":    user,
		"Committees": committees,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "user_edit.tmpl", data))
}
