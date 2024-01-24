# NilAway

[![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov]

> [!WARNING]  
> NilAway is currently under active development: false positives and breaking changes can happen. 
> We highly appreciate any feedback and contributions!

NilAway is a static analysis tool that seeks to help developers avoid nil panics in production by catching them at 
compile time rather than runtime. NilAway is similar to the standard
[nilness analyzer](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/nilness), however, it employs much more 
sophisticated and powerful static analysis techniques to track nil flows within a package as well _across_ packages, and
report errors providing users with the nilness flows for easier debugging.

NilAway enjoys three key properties that make it stand out:

* It is **fully-automated**: NilAway is equipped with an inference engine, making it require _no_ any additional 
information from the developers (e.g., annotations) besides standard Go code.

* It is **fast**: we have designed NilAway to be fast and scalable, making it suitable for large codebases. In our
measurements, we have observed less than 5% build-time overhead when NilAway is enabled. We are also constantly applying
optimizations to further reduce its footprint.

* It is **practical**: it does not prevent _all_ possible nil panics in your code, but it catches most of the potential
nil panics we have observed in production, allowing NilAway to maintain a good balance between usefulness and build-time 
overhead.

:star2: For more detailed technical discussion, please check our [Wiki][wiki], [Engineering Blog][blog], and paper (WIP).

## Running NilAway

NilAway is implemented using the standard [go/analysis][go-analysis], making it easy to integrate with existing analyzer
drivers (i.e., [golangci-lint][golangci-lint], [nogo][nogo], or [running as a standalone checker][singlechecker]).

> [!IMPORTANT]  
> Due to the sophistication of the analyses that NilAway does, NilAway caches its findings about a particular 
> package via the [Fact Mechanism][fact-mechanism] from the [go/analysis][go-analysis] framework. Therefore, it is 
> _highly_ recommended to leverage a driver that supports modular analysis (i.e., bazel/nogo or golangci-lint, but _not_
> the standalone checker since it stores all facts in memory) for better performance on large projects. For example,
> see [instructions][nogo-instructions] below for running NilAway on your project with bazel/nogo.

> [!IMPORTANT]  
> By default, NilAway analyzes _all_ Go code, including the standard libraries and dependencies. This helps NilAway 
> better understand the code form dependencies and reduce its false negatives. However, this would also incur a 
> significant performance cost (only once for drivers with modular support) and increase the number of non-actionable 
> errors in dependencies, for large Go projects with a lot of dependencies.
> 
> We highly recommend using the [include-pkgs][include-pkgs-flag] flag to narrow down the analysis to your project's 
> code exclusively. This directs NilAway to skip analyzing dependencies (e.g., third-party libraries), allowing you to 
> focus solely on potential nil panics reported by NilAway in your first-party code!

### Standalone Checker

Install the binary from source by running: 
```shell
go install go.uber.org/nilaway/cmd/nilaway@latest
```

Then, run the linter by:
```shell
nilaway -include-pkgs="<YOUR_PKG_PREFIX>,<YOUR_PKG_PREFIX_2>" ./...
```

### Bazel/nogo

Running with bazel/nogo requires slightly more efforts. First follow the instructions from [rules_go][rules-go], 
[gazelle][gazelle], and [nogo][nogo] to set up your Go project such that it can be built with bazel/nogo with no or 
default set of linters configured. Then,

(1) Add `import _ "go.uber.org/nilaway"` to your `tools.go` file (or other file that you use for configuring tool 
dependencies, see [How can I track tool dependencies for a module?][track-tool-dependencies] from Go Modules 
documentation) to avoid `go mod tidy` from removing NilAway as a tool dependency.

(2) Run the following commands to add NilAway as a tool dependency to your project:
```bash
# Get NilAway as a dependency, as well as getting its transitive dependencies in go.mod file.
$ go get go.uber.org/nilaway@latest
# This should not remove NilAway as a dependency in your go.mod file.
$ go mod tidy
# Run gazelle to sync dependencies from go.mod to WORKSPACE file.
$ bazel run //:gazelle -- update-repos -from_file=go.mod
```

(3) Add NilAway to nogo configurations (usually in top-level `BUILD.bazel` file):

```diff
nogo(
    name = "my_nogo",
    visibility = ["//visibility:public"],  # must have public visibility
    deps = [
+++     "@org_uber_go_nilaway//:go_default_library",
    ],
    config = "config.json",
)
```

(4) Run bazel build to see NilAway working (any nogo error will stop the bazel build, you can use the `--keep_going` 
flag to request bazel to build as much as possible):

```bash
$ bazel build --keep_going //...
```

(5) See [nogo documentation][nogo-configure-analyzers] on how to pass a configuration JSON to the nogo driver, and see 
our [wiki page][nogo-configure-nilaway] on how to pass configurations to NilAway.

### golangci-lint

NilAway, as its current form, still reports a fair number of false positives. This makes NilAway fail to be merged with 
[golangci-lint][golangci-lint] and be offered as a linter (see [PR#4045][pr-4045]). The alternatives are to:

(1) [build NilAway as a plugin to golangci-lint][nilaway-as-a-plugin]: the Go plugin system _requires_ that NilAway 
  shares the exact same versions of dependencies as the ones from golangci-lint. This is almost impossible for us to
  maintain.

(2) fork golangci-lint and add NilAway: it would also require us to keep in sync with upstream and cause unnecessary
  confusions to the users.

:raising_hand: We would love to integrate NilAway with golangci-lint! If you have any other ideas here, please raise an issue (or 
better, a PR)!

## Code Examples

Let's look at a few examples to see how NilAway can help prevent nil panics.

```go
// Example 1:
var p *P
if someCondition {
      p = &P{}
}
print(p.f) // nilness reports NO error here, but NilAway does.
```

In this example, the local variable `p` is only initialized when `someCondition` is true. At the field access `p.f`, a
panic may occur if `someCondition` is false. NilAway is able to catch this potential nil flow and reports the following
error showing this nilness flow:

```
go.uber.org/example.go:12:9: error: Potential nil panic detected. Observed nil flow from source to dereference point:
    - go.uber.org/example.go:12:9: unassigned variable `p` accessed field `f`
```

If we guard this dereference with a nilness check (`if p != nil`), the error goes away.

NilAway is also able to catch nil flows across functions. For example, consider the following code snippet:

```go
// Example 2:
func foo() *int {
      return nil
}
func bar() {
     print(*foo()) // nilness reports NO error here, but NilAway does.
}
```

In this example, the function `foo` returns a nil pointer, which is directly dereferenced in `bar`, resulting in a panic
whenever `bar` is called. NilAway is able to catch this potential nil flow and reports the following error, describing
the nilness flow across function boundaries: 

```
go.uber.org/example.go:23:13: error: Potential nil panic detected. Observed nil flow from source to dereference point:
    - go.uber.org/example.go:20:14: literal `nil` returned from `foo()` in position 0
    - go.uber.org/example.go:23:13: result 0 of `foo()` dereferenced
```

Note that in the above example, `foo` does not necessarily have to reside in the same package as `bar`. NilAway is able
to track nil flows across packages as well. Moreover, NilAway handles Go-specific language constructs such as receivers,
interfaces, type assertions, type switches, and more.

## Configurations

We expose a set of flags via the standard flag passing mechanism in [go/analysis](https://pkg.go.dev/golang.org/x/tools/go/analysis).
Please check [wiki/Configuration](https://github.com/uber-go/nilaway/wiki/Configuration) to see the available flags and
how to pass them using different linter drivers.

## Support 

We follow the same [version support policy](https://go.dev/doc/devel/release#policy) as the [Go](https://golang.org/) 
project: we support and test the last two major versions of Go.

Please feel free to [open a GitHub issue](https://github.com/uber-go/nilaway/issues) if you have any questions, bug 
reports, and feature requests.

## Contributions

We'd love for you to contribute to NilAway! Please note that once you create a pull request, you will be asked to sign 
our [Uber Contributor License Agreement](https://cla-assistant.io/uber-go/nilaway).

## License

This project is copyright 2023 Uber Technologies, Inc., and licensed under Apache 2.0.

[go-analysis]: https://pkg.go.dev/golang.org/x/tools/go/analysis
[golangci-lint]: https://github.com/golangci/golangci-lint
[singlechecker]: https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker
[nogo]: https://github.com/bazelbuild/rules_go/blob/master/go/nogo.rst
[doc-img]: https://pkg.go.dev/badge/go.uber.org/nilaway.svg
[doc]: https://pkg.go.dev/go.uber.org/nilaway
[ci-img]: https://github.com/uber-go/nilaway/actions/workflows/ci.yml/badge.svg
[ci]: https://github.com/uber-go/nilaway/actions/workflows/ci.yml
[cov-img]: https://codecov.io/gh/uber-go/nilaway/branch/main/graph/badge.svg
[cov]: https://codecov.io/gh/uber-go/nilaway
[wiki]: https://github.com/uber-go/nilaway/wiki
[blog]: https://www.uber.com/blog/nilaway-practical-nil-panic-detection-for-go/
[fact-mechanism]: https://pkg.go.dev/golang.org/x/tools/go/analysis#hdr-Modular_analysis_with_Facts
[include-pkgs-flag]: https://github.com/uber-go/nilaway/wiki/Configuration#include-pkgs
[pr-4045]: https://github.com/golangci/golangci-lint/issues/4045
[nilaway-as-a-plugin]: https://golangci-lint.run/contributing/new-linters/#how-to-add-a-private-linter-to-golangci-lint
[rules-go]: https://github.com/bazelbuild/rules_go
[gazelle]: https://github.com/bazelbuild/bazel-gazelle
[track-tool-dependencies]: https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
[nogo-configure-analyzers]: https://github.com/bazelbuild/rules_go/blob/master/go/nogo.rst#id14
[nogo-configure-nilaway]: https://github.com/uber-go/nilaway/wiki/Configuration#nogo
[nogo-instructions]: https://github.com/uber-go/nilaway?tab=readme-ov-file#bazelnogo
