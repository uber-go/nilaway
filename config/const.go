package config

import (
	"fmt"
	"time"
)

// This file hosts non-user-configurable parameters --- these are for development and testing purposes only.

// StableRoundLimit is the number of rounds in backpropagation algorithm after which, if there is no change
// in the collected triggers, the algorithm halts. It is possible to carefully craft known false negative for any value
// of StableRoundLimit (check test loopflow.go/longRotNilLoop). Setting this value too low may result in false negatives
// going undetected, while setting it too high may lead to longer analysis times without significant precision gains.
// In practice, a value of StableRoundLimit >= 2 has shown to provide sound analysis, capturing most false negatives.
// After experimentation, we observed that using StableRoundLimit = 5 with NilAway yields similar analysis time compared
// to lower values, making it a good compromise for precise results.
const StableRoundLimit = 5

// BackpropTimeout is the timeout set for analysis of each function. This ensures build time SLAs.
// NilAway should report an error if this timeout is ever hit, in order not to silently ignore any
// functions due to this.
const BackpropTimeout = 10 * time.Second

// ErrorOnNilableMapRead configures whether reading from nil maps should be considered an error.
// Since Go does not panic on this, right now we do not interpret it as one, but this could be
// considered undesirable behavior and worth catching in the future.
const ErrorOnNilableMapRead = false

// NilAwayNoInferString is the string that may be inserted into the docstring for a package to prevent
// NilAway from inferring the annotations for that package - this is useful for unit tests
const NilAwayNoInferString = "<nilaway no inference>"

const uberPkgPathPrefix = "go.uber.org"

// NilAwayPkgPathPrefix is the package prefix for NilAway.
const NilAwayPkgPathPrefix = uberPkgPathPrefix + "/nilaway"

// DirLevelsToPrintForTriggers controls the number of enclosing directories to print when referring
// to the locations that triggered errors - right now it seems as if 1 is sufficient disambiguation,
// but feel free to increase.
const DirLevelsToPrintForTriggers = 1

// DefaultNilableNamedTypes is the list of type names that we interpret as default nilable.
var DefaultNilableNamedTypes = [...]string{}

// StructInitCheckType Config for setting the level of struct initialization check
type StructInitCheckType int

const (
	// NoCheck if the checker is not enabled (current default)
	NoCheck StructInitCheckType = iota

	// DepthOneFieldCheck in this setting we track the nilability of fields at depth 1
	// i.e. we track nilability of x.y, but we do not track x.y.z
	DepthOneFieldCheck
)

// NilAwayStructInitCheckString is the string that may be inserted into the docstring for a package to
// force NilAway to enable struct init checking
const NilAwayStructInitCheckString = "<nilaway struct enable>"

// NilAwayAnonymousFuncCheckString is the string that may be inserted into the docstring for a package to
// force NilAway to enable anonymous func checking
const NilAwayAnonymousFuncCheckString = "<nilaway anonymous function enable>"

func maxRoundsFromBlocks(numBlocks int) int {
	return numBlocks * numBlocks * 2
}

// CheckCFGFixedPointRuntime throws a panic if a fixed point iteration loop runs beyond some upper
// bounded round number, determined by the number of blocks in the CFG of the analyzed function,
// processed by maxRoundsFromBlocks
func CheckCFGFixedPointRuntime(passName string, numBlocks, currRound int) {
	if currRound > maxRoundsFromBlocks(numBlocks) {
		panic(fmt.Sprintf("ERROR: Propagation over %d-block CFG in pass '%s' ran for "+
			"%d rounds, when maximum allowed was %d rounds. This indicates a failure of the fixpoint"+
			" logic and must be debugged at the source level.",
			numBlocks, passName, currRound, maxRoundsFromBlocks(numBlocks)))
	}
}
