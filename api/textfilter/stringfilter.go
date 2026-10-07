package textfilter

import (
	"unicode"
	"unicode/utf8"
)

type stringFilter struct {
	baseNode *letterNode
	reals    map[rune][]rune
}

func newStringFilter(standins map[rune][]rune, strings []string) *stringFilter {
	baseNode := newLetterNode(0)

	// Add each string to the filter
	for _, str := range strings {
		prevNode := baseNode

		// Add letters to tree as needed
		for i, r := range str {
			// Add a branch for this letter,
			// if there wasn't one already
			node := prevNode.connect(r)

			// If it last letter of string,
			// mark it as end node.
			if i+1 == len(str) {
				node.end = true
			}

			// Set current node as prev node
			prevNode = node
		}
	}

	// Invert direction of standins.
	// Right now it's real->fakes, we want
	// fake->reals (to map to reals during scan)
	reals := make(map[rune][]rune)
	for real, fakes := range standins {
		for _, fake := range fakes {
			reals[fake] = append(reals[fake], real)
		}
	}

	return &stringFilter{baseNode, reals}
}

// Recursive function that handles the logic of
// checking for a bad prefix.
func (sf *stringFilter) hasBadPrefixStep(prevNode *letterNode, text string, depth int, farthest int) int {
	// If string is empty, there is nothing to find
	// Just return early
	if text == "" {
		return farthest
	}

	// Decode the rune we will be working with
	r, size := utf8.DecodeRuneInString(text)

	// Work out params for next func. call
	nextDepth := depth + 1
	nextText := text[size:]

	// Check if it's a space
	if unicode.IsSpace(r) {
		// It is, continue searching..
		end := sf.hasBadPrefixStep(prevNode, nextText, nextDepth, farthest)
		if end > farthest {
			return end
		}
	}

	// Get list of reals
	// (This not optimal!!)
	reals := sf.reals[r]
	reals = append(reals, r)

	// Test out the possible real lettes
	for _, real := range reals {
		// Is it the same as last real we went with?
		if real == prevNode.letter {
			// Yes so it's a repeat.
			// It could be an attempt to evade the filter,
			// so even if we already have an end node, we need
			// to check it
			if farthest != 0 {
				// If we already saw the end, this is a repeat of the end.
				// Because it is farther than the old end, it is the new end.
				farthest = nextDepth
			}
			if end := sf.hasBadPrefixStep(prevNode, nextText, nextDepth, farthest); end > farthest {
				farthest = end
			}
		}

		// Is there a branch node for this letter?
		node := prevNode.get(real)
		if node == nil {
			// No there is not.
			continue
		}

		// If this node itself is an end, also
		// make sure to note it down if it's the farthest yet
		if node.end && nextDepth > farthest {
			farthest = nextDepth
		}

		// Try to continue scanning for a word down this path
		if end := sf.hasBadPrefixStep(node, nextText, nextDepth, farthest); end > farthest {
			farthest = end
		}
	}

	// Return what we found, if anything
	return farthest
}

// Check if the start of a string matches with any of the
// strings that we want to filter out.
func (sf *stringFilter) hasBadPrefix(text string) int {
	return sf.hasBadPrefixStep(sf.baseNode, text, 0, 0)
}

// A start and end index of characters.
type index struct {
	start int
	end   int
}

// Find indices of bad strings.
func (sf *stringFilter) indicesOf(text string, offset int) (indices []index) {
	for i := 0; i < len(text); i++ {
		// Get segment of text, if it begins
		// with whitespace skip it.
		segment := text[i:]
		r, _ := utf8.DecodeRuneInString(segment)
		if unicode.IsSpace(r) {
			continue
		}
		// If we find a bad string, add it to indices
		// We still scan through parts we already know are partially
		// filtered, because we might find a longer bad string.
		// (e.g. if "abcde" and "bcdef" were filtered, skipping forward
		// while filtering "abcdef" would result in the "f" from "bcdef"
		// getting missed)
		if end := sf.hasBadPrefix(text[i:]); end != 0 {
			// Calculate start and end
			start := offset + i
			end := start + end

			// If we overlap the last index, expand it
			last := len(indices) - 1
			if last >= 0 && indices[last].end >= start && end > indices[last].end {
				indices[last].end = end
			} else if last < 0 || start > indices[last].end {
				// We don't so start a new index
				indices = append(indices, index{start, end})
			}
		}
	}

	return
}
