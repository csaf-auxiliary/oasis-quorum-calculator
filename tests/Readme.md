# Tests

The sqlite files in this folder contain different states of the application to
be able to test different scenarios.

## General entities

All db files contain a **committee named CSAF** and the following users:

| Login    | First- and lastname | PW (without whitespace) | Admin | Role in CSAF |
| -------- | ------------------- | ----------------------- | :---: | ------------ |
| admin    |                     | tujdlX61WhpQ            |   x   |
| florence | Florence Gordon     | W8oIKyWsHk0z            |       | `chair`      |
| mason    | Mason Shaw          | 0VeMpHOOlTMc            |       |
| konrad   | Konrad Haas         | XATzUHvpCMj7            |       |
| zehra    | Zehra Can           | vG5duUgqNBht            |       |

## Test cases

### 1. Initial assignment of role `chair` works

#### 1.1 Reproduce the problem

File: `1-main-no-roles.sqlite`

Steps:
- Checkout branch `main`
- Build OQC
- Start backend
- Sign in as admin
- Open user settings for `florence`
- Set member status `Voting Member` and role `Chair` for user `florence`
- Save

Result: `Chair` is not choosen.

Expected result: `Chair` should be selected.

#### 1.2. Test if the problem was solved das Problem durch die Änderungen beseitigt wurde

Same file and same steps as in 1.1.

Result: `Chair` is selected.

### 2. Migration

#### 2.1 User, roles and member history are preserved

File: `2-main-with-roles-and-statuses.sqlite`

Steps:
- Checkout branch `main`
- Build OQC and start the backend so it creates a new DB
- Stop the backend
- Checkout branch `add-review-step`
- Build OQC again
- Apply the migration

Result: The users should have the following statuses and roles:

| Login    | Status                 | Role    |
| -------- | ---------------------- | ------- |
| florence | `Voting`               | `chair` |
| mason    | `Permanent-non-voting` |         |
| konrad   | `Non-voting`           |         |
| zehra    | `Non-voting`           |         |

### 3. Status changes

#### 3.1 Review step with status changes

File: `3.1-one-meeting.sqlite`

Steps:
- Checkout branch `add-review-step`
- Build OQC and start the backend
- Sign in as `florence`
- Navigate to `/chair` and click on "Create meeting"
- Enter any valid value in the input `Duration`
- Click on "Create"
- Click on "Waiting"
- Click on "Run"
- Select the checkbox in the row with user `zehra`
- Click on "Mark as Attending"
- Click on "Conclude"

Exptected interim result: A page appears where the status "Review" is displayed. A table on this
page contains the columns "Previous Status" and "New Status". There is no difference between both
columns for any of the users. Only for user `zehra` there is a change because the previous
status is "Non-Voting Member" and the new status is "Voting-Member".

Next steps:
- Click on "Edit" in the row with user `konrad`
- Select the option "Voting Member" for user `konrad`
- Click on "Apply"
- Click on "Confirm"

Exptected interim result: The meeting overview appears. In the lower area is a section called
"Member status changes". Two entries are listed there - one for `zehra` and one for `konrad` -
which tell that these users have the new status "Voting Member".

Next steps:
- Click on "chair" in the header
- Click on "Meetings overview"

Exptected interim result: A new page appears and it also contains a table in the lower area where
"Member status changes" are listed. There are entries for the two previous meetings. Below the
date of the first meeting a hint "No changes" is located and below the date of the latest meeting
the status changes of `zehra` and `konrad` are shown.

Next steps:
- Click on "zehra" in the table at the top of this page

Exptected result: A new page appears with a "Member status history". In a table there are
two status changes. One from "Non-Voting-Member" without a value for Login (because this field
was added to the table in the database _after_ the migration) and one to the status "Voting
Member" which was caused by `florence`. The second entry also includes a link to the meeting
which led to the status change.

#### 3.2 Status change in the members overview of the committee

File: `3.2-two-meetings.sqlite`

Steps:
- Checkout branch `add-review-step`
- Build OQC and start the backend
- Sign in as `florence`
- Click on "chair" in the header
- Click on "Members overview"
- Click on "Edit" in the row with the nickname `mason`
- Set the status to "Non-Voting Member"
- Click on "Apply"

Exptected interim result: The select element disappeared and the status "Non-Voting Member" is
displayed as normal text in the table row.

Steps:
- Click on the nickname `mason`

Expected result: A table with two entries is shown. The latest entry contains the newest status
"Non-Voting Member".

### 4. Deactivate user

#### 4.1 Deactivate a normal user

File: `3.2-two-meetings.sqlite`

Steps:
- Checkout branch `add-review-step`
- Build OQC and start the backend
- Sign in as `admin`
- Click on "users" in the header
- Select the checkbox of `mason`
- Click on "Deactivate"

Exptected interim result: `mason` is not listed anymore.

Next steps:
- Sign out
- Sign in as `florence`
- Open the first meeting (2026-09-01 06:29 UTC)

Exptected interim result: `mason` is listed in the table with the attendees.

Next steps:
- Click on `chair` in the header
- Click on "Create meeting"
- Click on "Create"
- Click on "Waiting" in the list of meetings

Expected result: `mason` is not listed as an attendee.

#### 4.2 Deactivate chair

File: `3.2-two-meetings.sqlite`

Steps:
- Sign in as `admin`
- Click on "users" in the header
- Select the checkbox of `florence`
- Click on "Deactivate"

Exptected interim result: `florence` is not listed anymore.
- Sign out
- Sign in as `mason`
- Click on the first meeting

Result: `florence` is listed in the list of attendees.

### 5. Hint about meetings in review

#### 5.1 Hint about single meeting in review

File: `5.1-one-meeting-in-review.sqlite`

Steps:

- Sign in as `florence`
- Click on "chair" in the header
- Click on "Waiting"

Expected result: A message at the top explains that there is one meeting
still in review. There is also a link to this meeting.

#### 5.2 Hint about two meetings in review

File: `5.2-two-meetings-in-review.sqlite`

Steps:

- Sign in as `florence`
- Click on "chair" in the header
- Click on "Waiting"

Expected result: A message at the top explains that there are two meetings
still in review. There are also links to this meetings.
