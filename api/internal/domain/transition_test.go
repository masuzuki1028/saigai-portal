package domain

import "testing"

func TestDecide(t *testing.T) {
	def := LevelSetting{Display: SeverityAdvisory, Notify: SeverityWarning}
	tests := []struct {
		name     string
		old, new Severity
		changed  bool
		setting  LevelSetting
		want     Decision
	}{
		{"変化なし", SeverityWarning, SeverityWarning, false, def, Decision{}},
		{"注意報の発表: 表示のみ", SeverityNone, SeverityAdvisory, false, def, Decision{AddItem: true, Status: StatusIssued}},
		{"注意報から警報: 表示と通知", SeverityAdvisory, SeverityWarning, false, def, Decision{AddItem: true, Status: StatusIssued, Notify: true}},
		{"いきなり特別警報", SeverityNone, SeverityEmergency, false, def, Decision{AddItem: true, Status: StatusIssued, Notify: true}},
		{"警報から特別警報: もう一度通知", SeverityWarning, SeverityEmergency, false, def, Decision{AddItem: true, Status: StatusIssued, Notify: true}},
		{"特別警報から警報: 通知しない", SeverityEmergency, SeverityWarning, false, def, Decision{AddItem: true, Status: StatusContinued}},
		{"警報から注意報: 通知の対象から外れたので通知", SeverityWarning, SeverityAdvisory, false, def, Decision{AddItem: true, Status: StatusContinued, Notify: true}},
		{"注意報の解除: 表示のみ", SeverityAdvisory, SeverityNone, false, def, Decision{AddItem: true, Status: StatusCleared}},
		{"警報の解除: 表示と通知", SeverityWarning, SeverityNone, false, def, Decision{AddItem: true, Status: StatusCleared, Notify: true}},
		{"同じレベルで種類が変わった: 表示のみ", SeverityWarning, SeverityWarning, true, def, Decision{AddItem: true, Status: StatusContinued}},
		{"情報レベルは表示しきい値未満", SeverityNone, SeverityInfo, false, def, Decision{}},
		{"表示も通知も OFF", SeverityNone, SeverityEmergency, false, LevelSetting{}, Decision{}},
		{"表示 OFF・通知のみ", SeverityNone, SeverityWarning, false, LevelSetting{Notify: SeverityWarning}, Decision{Notify: true}},
		{"特別警報のみ通知: 警報では送らない", SeverityNone, SeverityWarning, false, LevelSetting{Display: SeverityWarning, Notify: SeverityEmergency}, Decision{AddItem: true, Status: StatusIssued}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.old, tt.new, tt.changed, tt.setting)
			if got != tt.want {
				t.Errorf("Decide(%d, %d, %v) = %+v, want %+v", tt.old, tt.new, tt.changed, got, tt.want)
			}
		})
	}
}

func TestIntensity(t *testing.T) {
	tests := []struct {
		in       Intensity
		severity Severity
		label    string
	}{
		{"1", SeverityInfo, "震度1"},
		{"2", SeverityInfo, "震度2"},
		{"3", SeverityAdvisory, "震度3"},
		{"4", SeverityAdvisory, "震度4"},
		{"5-", SeverityWarning, "震度5弱"},
		{"5+", SeverityWarning, "震度5強"},
		{"6-", SeverityEmergency, "震度6弱"},
		{"6+", SeverityEmergency, "震度6強"},
		{"7", SeverityEmergency, "震度7"},
		{"off", SeverityNone, "なし"},
		{"x", SeverityNone, "震度x"},
	}
	for _, tt := range tests {
		if got := tt.in.Severity(); got != tt.severity {
			t.Errorf("%q.Severity() = %d, want %d", tt.in, got, tt.severity)
		}
		if got := tt.in.Label(); got != tt.label {
			t.Errorf("%q.Label() = %q, want %q", tt.in, got, tt.label)
		}
	}
	if !Intensity("5+").AtLeast("5-") || Intensity("4").AtLeast("5-") || Intensity("7").AtLeast(IntensityOff) {
		t.Error("AtLeast の比較が正しくない")
	}
}

func TestHazardSettingsValidate(t *testing.T) {
	if err := DefaultHazardSettings().Validate(); err != nil {
		t.Fatalf("初期値が不正: %v", err)
	}
	bad := DefaultHazardSettings()
	bad.Rain.Notify = 5
	if err := bad.Validate(); err == nil {
		t.Error("範囲外の severity を受け付けた")
	}
	bad = DefaultHazardSettings()
	bad.Earthquake.NotifyMinIntensity = "8"
	if err := bad.Validate(); err == nil {
		t.Error("不正な震度を受け付けた")
	}
}
