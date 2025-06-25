<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2025 Intevation GmbH <https://intevation.de>
-->

# oasis-quorum-calculator

:warning: **This is work in progress!**

A simple tool to calculate the quorum for OASIS TCs (definitely JavaScript-free)


See [here](./docs/build.md) how to build.

Create an initial config file and notices file:
```shell
cp docs/example-oqcd.toml oqcd.toml
cp docs/example-notices.tmpl web/templates/notices.tmpl
```

Do the initial database migration.
```shell
OQC_DB_MIGRATE=true ./bin/oqcd
```

Extract the password of `admin`. Use it to log in.
```shell
grep -oP 'user=admin.+password=\K[0-9a-zA-Z]+' oqcd.log
```

The sessions are signed with a key.
To have sessions that survive restaring oqcd
you need to store the signing secret into the config file.
```shell
sed -i -e "s|^#secret =.*|secret = \"$(grep -oP 'session key.+secret=\K[0-9a-f]+' oqcd.log)\"|" \
       -e 's|^#\[sessions\]|[sessions]|' oqcd.toml
```

Starting
```shell
./bin/oqcd
```
