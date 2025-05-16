-- This file is Free Software under the Apache-2.0 License
-- without warranty, see README.md and LICENSE for details.
--
-- SPDX-License-Identifier: Apache-2.0
--
-- SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
-- Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

CREATE TABLE member_absent (
    nickname       VARCHAR NOT NULL REFERENCES users(nickname)    ON DELETE CASCADE,
    start_time     TIMESTAMP NOT NULL,
    stop_time      TIMESTAMP NOT NULL,
    committee_id  INTEGER NOT NULL REFERENCES committees(id)     ON DELETE CASCADE,
    CHECK (start_time < stop_time),
    UNIQUE (nickname, start_time)
);
