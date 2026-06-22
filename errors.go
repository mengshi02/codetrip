package codetrip

import "errors"

// ============ Common Error Definitions ============

// ErrRepoNotFound indicates the repository was not found
var ErrRepoNotFound = errors.New("repo not found")

// ErrNoGraphStore indicates no graph store is available
var ErrNoGraphStore = errors.New("no graph store available")

// ErrSymbolNotFound indicates the symbol was not found
var ErrSymbolNotFound = errors.New("symbol not found")

// ErrRepoAlreadyExists indicates the repository has already been indexed
var ErrRepoAlreadyExists = errors.New("repo already indexed")

// ErrInvalidRequest indicates an invalid request
var ErrInvalidRequest = errors.New("invalid request")

// ErrTraversalLimitExceeded indicates the traversal exceeded the maximum node visit limit
var ErrTraversalLimitExceeded = errors.New("traversal exceeded maximum node visit limit")

// ErrQueryTimeout indicates a query exceeded its timeout
var ErrQueryTimeout = errors.New("query exceeded timeout")

// ErrRepoNotIndexed indicates the repository has not been indexed yet
var ErrRepoNotIndexed = errors.New("repository has not been indexed")