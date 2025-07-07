# Configurations

## General Flags

#### `include-pkgs`

> Default `""` (all packages are included)

A comma-separated list of package prefixes to include for analysis.

By default all packages (including standard and 3rd party libraries) will be loaded by the driver and passed to the linters for analysis. This may not be desirable in practice: the errors reported on them cannot be fixed locally, and the overhead for analyzing them is pointless. Hence this flag can be set to include packages that have certain prefixes (e.g., `go.uber.org`), such that only first-party code is analyzed. If a package is excluded from analysis, default nilabilities will be assumed (e.g., functions will be assumed to always return nonnil pointers).

> [!WARNING]  
> Please note that even if a package is excluded from analysis, errors might still be reported on them if the nilness flows happens within the analyzed package, but it includes using a struct from the excluded package. For example, if the analyzed package constructs a struct from the excluded package, assigns a `nil` to a field, and then immediately dereferences the field. In this case, NilAway will report an error for this flow, but the location will be on the field (that resides in the excluded package). To only report errors on the desired packages, please use the error suppression mechanisms specified in the driver ([golangci-lint](https://golangci-lint.run/usage/false-positives/), [nogo](https://github.com/bazelbuild/rules_go/blob/master/go/nogo.rst)).

Added in v0.1.0

#### `exclude-pkgs`

> Default `""` (no packages are excluded)

A comma-separated list of package prefixes to exclude for analysis. This takes precedence over `include-pkgs`.

Added in v0.1.0

#### `exclude-file-docstrings`

> Default `""` (no files are excluded)

A comma-separated list of strings to search for in a file's docstrings to exclude a file from analysis.

Added in v0.1.0

#### `pretty-print`

> Default `false`

A boolean flag indicating if NilAway should pretty print the errors (with ANSI color codes to highlight different components in the errors).

Added in v0.1.0

#### `group-error-messages`

> Default `true`

A boolean flag indicating if NilAway should group similar error messages together.

Added in v0.1.0

#### `experimental-struct-init`

> Default `false`

A boolean flag enabling experimental support in NilAway for more sophisticated tracking / analysis around struct fields and struct initializations (e.g., initializing an empty struct and then calling a function to populate the fields, such that the fields should be considered nonnil). Note that this feature brings performance penalties and false positives. We plan to further improve this feature and merge it in main NilAway, so this flag could be removed in the future.

Added in v0.1.0

#### `experimental-anonymous-function`

> Default `false`

A boolean flag enabling experimental support for anonymous functions in NilAway. Note that this feature brings a fair number of false positives. We plan to further improve this feature and merge it in main NilAway, so this flag could be removed in the future.

Added in v0.1.0

## Driver-Specific Flags

#### Standalone Checker

Since the standalone linter driver does not support error suppression, which may be required for some deployments (see [Include Package](https://github.com/uber-go/nilaway/wiki/Configuration/#include-packages-include-pkgs) for a detailed discussion), we add two more flags to this specific checker for such functionality:

#### `include-errors-in-files`

> Default is the current working directory, meaning we only report errors for files within the current working directory.

A comma-separated list of file prefixes to report errors.

Added in v0.1.0

#### `exclude-errors-in-files`

> Default "", meaning no errors are suppressed.

A comma-separated list of file prefixes to exclude from error reporting. This takes precedence over include-errors-in-files.

Added in v0.1.0

#### `json`

> Default `false`

A boolean flag indicating if the NilAway errors should be printed in JSON format for further post-processing.

Added in v0.1.0

## Passing Configurations

### Standalone Checker

Flags can be specified directly on the command line:

```
nilaway -flag1 <VALUE> -flag2 <VALUE> ./...
```

### golangci-lint

Flags can be specified in `.golangci.yaml` configuration file:

```yaml
linters-settings:
  custom:
    nilaway:
      type: "module"
      description: Static analysis tool to detect potential nil panics in Go code.
      settings:
        # Settings must be a "map from string to string" to mimic command line flags: the keys are
        # flag names and the values are the values to the particular flags.
        include-pkgs: "<YOUR_PACKAGE_PREFIXES>"
# NilAway can be referred to as `nilaway` just like any other golangci-lint analyzers in other
# parts of the configuration file.
```

### Bazel/nogo

Configurations of NilAway is slightly confusing: the flags should be passed to a specific `nilaway_config` analyzer, and error suppressions should be passed to the top-level `nilaway` analyzer instead.

<details>

<summary> Here is the technical explanation </summary>

> NilAway adopts the standard flag passing mechanism in the [`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis) framework. However, the multi-analyzer architecture of NilAway (see [wiki/Architecture](https://github.com/uber-go/nilaway/wiki/Architecture)) makes it slightly difficult to pass flags to NilAway. If we simply set the `Flags` fields to the top-level `nilaway.Analyzer`, by the time it is executed, the sub-analyzers have already done the work, making the flags meaningless (i.e., it can no long alter their behaviors).
>
> To address this problem, we have defined a separate `nilaway_config` analyzer which is only responsible for defining the `Flags` field and expose the configs through its return value. All other sub-analyzers will depend on the `config.Analyzer` and use the values there to execute different logic.
>
> This inevitably makes the configurations for NilAway slightly confusing: you will have to pass flags to the separate `nilaway_config` analyzer, while any error suppressions must be set for the top-level `nilaway` analyzer (since it is the one that eventually reports the errors).
>
> For drivers other than Bazel/nogo, we usually have a chance to run extra logic (e.g., during standalone driver initialiation, or during NilAway registration to golangci-lint) before the execution, where we can pass the top-level flags to the specific `nilaway_config` analyzer. However, unfortunately Bazel/nogo does not allow us to do so.

</details>

In your nogo's `config.json` file (see [nogo's documentation on configurations](https://github.com/bazelbuild/rules_go/blob/master/go/nogo.rst#configuring-analyzers) on how to pass this file to the nogo driver), you have to specify configurations for two analyzers. For example:

```json
{
  "nilaway_config": {
    "analyzer_flags": {
      "include-pkgs": "go.uber.org",
      "exclude-pkgs": "vendor/",
      "exclude-file-docstrings": "@generated,Code generated by,Autogenerated by"
    }
  },
  "nilaway": {
    "exclude_files": {
      "bazel-out": "this prevents nilaway from outputting diagnostics on intermediate test files"
    },
    "only_files": {
      "my/code/path": "This is the comment for why we want to enable NilAway on this code path"
    }
  }
}
```
