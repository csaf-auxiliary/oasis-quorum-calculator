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
	"net/url"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/auth"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
)

func (c *Controller) authFailed(w http.ResponseWriter, r *http.Request, nickname, msg string) {
	data := map[string]string{
		"nickname": nickname,
		"error":    msg,
	}
	check(w, r, c.tmpls.ExecuteTemplate(w, "auth.tmpl", data))
}

func (c *Controller) auth(w http.ResponseWriter, r *http.Request) {
	check(w, r, c.tmpls.ExecuteTemplate(w, "auth.tmpl", nil))
}

func (c *Controller) login(w http.ResponseWriter, r *http.Request) {
	// Check if we are already logged in.
	if auth.UserFromContext(r.Context()) != nil {
		c.home(w, r)
		return
	}
	nickname := r.FormValue("nickname")
	if nickname == "" {
		c.authFailed(w, r, "", "Missing user name")
		return
	}
	password := r.FormValue("password")
	if password == "" {
		c.authFailed(w, r, nickname, "Missing password")
		return
	}
	session, err := auth.NewSession(
		r.Context(),
		c.cfg, c.db,
		nickname, password)
	if !check(w, r, err) {
		return
	}
	if session == nil {
		c.authFailed(w, r, nickname, "Login failed")
		return
	}
	_, err = models.LoadUser(r.Context(), c.db, nickname, nil)
	if !check(w, r, err) {
		return
	}

	http.Redirect(w, r, "/?SESSIONID="+url.QueryEscape(session.ID()), http.StatusFound)
}

func (c *Controller) logout(_ http.ResponseWriter, r *http.Request) {
	auth.SessionFromContext(r.Context()).Delete()
}
