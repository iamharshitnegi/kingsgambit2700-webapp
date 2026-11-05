package service

import "testing"

func TestSafeRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "relative portal path", value: "/guardian?tab=schedule", want: "/guardian?tab=schedule"},
		{name: "cleans path segments", value: "/guardian/../portal", want: "/portal"},
		{name: "absolute URL", value: "https://attacker.example/", want: "/portal"},
		{name: "network path", value: "//attacker.example/", want: "/portal"},
		{name: "backslash network path", value: "/\\attacker.example/", want: "/portal"},
		{name: "encoded backslash network path", value: "/%5cattacker.example/", want: "/portal"},
		{name: "header control character", value: "/portal\r\nLocation: https://attacker.example", want: "/portal"},
		{name: "fragment", value: "/portal#token", want: "/portal"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := safeRedirect(test.value); got != test.want {
				t.Fatalf("safeRedirect(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}
