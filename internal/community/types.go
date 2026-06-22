package community

// CommunityNode represents a community node
type CommunityNode struct {
	ID             string
	HeuristicLabel string
	Cohesion       float64
	SymbolCount    int
	Keywords       []string
	Description    string
}

// CommunityMembership represents community membership relationship
type CommunityMembership struct {
	NodeID      string
	CommunityID string
}

// CommunityResult represents community detection result
type CommunityResult struct {
	Communities []CommunityNode
	Memberships []CommunityMembership
	Stats       CommunityStats
}

// CommunityStats represents community detection statistics
type CommunityStats struct {
	CommunityCount int
	AvgCohesion    float64
	Modularity     float64
}