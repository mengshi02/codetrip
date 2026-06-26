package c

// CCaptureSideChannel is a plain JSON-serializable snapshot of the per-file C
// capture-time side-channel. Carried opaquely on ParsedFile.captureSideChannel.
// The `kind` tag makes the payload self-describing so `apply` can distinguish
// a C snapshot from another language's (C++ and Kotlin share the same field).
// Ported from GitNexus c/capture-side-channel.ts.
type CCaptureSideChannel struct {
	Kind        string   `json:"kind"`        // always "c"
	StaticNames []string `json:"staticNames"` // simple names of static (file-local) functions
}

// CollectCStaticLinkageSideChannel returns a side-channel snapshot for the given file.
// Returns nil when this file recorded no static names at all.
func CollectCStaticLinkageSideChannel(filePath string) *CCaptureSideChannel {
	names := GetStaticNamesForFile(filePath)
	if len(names) == 0 {
		return nil
	}
	return &CCaptureSideChannel{
		Kind:        "c",
		StaticNames: names,
	}
}

// ApplyCStaticLinkageSideChannel restores the worker-serialized snapshot from
// a ParsedFile's captureSideChannel and re-populates the module-level
// static-linkage map via MarkStaticName. Tolerant of nil and unexpected shapes.
// Does NO tree-sitter parse.
func ApplyCStaticLinkageSideChannel(filePath string, sideChannel interface{}) {
	if sideChannel == nil {
		return
	}

	data, ok := sideChannel.(*CCaptureSideChannel)
	if !ok {
		return
	}
	if data.Kind != "c" {
		return
	}
	for _, name := range data.StaticNames {
		MarkStaticName(filePath, name)
	}
}