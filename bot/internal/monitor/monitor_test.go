package monitor

import (
	"testing"
	"time"

	"github.com/maxysoft/trenitalia_gtfs/bot/internal/trenitalia"
)

func TestTipiTrenoSet_Normalization(t *testing.T) {
	set := tipiTrenoSet([]string{"r", "rv", "RE", " IC "})

	for _, want := range []string{"R", "RV", "RE", "IC"} {
		if !set[want] {
			t.Errorf("expected %q in set", want)
		}
	}
	if set["FB"] {
		t.Error("FB should not be in set")
	}
}

func TestTipiTrenoSet_Empty(t *testing.T) {
	set := tipiTrenoSet(nil)
	if len(set) != 0 {
		t.Errorf("expected empty set, got %d items", len(set))
	}

	set2 := tipiTrenoSet([]string{})
	if len(set2) != 0 {
		t.Errorf("expected empty set, got %d items", len(set2))
	}
}

func TestParsaOrario_RFC3339(t *testing.T) {
	got := parsaOrario("2024-03-15T10:30:00Z")
	if got.IsZero() {
		t.Fatal("expected non-zero time for RFC3339 input")
	}
	if got.UTC().Hour() != 10 || got.UTC().Minute() != 30 {
		t.Errorf("unexpected time: %v", got)
	}
}

func TestParsaOrario_WithMilliseconds(t *testing.T) {
	got := parsaOrario("2024-03-15T10:30:00.000+01:00")
	if got.IsZero() {
		t.Error("expected non-zero time with milliseconds format")
	}
}

func TestParsaOrario_WithOffset(t *testing.T) {
	got := parsaOrario("2024-03-15T10:30:00+01:00")
	if got.IsZero() {
		t.Error("expected non-zero time with offset format")
	}
	// 10:30 +01:00 = 09:30 UTC
	if got.UTC().Hour() != 9 || got.UTC().Minute() != 30 {
		t.Errorf("unexpected UTC time: %v", got.UTC())
	}
}

func TestParsaOrario_Empty(t *testing.T) {
	got := parsaOrario("")
	if !got.IsZero() {
		t.Errorf("expected zero time for empty string, got %v", got)
	}
}

func TestParsaOrario_Invalid(t *testing.T) {
	got := parsaOrario("not-a-date")
	if !got.IsZero() {
		t.Errorf("expected zero time for invalid string, got %v", got)
	}
}

func TestPrimoNonVuoto(t *testing.T) {
	cases := []struct {
		values   []string
		expected string
	}{
		{[]string{"a", "b", "c"}, "a"},
		{[]string{"", "b", "c"}, "b"},
		{[]string{"", "", "c"}, "c"},
		{[]string{"", "", ""}, ""},
		{[]string{}, ""},
	}

	for _, tc := range cases {
		got := primoNonVuoto(tc.values...)
		if got != tc.expected {
			t.Errorf("primoNonVuoto(%v) = %q, want %q", tc.values, got, tc.expected)
		}
	}
}

func TestEstraiAcronimo(t *testing.T) {
	t.Run("from Train field", func(t *testing.T) {
		nodo := trenitalia.Nodo{
			Train: &trenitalia.InfoTreno{Acronym: "rv"},
		}
		if got := estraiAcronimo(nodo); got != "RV" {
			t.Errorf("expected RV, got %s", got)
		}
	})

	t.Run("from TrainInfo field", func(t *testing.T) {
		nodo := trenitalia.Nodo{
			TrainInfo: &trenitalia.InfoTreno{Acronym: "re"},
		}
		if got := estraiAcronimo(nodo); got != "RE" {
			t.Errorf("expected RE, got %s", got)
		}
	})

	t.Run("from TrainAcronym fallback", func(t *testing.T) {
		nodo := trenitalia.Nodo{
			TrainAcronym: "ic",
		}
		if got := estraiAcronimo(nodo); got != "IC" {
			t.Errorf("expected IC, got %s", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		nodo := trenitalia.Nodo{}
		if got := estraiAcronimo(nodo); got != "" {
			t.Errorf("expected empty string, got %s", got)
		}
	})
}

func TestPulisciStati(t *testing.T) {
	m := &Monitor{
		stati: map[string]*statoTreno{
			"old_key":    {inRitardo: false, aggiornatoAl: time.Now().Add(-25 * time.Hour)},
			"recent_key": {inRitardo: true, aggiornatoAl: time.Now().Add(-1 * time.Hour)},
		},
	}

	m.pulisciStati()

	if _, ok := m.stati["old_key"]; ok {
		t.Error("expected old_key to be removed after cleanup")
	}
	if _, ok := m.stati["recent_key"]; !ok {
		t.Error("expected recent_key to be retained after cleanup")
	}
}

func TestAggiornaStato(t *testing.T) {
	m := &Monitor{
		stati: make(map[string]*statoTreno),
	}

	// New key: not delayed → no state change on first insert when not delayed.
	changed, nowDelayed := m.aggiornaStato("train1", false)
	if changed {
		t.Error("expected no change for new entry starting as not-delayed")
	}
	if nowDelayed {
		t.Error("expected nowDelayed=false")
	}

	// Transition false → true: should report change.
	changed, nowDelayed = m.aggiornaStato("train1", true)
	if !changed {
		t.Error("expected change when transitioning not-delayed to delayed")
	}
	if !nowDelayed {
		t.Error("expected nowDelayed=true")
	}

	// Same state again: no change.
	changed, nowDelayed = m.aggiornaStato("train1", true)
	if changed {
		t.Error("expected no change when state stays delayed")
	}
	if !nowDelayed {
		t.Error("expected nowDelayed=true")
	}

	// Transition true → false (recovery): should report change.
	changed, nowDelayed = m.aggiornaStato("train1", false)
	if !changed {
		t.Error("expected change when recovering")
	}
	if nowDelayed {
		t.Error("expected nowDelayed=false after recovery")
	}
}
