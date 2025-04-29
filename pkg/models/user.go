// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package models

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"
	"time"

	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/database"
	"github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/misc"
)

// Role is the role in the committee.
type Role int

const (
	// ChairRole is the manager role.
	ChairRole Role = iota
	// MemberRole is the member role.
	MemberRole
)

// MemberStatus is the status of a member in a committee.
type MemberStatus int

const (
	// Member has no voting rights.
	Member MemberStatus = iota
	// Voting is member with voting rights.
	Voting
	// NoneVoting is a persistent none voter.
	NoneVoting
	// NoMember represents that the person is not a member any more.
	NoMember
)

// Membership is the membership of a user in a committee.
type Membership struct {
	Committee *Committee
	Status    MemberStatus
	Roles     []Role
}

// User is the from the database.
type User struct {
	Nickname    string
	Firstname   *string
	Lastname    *string
	IsAdmin     bool
	Memberships []*Membership
	Password    *string
}

// UserHistoryEntry is a point in time after this status applys.
type UserHistoryEntry struct {
	Since  time.Time
	Status MemberStatus
}

// UserHistory is a list of status values over time.
// Assumed to be sorted in increasing over the Since values.
type UserHistory []*UserHistoryEntry

// UsersHistories is a map of user names to their histories.
type UsersHistories map[string]UserHistory

// ParseRole parses a role from a string.
func ParseRole(s string) (Role, error) {
	switch strings.ToLower(s) {
	case "chair":
		return ChairRole, nil
	case "member":
		return MemberRole, nil
	default:
		return 0, fmt.Errorf("invalid role %q", s)
	}
}

// String implements [fmt.Stringer].
func (r Role) String() string {
	switch r {
	case ChairRole:
		return "manager"
	case MemberRole:
		return "member"
	default:
		return fmt.Sprintf("unknown role (%d)", r)
	}
}

// ParseMemberStatus parses a member status from a string.
func ParseMemberStatus(s string) (MemberStatus, error) {
	switch strings.ToLower(s) {
	case "member":
		return Member, nil
	case "voting":
		return Voting, nil
	case "nonevoting": // "nonvoting"
		return NoneVoting, nil
	case "nomember":
		return NoMember, nil
	default:
		return 0, fmt.Errorf("invalid member status %q", s)
	}
}

// String implements [fmt.Stringer].
func (ms MemberStatus) String() string {
	switch ms {
	case Member:
		return "member"
	case Voting:
		return "voting"
	case NoneVoting:
		return "nonevoting"
	default:
		return fmt.Sprintf("unknown member status (%d)", ms)
	}
}

// Compare compares this user with the other by its
// firstname, lastname and nickname.
func (u *User) Compare(o *User) int {
	return cmp.Or(
		misc.CompareEmptyStrings(u.Firstname, o.Firstname),
		misc.CompareEmptyStrings(u.Lastname, o.Lastname),
		strings.Compare(u.Nickname, o.Nickname),
	)
}

// CommitteeByID return the committee for a given id.
func (u *User) CommitteeByID(id int64) *Committee {
	if ms := u.FindMembershipCriterion(MembershipByID(id)); ms != nil {
		return ms.Committee
	}
	return nil
}

// IsMember returns true if user is member of a committee with a given name.
func (u *User) IsMember(committeeName string) bool {
	return u.FindMembership(committeeName) != nil
}

// MembershipByName matches a membership by the committee name.
func MembershipByName(name string) func(*Membership) bool {
	return func(m *Membership) bool {
		return m.Committee.Name == name
	}
}

// MembershipByID matches a membership by the committee id.
func MembershipByID(id int64) func(*Membership) bool {
	return func(m *Membership) bool {
		return m.Committee.ID == id
	}
}

// FindMembershipCriterion finds a membership by a given criterion.
func (u *User) FindMembershipCriterion(crit func(*Membership) bool) *Membership {
	if idx := slices.IndexFunc(u.Memberships, crit); idx != -1 {
		return u.Memberships[idx]
	}
	return nil
}

// FindMembership looks up a membership of a user by name.
func (u *User) FindMembership(committeeName string) *Membership {
	return u.FindMembershipCriterion(MembershipByName(committeeName))
}

// HasRole checks if a membership contains a certain role.
func (m *Membership) HasRole(role Role) bool {
	return m != nil && slices.Contains(m.Roles, role)
}

// HasAnyRole checks if a membership contain any of the given roles.
func (m *Membership) HasAnyRole(roles ...Role) bool {
	return m != nil && slices.ContainsFunc(m.Roles, func(r Role) bool {
		return slices.Contains(roles, r)
	})
}

// GetCommittee returns the committee of this membership.
func (m *Membership) GetCommittee() *Committee {
	return m.Committee
}

// CountMemberships count the memberships with a given role.
func (u *User) CountMemberships(role Role) int {
	count := 0
	for _, m := range u.Memberships {
		if m.HasRole(role) {
			count++
		}
	}
	return count
}

// CommitteesWithRole returns a sequence of Committees
// in which the user has the given role.
func (u *User) CommitteesWithRole(role Role) iter.Seq[*Committee] {
	return misc.Map(
		misc.Filter(slices.Values(u.Memberships),
			func(m *Membership) bool { return m.HasRole(role) }),
		(*Membership).GetCommittee)
}

// Committees returns an iterator over the committees of the user.
func (u *User) Committees() iter.Seq[*Committee] {
	return misc.Map(slices.Values(u.Memberships), (*Membership).GetCommittee)
}

// Status member returns the status of the user at a given time.
func (uh UserHistory) Status(when time.Time) MemberStatus {
	if len(uh) == 0 {
		return NoMember
	}
	target := &UserHistoryEntry{Since: when}
	idx, found := slices.BinarySearchFunc(uh, target, func(a, b *UserHistoryEntry) int {
		return a.Since.Compare(b.Since)
	})
	switch {
	case found:
		return uh[idx].Status
	case idx == 0:
		return NoMember
	case idx == len(uh):
		return uh[len(uh)-1].Status
	default:
		return uh[idx-1].Status
	}
}

// LoadUser loads a user with a given nickname from the database.
func LoadUser(ctx context.Context, db *database.Database, nickname string) (*User, error) {
	tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return loadUserTx(ctx, tx, nickname)
}

func loadBasicUserTx(
	ctx context.Context,
	tx *sql.Tx,
	nickname string,
) (*User, error) {
	// Collect user details
	user := User{Nickname: nickname}
	const userSQL = `SELECT firstname, lastname, is_admin ` +
		`FROM users ` +
		`WHERE nickname = ?`

	switch err := tx.QueryRowContext(ctx, userSQL, nickname).Scan(
		&user.Firstname,
		&user.Lastname,
		&user.IsAdmin,
	); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("loading user failed: %w", err)
	}
	return &user, nil
}

func loadUserTx(
	ctx context.Context,
	tx *sql.Tx,
	nickname string,
) (*User, error) {
	user, err := loadBasicUserTx(ctx, tx, nickname)
	if err != nil || user == nil {
		return user, err
	}

	// Collect memberships
	const committeeRolesSQL = `SELECT committee_role_id, committees_id, name, description ` +
		`FROM committee_roles JOIN committees ` +
		`ON committee_roles.committees_id = committees.id ` +
		`WHERE nickname = ? ` +
		`ORDER BY committees_id, committee_role_id`

	rows, err := tx.QueryContext(ctx, committeeRolesSQL, nickname)
	if err != nil {
		return nil, err
	}
	if err := func() error {
		defer rows.Close()
		for rows.Next() {
			var (
				cid         int64
				rid         int
				name        string
				description *string
			)
			if err := rows.Scan(&rid, &cid, &name, &description); err != nil {
				return err
			}
			if n := len(user.Memberships); n == 0 || user.Memberships[n-1].Committee.ID != cid {
				user.Memberships = append(user.Memberships, &Membership{
					Committee: &Committee{
						ID:          cid,
						Name:        name,
						Description: description,
					},
				})
			}
			ms := user.Memberships[len(user.Memberships)-1]
			ms.Roles = append(ms.Roles, Role(rid))
		}
		return rows.Err()
	}(); err != nil {
		return nil, err
	}

	// Collect member status in comittees.
	if len(user.Memberships) > 0 {
		const memberStatusSQL = `SELECT status FROM member_history ` +
			`WHERE nickname = ? AND committees_id = ? ` +
			`ORDER BY unixepoch(since) DESC LIMIT 1`
		stmt, err := tx.PrepareContext(ctx, memberStatusSQL)
		if err != nil {
			return nil, fmt.Errorf("preparing status failed: %w", err)
		}
		defer stmt.Close()
		for _, ms := range user.Memberships {
			switch err := stmt.QueryRowContext(
				ctx, user.Nickname, ms.Committee.ID).Scan(&ms.Status); {
			case errors.Is(err, sql.ErrNoRows):
				// default to member,
				ms.Status = Member
			case err != nil:
				return nil, fmt.Errorf("querying member status failed: %w", err)
			}
		}
	}

	return user, nil
}

// Store updates user in the database.
func (u *User) Store(ctx context.Context, db *database.Database) error {
	var sets []string
	var args []any
	add := func(s string, arg any) {
		sets = append(sets, fmt.Sprintf("%s=?", s))
		args = append(args, arg)
	}
	add("firstname", u.Firstname)
	add("lastname", u.Lastname)
	if u.Password != nil {
		encoded := misc.EncodePassword(*u.Password)
		add("password", encoded)
	}
	args = append(args, u.Nickname)
	updates := strings.Join(sets, ",")
	const storeSQL = `UPDATE users SET %s WHERE nickname=?`
	sql := fmt.Sprintf(storeSQL, updates)
	if _, err := db.DB.ExecContext(ctx, sql, args...); err != nil {
		return fmt.Errorf("storing user failed: %w", err)
	}
	return nil
}

// LoadAllUsers loads all user ordered by their nickname.
func LoadAllUsers(ctx context.Context, db *database.Database) ([]*User, error) {
	var users []*User
	const loadSQL = `SELECT nickname, firstname, lastname, is_admin FROM users ` +
		`ORDER BY nickname`
	rows, err := db.DB.QueryContext(ctx, loadSQL)
	if err != nil {
		return nil, fmt.Errorf("loading users failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.Nickname,
			&user.Firstname,
			&user.Lastname,
			&user.IsAdmin,
		); err != nil {
			return nil, fmt.Errorf("scanning users failed: %w", err)
		}
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loading users failed: %w", err)
	}
	return users, nil
}

// DeleteUsersByNickname deletes users by their nicknames.
func DeleteUsersByNickname(
	ctx context.Context,
	db *database.Database,
	nicknames iter.Seq[string],
) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()
	const deleteSQL = `DELETE FROM users WHERE nickname = ?`
	for nickname := range nicknames {
		if _, err := tx.ExecContext(ctx, deleteSQL, nickname); err != nil {
			return fmt.Errorf("deleting users failed: %w", err)
		}
	}
	return tx.Commit()
}

// StoreNew stores the user with a given password into the database.
// Returns false if the user already exists.
func (u *User) StoreNew(ctx context.Context, db *database.Database, password string) (bool, error) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var exists bool
	const userExistsSQL = `SELECT EXISTS(SELECT 1 FROM users WHERE nickname = ?)`
	if err := tx.QueryRowContext(ctx, userExistsSQL, u.Nickname).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking user existance failed: %w", err)
	}
	if exists {
		return false, nil
	}
	encoded := misc.EncodePassword(password)
	const insertSQL = `INSERT INTO users (nickname, firstname, lastname, is_admin, password) ` +
		`VALUES (?, ?, ?, ?, ?)`
	if _, err := tx.ExecContext(
		ctx, insertSQL,
		u.Nickname, u.Firstname, u.Lastname, u.IsAdmin, encoded); err != nil {
		return false, fmt.Errorf("inserting user failed: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("storing new user failed: %w", err)
	}
	return true, nil
}

// UpdateMemberships updates the memberships of the user with a given nickname.
func UpdateMemberships(
	ctx context.Context,
	db *database.Database,
	nickname string,
	memberships iter.Seq[*Membership],
) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const deleteSQL = `DELETE FROM committee_roles WHERE nickname = ?`
	if _, err := tx.ExecContext(ctx, deleteSQL, nickname); err != nil {
		return fmt.Errorf("deleting committee roles failed: %w", err)
	}

	const (
		insertRoleSQL = `INSERT INTO committee_roles ` +
			`(nickname, committees_id, committee_role_id) ` +
			`VALUES (?, ?, ?)`
		queryStatusSQL = `SELECT status FROM member_history ` +
			`WHERE nickname = ? AND committees_id = ? ` +
			`ORDER BY unixepoch(since) DESC LIMIT 1`
		insertStatusSQL = `INSERT INTO member_history ` +
			`(nickname, committees_id, status, since) ` +
			`VALUES (?, ?, ?, ?)`
	)
	var insertRoleStmt, queryStatusStmt, insertStatusStmt *sql.Stmt

	for _, s := range []struct {
		query string
		stmt  **sql.Stmt
	}{
		{insertRoleSQL, &insertRoleStmt},
		{queryStatusSQL, &queryStatusStmt},
		{insertStatusSQL, &insertStatusStmt},
	} {
		stmt, err := tx.PrepareContext(ctx, s.query)
		if err != nil {
			return fmt.Errorf("preparing %q failed: %w", s.query, err)
		}
		*s.stmt = stmt
		defer stmt.Close()
	}

	now := time.Now().UTC()
	for ms := range memberships {
		for _, r := range ms.Roles {
			if _, err := insertRoleStmt.ExecContext(
				ctx, nickname, ms.Committee.ID, r); err != nil {
				return fmt.Errorf("inserting into committee roles failed: %w", err)
			}
		}
		if !ms.HasRole(MemberRole) {
			continue
		}
		var status MemberStatus
		switch err := queryStatusStmt.QueryRowContext(
			ctx, nickname, ms.Committee.ID).Scan(&status); {
		case errors.Is(err, sql.ErrNoRows):
			status = MemberStatus(^0) // Invalid value to force insert.
		case err != nil:
			return fmt.Errorf("querying status failed: %w", err)
		}
		// Only insert new one if it differs from the previous.
		if status != ms.Status {
			if _, err := insertStatusStmt.ExecContext(
				ctx, nickname, ms.Committee.ID, ms.Status, now); err != nil {
				return fmt.Errorf("inserting status failed: %w", err)
			}
		}
	}
	return tx.Commit()
}

// LoadCommitteeUsers loads all users of a committee.
func LoadCommitteeUsers(
	ctx context.Context,
	db *database.Database,
	committeeID int64,
) ([]*User, error) {
	tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return LoadCommitteeUsersTx(ctx, tx, committeeID)
}

// LoadCommitteeUsersTx loads all users of a committee.
func LoadCommitteeUsersTx(
	ctx context.Context,
	tx *sql.Tx,
	committeeID int64,
) ([]*User, error) {
	// Load nicknames.
	const committeeUsersSQL = `SELECT distinct(nickname) FROM committee_roles ` +
		`WHERE committees_id = ? ` +
		`ORDER BY nickname`
	rows, err := tx.QueryContext(ctx, committeeUsersSQL, committeeID)
	if err != nil {
		return nil, fmt.Errorf("querying committee users failed: %w", err)
	}
	var nicknames []string
	if err := func() error {
		defer rows.Close()
		for rows.Next() {
			var nickname string
			if err := rows.Scan(&nickname); err != nil {
				return err
			}
			nicknames = append(nicknames, nickname)
		}
		return rows.Err()
	}(); err != nil {
		return nil, fmt.Errorf("scanning committee users failed: %w", err)
	}
	// Load users.
	users := make([]*User, 0, len(nicknames))
	for _, nickname := range nicknames {
		user, err := loadUserTx(ctx, tx, nickname)
		if err != nil {
			return nil, fmt.Errorf("loading user failed: %w", err)
		}
		users = append(users, user)
	}
	return users, nil
}

// UserMemberStatusSinceTx figures out the member status
// for a given user in a committee after a given point in time.
// Returns false the user was not in the committee at this time.
func UserMemberStatusSinceTx(
	ctx context.Context,
	tx *sql.Tx,
	nickname string, committeeID int64,
	when time.Time,
) (MemberStatus, bool, error) {
	var status MemberStatus
	const statusSQL = `SELECT status FROM member_history ` +
		`WHERE nickname = ? AND committees_id = ? AND unixepoch(since) <= unixepoch(?) ` +
		`ORDER BY unixepoch(since) DESC LIMIT 1`
	switch err := tx.QueryRowContext(ctx, statusSQL, nickname, committeeID, when).Scan(&status); {
	case errors.Is(err, sql.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, fmt.Errorf("fetching member status failed: %w", err)
	}
	return status, true, nil
}

// UpdateUserCommitteeStatusTx updates the status history of
// a sequence of users in a committee.
func UpdateUserCommitteeStatusTx(
	ctx context.Context,
	tx *sql.Tx,
	users iter.Seq2[string, MemberStatus],
	committeeID int64,
	since time.Time,
) error {
	const (
		queryLastSQL = `SELECT status FROM member_history ` +
			`WHERE nickname = ? AND committees_id = ? ` +
			`ORDER by unixepoch(since) DESC LIMIT 1`
		insertSQL = `INSERT INTO member_history ` +
			`(nickname, committees_id, status, since) ` +
			`VALUES(?, ?, ?, ?)`
	)
	qStmt, err := tx.PrepareContext(ctx, queryLastSQL)
	if err != nil {
		return fmt.Errorf("preparing user committee status query failed: %w", err)
	}
	defer qStmt.Close()
	iStmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("preparing user committee status insert failed: %w", err)
	}
	defer iStmt.Close()
	for nickname, status := range users {
		var prev MemberStatus
		switch err := qStmt.QueryRowContext(ctx, nickname, committeeID).Scan(&prev); {
		case errors.Is(err, sql.ErrNoRows):
			//	No previous -> insert.
		case err != nil:
			return fmt.Errorf("fetching previous member status failed: %w", err)
		default:
			if prev == status {
				continue
			}
		}
		if _, err := iStmt.ExecContext(
			ctx, nickname, committeeID, status, since); err != nil {
			return fmt.Errorf("inserting member status failed: %w", err)
		}
	}
	return nil
}

// LoadUsersHistoriesTx loads the histories of the users of a committee.
func LoadUsersHistoriesTx(
	ctx context.Context,
	tx *sql.Tx,
	committeeID int64,
) (UsersHistories, error) {
	const loadHistorySQL = `SELECT nickname, status, since FROM member_history ` +
		`WHERE committees_id = ? ` +
		`ORDER BY nickname, unixepoch(since)`
	rows, err := tx.QueryContext(ctx, loadHistorySQL, committeeID)
	if err != nil {
		return nil, fmt.Errorf("querying user histories failed: %w", err)
	}
	defer rows.Close()
	userHistories := make(UsersHistories)
	for rows.Next() {
		var entry UserHistoryEntry
		var nickname string
		if err := rows.Scan(&nickname, &entry.Status, &entry.Since); err != nil {
			return nil, fmt.Errorf("scanning user histories failed: %w", err)
		}
		userHistories[nickname] = append(userHistories[nickname], &entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("querying user histories failed: %w", err)
	}
	return userHistories, nil
}
