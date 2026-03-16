package rewriter

import (
	"math/rand/v2"
	"strings"

	"github.com/miekg/dns"
	"github.com/mehrshad/dns-split/internal/config"
)

type Rewriter struct {
	originalToReplacements map[string][]string
	replacementToOriginal  map[string]string
}

func New(mappings []config.Mapping) *Rewriter {
	r := &Rewriter{
		originalToReplacements: make(map[string][]string, len(mappings)),
		replacementToOriginal:  make(map[string]string),
	}
	for _, m := range mappings {
		orig := dns.Fqdn(strings.ToLower(m.Original))
		repls := make([]string, len(m.Replacements))
		for i, rep := range m.Replacements {
			repls[i] = dns.Fqdn(strings.ToLower(rep))
			r.replacementToOriginal[repls[i]] = orig
		}
		r.originalToReplacements[orig] = repls
	}
	return r
}

// Replace rewrites a domain name by replacing a matching original suffix
// with a randomly chosen replacement. Returns the rewritten name and the
// replacement domain used (for tracking), or the original name and empty
// string if no mapping matched.
func (rw *Rewriter) Replace(name string) (rewritten string, replacement string) {
	name = dns.Fqdn(strings.ToLower(name))
	for orig, repls := range rw.originalToReplacements {
		if matchesSuffix(name, orig) {
			chosen := repls[rand.IntN(len(repls))]
			return swapSuffix(name, orig, chosen), chosen
		}
	}
	return name, ""
}

// Restore reverses a replacement domain back to the original domain.
// Returns the restored name and true if a mapping was found.
func (rw *Rewriter) Restore(name string) (restored string, ok bool) {
	name = dns.Fqdn(strings.ToLower(name))
	for repl, orig := range rw.replacementToOriginal {
		if matchesSuffix(name, repl) {
			return swapSuffix(name, repl, orig), true
		}
	}
	return name, false
}

// ReplacementToOriginal returns the original domain for a given replacement,
// or empty string if not found. Does not handle subdomains — use Restore for that.
func (rw *Rewriter) ReplacementToOriginal(replacement string) string {
	return rw.replacementToOriginal[dns.Fqdn(strings.ToLower(replacement))]
}

// matchesSuffix checks if name equals suffix or is a subdomain of it.
func matchesSuffix(name, suffix string) bool {
	if name == suffix {
		return true
	}
	return strings.HasSuffix(name, "."+suffix)
}

// swapSuffix replaces oldSuffix at the end of name with newSuffix.
func swapSuffix(name, oldSuffix, newSuffix string) string {
	if name == oldSuffix {
		return newSuffix
	}
	prefix := strings.TrimSuffix(name, oldSuffix)
	return prefix + newSuffix
}
