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
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"sync"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/config"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
)

// Session encapsulte a database session.
type Session struct {
	sync.Mutex
	delete   bool
	id       string
	nickname string
}

// Nickname returns the user connected with the session.
func (s *Session) Nickname() string {
	return s.nickname
}

// ID returns the session id.
func (s *Session) ID() string {
	return s.id
}

// Delete marks the session to be deleted.
func (s *Session) Delete() {
	s.Lock()
	defer s.Unlock()
	s.delete = true
}

// NewSession checks nickname and password and returns a new session on success.
func NewSession(
	ctx context.Context,
	cfg *config.Config,
	db *database.Database,
	nickname, password string,
) (*Session, error) {
	var dbPassword string
	const passwordSQL = `SELECT password FROM users WHERE nickname = ?`
	switch err := db.DB.QueryRowContext(
		ctx, passwordSQL, nickname).Scan(&dbPassword); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}
	raw, err := base64.URLEncoding.DecodeString(dbPassword)
	if err != nil {
		return nil, err
	}
	if len(raw) < 4 {
		return nil, errors.New("db password is too short")
	}
	// Check the password.
	salt, rest := raw[:4], raw[4:]
	hash := sha256.New()
	hash.Write(salt)
	io.WriteString(hash, password)
	hashed := hash.Sum(nil)
	if subtle.ConstantTimeCompare(rest, hashed) == 0 {
		return nil, nil
	}
	// Create a new session.
	stored, sign := cfg.Sessions.GenerateKey()
	const insertSQL = `INSERT INTO sessions (nickname, token) VALUES (?, ?)`
	if _, err := db.DB.ExecContext(ctx, insertSQL, nickname, stored); err != nil {
		return nil, err
	}
	return &Session{
		id:       stored + ":" + sign,
		nickname: nickname,
	}, nil
}
