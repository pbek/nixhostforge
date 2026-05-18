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
		{name: "manual retries failure", previous: &Build{Status: "failed"}, manual: true, want: true},
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
