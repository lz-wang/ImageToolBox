package s3

import (
	"encoding/json"
	"testing"
)

func TestFormatListJSON(t *testing.T) {
	tests := []struct {
		name    string
		objects []ObjectInfo
		wantLen int
	}{
		{"nil empty list", nil, 0},
		{"non-empty list", []ObjectInfo{{Key: "images/photo.jpg", Size: 42}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []ObjectInfo
			if err := json.Unmarshal([]byte(FormatOutput(tt.objects, "json")), &got); err != nil {
				t.Fatalf("JSON output is invalid: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("object count = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}
