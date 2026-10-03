package industrialscraper

// Batch splits items into consecutive groups of at most size; size <= 0
// returns a single group. See README.
func Batch[T any](items []T, size int) [][]T {
	if len(items) == 0 {
		return nil
	}
	if size <= 0 {
		return [][]T{items}
	}
	var groups [][]T
	for start := 0; start < len(items); start += size {
		end := min(start+size, len(items))
		groups = append(groups, items[start:end])
	}
	return groups
}
