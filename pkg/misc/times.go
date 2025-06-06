// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

// Package misc implements miscellaneous functions.
package misc

import (
	"time"
)

// CalculateEndpoint determines whether time.Now is happening during a duration from a set point in time.
// If it is, then time.Now() is returned, otherwise the endpoint of the duration is returned.
func CalculateEndpoint(begin time.Time, end time.Time) time.Time {
	now := time.Now()

	if now.After(begin) && now.Before(end) {
		return now
	}
	return end
}
