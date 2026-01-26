//  Copyright (c) 2023 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

// This file hosts non-user-configurable parameters --- these are for development and testing purposes only.

// StableRoundLimit is the number of rounds in backpropagation algorithm after which, if there is no change
// in the collected triggers, the algorithm halts. It is possible to carefully craft known false negative for any value
// of StableRoundLimit (check test loopflow.go/longRotNilLoop). Setting this value too low may result in false negatives
// going undetected, while setting it too high may lead to longer analysis times without significant precision gains.
// In practice, a value of StableRoundLimit >= 2 has shown to provide sound analysis, capturing most false negatives.
// After experimentation, we observed that using StableRoundLimit = 5 with NilAway yields similar analysis time compared
// to lower values, making it a good compromise for precise results.
const StableRoundLimit = 5

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
