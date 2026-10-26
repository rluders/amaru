# WP-M1-02 per-OS runbook

How to run the storage and execution spike natively on one operating system and produce the record that [validation](../../engineering/validation.md#cross-platform-validation-m3-and-m4) requires. Experiments: [spikes/wp-m1-02](../../../spikes/wp-m1-02/README.md). Report: [investigations](../../architecture/investigations/).

A run counts for an OS only if it executes **natively on that OS** on a physical machine or a full VM (WSL, Docker, Wine and cross-compilation do not count); a **non-admin** user is required for the Windows non-admin claim. GitHub-hosted runners are accepted (see below); this runbook is for a manual run, for example on a standard-user Windows 11 or a physical Mac. Before the first run, add the machine or VM (OS version, architecture, kind) to [issue #3](https://github.com/rluders/amaru/issues/3).

## Prerequisites
- The commit to test: the head of `feat/3-wp-m1-02-storage-and-execution` (record `git rev-parse HEAD`).
- Go 1.26.x on `PATH` (`go version`). Network is needed only for `spike bundle`.
- Windows: PowerShell; run as a **standard user** (not "Run as administrator"). macOS: Apple silicon.

## Steps
Run in `spikes/wp-m1-02`. Keep the terminal output; every command prints JSON lines or a final summary.

```sh
go build -o spike ./cmd/spike            # Windows: go build -o spike.exe ./cmd/spike
./spike bundle -out ./bundle              # build-time fetch of PostgreSQL 18.6.1 + supervisor
./spike pg -bundle ./bundle -state ./state-pg -out pg.jsonl
./spike nats | tee nats.json
./spike matrix -bundle ./bundle -work ./mx -out matrix.jsonl -fuzz 10
SPIKE_BUNDLE=$PWD/bundle go test -count=1 -v ./internal/... 2>&1 | tee gotest.log     # Windows: $env:SPIKE_BUNDLE="$PWD\bundle"
```

`spike matrix` writes one log per Platform run under `./mx/logs/`. Attach `pg.jsonl`, `nats.json`, `matrix.jsonl`, `gotest.log` and `mx/logs` (zipped) to the GitHub comment. Check the files for secrets before attaching (the spike writes none, and redacts the password in URLs).

## Extra checks per OS (record the output verbatim)
**All:** `whoami` (or `id`), and proof the user is not an administrator. `go version`, OS version (`sw_vers`, `systeminfo`/`winver`, `/etc/os-release`), `uname -m` / `$env:PROCESSOR_ARCHITECTURE`. Disk footprint: `du -sh bundle` (Windows: `(Get-ChildItem bundle -Recurse | Measure-Object Length -Sum).Sum`) and the compressed size of the archive you would ship (`tar -czf bundle.tgz bundle`).

**macOS (signed child binary).** Record each step's output and whether it worked without a Gatekeeper prompt:
1. Quarantine: `xattr -lr bundle | head`. A downloaded bundle carries `com.apple.quarantine`; the installer path must not. Say which case you ran.
2. Signature of the shipped children: `codesign -dvv bundle/supervisor` and `codesign -dvv bundle/pg/bin/postgres`; `codesign --verify --deep --strict bundle/pg/bin/postgres`. Say whether the binaries arrive signed upstream, ad-hoc signed or unsigned.
3. If unsigned: sign ad-hoc (`codesign --force --sign - <file>` for every Mach-O under `bundle`) or with a Developer ID, rerun `spike pg`, and record both outcomes. Notarization needs a Developer ID and is **not** part of this run unless you have one; say so.
4. Socket path length: print `./state-pg/s` path length from the `cold-start` line (macOS limit for `sun_path` is 104 bytes).

**Windows (non-admin, loopback + SCRAM).**
1. `whoami /groups | findstr /i "S-1-16-12288 S-1-16-8192"` (12288 = High integrity = elevated, 8192 = Medium) and the `env` line of `pg.jsonl` (`elevated`).
2. Antivirus: `Get-MpComputerStatus | Select AMRunningMode, RealTimeProtectionEnabled`. Run once with the default Defender settings; do not add exclusions.
3. The `cold-start` line must show `listen_addresses="127.0.0.1"` and `password_encryption=scram-sha-256`; that is PostgreSQL's own answer, and it is what the record cites. A Windows Firewall prompt during the run is a finding: record it.
4. The `nats` command deletes the JetStream store after a graceful stop: `remove_store_after_stop_error` must be empty; any value is a file-locking finding.

## Record to commit
One file per OS: `docs/validation/m1/<YYYY-MM-DD>-<os>-wp-m1-02.md` with: date, commit SHA, OS name/version/architecture, machine kind, tool versions (`go version`, PostgreSQL `version()` from `pg.jsonl`, `nats-server` v2.15.1, `embedded-postgres` v2.0.1), the exact commands, pass/fail counts per command, failing or skipped scenarios with the cause, who ran it. Then comment on [#3](https://github.com/rluders/amaru/issues/3) with the OS, the commit and a link, and tick the OS in the issue's checklist. A Linux record is [2026-10-26-linux-wp-m1-02.md](2026-10-26-linux-wp-m1-02.md).


## Hosted runs
The workflow [`spike-m1-02.yml`](../../../.github/workflows/spike-m1-02.yml) ran all of the above on GitHub-hosted runners; records: [macOS](2026-10-26-macos-wp-m1-02.md), [Windows](2026-10-26-windows-wp-m1-02.md), and a hosted-Linux section in the [Linux record](2026-10-26-linux-wp-m1-02.md). Still worth a manual run with this runbook: Windows 10/11 as a **standard user**, and macOS with signed/quarantined binaries from a downloaded installer.
