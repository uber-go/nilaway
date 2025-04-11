package tokenhelper

import (
	"os"
	"path/filepath"
)

var _cwd = func() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic("failed to get current working directory: " + err.Error())
	}
	return cwd
}()

// RelToCwd returns the relative path of the given filename with respect to the current
// working directory (retrieved during initialization). If the filename is not a child of
// the current working directory, it returns the filename itself.
func RelToCwd(filename string) string {
	rel, err := filepath.Rel(_cwd, filename)
	if err != nil {
		return rel
	}
	return filename
}
