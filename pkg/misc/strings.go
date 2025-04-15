// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package misc

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Shorten shortens a string to max. 40 characters.
func Shorten(v any) string {
	var s string
	switch x := v.(type) {
	case *string:
		if x == nil {
			return ""
		}
		s = *x
	case string:
		s = x
	default:
		return ""
	}
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > 40 {
		runes := []rune(s)
		return string(runes[:37]) + "..."
	}
	return s
}

// Atoi64 is a [strconv.Atoi] like wrapper for int64s.
func Atoi64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// NilString returns nil if the given string is empty else a pointer to
// the string is returned.
func NilString(s string) *string {
	if s != "" {
		return &s
	}
	return nil
}

// EmptyString returns an empty string if the pointer is nil
// the dereferenced string otherwise.
func EmptyString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// CompareEmptyStrings compares to strings.
func CompareEmptyStrings(a, b *string) int {
	return strings.Compare(EmptyString(a), EmptyString(b))
}

// NilChanger updates a potential nil string.
func NilChanger(changed *bool, s **string, v string) {
	switch {
	case v == "" && *s == nil:
		return
	case v != "" && *s != nil && v == **s:
		return
	case v == "" && *s != nil:
		*s = nil
	default:
		*s = &v
	}
	*changed = true
}
