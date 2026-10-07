package textfilter

import (
	"slices"
	"unicode"
)

type TextFilter struct {
	stringLayer *stringFilter
	wordLayer   *stringFilter
}

func New(standins map[rune][]rune, strings []string, words []string) *TextFilter {
	stringLayer := newStringFilter(standins, strings)
	wordLayer := newStringFilter(standins, words)

	return &TextFilter{stringLayer, wordLayer}
}

// Consolidate a list of ranges into a smaller amount.
// We look for ranges that overlap and turn them into
// bigger ranges, so that we can use them for editing
// text strings without any problems
func consolidateRanges(ranges []index) []index {
	// If there are no ranges, there is no work to do
	if len(ranges) == 0 {
		return ranges
	}

	// Sort all of the ranges by their start position
	slices.SortFunc(ranges, func(a, b index) int {
		switch {
		case a.start < b.start:
			return -1
		case a.start > b.start:
			return 1
		default:
			return 0
		}
	})

	// Start off with the first range
	consolidated := []index{ranges[0]}
	for _, new := range ranges {
		cur := &consolidated[len(consolidated)-1]

		// If we start within or adjacent to the last range
		if new.start <= cur.end {
			// And we end outside of the last range
			if new.end > cur.end {
				// That means we overlap it, and we should
				// extend that range
				cur.end = new.end
			}
		} else {
			// We are completely separate from the last range
			// Add it on its own
			consolidated = append(consolidated, new)
		}
	}

	return consolidated
}

// Find indices of bad strings and words.
func (tf *TextFilter) indicesOf(text string) (indices []index) {
	// First get indices of strings
	indices = tf.stringLayer.indicesOf(text, 0)

	// Scan each word individually
	var wordStart int
	for i, r := range text {
		// Is this the end of a word?
		// (or, the entire string)
		isEnd := i+1 == len(text)
		if unicode.IsSpace(r) || isEnd {
			// Extract the word.
			// If it is a space, we do not want
			// to include it in the string
			wordEnd := i
			if isEnd {
				wordEnd += 1
			}
			word := text[wordStart:wordEnd]

			// If the word is not empty, scan it
			if word != "" {
				newIndices := tf.wordLayer.indicesOf(word, wordStart)
				// If the whole span of the word was filtered,
				// add it to our list
				if len(newIndices) == 1 && newIndices[0].end-newIndices[0].start == len(word) {
					indices = append(indices, newIndices...)
				}
			}

			// Start new word..
			wordStart = i + 1
		}
	}

	// Return a consolidated list of indices
	return consolidateRanges(indices)
}

// Whether or not the string would get filtered.
// Maybe this should return what triggered the filter?
// It's meant for filtering usernames, and maybe deciding
// if a chat message should get a user instantly muted/banned
func (tf *TextFilter) ContainsAny(text string) bool {
	return len(tf.indicesOf(text)) != 0
}

// Swap filtered text out with a replacement string.
func (tf *TextFilter) ReplaceAll(text, replacement string, repeat bool) string {
	// Find all indices we need to replace
	indices := tf.indicesOf(text)

	// Fast path for text we didn't find anything in
	if len(indices) == 0 {
		return text
	}

	// Reverse list. If we iterate over the list
	// front-to-back, the string will turn into non-sense
	// because replacing text with anything of differing
	// length will shift the indices.
	slices.Reverse(indices)

	// Replace parts that need to be replaced
	textBytes := []byte(text)
	replacementBytes := []byte(replacement)
	for _, idx := range indices {
		// If we're meant to repeat the replacement, do that now
		replacementBytes := replacementBytes
		if repeat {
			size := idx.end - idx.start
			replacementBytes = slices.Repeat(replacementBytes, size)
		}
		// Replace that range
		textBytes = slices.Replace(textBytes, idx.start, idx.end, replacementBytes...)
	}

	// Return new string
	return string(textBytes)
}
