package llm

import "strings"

// repetitionDetector watches a growing stream of text for the telltale sign of
// a looping model: the same phrase or paragraph repeated back-to-back at the
// end of the output. Local models in particular can get stuck emitting the
// same block forever; the provider uses a rejected Accept to cut the stream
// and retry the request (see retryMessagesAfterRepetition).
type repetitionDetector struct {
	// tail is the last repetitionWindowRunes of accepted text — enough history
	// to detect repeats without holding the whole response in memory twice.
	tail string
}

// Two detection profiles: "short" catches a small phrase repeated several
// times in a row, "long" catches a whole paragraph repeated twice. The
// MinUnitChars floors stop tiny common phrases ("of the of the") or
// legitimately repeated list stems from tripping the detector.
const (
	repetitionWindowRunes       = 12000
	repetitionWindowWords       = 900
	repetitionShortMinUnitWords = 6
	repetitionShortMaxUnitWords = 80
	repetitionShortRepeatCount  = 3
	repetitionShortMinUnitChars = 30
	repetitionLongMinUnitWords  = 40
	repetitionLongMaxUnitWords  = 300
	repetitionLongRepeatCount   = 2
	repetitionLongMinUnitChars  = 240
)

// Accept appends next to the tracked window and reports whether the stream is
// still repetition-free. A false return means the caller should stop streaming.
func (d *repetitionDetector) Accept(next string) bool {
	if next == "" {
		return true
	}
	candidate := d.tail + next
	if hasRepeatedSuffix(candidate) {
		return false
	}
	d.tail = tailRunes(candidate, repetitionWindowRunes)
	return true
}

// hasRepeatedSuffix checks whether the text currently ends in a repeated unit
// under either the short-phrase or long-paragraph profile. Comparison is done
// on lowercased whitespace-split words so formatting differences don't hide a
// loop.
func hasRepeatedSuffix(text string) bool {
	words := repetitionWords(text)
	if len(words) > repetitionWindowWords {
		words = words[len(words)-repetitionWindowWords:]
	}

	return hasRepeatedWordSuffix(words, repetitionShortMinUnitWords, repetitionShortMaxUnitWords, repetitionShortRepeatCount, repetitionShortMinUnitChars) ||
		hasRepeatedWordSuffix(words, repetitionLongMinUnitWords, repetitionLongMaxUnitWords, repetitionLongRepeatCount, repetitionLongMinUnitChars)
}

// hasRepeatedWordSuffix tries every candidate unit length in
// [minUnitWords, maxUnitWords] and reports whether the final unit of that
// length appears repeatCount times consecutively at the end of words.
func hasRepeatedWordSuffix(words []string, minUnitWords int, maxUnitWords int, repeatCount int, minUnitChars int) bool {
	if len(words) < minUnitWords*repeatCount {
		return false
	}

	maxUnitWords = min(maxUnitWords, len(words)/repeatCount)
	for unitWords := minUnitWords; unitWords <= maxUnitWords; unitWords++ {
		if !repeatedWordSuffix(words, unitWords, repeatCount) {
			continue
		}
		unitText := strings.Join(words[len(words)-unitWords:], " ")
		if len(unitText) >= minUnitChars {
			return true
		}
	}
	return false
}

func repetitionWords(text string) []string {
	return strings.Fields(strings.ToLower(text))
}

// repeatedWordSuffix reports whether the last unitWords words appear
// repeatCount times in a row at the end of words, comparing each earlier
// repeat word-by-word against the final occurrence.
func repeatedWordSuffix(words []string, unitWords int, repeatCount int) bool {
	end := len(words)
	baseStart := end - unitWords
	for repeat := 2; repeat <= repeatCount; repeat++ {
		start := end - unitWords*repeat
		for offset := 0; offset < unitWords; offset++ {
			if words[start+offset] != words[baseStart+offset] {
				return false
			}
		}
	}
	return true
}

func tailRunes(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[len(runes)-maxRunes:])
}
