package update

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"v1.2.3", []int{1, 2, 3}},
		{"0.1.0", []int{0, 1, 0}},
		{"v10.20.30", []int{10, 20, 30}},
		{"v0.0.0", []int{0, 0, 0}},
		{"bad", nil},
		{"v1.2", nil},
		{"v1.2.x", nil},
		{"", nil},
	}
	for _, tt := range tests {
		got := parseVersion(tt.input)
		if tt.want == nil {
			if got != nil {
				t.Errorf("parseVersion(%q) = %v, want nil", tt.input, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("parseVersion(%q) = nil, want %v", tt.input, tt.want)
			continue
		}
		for i := 0; i < 3; i++ {
			if got[i] != tt.want[i] {
				t.Errorf("parseVersion(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"v0.1.0", "v0.1.1", true},
		{"v0.1.0", "v0.2.0", true},
		{"v0.1.0", "v1.0.0", true},
		{"v0.1.1", "v0.1.0", false},
		{"v0.1.0", "v0.1.0", false},
		{"v1.0.0", "v0.9.9", false},
		{"dev", "v0.1.0", false},
		{"v0.1.0", "bad", false},
	}
	for _, tt := range tests {
		got := IsNewer(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestValidateAssetURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://github.com/donnycrash/clasp/releases/download/v0.1.0/clasp_0.1.0_darwin_arm64.tar.gz", false},
		{"https://objects.githubusercontent.com/some/path", false},
		{"https://evil.com/binary.tar.gz", true},
		{"http://github.com/something", true},
		{"ftp://github.com/something", true},
	}
	for _, tt := range tests {
		err := validateAssetURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateAssetURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
		}
	}
}
