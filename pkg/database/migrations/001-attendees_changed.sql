-- This file is Free Software under the Apache-2.0 License
-- without warranty, see README.md and LICENSE for details.
--
-- SPDX-License-Identifier: Apache-2.0
--
-- SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
-- Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

CREATE TABLE attendees_changes (
    time        TIMESTAMP NOT NULL,
    meetings_id INTEGER NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    nickname    VARCHAR NOT NULL REFERENCES users(nickname) ON DELETE CASCADE,
    UNIQUE(meetings_id, nickname)
);

CREATE TRIGGER attendees_changes_after_insert
AFTER INSERT ON attendees
BEGIN
    INSERT INTO attendees_changes (time, meetings_id, nickname)
    VALUES (CURRENT_TIMESTAMP, NEW.meetings_id, NEW.nickname)
    ON CONFLICT DO UPDATE SET time = CURRENT_TIMESTAMP;
END;

CREATE TRIGGER attendees_changes_after_update
AFTER UPDATE ON attendees
BEGIN
    INSERT INTO attendees_changes (time, meetings_id, nickname)
    VALUES (CURRENT_TIMESTAMP, NEW.meetings_id, NEW.nickname)
    ON CONFLICT DO UPDATE SET time = CURRENT_TIMESTAMP;
END;

CREATE TRIGGER attendees_changes_after_delete
AFTER DELETE ON attendees
BEGIN
    INSERT INTO attendees_changes (time, meetings_id, nickname)
    VALUES (CURRENT_TIMESTAMP, OLD.meetings_id, OLD.nickname)
    ON CONFLICT DO UPDATE SET time = CURRENT_TIMESTAMP;
END;
