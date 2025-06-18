<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2025 Intevation GmbH <https://intevation.de>
-->

# Committee Import Tool

## Overview

The `importcommittee` is a command-line application that reads a CSV file containing committee membership and meeting
attendance data and imports it into th SQLite database used by the Quorum Calculator. This tool helps to import
historical meeting data.

### CSV Format

The CSV file should be structured as follows:

| Status    | Role          | Name    | 2023-01-01 | 2023-02-01 | ... |
|-----------|---------------|---------|------------|------------|-----|
| Voter     | Voting Member | alice   | alice      | alice      | ... |
| Non-voter | Member        | bob     |            | bob        | ... |
| Voter     | Chair         | charlie | charlie    |            | ... |

- The **first three columns** represent:
    - **Initial status**: `Voter` or `Non-voter`
    - **Role**: `Voting Member`, `Member`, `Chair`, `Secretary`
    - **Name**: The username of the member
- **Remaining columns** represents meetings:
    - Header is da date in `YYYY-MM-DD` format.
    - Each subsequent cell lists the name of a participant if they attended the meeting.

Hint: Libreoffice can sort columns: Select them, let it extend
the selection and use the menu entry
_Data_ -> _Sort_ and select _Direction Left to Right (sort columns)_.

## Command-Line Usage

```sh
./bin/importcommittee -committee="TC 1" -csv="committee.csv" -database="oqcd.sqlite"
```

### Flags

| Flag         | Description                                              | Default         |
|--------------|----------------------------------------------------------|-----------------|
| `-committee` | **(Required)** Name of the committee to import data into |                 |
| `-csv`       | CSV file containing committee and meetings               | `committee.csv` |
| `-database`  | SQLite database file                                     | `oqcd.sqlite`   |
| `-d`         | Shorthand for `-database`                                | `oqcd.sqlite`   |
