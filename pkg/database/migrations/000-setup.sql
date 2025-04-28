-- This file is Free Software under the Apache-2.0 License
-- without warranty, see README.md and LICENSE for details.
--
-- SPDX-License-Identifier: Apache-2.0
--
-- SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
-- Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

CREATE TABLE versions (
    version     int       PRIMARY KEY,
    description text      NOT NULL,
    time        timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users (
    nickname  VARCHAR PRIMARY KEY,
    password  VARCHAR NOT NULL,
    firstname VARCHAR,
    lastname  VARCHAR,
    is_admin  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE sessions (
    token       VARCHAR   PRIMARY KEY,
    nickname    VARCHAR   NOT NULL REFERENCES users(nickname) ON DELETE CASCADE,
    last_access timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE committees (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        VARCHAR NOT NULL,
    description VARCHAR
);

CREATE TABLE committee_role (
    id          INTEGER PRIMARY KEY,
    name        VARCHAR NOT NULL UNIQUE,
    description VARCHAR NOT NULL
);

INSERT INTO committee_role (id, name, description) VALUES
    (0, 'member', 'Regular committee member'),
    (1, 'chair', 'Committee chair');

CREATE TABLE member_status (
    id          INTEGER PRIMARY KEY,
    name        VARCHAR NOT NULL UNIQUE,
    description VARCHAR NOT NULL
);

INSERT INTO member_status (id, name, description) VALUES
    (0, 'member', 'Regular committee member'),
    (1, 'voting', 'Voting member'),
    (2, 'nonevoting', 'Persistent none voting member'),
    (3, 'nomember', 'Not a member');

CREATE TABLE member_history (
    nickname      VARCHAR   NOT NULL,
    committees_id INTEGER   NOT NULL REFERENCES committees(id) ON DELETE CASCADE,
    status        INTEGER   NOT NULL DEFAULT 0 REFERENCES member_status(id) ON DELETE CASCADE,
    since         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(nickname, committees_id, since)
);

CREATE TABLE committee_roles (
    nickname          VARCHAR NOT NULL REFERENCES users(nickname)    ON DELETE CASCADE,
    committee_role_id INTEGER NOT NULL REFERENCES committee_role(id) ON DELETE CASCADE,
    committees_id     INTEGER NOT NULL REFERENCES committees(id)     ON DELETE CASCADE,
    UNIQUE(nickname, committee_role_id, committees_id)
);

CREATE TABLE meeting_status (
    id          INTEGER PRIMARY KEY,
    name        VARCHAR NOT NULL UNIQUE,
    description VARCHAR
);

INSERT INTO meeting_status (id, name, description) VALUES
    (0, 'onhold',  'Waiting to get started or paused'),
    (1, 'running', 'In progress'),
    (2, 'concluded', 'Finalized');

CREATE TABLE meetings (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    committees_id INTEGER   NOT NULL REFERENCES committees(id) ON DELETE CASCADE,
    gathering     BOOLEAN   NOT NULL DEFAULT FALSE,
    status        INTEGER   NOT NULL DEFAULT 0 REFERENCES meeting_status(id) ON DELETE CASCADE, -- on hold
    start_time    TIMESTAMP NOT NULL,
    stop_time     TIMESTAMP NOT NULL,
    description   VARCHAR,
    UNIQUE(committees_id, start_time),
    CHECK (strftime('%s', start_time) <= strftime('%s', stop_time))
);

CREATE TABLE attendees (
    meetings_id    INTEGER NOT NULL REFERENCES meetings(id)    ON DELETE CASCADE,
    nickname       VARCHAR NOT NULL REFERENCES users(nickname) ON DELETE CASCADE,
    voting_allowed BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(meetings_id, nickname)
);

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

INSERT INTO users (nickname, password, lastname, is_admin)
    VALUES ('admin', {{ generatePassword "admin" | sqlQuote }}, 'Administrator', true);
