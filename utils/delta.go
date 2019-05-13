package utils

func Delta(a []string, b []string) ([]string, []string, []string) {
	maintained := make([]string, 0)
	dismissed := make([]string, 0)
	hash := make(map[string]bool)

	for _, item := range a {
		hash[item] = true
	}

	for _, item := range b {
		if hash[item] {
			maintained = append(maintained, item)
			delete(hash, item)
		} else {
			dismissed = append(dismissed, item)
		}
	}

	introduced := make([]string, len(hash))

	i := 0
	for k := range hash {
		introduced[i] = k
		i++
	}

	return introduced, maintained, dismissed
}