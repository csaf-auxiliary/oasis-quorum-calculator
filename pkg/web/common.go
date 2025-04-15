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
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
)

// datetimeHoursMinutes rounds the duration to minutes
// and returns a value suitable for datetime attributes.
func datetimeHoursMinutes(d time.Duration) string {
	d = d.Round(time.Minute)
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("PT%dH%dM", hours, minutes)
}

// hoursMinutes rounds the duration to minutes
// and returns a human form.
func hoursMinutes(d time.Duration) string {
	d = d.Round(time.Minute)
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	var b strings.Builder
	b.WriteString(strconv.Itoa(hours))
	b.WriteByte('h')
	if minutes != 0 {
		b.WriteByte(' ')
		b.WriteString(strconv.Itoa(minutes))
		b.WriteByte('m')
	}
	return b.String()
}

// args is used in templates to construct maps of key/value pairs.
func args(args ...any) (any, error) {
	n := len(args)
	if n%2 == 1 {
		return nil, errors.New("number of args have to be even")
	}
	m := make(map[any]any, n/2)
	for i := 0; i < n; i += 2 {
		key, value := args[i], args[i+1]
		m[key] = value
	}
	return m, nil
}

var durationRe = regexp.MustCompile(`^\s*(?:(\d+)\s*h)?\s*(?:(\d+)\s*m)?\s*$`)

// parseDuration parses hours an minutes to a duration.
func parseDuration(d string) (time.Duration, error) {
	match := durationRe.FindStringSubmatch(d)
	if match == nil {
		return 0, errors.New("not a valid duration")
	}
	var h, m int64
	if match[1] != "" {
		h, _ = misc.Atoi64(match[1])
	}
	if match[2] != "" {
		m, _ = misc.Atoi64(match[2])
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}

// checkParam checks a list of errors if there are any.
// In this case it issues a bad request into the given response writer.
func checkParam(w http.ResponseWriter, errs ...error) bool {
	if err := errors.Join(errs...); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// check checks a given error, logs it and issues an internal server error
// into the given response writer.
func check(w http.ResponseWriter, r *http.Request, err error) bool {
	if err != nil {
		slog.ErrorContext(r.Context(), "internal error", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return false
	}
	return true
}
