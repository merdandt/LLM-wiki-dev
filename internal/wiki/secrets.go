package wiki

import "regexp"

var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{name: "private-key", re: regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`)},
	{name: "github-token", re: regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{30,}\b`)},
	{name: "aws-access-key", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{name: "openai-api-key", re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)},
	{name: "slack-token", re: regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{20,}\b`)},
}

func SecretMatches(data []byte) []string {
	var matches []string
	for _, pattern := range secretPatterns {
		if pattern.re.Match(data) {
			matches = append(matches, pattern.name)
		}
	}
	return matches
}
