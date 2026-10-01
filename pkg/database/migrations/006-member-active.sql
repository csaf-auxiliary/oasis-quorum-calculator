-- This file is Free Software under the Apache-2.0 License
-- without warranty, see README.md and LICENSE for details.
--
-- SPDX-License-Identifier: Apache-2.0
--
-- SPDX-FileCopyrightText: 2026 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
-- Software-Engineering: 2026 Intevation GmbH <https://intevation.de>

ALTER TABLE users ADD COLUMN
    active BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE users SET active = TRUE;
