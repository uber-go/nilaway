# NilAway

[![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov]

> [!WARNING]  
> NilAway is currently under active development: false positives and breaking changes can happen. 
> We highly appreciate any feedback and contributions!

## 🚀 Quick start

Install the binary from source by running:
```shell
go install go.uber.org/nilaway/cmd/nilaway@latest
```

Then, run the linter by:
```shell
nilaway ./...
```

## 🧠 Philosophy
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
* Read more about NilAway in [Uber Engineering blog](https://www.uber.com/en-NL/blog/nilaway-practical-nil-panic-detection-for-go/)

NilAway is implemented using the standard [go/analysis](https://pkg.go.dev/golang.org/x/tools/go/analysis) framework, 
making it easy to integrate with existing analyzer drivers (e.g., [golangci-lint](https://github.com/golangci/golangci-lint),
[nogo](https://github.com/bazelbuild/rules_go/blob/master/go/nogo.rst), or 
[running as a standalone checker](https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker)). Here, we list the 
instructions for running NilAway as a standalone checker. More integration supports will be added soon.

## 🏁 NilAway in Action

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
    -> go.uber.org/example.go:12:9: unassigned variable `p` accessed field `f`
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
    -> go.uber.org/example.go:20:14: literal `nil` returned from `foo()` in position 0
    -> go.uber.org/example.go:23:13: result 0 of `foo()` dereferenced
```

Note that in the above example, `foo` does not necessarily have to reside in the same package as `bar`. NilAway is able
to track nil flows across packages as well. Moreover, NilAway handles Go-specific language constructs such as receivers,
interfaces, type assertions, type switches, and more. For more detailed discussion, please check our paper.

## ⚙️ Configurations

We expose a set of flags via the standard flag passing mechanism in [go/analysis](https://pkg.go.dev/golang.org/x/tools/go/analysis).
Please check [wiki/Configuration](https://github.com/uber-go/nilaway/wiki/Configuration) to see the available flags and
how to pass them using different linter drivers.

## 🛫 Integrate in CI pipline
NilAway is deployed centrally in the Go monorepo, integrating tightly with the Bazel+Nogo framework, allowing it to run as a default linter on every build in the CI pipeline and local builds. The error reporting is, however, in the testing phase, where nil panic errors are only reported for services in the Go monorepo that are onboarded onto NilAway.

For service owners, we currently offer two options of error reporting: (1) comprehensive and blocking, and (2) stop-the-bleed and non-blocking.

In the first option, NilAway causes the build to fail, if any errors are found (suppressions are possible if needed, through //nolint:nilaway). NilAway comprehensively reports errors on all code, existing and new. This option is preferable to ensure a nil panic free codebase. However, it requires all reported nil panics in the service’s code to be addressed, before any build can be allowed to pass. This may incur a high upfront cost for the service’s development, which can cause friction among service owners.

To address the above problem, we offer a lightweight version in Option 2, in which we only report NilAway errors for changed code in the service. These errors are directly reported in a non-blocking way on every differential code revision (i.e., a pull request) of the onboarded service. This stop-the-bleed approach helps to prevent new nil panics from being introduced into the service code, while allowing teams to gradually address nil panics in existing code without the need for a development-slowing upfront onboarding effort.

We have onboarded several services at Uber onto NilAway, across both the options, and the overall feedback that we have received from the teams has been positive. One such happy user says “NilAway has helped their team catch issues early, preventing deployment rollbacks,” while another says “The comments left by NilAway are very actionable and it hasn’t caused any noise.” The users also actively report false positives that they may encounter and suggest usability improvements that we actively work upon.

## 💬 Support 

We follow the same [version support policy](https://go.dev/doc/devel/release#policy) as the [Go](https://golang.org/) 
project: we support and test the last two major versions of Go.

Please feel free to [open a GitHub issue](https://github.com/uber-go/nilaway/issues) if you have any questions, bug 
reports, and feature requests.

## 👍 Contributions

We'd love for you to contribute to NilAway! Please note that once you create a pull request, you will be asked to sign 
our [Uber Contributor License Agreement](https://cla-assistant.io/uber-go/nilaway).

## ⚠️ License

This project is copyright 2023 Uber Technologies, Inc., and licensed under Apache 2.0.

[doc-img]: https://pkg.go.dev/badge/go.uber.org/nilaway.svg
[doc]: https://pkg.go.dev/go.uber.org/nilaway
[ci-img]: https://github.com/uber-go/nilaway/actions/workflows/ci.yml/badge.svg
[ci]: https://github.com/uber-go/nilaway/actions/workflows/ci.yml
[cov-img]: https://codecov.io/gh/uber-go/nilaway/branch/main/graph/badge.svg
[cov]: https://codecov.io/gh/uber-go/nilaway