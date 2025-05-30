-- This file is Free Software under the Apache-2.0 License
-- without warranty, see README.md and LICENSE for details.
--
-- SPDX-License-Identifier: Apache-2.0
--
-- SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
-- Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

CREATE TRIGGER delete_references_before_user
    BEFORE DELETE ON users
    FOR EACH ROW
BEGIN
    DELETE FROM attendees         WHERE nickname = OLD.nickname;
    DELETE FROM attendees_changes WHERE nickname = OLD.nickname;
    DELETE FROM committee_roles   WHERE nickname = OLD.nickname;
    DELETE FROM sessions          WHERE nickname = OLD.nickname;
    DELETE FROM member_absent     WHERE nickname = OLD.nickname;
END;
