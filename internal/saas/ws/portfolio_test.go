package ws

import "testing"

func TestEngineFromOrderID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"inst123-MACRO-1716800000", "MACRO"},
		{"inst1-MICRO-99", "MICRO"},
		{"inst1-MACRO", "MACRO"}, // missing trailing ts
		{"single", ""},           // no separator
		{"", ""},
	}
	for _, tt := range tests {
		if got := engineFromOrderID(tt.in); got != tt.want {
			t.Errorf("engineFromOrderID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenormalise(t *testing.T) {
	tests := []struct {
		name                string
		dead, float, total  float64
		wantDead, wantFloat float64
	}{
		{"bootstrap from zero", 0, 0, 100, 0, 100},
		{"preserve ratio", 30, 70, 200, 60, 140},
		{"all dead", 50, 0, 25, 25, 0},
		{"shrink", 80, 20, 50, 40, 10},
	}
	for _, tt := range tests {
		gd, gf := renormalise(tt.dead, tt.float, tt.total)
		if gd != tt.wantDead || gf != tt.wantFloat {
			t.Errorf("%s: renormalise(%v,%v,%v) = (%v,%v), want (%v,%v)",
				tt.name, tt.dead, tt.float, tt.total, gd, gf, tt.wantDead, tt.wantFloat)
		}
	}
}

func TestAuditEventFor(t *testing.T) {
	tests := []struct {
		name string
		in   DeltaReport
		want string
	}{
		{"snapshot", DeltaReport{}, "delta_report_snapshot"},
		{"failed", DeltaReport{ClientOrderID: "x", Execution: &Execution{Status: "failed"}}, "delta_report_failed"},
		{"filled", DeltaReport{ClientOrderID: "x", Execution: &Execution{Status: "filled"}}, "delta_report_filled"},
	}
	for _, tt := range tests {
		if got := auditEventFor(tt.in); got != tt.want {
			t.Errorf("%s: auditEventFor() = %q, want %q", tt.name, got, tt.want)
		}
	}
}
