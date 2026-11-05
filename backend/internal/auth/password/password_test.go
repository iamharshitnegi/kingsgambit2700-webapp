package password

import (
	"strings"
	"testing"
)

func TestSaltedPasswordHash(t *testing.T) {
	value := "a long memorable passphrase"
	first, err := Hash(value)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Hash(value)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !Verify(first, value) || Verify(first, value+"!") || Verify(first, strings.Repeat("a", 513)) {
		t.Fatal("password salt, match, or mismatch handling failed")
	}
	for _, invalid := range []string{"", "plain text", "$argon2id$v=19$m=999999999,t=99,p=1$invalid$invalid", prefix + "bad$hash"} {
		if Verify(invalid, value) {
			t.Fatal("accepted malformed hash")
		}
	}
	if _, err := Hash("short"); err == nil {
		t.Fatal("accepted short password")
	}
}
