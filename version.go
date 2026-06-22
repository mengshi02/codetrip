package codetrip

// Version is the current codetrip version.
// It is automatically set at build time via -ldflags from the VERSION file.
// When built without ldflags, it defaults to "dev".
var Version = "dev"

// DataSchemaVersion tracks the on-disk data format version.
// When the key format or directory layout changes, increment this value
// and add a migration path in Open().
const DataSchemaVersion = "1"