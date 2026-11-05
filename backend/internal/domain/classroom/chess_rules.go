package classroom

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	chess "github.com/corentings/chess/v2"
)

var uciMovePattern = regexp.MustCompile(`^[a-h][1-8][a-h][1-8][qrbn]?$`)

// ChessRules is the production legality boundary. Wire commands use UCI so a
// move has one unambiguous representation across Go and browser clients.
type ChessRules struct{}

func (ChessRules) ApplyMove(ctx context.Context, fen, move string) (MoveResult, error) {
	if err := ctx.Err(); err != nil {
		return MoveResult{}, err
	}
	move = strings.ToLower(strings.TrimSpace(move))
	if !uciMovePattern.MatchString(move) {
		return MoveResult{}, ErrIllegalMove
	}
	option, err := chess.FEN(fen)
	if err != nil {
		return MoveResult{}, fmt.Errorf("%w: invalid fen: %v", ErrIllegalMove, err)
	}
	game := chess.NewGame(option)
	if err := game.PushNotationMove(move, chess.UCINotation{}, nil); err != nil {
		return MoveResult{}, fmt.Errorf("%w: %v", ErrIllegalMove, err)
	}
	return MoveResult{FEN: game.Position().String(), NormalizedMove: move}, nil
}

func (ChessRules) ValidatePosition(ctx context.Context, fen string, moves []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := chess.FEN(fen); err != nil {
		return fmt.Errorf("%w: invalid fen: %v", ErrInvalidBoard, err)
	}
	for _, move := range moves {
		if !uciMovePattern.MatchString(strings.ToLower(strings.TrimSpace(move))) {
			return fmt.Errorf("%w: invalid UCI move %q", ErrInvalidBoard, move)
		}
	}
	return nil
}
