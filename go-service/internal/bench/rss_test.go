package bench

import "testing"

func TestParseWMICWorkingSet(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"typical", "WorkingSetSize  \n52428800  \n\n", 52428800, false},
		{"minimal", "WorkingSetSize\n1048576\n", 1048576, false},
		{"empty", "", 0, true},
		{"no_value", "WorkingSetSize\n", 0, true},
		{"garbage", "foo\nbar\n", 0, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseWMICWorkingSet(c.input)
			if c.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestParsePowerShellWorkingSet(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"typical", "52428800\r\n", 52428800, false},
		{"with_spaces", " 1048576 \n", 1048576, false},
		{"empty", "", 0, true},
		{"garbage", "WorkingSet64\n", 0, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParsePowerShellWorkingSet(c.input)
			if c.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}
