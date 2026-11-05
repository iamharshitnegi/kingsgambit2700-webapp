package learning

import "testing"

func TestValidateFileRejectsDisguisedAndActiveDocuments(t *testing.T) {
	for _, test := range []struct {
		name, filename, body string
		valid                bool
	}{
		{"plain pdf", "lesson.pdf", "%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n%%EOF", true},
		{"disguised html", "lesson.pdf", "<html><script>alert(1)</script></html>", false},
		{"active pdf", "lesson.pdf", "%PDF-1.4\n/JavaScript (alert(1))\n%%EOF", false},
		{"svg", "lesson.svg", "<svg></svg>", false},
		{"bad image", "lesson.png", "not an image", false},
		{"valid pgn", "lesson.pgn", "[Event \"Study\"]\n\n1. e4 e5 *", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := validateFile(test.filename, []byte(test.body))
			if (err == nil) != test.valid {
				t.Fatalf("validateFile=%v, valid=%v", err, test.valid)
			}
		})
	}
}
