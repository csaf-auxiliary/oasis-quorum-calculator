# Hints for installing oqc

A few recommendations how to run oqc 
(writen for Debian GNU/Linux 12 Bookworm).
(**may not be fully tested**)

* Use a new user `adduser oqc`
* create directory like /opt/oqc`
* use caddy for `reverse_proxy unix//run/oqc/oqc_8083.sock`


(after initialisation of the database)

## Install to directory

* `bin/oqcd`
* `oqcd.sqlite`
* `oqcd.toml`
* `web/` <- this has the templates

## example system.service
To autostart it, use systemd with the user,
e.g. `/etc/systemd/system/oqc.service`:

```
[Unit]
Description=OQCd
After=network.target

[Service]
Type=idle
User=oqc
Group=oqc
RuntimeDirectory=oqc
WorkingDirectory=/opt/oqc
ExecStart=/opt/oqc/bin/oqcd -c /opt/oqc/oqcd.toml

[Install]
WantedBy=multi-user.target
```

(both caddy (by root) and oqc (by the user)
need to be enabled and started with systemctl)
