package diagnostic

import (
	"fmt"
	"go/token"
	"strings"
)

type conflict struct {
	pos              token.Pos   // stores position where the error should be reported (note that this field is used only within the current, and should NOT be exported)
	flow             nilFlow     // stores nil flow from source to dereference point
	similarConflicts []*conflict // stores other conflicts that are similar to this one
}

func (c *conflict) String() string {
	// build string for similar conflicts (i.e., conflicts with the same nil path)
	similarConflictsString := ""
	if len(c.similarConflicts) > 0 {
		similarPos := make([]string, len(c.similarConflicts))
		for i, s := range c.similarConflicts {
			similarPos[i] = fmt.Sprintf("\"%s\"", s.flow.nonnilPath[len(s.flow.nonnilPath)-1].consumerPosition.String())
		}

		posString := strings.Join(similarPos[:len(similarPos)-1], ", ")
		if len(similarPos) > 1 {
			posString = posString + ", and "
		}
		posString = posString + similarPos[len(similarPos)-1]

		similarConflictsString = fmt.Sprintf("\n\n(Same nil source could also cause potential nil panic(s) at %d "+
			"other place(s): %s.)", len(c.similarConflicts), posString)
	}

	return fmt.Sprintf("Potential nil panic detected. Observed nil flow from "+
		"source to dereference point: %s%s\n", c.flow.String(), similarConflictsString)
}

func (c *conflict) addSimilarConflict(conflict conflict) {
	c.similarConflicts = append(c.similarConflicts, &conflict)
}

// groupConflicts groups conflicts with the same nil path together and update conflicts list.
func groupConflicts(allConflicts []conflict) []conflict {
	conflictsMap := make(map[string]int)  // key: nil path string, value: index in `allConflicts`
	indicesToIgnore := make(map[int]bool) // indices of conflicts to be ignored from `allConflicts`, since they are grouped with other conflicts

	for i, c := range allConflicts {
		key := pathString(c.flow.nilPath)

		// Handle the case of single assertion conflict separately
		if len(c.flow.nilPath) == 0 && len(c.flow.nonnilPath) == 1 {
			// This is the case of single assertion conflict. Use producer position and repr from the non-nil path as the key.
			if p := c.flow.nonnilPath[0]; p.producerPosition.IsValid() {
				key = p.producerPosition.String() + ": " + p.producerRepr
			}
		}

		if existingConflictIndex, ok := conflictsMap[key]; ok {
			// Grouping condition satisfied. Add new conflict to `similarConflicts` in `existingConflict`, and update groupedConflicts map
			allConflicts[existingConflictIndex].addSimilarConflict(c)
			indicesToIgnore[i] = true
		} else {
			conflictsMap[key] = i
		}
	}

	// update groupedConflicts list with grouped groupedConflicts
	var groupedConflicts []conflict
	for i, c := range allConflicts {
		if _, ok := indicesToIgnore[i]; !ok {
			groupedConflicts = append(groupedConflicts, c)
		}
	}
	return groupedConflicts
}
