# False-Positive Reductions: Contracts, Effects, and Lazy Initialization

This document describes three stacked revisions that reduce false positives (FPs) in the
allocation-site-sensitive struct-initialization analysis. The guiding constraint is **coarse +
sound**: accept fewer cases rather than infer a fact that can be invalidated by an alias, a
re-evaluation, or an unverified implementation detail.

## The three revisions

| Revision | Reduction | FP families killed |
|---|---|---|
| `75dcf257` | Function-contract inference for error-returning validators and boolean predicates | `validator errnil=>arg.field`, `predicate true=>arg/recv.field` |
| `1e8fb953` | Lazy-init suppression from dominating, definitely-non-nil field stores and guarded paths | getter-guard lazy-init, singleton repair, post-literal writes |
| `910d1d75` | Caller-pinned lazy-init handling for parameter/receiver field context | caller-pinned literals |

The first revision turns local SSA observations into reusable contracts. The latter two suppress a
struct-field diagnostic when the caller's actual allocation and stores establish the field. They
are additive: a getter guard is not treated as a general getter contract, and a caller-pinned
summary is not treated as a global property of every value of the same type.

## Architecture

* **Inference:** `assertion/function/functioncontracts/infer.go` derives the small validator and
  predicate contracts from SSA. `assertion/function/functioncontracts/analyzer.go` selects eligible
  functions, applies the `experimental-struct-init-v2` gate, and collects handwritten or inferred
  contracts.
* **Consumption:** `assertion/function/assertiontree/rich_check_effect.go` models checks as effect
  types. A successful validator or predicate produces a non-nil field/argument effect; invalidation
  is limited to assignments and calls that mention the tracked expression. Lazy-init is different:
  its evidence is a store and an all-paths field proof, not a conditional check.
* **Lazy-init matching:** `triggerProductions` in
  `assertion/function/assertiontree/root_assertion_node.go` calls `lazyInitSuppressed` before adding
  a producer/consumer trigger. `assertion/function/assertiontree/lazy_init.go` handles both
  allocation-local `StructFieldNil` and caller-pinned `StructFieldFromContext` producers.
* **Facts:** `Contracts` implements `analysis.Fact`. The contracts analyzer imports object facts
  from dependencies and exports contracts for exported functions and methods. Multi-package
  struct-field information is represented by the inference engine's `InferredMap`, which is also
  exported/imported as a package fact and reduced to serializable primitive sites.
* **Gating:** the config flag `-experimental-struct-init-v2` is off by default. It gates contract
  inference and the struct-init-v2 effect/lazy-init paths, so these reductions do not change the
  default analysis.

## Soundness invariants and current fixes

The current working copy makes four deliberately narrow fixes.

### 1. Normalize the non-nil branch edge

`proveTrueOnBranch` now identifies the non-nil successor once and uses that edge for both `EQL` and
`NEQ`. A comparison's spelling must not change which branch is understood as proving non-nil.
This protects the `PhiFromCompare` shape: a phi value assembled from a comparison branch is
accepted only when every incoming edge is the non-nil edge, not merely because one operator happens
to be `!=`.

### 2. Verify `Get<Field>` structurally

The lazy-init path accepts a getter guard only for a same-package static callee whose SSA body
passes `validGetter`. Every return must be either nil or a direct, exact field load from the getter's
receiver; there must be at least one such load. Stores, calls, goroutines, defers, sends, and other
return values fail closed. Thus a misleading `GetF` method that returns an unrelated non-nil value
cannot make `x.F` appear initialized merely because its name matches the field.

### 3. Poison aliases conservatively

Store checks apply to both `StructFieldNil` and `StructFieldFromContext` producers. `mayAliasBase`
distinguishes only identical bases and distinct direct allocations; phi values, parameters, loads,
getters/call results, and unknown values may alias and therefore fail closed. An unknown or nil
store to a compatible field kills suppression. The `localPhiAlias` shape consequently remains a
diagnostic rather than being suppressed from a proof tied to a possibly aliased local.

### 4. Do not transfer proofs across re-evaluated calls

At the consumer, `exprContainsCall` rejects tracked arguments and receivers containing a call. A
proof about one evaluation must not be applied to a later evaluation of that call expression, which
could return a different value or have effects. The `repeated-call` regression shape therefore does
not inherit a predicate proof from one `get()` evaluation to another.

These checks complement the existing dominance and all-paths requirements: a non-nil store must be
visible before the dereference, and every path to the use must be established. Unknown values are
never upgraded to definitely non-nil.

## One design principle: coarse + sound

The implementation intentionally uses strict use whitelists, definitely-non-nil-only stores,
dominance/all-paths checks, and no new fixpoints. This is less precise than a whole-program alias
analysis, but its accepted facts have a short and inspectable proof. In particular, it deliberately
does **not** handle:

* points-to or general alias analysis;
* cross-function getter summaries beyond verified local getters;
* symbolic predicates or correlations outside the recognized nil comparisons; or
* interface dispatch and the set of implementations reachable through an interface.

The trade-off is intentional over-rejection: a safe-looking case can remain an FP when its proof
requires one of those analyses, while an accepted case has conservative mutation and control-flow
guards.

## Performance

Measurements used per-commit parent-to-child detached worktrees, with interleaved timed runs to
reduce ordering effects. Each revision was measured with `nilaway
-experimental-struct-init-v2 -json -pretty-print=false std` for 3 rounds and with self-analysis
`-include-pkgs=go.uber.org/nilaway ./...` for 5 rounds. The table reports median deltas from the
parent.

| Revision | Workload | Wall | CPU | RSS |
|---|---|---:|---:|---:|
| `75dcf257` | std | -0.1% | +5.1% | +1.4% |
| `75dcf257` | self | +2.4% | -3.3% | +2.4% |
| `1e8fb953` | std | -1.7% | +3.3% | -1.9% |
| `1e8fb953` | self | -1.2% | -6.2% | +3.4% |
| `910d1d75` | std | -3.9% | +1.8% | +0.1% |
| `910d1d75` | self | +2.0% | +2.3% | +0.8% |

There is no statistically meaningful overhead: all deltas are within measurement noise on the
Linux host with the repository's Go 1.26.x toolchain.
