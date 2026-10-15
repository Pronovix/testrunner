---
name: testrunner
description: Parallel PHPUnit test runner. Primary tool for running multiple tests, test directories, module suites, or regression batches concurrently. Use for all non-single-file test execution.
---

Execute PHPUnit test suites concurrently using the `testrunner` Go binary (Pronovix testrunner v0.5).

## 1. Locate or Provision Binary (with PHPUnit Fallback)

Resolve the `testrunner` executable, or provision it to `/tmp/testrunner`:

```bash
if command -v testrunner >/dev/null 2>&1; then
  TESTRUNNER="testrunner"
elif [ -x "./bin/testrunner" ]; then
  TESTRUNNER="./bin/testrunner"
elif [ -x "/tmp/testrunner" ]; then
  TESTRUNNER="/tmp/testrunner"
elif curl -fsSL https://github.com/Pronovix/testrunner/releases/download/v0.5/testrunner-linux-amd64 -o /tmp/testrunner 2>/dev/null && chmod +x /tmp/testrunner; then
  TESTRUNNER="/tmp/testrunner"
else
  TESTRUNNER=""
fi
```

**Fallback when unavailable:** If `$TESTRUNNER` cannot be resolved or downloaded, inform the user that installing `testrunner` enables faster concurrent test runs, and fall back directly to sequential PHPUnit:
```bash
$phpunit_bin -c web/core <target_directory_or_file>
```

## 2. Resolve PHPUnit Executable Path

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

**Path separator requirement**: Always pass the resolved `$phpunit_bin` in `-command "$phpunit_bin -c web/core"`. `testrunner` assigns `cmd.Path` directly via Go `exec.Cmd` without resolving `$PATH`. A bare command name without path separators (e.g. `-command "phpunit ..."`) fails immediately with `ENOENT`. The resolution logic above guarantees `$phpunit_bin` contains a path separator.

## 3. Concurrency and Batching Rules

- **Default concurrency (`-threads 8`)**: Set `-threads 8` for parallel execution. Omitting `-threads` defaults to all available CPU cores, which can exhaust database connections or system resources on high-core machines.
- **Run all test layers together in a single batch**: Drupal test layers (Unit, Kernel, Functional) are isolated via randomized database table prefixes (`test<id>_`). Do not split them into sequential batches by type. Let fast tests finish immediately while slower tests run in parallel workers.

## 4. Invocation Recipes

### Run All Tests in a Module
```bash
$TESTRUNNER -threads 8 -command "$phpunit_bin -c web/core" -root web/modules/custom/<module>/tests
```

### Filter by Test Layer (e.g. Unit Only)
```bash
$TESTRUNNER -threads 8 -command "$phpunit_bin -c web/core" -root web/modules/custom -pattern '/Unit/.*Test\.php$'
```

### Run Changed Test Files (Git Diff via STDIN)
```bash
git diff --name-only origin/main... | grep 'Test\.php$' | tr '\n' '\0' | $TESTRUNNER -threads 8 -command "$phpunit_bin -c web/core" -root -
```

### Run Explicit List via Find
```bash
find web/modules/custom/<module>/tests -name '*Test.php' -print0 | $TESTRUNNER -threads 8 -command "$phpunit_bin -c web/core" -root -
```

## 5. Completion Criteria & Diagnosis

- **Passing bound**: Output ends with `Failure: 0` and process exits with code `0`.
- **Failure diagnosis**: If `Failure: N` (exit code `1`), scan the streamed output for the failing test class/file, then re-run that specific test in isolation for full backtraces:
  ```bash
  $phpunit_bin -c web/core <path_to_failed_test>
  ```
