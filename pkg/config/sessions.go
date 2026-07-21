// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2024 Intevation GmbH <https://intevation.de>

package config

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const defaultSessionMaxAge = time.Hour

// HexBytes is a hex encoded string.
type HexBytes []byte

// Sessions are the config options of the session management.
type Sessions struct {
	MaxAge time.Duration `toml:"max_age"`
	Secret HexBytes      `toml:"secret"`
	Secure bool          `toml:"secure"`
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (hb *HexBytes) UnmarshalText(text []byte) error {
	bytes, err := hex.DecodeString(string(text))
	if err != nil {
		return fmt.Errorf("not a valid hex encoded string: %w", err)
	}
	*hb = bytes
	return nil
}

func (s *Sessions) presetDefaults() {
	if len(s.Secret) == 0 {
		s.Secret = make([]byte, 16)
		rand.Read(s.Secret)
		skey := hex.EncodeToString(s.Secret)
		slog.Info("Generated new secret session key. "+
			"Store in config to reuse it.", "secret", skey)
	}
}

// GenerateKey generates a new session key signed by the session secret.
func (s *Sessions) GenerateKey() (string, string) {
	key := make([]byte, 16)
	rand.Read(key)
	mac := hmac.New(sha1.New, s.Secret)
	mac.Write(key)
	sign := mac.Sum(nil)
	return base64.URLEncoding.EncodeToString(key),
		base64.URLEncoding.EncodeToString(sign)
}

// CheckKey checks if the given key is a valid key signed by the session secret.
func (s *Sessions) CheckKey(skey string) (string, bool) {
	k, sign, ok := strings.Cut(skey, ":")
	if !ok {
		return "", false
	}
	kb, err1 := base64.URLEncoding.DecodeString(k)
	sb, err2 := base64.URLEncoding.DecodeString(sign)
	if err1 != nil || err2 != nil {
		return "", false
	}
	mac := hmac.New(sha1.New, s.Secret)
	mac.Write(kb)
	expected := mac.Sum(nil)
	if !hmac.Equal(sb, expected) {
		return "", false
	}
	return k, true
}
