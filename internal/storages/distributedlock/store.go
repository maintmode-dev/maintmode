package distributedlock

import (
	valkeylib "github.com/redis/go-redis/v9"
)

const lockPrefix = "lock:"

// Store implements distributed locking using Valkey SET NX.
type Store struct {
	client *valkeylib.Client
}

// NewStore creates a new Valkey-backed distributed lock.
func NewStore(client *valkeylib.Client) *Store {
	return &Store{client: client}
}
