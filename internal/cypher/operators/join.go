package operators

import "fmt"

// JoinIterator implements a cross-product (Cartesian join) between a left
// and right iterator. The right iterator is fully materialized on first use
// so that it can be replayed for each left row.
//
// When joinKeys are provided, it performs a hash join instead: the right side
// is partitioned into a hash table keyed by the join column values, and each
// left row probes only the matching partition. This eliminates the O(L*R)
// Cartesian product in favor of O(L + R) when equi-join conditions exist.
type JoinIterator struct {
	left  Iterator
	right Iterator

	// join keys: column names that must match between left and right rows.
	// When non-empty, hash join is used; otherwise cross-product.
	joinKeys []string

	// materialized right rows (cross-product mode)
	rightRows []Row
	rightInit bool

	// hash table (hash-join mode): key → rows sharing that key
	hashTable map[string][]Row
	// all right-side keys seen (for null-safe probing)
	rightKeys []string

	// current state
	curLeft  Row
	rightIdx int
	closed   bool
}

// NewJoinIterator creates a cross-product join iterator (no equi-join keys).
func NewJoinIterator(left, right Iterator) *JoinIterator {
	return &JoinIterator{
		left:  left,
		right: right,
	}
}

// NewHashJoinIterator creates a hash join iterator that matches rows on the
// given column names. Both left and right rows must have the same value for
// each joinKey column to be combined.
func NewHashJoinIterator(left, right Iterator, joinKeys []string) *JoinIterator {
	return &JoinIterator{
		left:     left,
		right:    right,
		joinKeys: joinKeys,
	}
}

func (j *JoinIterator) materializeRight() {
	if j.rightInit {
		return
	}
	j.rightInit = true

	if len(j.joinKeys) > 0 {
		// Hash join: build hash table from right side
		j.hashTable = make(map[string][]Row)
		for {
			row, err := j.right.Next()
			if err != nil || row == nil {
				break
			}
			key := RowKey(row, j.joinKeys)
			j.hashTable[key] = append(j.hashTable[key], row)
		}
	} else {
		// Cross-product: collect all right rows
		for {
			row, err := j.right.Next()
			if err != nil || row == nil {
				break
			}
			j.rightRows = append(j.rightRows, row)
		}
	}
	_ = j.right.Close()
}

// Next returns the next row from the join.
func (j *JoinIterator) Next() (Row, error) {
	if j.closed {
		return nil, nil
	}

	j.materializeRight()

	for {
		// If no current left row, advance left
		if j.curLeft == nil {
			leftRow, err := j.left.Next()
			if err != nil {
				return nil, err
			}
			if leftRow == nil {
				return nil, nil
			}
			j.curLeft = leftRow
			j.rightIdx = 0

			if len(j.joinKeys) > 0 {
				// Hash join: probe only matching partition
				key := RowKey(j.curLeft, j.joinKeys)
				j.rightRows = j.hashTable[key]
			}
		}

		// Return next combination with right row
		if j.rightIdx < len(j.rightRows) {
			rightRow := j.rightRows[j.rightIdx]
			j.rightIdx++

			merged := CopyRow(j.curLeft)
			for k, v := range rightRow {
				merged[k] = v
			}
			return merged, nil
		}

		// Exhausted right side for this left row; advance left
		j.curLeft = nil
	}
}

// Close releases resources held by the iterator and its children.
func (j *JoinIterator) Close() error {
	j.closed = true
	j.hashTable = nil
	j.rightRows = nil
	err := j.left.Close()
	if !j.rightInit {
		if err2 := j.right.Close(); err2 != nil && err == nil {
			err = err2
		}
	}
	return err
}

// DetectJoinKeys inspects two rows and returns column names that appear in
// both. These are candidate equi-join keys for hash join optimization.
func DetectJoinKeys(leftCols, rightCols []string) []string {
	rightSet := make(map[string]bool, len(rightCols))
	for _, c := range rightCols {
		rightSet[c] = true
	}
	var common []string
	for _, c := range leftCols {
		if rightSet[c] {
			common = append(common, c)
		}
	}
	return common
}

// String returns a debug description of the join iterator.
func (j *JoinIterator) String() string {
	if len(j.joinKeys) > 0 {
		return fmt.Sprintf("HashJoin(keys=%v)", j.joinKeys)
	}
	return "CrossJoin"
}