package modelconfig

import "strings"

var registered []BackendSchema

// detectFuncs holds custom detection functions per backend name.
var detectFuncs = map[string]func(string) bool{}

// Register adds a backend schema to the registry. Call from init() in each
// backends_*.go file.
func Register(s BackendSchema) {
	registered = append(registered, s)
}

// RegisterDetect registers a custom detection function for a named backend.
// Call alongside Register in each backends_*.go init().
func RegisterDetect(name string, f func(cmd string) bool) {
	detectFuncs[name] = f
}

// Detect returns the backend name for the given cmd string, or "" if unknown.
// Checks in registration order — first match wins.
func Detect(cmd string) string {
	for _, s := range registered {
		if detectBackend(s.Name, cmd) {
			return s.Name
		}
	}
	return ""
}

// Get returns the BackendSchema for the named backend, and whether it was found.
func Get(name string) (BackendSchema, bool) {
	for _, s := range registered {
		if s.Name == name {
			return s, true
		}
	}
	return BackendSchema{}, false
}

// IsTunable returns true if the cmd belongs to a backend that has a Configure
// panel (excludes TTS/STT FastAPI wrappers and script-based servers).
func IsTunable(cmd string) bool {
	return Detect(cmd) != ""
}

func detectBackend(name, cmd string) bool {
	if f, ok := detectFuncs[name]; ok {
		return f(cmd)
	}
	return strings.Contains(cmd, name)
}
