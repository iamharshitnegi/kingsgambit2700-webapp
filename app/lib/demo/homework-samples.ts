import type { Assignment, LearningAttempt, LearningMetrics, Puzzle } from "../academy-types";

const emptyMetrics = (): LearningMetrics => ({ completed: 0, firstTry: 0, averageSeconds: null, inProgress: 0, notSolved: 0, notAttempted: 0, accuracy: null, incorrect: 0, hints: 0, reveals: 0, seconds: 0 });

export function sampleHomework(studentId: string, at = Date.now()): Assignment[] {
  const due = (days: number) => { const date = new Date(at); date.setDate(date.getDate() + days); date.setHours(20, 0, 0, 0); return date.toISOString(); };
  const puzzles: Puzzle[] = [
    { title: "A back-rank finish", fen: "6k1/5ppp/8/8/8/8/5PPP/4R1K1 w - - 0 1", solutions: [["e1e8"]] },
    { title: "Queen and king, together", fen: "7k/6pp/5KQ1/8/8/8/8/8 w - - 0 1", solutions: [["g6g7"]] },
    { title: "Close the escape squares", fen: "7k/7p/5K2/8/8/8/8/R7 w - - 0 1", solutions: [["a1a8"]] },
    { title: "Black gets the last word", fen: "4r1k1/5ppp/8/8/8/8/5PPP/6K1 b - - 0 1", solutions: [["e8e1"]] },
    { title: "An open file", fen: "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1", solutions: [["a1a8"]] },
  ];
  function assignment(id: string, title: string, kind: string, days: number, description: string, status = "assigned"): Assignment {
    return { id: `demo-homework-${id}`, studentId, title, kind, description, status, version: 1, dueAt: due(days), puzzles: [], resources: [], attempts: [] };
  }
  function attempt(item: Assignment, status: string, answer = "", feedback = "", solved = 0): LearningAttempt {
    return {
      id: `${item.id}-attempt-1`, number: 1, status, answer, feedback, files: [],
      puzzles: item.puzzles.map((puzzle, index) => ({ moves: index < solved ? [...puzzle.solutions![0]] : [], incorrect: 0, hints: 0, seconds: index < solved ? 35 : 0, solved: index < solved, revealed: false, lastActionAt: index < solved ? new Date(at - 3600000).toISOString() : "0001-01-01T00:00:00Z" })),
      metrics: { ...emptyMetrics(), completed: solved, firstTry: solved, notAttempted: item.puzzles.length - solved, seconds: solved * 35, accuracy: solved ? 100 : null, averageSeconds: solved ? 35 : null },
    };
  }
  const tactics = assignment("tactics", "The art of the checkmate", "puzzle_set", 0, "Look for forcing moves. Find the check that leaves your opponent with no escape.\n\nSolve all 5 positions. Check every escape square before you move, then share what you noticed.");
  tactics.puzzles = puzzles;
  tactics.attempts = [attempt(tactics, "started", "", "", 2)];
  const review = assignment("review", "Find the turning point", "game_review", -1, "Think back to a recent game. Describe one decision that changed the position, explain your opponent’s threat, and suggest a better plan. You can attach your game as a PGN.");
  review.attempts = [attempt(review, "returned", "I moved my queen to attack, but left my back rank undefended.", "Good start. You found the critical moment. Now look at your opponent’s threat before choosing your next move. What would you change?")];
  const endgame = assignment("endgame", "King & pawn: the opposition", "study", 2, "Imagine the white king on e4 and the black king on e6. Compare the position with White to move and Black to move. Explain why the side to move has to give up the opposition.");
  endgame.resources = [{ id: "demo-opposition-note", title: "Understanding the opposition", kind: "study", version: 1, content: "When two kings face each other with one square between them, the side to move must give way. The other side has the opposition. Use this idea to help your king reach important squares in pawn endings." }];
  const opening = assignment("opening", "A plan for your first 10 moves", "study", 4, "List three goals for the opening. Choose which pieces you would develop first. Explain when you would castle and why.");
  const submitted = assignment("submitted", "Checks, captures, threats", "other", -2, "Describe a simple routine for finding candidate moves. Consider the opponent’s replies as well as your own ideas.");
  submitted.attempts = [attempt(submitted, "submitted", "I look for checks, then captures, then threats for both sides before choosing a move.")];
  const completed = assignment("completed", "Give your king some breathing room", "study", -4, "Recognise a back-rank weakness and explain how to give your king an escape square.");
  completed.attempts = [attempt(completed, "reviewed", "A move like h3 can create an escape square, but I should check for immediate threats first.", "Clear thinking. You explained both the threat and the solution. Keep checking your king’s escape squares in every game.")];
  const evidence = assignment("evidence", "Share a game you’re proud of", "file_upload", 5, "Upload a PGN, a board image, or a PDF of a recent game. Add a short reflection on one move that made you proud and one decision you want to improve.");
  const draft = assignment("draft", "Build a better thinking routine", "other", 7, "After your next game, describe how checking your opponent’s threats changed the moves you considered.", "draft");
  return [tactics, review, endgame, opening, submitted, completed, evidence, draft];
}
