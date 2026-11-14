package services

import (
	"testing"

	"github.com/rauni5/gclone/internal/installer"
)

func TestOfferInstallOptions(t *testing.T) {
	nat := &installer.Option{Manager: "brew", Note: "note"}
	dockerInstall := &installer.Option{Manager: "winget", Note: "large"}

	tests := []struct {
		name          string
		state         string
		nat           *installer.Option
		dockerInstall *installer.Option
		wantKinds     []string
		wantHeavy     []bool
	}{
		{
			name:      "engine ready",
			state:     "ready",
			nat:       nat,
			wantKinds: []string{"docker", "native"},
			wantHeavy: []bool{false, false},
		},
		{
			name:      "engine stopped",
			state:     "stopped",
			nat:       nat,
			wantKinds: []string{"docker", "native"},
			wantHeavy: []bool{false, false},
		},
		{
			name:      "permission denied no native",
			state:     "denied",
			wantKinds: nil,
			wantHeavy: nil,
		},
		{
			name:          "missing with docker install",
			state:         "missing",
			dockerInstall: dockerInstall,
			wantKinds:     []string{"docker"},
			wantHeavy:     []bool{true},
		},
		{
			name:          "missing with native and docker install",
			state:         "missing",
			nat:           nat,
			dockerInstall: dockerInstall,
			wantKinds:     []string{"native", "docker"},
			wantHeavy:     []bool{false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := offerInstallOptions(tt.state, tt.nat, tt.dockerInstall, "postgres")
			if len(got) != len(tt.wantKinds) {
				t.Fatalf("len=%d want %d (%+v)", len(got), len(tt.wantKinds), got)
			}
			for i, want := range tt.wantKinds {
				if got[i].kind != want {
					t.Fatalf("opt[%d].kind=%q want %q (%+v)", i, got[i].kind, want, got[i])
				}
				if got[i].heavy != tt.wantHeavy[i] {
					t.Fatalf("opt[%d].heavy=%v want %v (%+v)", i, got[i].heavy, tt.wantHeavy[i], got[i])
				}
				if got[i].label == "" || got[i].cmd == "" {
					t.Fatalf("opt[%d] missing label/cmd: %+v", i, got[i])
				}
			}
		})
	}
}
