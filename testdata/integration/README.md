# NilAway Integration Test Folder

To prevent functionality regressions when developing NilAway, we have leveraged `analysistest` 
test framework to write unit tests (`testdata/src` folder). However, it has two major limitations
that make it undesirable to ensure real-world behaviors:

* It does not read the "want" comments in the upstream package when analyzing downstream packages. 
  This makes it impossible to test our multi-package inference algorithm: NilAway might report an 
  error for an upstream package while analyzing downstream packages, but we cannot write expected 
  error strings for the framework to verify;
* For bazel/nogo, the package loader and [`Fact`][fact] caching behavior is slightly different from 
  bazel (which under the hood uses a custom nogo runner that is different from the `analysistest` 
  runner), and there is no guarantee that they will not diverge further in the future.

Therefore, this `integration` folder serves as a buildable Go project (both by standard Go 
toolchain and bazel) that contains multiple potential nil panics with `want` comments. We build 
this project with different drivers and check if the final output matches the desired output. It 
is meant to be built and tested with our integration test driver (`make integration-test`, where 
the code is at `tools/cmd/integration-test`), but it can also be built separately to see the NilAway
outputs. This also serves as an example of how to use NilAway in a real-world project.

Below shows how to run NilAway on this example project outside the integration test framework for
illustrating purposes. The integration test framework basically follows the same steps, but with
an extra step to verify if the output errors match the `want` comments. 

## Using Standalone Checker

Since this integration test project follows the standard Go project layout, you can simply run 
NilAway on it with the following command:

```shell
# Build NilAway
make build
# Run NilAway on the integration test project
cd testdata/integration
../../bin/nilaway ./...
```

[fact]: https://pkg.go.dev/golang.org/x/tools/go/analysis/internal/facts
