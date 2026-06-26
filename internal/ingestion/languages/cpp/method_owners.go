package cpp

// C++ Method Owners — assign impl method definitions to their class owner.
//
// In C++, method definitions may appear outside the class body:
//   void MyClass::method() { ... }
// The scope extractor may not always correctly attribute these to their
// owning class. This module reassigns such method definitions to their
// class owner during the PopulateOwners pass.
// Ported from TS languages/cpp/method-owners.ts (concept shared with Rust).

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PopulateCppOwners assigns owner IDs to C++ method definitions that
// the initial parse pass may have missed. Methods defined outside their
// class body (e.g. void MyClass::method()) are reassigned to the class.
// TODO: full implementation — walk parsed scopes and reassign owners.
func PopulateCppOwners(parsed *shared.ParsedFile) {
	// TODO: walk parsed scopes, find function_definition nodes with
	// qualified names (Class::method), and reassign their owner to the class.
}