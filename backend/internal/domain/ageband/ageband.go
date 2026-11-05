package ageband

type Value string

const (
	Age4To7   Value = "4_7"
	Age8To12  Value = "8_12"
	Age13To17 Value = "13_17"
	Age18Plus Value = "18_plus"
)

func Valid(value string) bool {
	switch Value(value) {
	case Age4To7, Age8To12, Age13To17, Age18Plus:
		return true
	default:
		return false
	}
}
