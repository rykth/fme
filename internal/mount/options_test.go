package mount

import (
	"testing"
)

func TestParseRemote(t *testing.T) {
	cases := []struct {
		input    string
		wantUser string
		wantHost string
		wantPath string
		wantErr  bool
	}{
		{"user@host:/path", "user", "host", "/path", false},
		{"host:/path", "", "host", "/path", false},
		{"host:", "", "host", ".", false},
		{"host:/", "", "host", "/", false},
		{"nocoron", "", "", "", true},
	}

	for _, tc := range cases {
		opts := DefaultOptions()
		err := ParseRemote(tc.input, &opts)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q: expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tc.input, err)
			continue
		}
		if opts.Conn.User != tc.wantUser {
			t.Errorf("%q: user want %q got %q", tc.input, tc.wantUser, opts.Conn.User)
		}
		if opts.Conn.Host != tc.wantHost {
			t.Errorf("%q: host want %q got %q", tc.input, tc.wantHost, opts.Conn.Host)
		}
		if opts.RemotePath != tc.wantPath {
			t.Errorf("%q: path want %q got %q", tc.input, tc.wantPath, opts.RemotePath)
		}
	}
}
