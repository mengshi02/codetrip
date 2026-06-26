package cpp

// C++ Capture Side Channel — auxiliary bindings for template/constraint resolution.
//
// During scope capture, C++ template instantiation and constraint checking
// produce auxiliary bindings that cannot be stored in the main scope bindings
// (they would conflict with non-template overloads). Instead, they are written
// to a side channel that is restored before the binding-merge pass.
//
// This is the only language that uses the capture side channel mechanism.
// Ported from TS languages/cpp/capture-side-channel.ts.

import (
	"sync"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// sideChannelMutex guards the side channel state.
var sideChannelMutex sync.RWMutex

// captureSideChannel maps file paths to their auxiliary bindings.
// Key: file path → Value: map[scopeID][]BindingRef
var captureSideChannel map[string]map[string][]shared.BindingRef

func init() {
	captureSideChannel = make(map[string]map[string][]shared.BindingRef)
}

// WriteCppCaptureSideChannel writes auxiliary bindings to the side channel.
func WriteCppCaptureSideChannel(filePath string, scopeID shared.ScopeID, bindings []shared.BindingRef) {
	sideChannelMutex.Lock()
	defer sideChannelMutex.Unlock()
	if captureSideChannel[filePath] == nil {
		captureSideChannel[filePath] = make(map[string][]shared.BindingRef)
	}
	captureSideChannel[filePath][string(scopeID)] = bindings
}

// RestoreCppCaptureSideChannel restores side channel bindings into the parsed file.
// Called by the scope resolver's RestoreCaptureSideChannel hook.
// TODO: full implementation — merge side channel bindings into parsed file scopes.
func RestoreCppCaptureSideChannel(parsed *shared.ParsedFile) {
	sideChannelMutex.RLock()
	defer sideChannelMutex.RUnlock()
	// TODO: look up captureSideChannel[parsed.FilePath] and merge
}

// ClearCppCaptureSideChannel resets all side channel state.
func ClearCppCaptureSideChannel() {
	sideChannelMutex.Lock()
	defer sideChannelMutex.Unlock()
	captureSideChannel = make(map[string]map[string][]shared.BindingRef)
}