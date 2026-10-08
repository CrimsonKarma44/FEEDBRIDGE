package store

func candidateURLs(input string, links []string) []string {
	out := make([]string, 0, 1+len(links))
	seen := make(map[string]struct{}, 1+len(links))
	add := func(u string) {
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	add(input)
	for _, l := range links {
		add(l)
	}
	return out
}

func firstMatching(candidates, existing []string) string {
	have := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		have[e] = struct{}{}
	}
	for _, c := range candidates {
		if _, ok := have[c]; ok {
			return c
		}
	}
	return ""
}

// MatchingURLs is the public form used by Subscribe: input plus detected feed links, deduped.
func MatchingURLs(input string, links []string) []string {
	return candidateURLs(input, links)
}
