package sourcegraph

import "testing"

func TestSourcegraphScopeFilter(t *testing.T) {
	cases := []struct {
		path, want string
		invalid    bool
	}{
		{"", "", false},
		{".", "", false},
		{"go", " file:^go(/|$)", false},
		{"go/pkg/main.go", " file:^go/pkg/main\\.go(/|$)", false},
		{"../escape", "", true},
		{"/absolute", "", true},
	}
	for _, tc := range cases {
		got, err := sourcegraphScopeFilter(tc.path)
		if tc.invalid && err == nil {
			t.Errorf("%q expected error", tc.path)
		}
		if !tc.invalid && (err != nil || got != tc.want) {
			t.Errorf("%q got %q, err=%v; want %q", tc.path, got, err, tc.want)
		}
	}
}
