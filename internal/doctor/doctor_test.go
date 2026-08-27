package doctor

import "testing"

func TestAtLeast(t *testing.T) {
	cases := []struct {
		have, want string
		ok         bool
	}{
		{"26.5.2", "14.2", true}, // a string compare would say false
		{"14.2", "14.2", true},
		{"14.2.1", "14.2", true},
		{"14.1", "14.2", false},
		{"13.7.6", "14.2", false},
		{"15", "14.2", true},
		{"14", "14.2", false}, // 14.0 is older than 14.2
	}
	for _, c := range cases {
		if got := atLeast(c.have, c.want); got != c.ok {
			t.Errorf("atLeast(%q, %q) = %v, want %v", c.have, c.want, got, c.ok)
		}
	}
}
