package fsx

import "testing"

// setTestHome points the package's home-directory seam at a fake home for
// the duration of one test. Restored automatically via t.Cleanup.
func setTestHome(t *testing.T, home string) {
	t.Helper()
	old := homeDir
	homeDir = home
	t.Cleanup(func() { homeDir = old })
}

func TestValidatePathSafety(t *testing.T) {
	setTestHome(t, "/Users/testuser")
	cases := []struct {
		name string
		path string
		want string // "" = safe; otherwise the exact refusal message
	}{
		{"tmp allowed", "/tmp/foo", ""},
		{"var folders allowed", "/var/folders/x", ""},
		{"private var folders allowed", "/private/var/folders/zz/abc/T/foo", ""},
		{"var tmp allowed", "/private/var/tmp/x", ""},
		{"system refused", "/System/Library", "Refusing to delete protected system path: /System/Library"},
		{"usr prefix refused", "/usr/local", "Refusing to delete protected system path: /usr/local"},
		{"usr exact refused", "/usr", "Refusing to delete protected system path: /usr"},
		{"usr2 NOT refused (no false prefix match)", "/usr2", ""},
		{"usr2 subpath NOT refused", "/usr2/bin/thing", ""},
		{"var log refused", "/var/log/system.log", "Refusing to delete protected system path: /var/log/system.log"},
		{"library apple refused", "/Library/Apple/x", "Refusing to delete protected system path: /Library/Apple/x"},
		{"applications utilities refused", "/Applications/Utilities/Terminal.app", "Refusing to delete protected system path: /Applications/Utilities/Terminal.app"},
		{"root refused", "/", "Refusing to delete root directory"},
		{"home refused", "/Users/testuser", "Refusing to delete home directory"},
		{"home with trailing slash refused", "/Users/testuser/", "Refusing to delete home directory"},
		{"home subdir allowed", "/Users/testuser/Library/Caches/Foo", ""},
		{"dot segments cleaned: escapes tmp into protected", "/tmp/../System/Library", "Refusing to delete protected system path: /tmp/../System/Library"},
		{"dot segments cleaned: lands in tmp", "/usr/../tmp/foo", ""},
		{"dot segments cleaned: collapses to home", "/Users/testuser/Downloads/..", "Refusing to delete home directory"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidatePathSafety(c.path); got != c.want {
				t.Errorf("ValidatePathSafety(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

func TestIsProtectedPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/tmp", false},
		{"/tmp/anything", false},
		{"/private/var/folders/xy/z/T", false},
		{"/System", true},
		{"/usr/local/bin", true},
		{"/usr2/bin", false},
		{"/etc/hosts", true},
		{"/private/var/db/x", true},
		{"/Applications/Utilities", true},
		{"/Applications/Foo.app", false},
		{"/Users/testuser", false}, // home is NOT "protected"; ValidatePathSafety handles it separately
	}
	for _, c := range cases {
		if got := IsProtectedPath(c.path); got != c.want {
			t.Errorf("IsProtectedPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
