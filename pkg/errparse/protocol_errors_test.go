package errparse

import "testing"

func TestExplainProtocolError_Known(t *testing.T) {
	codes := []string{"I1", "I2", "I3", "I4", "I5", "I6", "E2", "E16", "E20", "E21", "E21A", "E25", "E37"}

	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			info := ExplainProtocolError(code)
			if info == nil {
				t.Fatalf("ExplainProtocolError(%q) returned nil", code)
			}
			if info.Code != code {
				t.Errorf("Code = %q, want %q", info.Code, code)
			}
			if info.Title == "" {
				t.Error("Title is empty")
			}
			if info.PlainText == "" {
				t.Error("PlainText is empty")
			}
			if len(info.Remediation) == 0 {
				t.Error("Remediation is empty")
			}
			if info.SeeAlso == "" {
				t.Error("SeeAlso is empty")
			}
		})
	}
}

func TestExplainProtocolError_Unknown(t *testing.T) {
	unknowns := []string{"X99", "INVALID", "", "I999"}
	for _, code := range unknowns {
		t.Run(code, func(t *testing.T) {
			info := ExplainProtocolError(code)
			if info != nil {
				t.Errorf("ExplainProtocolError(%q) = %+v, want nil", code, info)
			}
		})
	}
}

func TestAllProtocolErrors(t *testing.T) {
	all := AllProtocolErrors()
	if len(all) == 0 {
		t.Fatal("AllProtocolErrors() returned empty slice")
	}

	for i, info := range all {
		if info.Code == "" {
			t.Errorf("entry %d has empty Code", i)
		}
		if info.PlainText == "" {
			t.Errorf("entry %d (%s) has empty PlainText", i, info.Code)
		}
	}

	// Should have at least 13 entries (I1-I6, E2, E16, E20, E21, E21A, E25, E37)
	if len(all) < 13 {
		t.Errorf("AllProtocolErrors() returned %d entries, want at least 13", len(all))
	}
}

func TestExplainProtocolError_CaseInsensitive(t *testing.T) {
	cases := []struct {
		input    string
		wantCode string
	}{
		{"e16", "E16"},
		{"i1", "I1"},
		{"e21a", "E21A"},
		{"e25", "E25"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			info := ExplainProtocolError(tc.input)
			if info == nil {
				t.Fatalf("ExplainProtocolError(%q) returned nil", tc.input)
			}
			if info.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", info.Code, tc.wantCode)
			}
		})
	}
}
