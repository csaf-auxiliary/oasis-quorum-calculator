<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2025 Intevation GmbH <https://intevation.de>
-->

# Build

To build OQC you need a Go compiler at least of version 1.24.1.
You can get it at [go.dev/dl](https://go.dev/dl/).

As OQC currently uses [SQLite 3](https://www.sqlite.org) as its
database backend which is written a C, you also need a working
`gcc` compiler on your build system. SQLite will be directly
compiled into the program.

To use `make` you also need that to be installed.

To build OQC you just need to call

```shell
make
```

The compiled binary can be found under `bin/oqcd`.
