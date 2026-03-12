package cmd

import (
"os"
)

// fileExists returns true if the given path exists on disk.
func fileExists(path string) bool {
_, err := os.Stat(path)
return err == nil
}
