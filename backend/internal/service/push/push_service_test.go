package push

import (
	"testing"
	"time"
)

// TestInQuietWindow 验证免打扰时段判断（含跨午夜窗口）
func TestInQuietWindow(t *testing.T) {
	loc := time.Local

	tests := []struct {
		name  string
		now   time.Time
		start string
		end   string
		want  bool
	}{
		{"跨午夜窗口内(23:00)", time.Date(2026, 8, 1, 23, 0, 0, 0, loc), "22:00", "08:00", true},
		{"跨午夜窗口内(07:59)", time.Date(2026, 8, 1, 7, 59, 0, 0, loc), "22:00", "08:00", true},
		{"跨午夜窗口外(09:00)", time.Date(2026, 8, 1, 9, 0, 0, 0, loc), "22:00", "08:00", false},
		{"跨午夜边界(22:00整点在窗口内)", time.Date(2026, 8, 1, 22, 0, 0, 0, loc), "22:00", "08:00", true},
		{"跨午夜边界(08:00整点在窗口外)", time.Date(2026, 8, 1, 8, 0, 0, 0, loc), "22:00", "08:00", false},
		{"同日窗口内(03:00)", time.Date(2026, 8, 1, 3, 0, 0, 0, loc), "01:00", "06:00", true},
		{"同日窗口外(07:00)", time.Date(2026, 8, 1, 7, 0, 0, 0, loc), "01:00", "06:00", false},
		{"非法配置不限制", time.Date(2026, 8, 1, 23, 0, 0, 0, loc), "xx", "08:00", false},
		{"空配置不限制", time.Date(2026, 8, 1, 23, 0, 0, 0, loc), "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inQuietWindow(tt.now, tt.start, tt.end)
			if got != tt.want {
				t.Errorf("inQuietWindow(%v, %q, %q) = %v, want %v", tt.now, tt.start, tt.end, got, tt.want)
			}
		})
	}
}

// TestParseHM 验证 HH:MM 解析
func TestParseHM(t *testing.T) {
	if v, ok := parseHM("22:00"); !ok || v != 22*60 {
		t.Errorf("parseHM(22:00) = %d, %v", v, ok)
	}
	if v, ok := parseHM("08:30"); !ok || v != 8*60+30 {
		t.Errorf("parseHM(08:30) = %d, %v", v, ok)
	}
	if _, ok := parseHM("25:00"); ok {
		t.Error("parseHM(25:00) 应失败")
	}
	if _, ok := parseHM("ab:cd"); ok {
		t.Error("parseHM(ab:cd) 应失败")
	}
}
