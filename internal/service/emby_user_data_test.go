package service

import (
	"testing"

	"github.com/ShukeBta/MMTL/internal/model"
)

func TestMergedRemoteUserData(t *testing.T) {
	tests := []struct {
		name     string
		raw      any
		history  model.PlaybackHistory
		position int64
		played   bool
		percent  float64
		count    int
		preserve any
	}{
		{
			name: "in-progress preserves remote fields",
			raw: map[string]any{
				"PlayCount": 2,
				"Custom":    "remote-value",
			},
			history:  model.PlaybackHistory{PositionMs: 25_000, DurationMs: 100_000},
			position: 250_000_000,
			played:   false,
			percent:  25,
			count:    2,
			preserve: "remote-value",
		},
		{
			name:     "completed ensures a play count",
			raw:      map[string]any{"PlayCount": 0},
			history:  model.PlaybackHistory{PositionMs: 100_000, DurationMs: 100_000, Completed: true},
			position: 1_000_000_000,
			played:   true,
			percent:  100,
			count:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := mergedRemoteUserData(tt.raw, &tt.history)
			if got := out["PlaybackPositionTicks"]; got != tt.position {
				t.Fatalf("PlaybackPositionTicks = %#v, want %d", got, tt.position)
			}
			if got := out["Played"]; got != tt.played {
				t.Fatalf("Played = %#v, want %t", got, tt.played)
			}
			if got := out["PlayedPercentage"]; got != tt.percent {
				t.Fatalf("PlayedPercentage = %#v, want %v", got, tt.percent)
			}
			if got := out["PlayCount"]; got != tt.count {
				t.Fatalf("PlayCount = %#v, want %d", got, tt.count)
			}
			if tt.preserve != nil && out["Custom"] != tt.preserve {
				t.Fatalf("Custom = %#v, want %#v", out["Custom"], tt.preserve)
			}
		})
	}
}

func TestRemoteItemMapsFindsEnvelopeItems(t *testing.T) {
	remoteID := EncodeEmbyRemoteID("mount-1", "item-1")
	payload := map[string]any{
		"Items": []any{
			map[string]any{"Id": remoteID},
			map[string]any{"Id": "local-item"},
		},
	}
	items := remoteItemMaps(payload)
	if len(items) != 2 {
		t.Fatalf("item count = %d, want 2", len(items))
	}
	if items[0]["Id"] != remoteID {
		t.Fatalf("first item ID = %#v, want %q", items[0]["Id"], remoteID)
	}
}
