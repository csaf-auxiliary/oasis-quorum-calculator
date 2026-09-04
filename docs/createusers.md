<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2025 Intevation GmbH <https://intevation.de>
-->

# Bulk User Creation Tool

## Overview

This reads a list from users from a CSV file and inserts them
into the SQLite database used by the quorum calculator.
If a user does not exist in the database, they are added
with a randomly generated password. The tool also creates a
separate CSV file storing the mapping of nicknames to their generated passwords.

## CSV Format

```csv
nickname,first name,last name,admin,committee,chair,member,status
anton,Anton,Amann,true,"TC 1",false,true,voting
```

### Field Descriptions

| Field        | Required | Type    | Description                                               |
|--------------|----------|---------|-----------------------------------------------------------|
| `nickname`   | ✅        | string  | Unique identifier for the user. (email address by convention)                           | 
| `first name` | ✅        | string  | First name of the user.                                   |
| `last name`  | ✅        | string  | Last name of the user.                                    |
| `admin`      | ✅        | boolean | Whether the user is an administrator (`true`/`false`).    |
| `committee`  | ✅        | string  | Committee name (not currently used, reserved for future). |
| `chair`      | Optional | boolean | Whether the user is a chair (`true`/`false`).             |
| `member`     | Optional | boolean | Whether the user is a committee member (`true`/`false`).  |
| `status`     | Optional | string  | One of: `member`, `voting`, `nonevoting`, `nomember`.     |

## Command-Line Usage

```sh
./bin/createusers -users=users.csv -passwords=passwords.csv -database=oqcd.sqlite
```

### Flags

| Flag         | Shorthand | Description                                         | Default         |
|--------------|-----------|-----------------------------------------------------|-----------------|
| `-users`     | `-u`      | Path to the CSV file containing users to import.    | `users.csv`     |
| `-passwords` | `-p`      | Output path for CSV containing usernames/passwords. | `passwords.csv` |
| `-database`  | `-d`      | SQLite database file path.                          | `oqcd.sqlite`   |

### Password File

A CSV file will be generated with each newly created user's nickname
and their generated password, for example:

```csv
"anton","8gTf93kL2qWZ"
"brenda","9xZqY8NuPw1T"
```

### Test data
Run this python code to create a file `testusers.csv`:
```python3
with open("testusers.csv","wt") as f:
   for no in range(1,31):
        _ = f.write(f"user{no}@example.org,User,Name{no},false,,,,\n")
```
