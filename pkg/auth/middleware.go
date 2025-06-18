// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/config"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/models"
)

// sessionParameter is the name of the sessionid.
const sessionParameter = "SESSIONID"

// Middleware is the middleware to handle authentication.
type Middleware struct {
	cfg      *config.Config
	db       *database.Database
	redirect string
}

type contextKeyType int

const (
	sessionKey contextKeyType = iota
	userKey
)

// NewMiddleware returns a new auth middleware.
func NewMiddleware(cfg *config.Config, db *database.Database, redirect string) *Middleware {
	return &Middleware{
		cfg:      cfg,
		db:       db,
		redirect: redirect,
	}
}

// SessionFromContext returns the session from the context.
func SessionFromContext(ctx context.Context) *Session {
	v := ctx.Value(sessionKey)
	if v == nil {
		return nil
	}
	return v.(*Session)
}

// UserFromContext returns the user from the context.
func UserFromContext(ctx context.Context) *models.User {
	v := ctx.Value(userKey)
	if v == nil {
		return nil
	}
	return v.(*models.User)
}

// Roles checks if the user has any of the given roles in her of his committees.
func (mw *Middleware) Roles(next http.HandlerFunc, roles ...models.Role) http.HandlerFunc {
	return mw.User(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if !slices.ContainsFunc(user.Memberships, func(m *models.Membership) bool {
			return m.HasAnyRole(roles...)
		}) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next(w, r)
	})
}

// CommitteeRoles checks if the user has any of the given roles in the committee
// passed as a form value.
func (mw *Middleware) CommitteeRoles(next http.HandlerFunc, roles ...models.Role) http.HandlerFunc {
	return mw.User(func(w http.ResponseWriter, r *http.Request) {
		committee := r.FormValue("committee")
		cid, err := misc.Atoi64(committee)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if !slices.ContainsFunc(user.Memberships, func(m *models.Membership) bool {
			return m.Committee.ID == cid && m.HasAnyRole(roles...)
		}) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next(w, r)
	})
}

// User loads the data of a logged in user and stores it in the context.
func (mw *Middleware) User(next http.HandlerFunc) http.HandlerFunc {
	return mw.LoggedIn(func(w http.ResponseWriter, r *http.Request) {
		session := SessionFromContext(r.Context())
		if session == nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		user, err := models.LoadUser(r.Context(), mw.db, session.Nickname(), nil)
		if err != nil {
			slog.ErrorContext(r.Context(), "loading user failed", "error", err)
			http.Error(w, "loading user failed", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		nctx := context.WithValue(r.Context(), userKey, user)
		next(w, r.WithContext(nctx))
	})
}

// AdminOrRoles only allows the given handler to be called if the user is an admin or has any given role.
func (mw *Middleware) AdminOrRoles(next http.HandlerFunc, roles ...models.Role) http.HandlerFunc {
	return mw.User(func(w http.ResponseWriter, r *http.Request) {
		if user := UserFromContext(r.Context()); user == nil || !user.IsAdmin {
			if !slices.ContainsFunc(user.Memberships, func(m *models.Membership) bool {
				return m.HasAnyRole(roles...)
			}) {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	})
}

// Admin only allows the given handler to be called if the user is an admin.
func (mw *Middleware) Admin(next http.HandlerFunc) http.HandlerFunc {
	return mw.User(func(w http.ResponseWriter, r *http.Request) {
		if user := UserFromContext(r.Context()); user == nil || !user.IsAdmin {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next(w, r)
	})
}

// LoggedIn wraps the middleware around the given next.
func (mw *Middleware) LoggedIn(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.FormValue(sessionParameter)
		if sessionID == "" {
			http.Redirect(w, r, mw.redirect, http.StatusSeeOther)
			return
		}
		token, ok := mw.cfg.Sessions.CheckKey(sessionID)
		if !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		var (
			user       string
			lastAccess time.Time
		)
		const userSQL = `SELECT nickname, last_access FROM sessions ` +
			`WHERE token = ?`

		switch err := mw.db.DB.QueryRowContext(r.Context(), userSQL, token).Scan(
			&user,
			&lastAccess,
		); {
		case errors.Is(err, sql.ErrNoRows):
			http.Redirect(w, r, mw.redirect, http.StatusSeeOther)
			return
		case err != nil:
			slog.ErrorContext(r.Context(), "cannot load session", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if expired := time.Now().Add(-mw.cfg.Sessions.MaxAge); lastAccess.Before(expired) {
			http.Redirect(w, r, mw.redirect, http.StatusSeeOther)
			return
		}
		session := &Session{
			nickname: user,
			id:       sessionID,
		}
		nctx := context.WithValue(r.Context(), sessionKey, session)
		defer func() {
			var sql string
			if session.delete {
				sql = `DELETE FROM sessions WHERE token = ?`
			} else {
				sql = `UPDATE sessions SET last_access = current_timestamp ` +
					`WHERE token = ?`
			}
			if _, err := mw.db.DB.ExecContext(r.Context(), sql, token); err != nil {
				slog.ErrorContext(r.Context(),
					"updating/deleting session failed", "error", err)
			}
			if session.delete {
				http.Redirect(w, r, mw.redirect, http.StatusSeeOther)
			}
		}()
		next(w, r.WithContext(nctx))
	}
}
