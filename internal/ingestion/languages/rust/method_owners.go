package rust

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PopulateRustOwners assigns owner IDs to Rust impl-block methods
// that the legacy parse pass missed.
// In Rust, `impl T for S` methods should be owned by the struct S,
// not the trait T.
// TODO: full implementation — requires scope tree traversal.
func PopulateRustOwners(parsed *shared.ParsedFile) {
	// TODO: implement when scope-resolution walkers are integrated.
	// Mirrors TS populateRustOwners(parsed):
	//   - Walk all scopes in parsed.Scopes
	//   - For impl scopes, collect their method defs
	//   - Set OwnerID on each method def to point to the struct def
}