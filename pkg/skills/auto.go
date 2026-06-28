package skills

import (
	"strings"
)

// MatchScore represents the relevance of a skill to a user prompt.
type MatchScore struct {
	Skill   Skill
	Score   float64
	Reasons []string
}

// AutoDetect selects the best matching skill for a user prompt.
// It uses simple keyword heuristics. A value of 0 means no confident match.
func AutoDetect(prompt string, skills []Skill) (MatchScore, bool) {
	lower := strings.ToLower(prompt)
	var best MatchScore

	for _, skill := range skills {
		score, reasons := scoreSkill(lower, skill)
		if score > best.Score {
			best = MatchScore{Skill: skill, Score: score, Reasons: reasons}
		}
	}

	// Require a minimum confidence before activating a skill automatically.
	if best.Score < 0.3 {
		return MatchScore{}, false
	}
	return best, true
}

func scoreSkill(prompt string, skill Skill) (float64, []string) {
	var score float64
	var reasons []string

	// Match against skill name.
	name := strings.ToLower(skill.Name)
	if strings.Contains(prompt, name) {
		score += 0.5
		reasons = append(reasons, "name match")
	}

	// Match against when_to_use.
	when := strings.ToLower(skill.WhenToUse)
	for _, word := range tokenize(when) {
		if len(word) > 3 && strings.Contains(prompt, word) {
			score += 0.2
			reasons = append(reasons, "purpose keyword")
			break
		}
	}

	// Match against examples.
	for _, ex := range skill.Examples {
		exLower := strings.ToLower(ex)
		for _, word := range tokenize(exLower) {
			if len(word) > 3 && strings.Contains(prompt, word) {
				score += 0.15
				reasons = append(reasons, "example keyword")
				break
			}
		}
	}

	// Match against steps.
	for _, step := range skill.Steps {
		stepLower := strings.ToLower(step)
		for _, word := range tokenize(stepLower) {
			if len(word) > 3 && strings.Contains(prompt, word) {
				score += 0.1
				reasons = append(reasons, "step keyword")
				break
			}
		}
	}

	return score, reasons
}

func tokenize(text string) []string {
	replacer := strings.NewReplacer(
		",", " ",
		".", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
	)
	text = replacer.Replace(text)
	var tokens []string
	for _, p := range strings.Fields(text) {
		tokens = append(tokens, strings.ToLower(p))
	}
	return tokens
}
