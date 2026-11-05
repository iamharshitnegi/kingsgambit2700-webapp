package httpx

import "testing"

func TestDecodeStrictJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "valid", body: `{"name":"student"}`},
		{name: "empty", body: "", wantErr: true},
		{name: "unknown field", body: `{"name":"student","consent":true}`, wantErr: true},
		{name: "multiple values", body: `{"name":"student"} {"name":"second"}`, wantErr: true},
		{name: "malformed", body: `{"name":`, wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var target struct {
				Name string `json:"name"`
			}
			err := DecodeStrictJSON([]byte(test.body), &target)
			if (err != nil) != test.wantErr {
				t.Fatalf("DecodeStrictJSON() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
