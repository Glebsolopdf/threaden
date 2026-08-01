package antispam

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"voice-rooms/internal/model"
)

func countSimilarMessages(body string, messages []model.GroupMessage) (int, int) {
	target := fingerprint(body)
	if target == "" {
		return 0, 0
	}
	users := map[string]struct{}{}
	similar := 1
	for _, item := range messages {
		if !nearlySame(target, fingerprint(item.Body)) {
			continue
		}
		similar++
		users[item.Author.ID] = struct{}{}
	}
	return similar, len(users)
}

func nearlySame(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if left == right || strings.ReplaceAll(left, " ", "") == strings.ReplaceAll(right, " ", "") {
		return true
	}
	if tokenScore(left, right) >= 0.84 {
		return true
	}
	diff := utf8.RuneCountInString(left) - utf8.RuneCountInString(right)
	if diff < 0 {
		diff = -diff
	}
	return diff <= 4 && (strings.Contains(left, right) || strings.Contains(right, left))
}

func fingerprint(body string) string {
	body = mentionPattern.ReplaceAllString(linkPattern.ReplaceAllString(strings.ToLower(body), " link "), " mention ")
	var b strings.Builder
	lastSpace, lastRune, run := true, rune(0), 0
	for _, r := range body {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if r == lastRune {
				run++
				if run > 2 {
					continue
				}
			} else {
				lastRune, run = r, 1
			}
			b.WriteRune(r)
			lastSpace = false
		case !lastSpace:
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func tokenScore(left, right string) float64 {
	leftSet, rightSet := map[string]struct{}{}, map[string]struct{}{}
	for _, part := range strings.Fields(left) {
		if utf8.RuneCountInString(part) > 2 {
			leftSet[part] = struct{}{}
		}
	}
	for _, part := range strings.Fields(right) {
		if utf8.RuneCountInString(part) > 2 {
			rightSet[part] = struct{}{}
		}
	}
	if len(leftSet) == 0 || len(rightSet) == 0 {
		return 0
	}
	shared := 0
	for part := range leftSet {
		if _, ok := rightSet[part]; ok {
			shared++
		}
	}
	size := len(leftSet)
	if len(rightSet) > size {
		size = len(rightSet)
	}
	return float64(shared) / float64(size)
}
