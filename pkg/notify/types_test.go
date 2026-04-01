package notify

import "testing"

func TestReadWithFallback(t *testing.T) {
	tests := []struct {
		name     string
		cfg      map[string]string
		key      string
		fallback string
		want     string
	}{
		{
			name:     "primary key present",
			cfg:      map[string]string{"token": "abc", "bot_token": "def"},
			key:      "token",
			fallback: "bot_token",
			want:     "abc",
		},
		{
			name:     "fallback key used",
			cfg:      map[string]string{"bot_token": "def"},
			key:      "token",
			fallback: "bot_token",
			want:     "def",
		},
		{
			name:     "neither key present",
			cfg:      map[string]string{"other": "val"},
			key:      "token",
			fallback: "bot_token",
			want:     "",
		},
		{
			name:     "primary key empty falls back",
			cfg:      map[string]string{"token": "", "bot_token": "def"},
			key:      "token",
			fallback: "bot_token",
			want:     "def",
		},
		{
			name:     "both empty",
			cfg:      map[string]string{"token": "", "bot_token": ""},
			key:      "token",
			fallback: "bot_token",
			want:     "",
		},
		{
			name:     "nil map",
			cfg:      nil,
			key:      "token",
			fallback: "bot_token",
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readWithFallback(tt.cfg, tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("readWithFallback() = %q, want %q", got, tt.want)
			}
		})
	}
}
