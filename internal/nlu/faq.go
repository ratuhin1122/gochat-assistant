// Package nlu provides natural language understanding and response generation.
package nlu

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

// DefaultThreshold is the minimum similarity score required to answer an FAQ.
const DefaultThreshold = 0.55

// Common English stopwords to filter during normalization.
var defaultStopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"in": {}, "on": {}, "at": {}, "by": {}, "for": {}, "with": {}, "about": {},
	"to": {}, "from": {}, "of": {}, "it": {}, "its": {}, "and": {}, "or": {},
	"do": {}, "does": {}, "did": {}, "be": {}, "been": {}, "being": {},
}

// FAQResponder evaluates incoming requests against loaded FAQ entries using
// token overlap and Levenshtein similarity.
type FAQResponder struct {
	entries   []FAQEntry
	threshold float64
}

// NewFAQResponder creates an FAQResponder with the default threshold.
// If entries is empty, it loads the default embedded FAQ entries.
func NewFAQResponder(entries ...FAQEntry) (*FAQResponder, error) {
	return NewFAQResponderWithThreshold(DefaultThreshold, entries...)
}

// NewFAQResponderWithThreshold creates an FAQResponder with a custom similarity threshold.
// If entries is empty, it loads the default embedded FAQ entries.
func NewFAQResponderWithThreshold(threshold float64, entries ...FAQEntry) (*FAQResponder, error) {
	if len(entries) == 0 {
		var err error
		entries, err = LoadDefault()
		if err != nil {
			return nil, fmt.Errorf("load default faq for responder: %w", err)
		}
	}

	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	return &FAQResponder{
		entries:   entries,
		threshold: threshold,
	}, nil
}

// Respond implements the Responder interface. If a question in the FAQ matches
// the request text with a score at or above the threshold, handled is true.
func (r *FAQResponder) Respond(ctx context.Context, req Request) (Response, bool, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, false, err
	}

	entry, _, matched := r.FindMatch(req.Text)
	if !matched {
		return Response{}, false, nil
	}

	return Response{Text: entry.Answer}, true, nil
}

// Name returns the identifier for this responder.
func (r *FAQResponder) Name() string {
	return "faq"
}

// FindMatch searches for the highest scoring FAQ entry for the given query.
// Returns the best entry, its score, and whether the score met the threshold.
func (r *FAQResponder) FindMatch(text string) (FAQEntry, float64, bool) {
	var bestEntry FAQEntry
	var bestScore float64

	for _, entry := range r.entries {
		for _, q := range entry.Questions {
			score := ScoreMatch(text, q)
			if score > bestScore {
				bestScore = score
				bestEntry = entry
			}
		}
	}

	if bestScore >= r.threshold {
		return bestEntry, bestScore, true
	}

	return FAQEntry{}, bestScore, false
}

// NormalizeString converts text to lowercase and strips punctuation and symbols.
func NormalizeString(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			sb.WriteRune(' ')
		} else {
			sb.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// Tokenize normalizes text and splits it into tokens, optionally filtering stopwords.
// If all tokens would be filtered out as stopwords, the original non-empty tokens are preserved.
func Tokenize(s string, filterStopwords bool) []string {
	normalized := NormalizeString(s)
	words := strings.Fields(normalized)
	if !filterStopwords {
		return words
	}

	var filtered []string
	for _, w := range words {
		if _, isStop := defaultStopwords[w]; !isStop {
			filtered = append(filtered, w)
		}
	}

	if len(filtered) == 0 {
		return words
	}
	return filtered
}

// Levenshtein computes the edit distance between two strings using runes.
func Levenshtein(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	dp := make([]int, m+1)
	for j := 0; j <= m; j++ {
		dp[j] = j
	}

	for i := 1; i <= n; i++ {
		prev := dp[0]
		dp[0] = i
		for j := 1; j <= m; j++ {
			temp := dp[j]
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			dp[j] = min(dp[j]+1, dp[j-1]+1, prev+cost)
			prev = temp
		}
	}

	return dp[m]
}

// LevenshteinSimilarity computes a normalized similarity score in [0.0, 1.0].
func LevenshteinSimilarity(s1, s2 string) float64 {
	r1, r2 := []rune(s1), []rune(s2)
	maxLen := max(len(r1), len(r2))
	if maxLen == 0 {
		return 1.0
	}
	d := Levenshtein(s1, s2)
	return 1.0 - float64(d)/float64(maxLen)
}

// TokenOverlap calculates a soft Dice token overlap score in [0.0, 1.0],
// allowing for minor spelling variations between tokens.
func TokenOverlap(uTokens, qTokens []string) float64 {
	if len(uTokens) == 0 && len(qTokens) == 0 {
		return 1.0
	}
	if len(uTokens) == 0 || len(qTokens) == 0 {
		return 0.0
	}

	usedQ := make([]bool, len(qTokens))
	var matchScore float64

	for _, u := range uTokens {
		bestSim := 0.0
		bestIdx := -1

		for j, q := range qTokens {
			if usedQ[j] {
				continue
			}

			sim := 0.0
			if u == q {
				sim = 1.0
			} else {
				levSim := LevenshteinSimilarity(u, q)
				if levSim >= 0.70 {
					sim = levSim
				}
			}

			if sim > bestSim {
				bestSim = sim
				bestIdx = j
			}
		}

		if bestIdx >= 0 && bestSim > 0 {
			usedQ[bestIdx] = true
			matchScore += bestSim
		}
	}

	return (2.0 * matchScore) / float64(len(uTokens)+len(qTokens))
}

// ScoreMatch calculates a combined score in [0.0, 1.0] using token overlap
// plus string Levenshtein similarity.
func ScoreMatch(uText, qText string) float64 {
	uNorm := NormalizeString(uText)
	qNorm := NormalizeString(qText)

	if uNorm == "" || qNorm == "" {
		return 0.0
	}

	if uNorm == qNorm {
		return 1.0
	}

	uTokens := Tokenize(uText, true)
	qTokens := Tokenize(qText, true)

	overlap := TokenOverlap(uTokens, qTokens)
	levSim := LevenshteinSimilarity(uNorm, qNorm)

	return (overlap + levSim) / 2.0
}
