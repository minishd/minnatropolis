package textfilter

// Representation of a letter, with a list of
// any letters that can come after this one.
//
// Each letter node is unique to what letters
// came before it. So the "t" node in "bat" is not
// the same one as "cat", each "p" in "apple" has
// a different node ("ap" vs "app"), etc.
type letterNode struct {
	// Whether or not any bad string ends at this node.
	// If we encounter an end node during a scan,
	// we know we have found a bad string, and we should
	// return info about it.
	end bool
	// What letters come after this letter, if any.
	next map[rune]*letterNode
	// What letter this letter node represents.
	// This is used when scanning to catch repeat
	// letters.
	letter rune
}

func newLetterNode(letter rune) *letterNode {
	next := make(map[rune]*letterNode)
	return &letterNode{next: next, letter: letter}
}

// Gets or creates a branch node for a letter.
func (node *letterNode) connect(r rune) *letterNode {
	branch := node.next[r]
	if branch == nil {
		branch = newLetterNode(r)
		node.next[r] = branch
	}
	return branch
}

// Get the node for a letter.
func (node *letterNode) get(r rune) *letterNode {
	return node.next[r]
}
