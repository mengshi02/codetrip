package symbol

import "testing"

func BenchmarkTokenize(b *testing.B) {
	for b.Loop() {
		tokenize("getUserByIDHTTPRequest")
	}
}
