package learning

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	chess "github.com/corentings/chess/v2"
)

func invalid(detail string) error         { return ValidationError{Detail: detail} }
func required(value string, max int) bool { return strings.TrimSpace(value) != "" && len(value) <= max }

// ValidatePGN parses every game, including variations, under a bounded input size.
func ValidatePGN(value string) error {
	if !required(value, 512<<10) {
		return invalid("PGN must contain text and be smaller than 512 KB")
	}
	if strings.Count(value, "(") > 1000 {
		return invalid("PGN contains too many variations")
	}
	scanner := chess.NewScanner(strings.NewReader(value))
	count := 0
	for {
		game, err := scanner.ParseNext()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return invalid("PGN contains an invalid game or move")
		}
		count++
		if count > 500 || len(game.Moves()) > 2048 {
			return invalid("PGN exceeds the game or move limit")
		}
	}
	if count == 0 {
		return invalid("PGN contains no games")
	}
	return nil
}
func validatePuzzles(puzzles []Puzzle) error {
	if len(puzzles) > 100 {
		return invalid("use at most 100 puzzles per assignment")
	}
	for _, p := range puzzles {
		if !required(p.Title, 200) || len(p.Solutions) == 0 || len(p.Solutions) > 20 {
			return invalid("each puzzle needs a title and 1–20 solution lines")
		}
		option, err := chess.FEN(p.FEN)
		if err != nil {
			return invalid("puzzle position is invalid")
		}
		for _, line := range p.Solutions {
			if len(line) == 0 || len(line) > 64 {
				return invalid("solution lines must contain 1–64 moves")
			}
			g := chess.NewGame(option)
			for _, move := range line {
				if err := g.PushNotationMove(move, chess.UCINotation{}, nil); err != nil {
					return invalid("puzzle solution includes an illegal move")
				}
			}
		}
	}
	return nil
}
func validateAssignment(c Command) error {
	if !required(c.Title, 200) || !required(c.Description, 10000) {
		return invalid("title and instructions are required")
	}
	if !slices.Contains([]string{"puzzle_set", "game_review", "study", "file_upload", "other"}, c.Kind) {
		return invalid("choose puzzle_set, game_review, study, file_upload or other")
	}
	if c.Kind == "puzzle_set" && len(c.Puzzles) == 0 {
		return invalid("add at least one puzzle")
	}
	return validatePuzzles(c.Puzzles)
}
func applyPuzzle(p Puzzle, progress PuzzleProgress, c Command, now time.Time) (PuzzleProgress, error) {
	if progress.Solved || progress.Revealed {
		return progress, ErrConflict
	}
	elapsed := c.ElapsedSeconds
	if elapsed < 0 || elapsed > 60 {
		return progress, invalid("elapsedSeconds must be between 0 and 60")
	}
	if !progress.LastActionAt.IsZero() {
		elapsed = min(elapsed, max(0, int(now.Sub(progress.LastActionAt).Seconds())))
	} else {
		elapsed = 0
	}
	progress.Seconds += elapsed
	progress.LastActionAt = now
	if progress.Moves == nil {
		progress.Moves = []string{}
	}
	candidates := [][]string{}
	for _, line := range p.Solutions {
		if len(line) > len(progress.Moves) && slices.Equal(line[:len(progress.Moves)], progress.Moves) {
			candidates = append(candidates, line)
		}
	}
	if len(candidates) == 0 {
		return progress, invalid("puzzle has no continuation")
	}
	switch c.Action {
	case "puzzleHint":
		progress.Hints++
		progress.Hint = "Look at the piece on " + candidates[0][len(progress.Moves)][:2] + "."
	case "puzzleReveal":
		progress.Revealed = true
		progress.Hint = "Solution: " + strings.Join(candidates[0], " ")
	case "puzzleMove":
		next := append(slices.Clone(progress.Moves), strings.ToLower(strings.TrimSpace(c.Move)))
		accepted := false
		for _, line := range candidates {
			if slices.Equal(line[:len(next)], next) {
				accepted = true
				if len(next) == len(line) {
					progress.Solved = true
				}
				break
			}
		}
		if !accepted {
			progress.Incorrect++
			progress.Hint = "Try another move."
			break
		}
		progress.Moves = next
		progress.Hint = ""
		// A puzzle asks the learner for their side's moves; play the authored reply automatically.
		if !progress.Solved {
			for _, line := range candidates {
				if len(line) > len(next) && slices.Equal(line[:len(next)], next) {
					progress.Moves = append(progress.Moves, line[len(next)])
					progress.Solved = len(progress.Moves) == len(line)
					break
				}
			}
		}
	default:
		return progress, invalid("unknown puzzle action")
	}
	return progress, nil
}
func metrics(progress []PuzzleProgress, total int, finalized bool) Metrics {
	result := Metrics{NotAttempted: total}
	for _, p := range progress {
		if p.LastActionAt.IsZero() {
			continue
		}
		result.NotAttempted--
		if p.Solved && !p.Revealed {
			result.Completed++
			if p.Incorrect == 0 && p.Hints == 0 {
				result.FirstTry++
			}
		} else if p.Revealed || finalized {
			result.NotSolved++
		} else {
			result.InProgress++
		}
		result.Incorrect += p.Incorrect
		result.Hints += p.Hints
		result.Seconds += p.Seconds
		if p.Revealed {
			result.Reveals++
		}
	}
	attempted := result.Completed + result.NotSolved
	if attempted > 0 {
		accuracy := 100 * float64(result.Completed) / float64(attempted)
		result.Accuracy = &accuracy
	}
	if practiced := attempted + result.InProgress; practiced > 0 {
		average := float64(result.Seconds) / float64(practiced)
		result.AverageSeconds = &average
	}
	return result
}
func validateAssessment(c Command, dimensions []string) error {
	if !required(c.Title, 200) {
		return invalid("assessment title is required")
	}
	if _, err := time.Parse("2006-01", c.Period); err != nil {
		return invalid("period must be YYYY-MM")
	}
	body := c.AssessmentContent
	for _, text := range []string{body.Learned, body.Strengths, body.FocusNext, body.PracticePlan, body.CoachComment} {
		if !required(text, 5000) {
			return invalid("complete all five feedback sections")
		}
	}
	if len(body.Ratings) != len(dimensions) {
		return invalid("assess every rubric dimension")
	}
	seen := map[string]bool{}
	for _, rating := range body.Ratings {
		if !slices.Contains(dimensions, rating.Dimension) || seen[rating.Dimension] {
			return invalid("rubric dimension is invalid or repeated")
		}
		seen[rating.Dimension] = true
		if !slices.Contains([]string{"not_assessed", "emerging", "developing", "secure", "extending"}, rating.Rating) {
			return invalid("rubric rating is invalid")
		}
		if !required(rating.Evidence, 3000) {
			return invalid(fmt.Sprintf("add evidence or an omission reason for %s", rating.Dimension))
		}
	}
	return nil
}
