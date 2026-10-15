package merge

// insertAfterHost inserts the entire limited block after the first matching
// primary host. An empty or missing anchor preserves append-at-end behavior.
func insertAfterHost[T any](primary, limited []T, anchor string, hostName func(T) string) []T {
	index := len(primary)
	if anchor != "" {
		for i, item := range primary {
			if hostName(item) == anchor {
				index = i + 1
				break
			}
		}
	}
	merged := make([]T, 0, len(primary)+len(limited))
	merged = append(merged, primary[:index]...)
	merged = append(merged, limited...)
	merged = append(merged, primary[index:]...)
	return merged
}
