# oasis-quorum-calculator

:warning: **This is work in progress!**

A simple tool to calculate the quorum for OASIS TCs (definitely JavaScript-free)


```bash
go build -o ./oqcd ./...

OQC_DB_MIGRATE=true ./oqcd
tail oqcd.log

./oqcd &
sleep 1
tail -1 oqcd.log
```
