<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2025 Intevation GmbH <https://intevation.de>
-->

# Export Meeting Tool

## Overview

The exportmeeting tool is a command-line application that extracts meeting attendance data from an SQLite database used
by the Quorum Calculator and writes it to a CSV file.

The tool supports exporting data for all meetings or for a specific committee.

## CSV Format

The generated CSV file is structured with:

- **First cell**: The start date of each meeting in `YYYY-MM-DD` format.

- **Following cells** contains attendees or metadata:
    - `(informational)` indicates gathering only, no decision-making
    - `Description: `... optional meeting note
    - `Stop-Time: `... specifies meeting ending
    - `Name (Leave of absence)` marks absence

For example:

| 2023-01-01 | 2023-02-01 | 2023-03-01 |
|------------|------------|------------|
| alice      | bob        | alice      |
| bob        | charlie    | bob        |
|            |            | charlie    |

In this example:

- Three meetings are shown.

- Each cell contains the nickname of an attendee if they attended that meeting.

## Command-Line Usage

```sh
./bin/exportmeeting -committee="TC 1" -meeting="meetings.csv" -database="oqcd.sqlite"
```

### Flags

| Flag         | Description                                          | Default            |
|--------------|------------------------------------------------------|--------------------|
| `-meeting`   | CSV file to write exported meeting data              | `meetings.csv`     |
| `-m`         | Shorthand for `-meeting`                             | `meetings.csv`     |
| `-committee` | Optional name of the committee to filter meetings by | *(all committees)* |
| `-database`  | SQLite database file                                 | `oqcd.sqlite`      |
| `-d`         | Shorthand for `-database`                            | `oqcd.sqlite`      |
