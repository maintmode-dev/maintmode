package entity

// DistributedLock is a held distributed lock. Token names the owner: only the
// caller that acquired Key can release it.
type DistributedLock struct {
	Key   string
	Token string
}
