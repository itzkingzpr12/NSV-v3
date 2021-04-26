package cache

// CacheError struct
type CacheError struct {
	Message string
	Err     error
}

// Error func
func (ce *CacheError) Error() string {
	return ce.Error()
}
