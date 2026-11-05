package learning

import (
	"testing"
	"time"
)

const initialFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func TestValidatePGN(t *testing.T) {
	for _, test := range []struct {
		name, text string
		valid      bool
	}{
		{"legal game", "[Event \"Lesson\"]\n\n1. e4 e5 2. Nf3 *", true},
		{"multiple games", "[Event \"One\"]\n\n1. e4 *\n\n[Event \"Two\"]\n\n1. d4 *", true},
		{"illegal move", "[Event \"Bad\"]\n\n1. e5 *", false},
		{"empty", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePGN(test.text)
			if (err == nil) != test.valid {
				t.Fatalf("ValidatePGN() = %v, valid=%v", err, test.valid)
			}
		})
	}
}
func TestPuzzleValidationAndAlternativeSolutions(t *testing.T) {
	p := Puzzle{Title: "Choose a centre pawn", FEN: initialFEN, Solutions: [][]string{{"e2e4"}, {"d2d4"}}}
	if err := validatePuzzles([]Puzzle{p}); err != nil {
		t.Fatal(err)
	}
	for _, move := range []string{"e2e4", "d2d4"} {
		t.Run(move, func(t *testing.T) {
			progress, err := applyPuzzle(p, PuzzleProgress{}, Command{Action: "puzzleMove", Move: move}, time.Now())
			if err != nil || !progress.Solved {
				t.Fatalf("alternative not accepted: %+v, %v", progress, err)
			}
		})
	}
	p.Solutions = [][]string{{"e2e5"}}
	if err := validatePuzzles([]Puzzle{p}); err == nil {
		t.Fatal("illegal authored solution accepted")
	}
}
func TestPuzzleRepliesHintsAndMetrics(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	p := Puzzle{Title: "Develop", FEN: initialFEN, Solutions: [][]string{{"e2e4", "e7e5", "g1f3"}}}
	progress, err := applyPuzzle(p, PuzzleProgress{}, Command{Action: "puzzleMove", Move: "e2e3", ElapsedSeconds: 60}, now)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Incorrect != 1 || progress.Seconds != 0 {
		t.Fatalf("first event counts wrong: %+v", progress)
	}
	progress, err = applyPuzzle(p, progress, Command{Action: "puzzleHint", ElapsedSeconds: 60}, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if progress.Hints != 1 || progress.Seconds != 3 {
		t.Fatalf("time not server-bounded: %+v", progress)
	}
	progress, err = applyPuzzle(p, progress, Command{Action: "puzzleMove", Move: "e2e4"}, now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.Moves) != 2 || progress.Moves[1] != "e7e5" || progress.Solved {
		t.Fatalf("reply was not applied correctly: %+v", progress)
	}
	progress, err = applyPuzzle(p, progress, Command{Action: "puzzleMove", Move: "g1f3"}, now.Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	m := metrics([]PuzzleProgress{progress, {}}, 2, true)
	if m.Completed != 1 || m.NotAttempted != 1 || m.Incorrect != 1 || m.Hints != 1 || m.FirstTry != 0 || m.Accuracy == nil || *m.Accuracy != 100 {
		t.Fatalf("wrong metrics: %+v", m)
	}
	if _, err = applyPuzzle(p, progress, Command{Action: "puzzleHint"}, now); err == nil {
		t.Fatal("completed puzzle accepted more hints")
	}
	revealed, err := applyPuzzle(p, PuzzleProgress{}, Command{Action: "puzzleReveal"}, now)
	if err != nil {
		t.Fatal(err)
	}
	m = metrics([]PuzzleProgress{revealed}, 1, true)
	if m.Completed != 0 || m.NotSolved != 1 || m.Reveals != 1 {
		t.Fatalf("reveal counted as a solution: %+v", m)
	}
}

func TestPracticeAccuracyExcludesUnfinishedPuzzles(t *testing.T) {
	progress := []PuzzleProgress{{LastActionAt: time.Now(), Incorrect: 1, Seconds: 12}, {}}
	active := metrics(progress, 2, false)
	if active.Accuracy != nil || active.NotSolved != 0 || active.InProgress != 1 || active.NotAttempted != 1 {
		t.Fatalf("unfinished accuracy = %+v", active)
	}
	submitted := metrics(progress, 2, true)
	if submitted.Accuracy == nil || *submitted.Accuracy != 0 || submitted.NotSolved != 1 || submitted.InProgress != 0 {
		t.Fatalf("submitted accuracy = %+v", submitted)
	}
}
