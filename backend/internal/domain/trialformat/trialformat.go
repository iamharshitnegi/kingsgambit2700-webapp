package trialformat

type Value string

const (
	Group    Value = "group"
	OneToOne Value = "one_to_one"
)

func Valid(value string) bool {
	return Value(value) == Group || Value(value) == OneToOne
}
