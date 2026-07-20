package graph

// ForEachNode visits every persisted node in deterministic Pebble key order.
func (s *GraphStore) ForEachNode(fn func(*Node) error) error {
	iterator := s.IterNodes(s.repo)
	defer iterator.Close()
	for iterator.Next() {
		if err := fn(iterator.Node()); err != nil {
			return err
		}
	}
	return nil
}

// ForEachEdge visits every persisted edge in deterministic Pebble key order.
func (s *GraphStore) ForEachEdge(fn func(*Edge) error) error {
	return s.store.ScanPrefix(edgePrefix(s.repo), func(_, value []byte) error {
		var edge Edge
		if err := Decode(value, &edge); err != nil {
			return err
		}
		return fn(&edge)
	})
}
