// Package logs provides a pluggable logs engine for Plomvix.
// token.go implements the log tokenizer that splits raw text into
// lowercase alphanumeric search tokens for inverted indexing.
package logs

import (
	"strings"
	"unicode"
)

// Tokenize splits text into unique lowercase alphanumeric tokens.
// Tokens shorter than 2 characters are skipped as noise.
func Tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return deduplicateTokens(tokens)
}

func deduplicateTokens(in []string) []string {
	m := make(map[string]struct{})
	var out []string
	for _, s := range in {
		if len(s) < 2 { // skip tiny noise tokens
			continue
		}
		if _, ok := m[s]; !ok {
			m[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
