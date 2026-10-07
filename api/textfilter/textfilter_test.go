package textfilter

import (
	"regexp"
	"testing"
)

func newTestTextFilter() *TextFilter {
	tf := New(
		map[rune][]rune{'e': {'c', '3'}, 'l': {'i', '1'}, 'o': {'c', '0', 'e'}, 'i': {'1'}, 'a': {'4', '@'}, 's': {'5'}},
		[]string{"hello", "hi", "hai", "haii", "world", "what", "this", "is", "test", "hii", "hiii", "hia"},
		[]string{"test"},
	)
	return tf
}

func newTestRegexp() *regexp.Regexp {
	return regexp.MustCompile(`([h]+\s*[ec3]+\s*[li1]+\s*[li1]+\s*[oc0](\s*[oc0])*)|([h]+\s*[i1](\s*[i1])*)|([h]+\s*[a4@]+\s*[i1](\s*[i1])*)|([h]+\s*[a4@]+\s*[i1]+\s*[i1](\s*[i1])*)|([w]+\s*[oc0]+\s*[r]+\s*[li1]+\s*[d](\s*[d])*)|([w]+\s*[h]+\s*[a4@]+\s*[t](\s*[t])*)|([t]+\s*[h]+\s*[i1]+\s*[s5](\s*[s5])*)|([i1]+\s*[s5](\s*[s5])*)|([t]+\s*[ec3]+\s*[s5]+\s*[t](\s*[t])*)|([h]+\s*[i1]+\s*[i1](\s*[i1])*)|([h]+\s*[i1]+\s*[i1]+\s*[i1](\s*[i1])*)|([h]+\s*[i1]+\s*[a4@](\s*[a4@])*)`)
}

const (
	strFind   = "this s s s s is a test"
	strNoFind = "that is not though"
)

func BenchmarkContainsAnyFind(b *testing.B) {
	tf := newTestTextFilter()

	for b.Loop() {
		tf.ContainsAny(strFind)
	}
}

func BenchmarkRegexEquivalentFind(b *testing.B) {
	re := newTestRegexp()

	for b.Loop() {
		re.FindAllStringIndex(strFind, -1)
	}
}

func BenchmarkRegexEquivalentNoFind(b *testing.B) {
	re := newTestRegexp()

	for b.Loop() {
		re.FindAllStringIndex(strNoFind, -1)
	}
}

func BenchmarkContainsAnyNoFind(b *testing.B) {
	tf := newTestTextFilter()

	for b.Loop() {
		tf.ContainsAny(strNoFind)
	}
}
