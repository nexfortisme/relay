package llm

import "strings"

type repetitionDetector struct {
	tail string
}

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

func hasRepeatedSuffix(text string) bool {
	words := repetitionWords(text)
	if len(words) > repetitionWindowWords {
		words = words[len(words)-repetitionWindowWords:]
	}

	return hasRepeatedWordSuffix(words, repetitionShortMinUnitWords, repetitionShortMaxUnitWords, repetitionShortRepeatCount, repetitionShortMinUnitChars) ||
		hasRepeatedWordSuffix(words, repetitionLongMinUnitWords, repetitionLongMaxUnitWords, repetitionLongRepeatCount, repetitionLongMinUnitChars)
}

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