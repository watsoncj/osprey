package selfupdate

import "testing"

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input   string
		want    semver
		wantErr bool
	}{
		{"v1.2.3", semver{1, 2, 3}, false},
		{"1.2.3", semver{1, 2, 3}, false},
		{"v0.0.1", semver{0, 0, 1}, false},
		{"v1.2.3-beta", semver{1, 2, 3}, false},
		{"v1.2.3+build", semver{1, 2, 3}, false},
		{"dev", semver{}, true},
		{"v1.2", semver{}, true},
		{"", semver{}, true},
	}
	for _, tt := range tests {
		got, err := parseSemver(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseSemver(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSemver(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNewerThan(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v1.1.0", "v1.0.0", true},
		{"v1.0.0", "v1.1.0", false},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.1", "v1.0.0", true},
		{"v1.0.0", "v1.0.0", false},
	}
	for _, tt := range tests {
		a, _ := parseSemver(tt.a)
		b, _ := parseSemver(tt.b)
		if got := a.newerThan(b); got != tt.want {
			t.Errorf("%s.newerThan(%s) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
