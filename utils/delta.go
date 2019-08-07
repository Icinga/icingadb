package utils

func Delta(a []string, b []string) ([]string, []string, []string) {
	maintained := make([]string, 0)
	dismissed := make([]string, 0)
	hash := make(map[string]int)

	for _, item := range a {
		hash[item] = 1
	}

	for _, item := range b {
		if hash[item] > 0 {
			maintained = append(maintained, item)
			hash[item] = 2
		} else {
			dismissed = append(dismissed, item)
		}
	}

	introduced := make([]string, 0)

	for k, v := range hash {
		if v == 1 {
			introduced = append(introduced, k)
		}
	}

	return introduced, maintained, dismissed
}