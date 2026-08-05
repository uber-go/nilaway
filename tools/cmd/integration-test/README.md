# NilAway Integration Tests

NilAway's unit tests use the [`analysistest`][analysistest] framework and the packages under
`testdata/src/go.uber.org` to exercise individual analysis behaviors. These tests are fast and
provide detailed coverage, but they do not fully represent how NilAway runs in a real project.

In particular, `analysistest` analyzes packages with its own test driver. Production drivers may
load packages, serialize and consume analysis facts, and report diagnostics differently. It is
also difficult for the unit-test framework to verify diagnostics reported for an upstream package
while analyzing a downstream package, which is important for testing NilAway's cross-package
inference.

The integration tests complement the unit tests by running NilAway through real drivers:

- the standalone NilAway binary; and
- a custom golangci-lint binary containing the NilAway module plugin.

This verifies the end-to-end driver setup, cross-package fact propagation, and diagnostic
reporting in addition to the analysis logic covered by the unit tests.

## How the tests work

The integration-test tool:

1. Collects the expected diagnostics from `// want` comments throughout
   `testdata/src/go.uber.org`.
2. Copies that corpus and its stubs into a temporary, buildable Go module.
3. Adds the small compatibility shims and driver configuration required to run the corpus with
   real Go build drivers.
4. Runs each supported driver over the complete module.
5. Compares every reported diagnostic with the expectations, including multiple diagnostics on
   the same source line.

Because the temporary module is generated from the same corpus used by the unit tests, new
default-configuration test cases automatically receive both unit-test and integration-test
coverage.

## Running the tests

Run the integration tests from the repository root:

```shell
make integration-test
```

[analysistest]: https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest
