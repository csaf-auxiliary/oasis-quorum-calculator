-- This file is Free Software under the Apache-2.0 License
-- without warranty, see README.md and LICENSE for details.
--
-- SPDX-License-Identifier: Apache-2.0
--
-- SPDX-FileCopyrightText: 2026 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
-- Software-Engineering: 2026 Intevation GmbH <https://intevation.de>

INSERT INTO meeting_status (id, name, description) VALUES
    (3, 'review', 'Changes have to be reviewed');

ALTER TABLE member_history ADD COLUMN
    pending      BOOLEAN   NOT NULL DEFAULT TRUE;

ALTER TABLE member_history ADD COLUMN
    decision_reason INTEGER   REFERENCES meetings(id);

ALTER TABLE member_history ADD COLUMN
    decision_maker  VARCHAR   REFERENCES users(nickname);

UPDATE member_history SET pending = FALSE;
