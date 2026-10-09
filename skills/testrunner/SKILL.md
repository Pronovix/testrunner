---
name: testrunner
description: Run PHPUnit tests in parallel with testrunner. Use when executing multiple test files, verifying test suites across modules, or batching changed test files.
---

Execute PHPUnit test suites concurrently using the `testrunner` Go binary (Pronovix testrunner v0.5).

## 1. Locate or Provision the Binary

Resolve the `testrunner` binary:

1. Check if available on `$PATH` or at `/tmp/testrunner`:
   ```bash
   which testrunner || test -x /tmp/testrunner
   ```
2. If missing, download release artifact `v0.5` to `/tmp/testrunner` (or a directory on `$PATH`) and ensure executable permissions:
   ```bash
   curl -fsSL https://github.com/Pronovix/testrunner/releases/download/v0.5/testrunner-linux-amd64 -o /tmp/testrunner && chmod +x /tmp/testrunner
   ```

## 2. Resolve PHPUnit and Command Invocation Rule

Resolve the PHPUnit executable path using fast-path checks, authoritative Composer configuration, and system `$PATH`, failing fast if absent:
```bash
if [ -x "vendor/bin/phpunit" ]; then
  phpunit_bin="vendor/bin/phpunit"
elif [ -x "bin/phpunit" ]; then
  phpunit_bin="bin/phpunit"
elif command -v composer >/dev/null 2>&1 && [ -x "$(composer config bin-dir 2>/dev/null)/phpunit" ]; then
  phpunit_bin="$(composer config bin-dir)/phpunit"
elif command -v phpunit >/dev/null 2>&1; then
  phpunit_bin="$(command -v phpunit)"
else
  echo "Error: PHPUnit executable not found in project or PATH." >&2
  exit 1
fi
```

Always pass the resolved explicit path in `-command`:
```bash
testrunner_bin=$(which testrunner 2>/dev/null || (test -x /tmp/testrunner && echo "/tmp/testrunner"))
$testrunner_bin -command "$phpunit_bin -c web/core" ...
```
`testrunner` assigns `cmd.Path` directly via Go `exec.Cmd` without resolving `$PATH`. A bare command name without path separators (e.g. `-command "phpunit ..."`) fails immediately on execution.

## 3. Invocation Modes

### Mode A: Directory Walk
Recursively scan a directory matching the `-pattern` regex (defaults to `Test.php$`):

```bash
# Run unit tests of a specific module
$testrunner_bin -command "$phpunit_bin -c web/core" -root web/modules/custom/<module>/tests/src/Unit

# Run all unit tests across custom modules using a pattern filter
$testrunner_bin -command "$phpunit_bin -c web/core" -root web/modules/custom -pattern '/Unit/.*Test\.php$'
```

### Mode B: STDIN Streaming (Null-Delimited)
Pass explicit file lists via STDIN with `-root -`. Paths must be separated by null bytes (`\0`):

```bash
# Changed test files on current branch
git diff --name-only origin/main... | grep 'Test\.php$' | tr '\n' '\0' | $testrunner_bin -command "$phpunit_bin -c web/core" -root -

# Explicit find pipeline
find web/modules/custom/<module>/tests -name '*Test.php' -print0 | $testrunner_bin -command "$phpunit_bin -c web/core" -root -
```

## 4. Concurrency Bounds

Tune worker threads (`-threads <N>`) to the architectural test layer:

- **Unit tests**: Pure logic decoupled from database and services. Maximize concurrency (`-threads 16` or default CPU count).
- **Kernel tests**: Spawns isolated database table prefixes. Limit to `-threads 4` to `-threads 8` to avoid MariaDB connection limits.
- **Functional / FunctionalJavascript**: Real browser sessions. Keep low (`-threads 2` to `-threads 4`) or run single-file if Mink / WebDriver session clashes appear.

## 5. Completion Criteria & Diagnosis

- **Passing bound**: Output ends with `Failure: 0` and process exits with code `0`.
- **Failure diagnosis**: If `Failure: N` (exit code `1`), scan the streamed output above the summary to identify the failed test file, then re-run the specific test in isolation for full backtraces:
  ```bash
  $phpunit_bin -c web/core <path_to_failed_test>
  ```
