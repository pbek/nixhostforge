package app

import "testing"

func TestShouldBuild(t *testing.T) {
	tests := []struct {
		name     string
		previous *Build
		manual   bool
		want     bool
	}{
		{name: "no previous", want: true},
		{name: "success skipped", previous: &Build{Status: "success"}, want: false},
		{name: "failed skipped", previous: &Build{Status: "failed"}, want: false},
		{name: "running skipped", previous: &Build{Status: "running"}, want: false},
		{name: "cancelled retried", previous: &Build{Status: "cancelled"}, want: true},
		{
			name:     "manual retries failure",
			previous: &Build{Status: "failed"},
			manual:   true,
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldBuild(tt.previous, tt.manual); got != tt.want {
				t.Fatalf("ShouldBuild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLastStorePath(t *testing.T) {
	got := lastStorePath("building\n/nix/store/abc-host\n")
	if got != "/nix/store/abc-host" {
		t.Fatalf("got %q", got)
	}
}

func TestNotificationTargetsFromLegacyLines(t *testing.T) {
	got := notificationTargets(
		"\n smtp://one.example.test \n\ntelegram://token@telegram?channels=1\r\n",
	)
	want := []notificationTarget{
		{URL: "smtp://one.example.test", Enabled: true},
		{URL: "telegram://token@telegram?channels=1", Enabled: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNotificationTargetsFromJSON(t *testing.T) {
	got := notificationTargets(
		`[{"url":"smtp://one.example.test","enabled":true},{"url":"telegram://token@telegram?channels=1","enabled":false}]`,
	)
	want := []notificationTarget{
		{URL: "smtp://one.example.test", Enabled: true},
		{URL: "telegram://token@telegram?channels=1", Enabled: false},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestEnabledNotificationURLs(t *testing.T) {
	got := enabledNotificationURLs(
		`[{"url":"smtp://one.example.test","enabled":true},{"url":"telegram://token@telegram?channels=1","enabled":false}]`,
	)
	want := []string{"smtp://one.example.test"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v, want %v", got, want)
	}
}
