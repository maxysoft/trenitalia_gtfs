package config

import (
	"testing"
	"time"
)

func TestLeggiConfig_MissingToken(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "12345")

	_, err := LeggiConfig()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_BOT_TOKEN is missing")
	}
}

func TestLeggiConfig_MissingChatID(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_CHAT_ID", "")

	_, err := LeggiConfig()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_CHAT_ID is missing")
	}
}

func TestLeggiConfig_InvalidChatID(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_CHAT_ID", "not-a-number")

	_, err := LeggiConfig()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_CHAT_ID is not a valid integer")
	}
}

func TestLeggiConfig_Defaults(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_CHAT_ID", "12345")
	t.Setenv("LINEA_TRENO", "")
	t.Setenv("STAZIONE_ORIGINE", "")
	t.Setenv("STAZIONE_DESTINAZIONE", "")
	t.Setenv("RITARDO_SOGLIA_MINUTI", "")
	t.Setenv("INTERVALLO_POLLING_SECONDI", "")
	t.Setenv("TIPI_TRENO", "")
	t.Setenv("INVIO_REPORT_MENSILE", "")
	t.Setenv("REPORT_TELEGRAM_CHAT_ID", "")
	t.Setenv("SQLITE_DB_PATH", "")
	t.Setenv("ADMIN_CHAT_ID", "")

	cfg, err := LeggiConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LineaTreno != "FL3" {
		t.Errorf("expected LineaTreno=FL3, got %s", cfg.LineaTreno)
	}
	if cfg.StazioneOrigine != "Roma Ostiense" {
		t.Errorf("expected StazioneOrigine=Roma Ostiense, got %s", cfg.StazioneOrigine)
	}
	if cfg.StazioneDestinazione != "Viterbo Porta Fiorentina" {
		t.Errorf("expected StazioneDestinazione=Viterbo Porta Fiorentina, got %s", cfg.StazioneDestinazione)
	}
	if cfg.RitardoSogliaMinuti != 10 {
		t.Errorf("expected RitardoSogliaMinuti=10, got %d", cfg.RitardoSogliaMinuti)
	}
	if cfg.IntervalloPolling != 300*time.Second {
		t.Errorf("expected IntervalloPolling=300s, got %v", cfg.IntervalloPolling)
	}
	if cfg.SQLiteDBPath != "/data/ritardi.sqlite" {
		t.Errorf("expected SQLiteDBPath=/data/ritardi.sqlite, got %s", cfg.SQLiteDBPath)
	}
	if !cfg.ReportMensileAbilitato {
		t.Error("expected ReportMensileAbilitato=true by default")
	}
	if cfg.ReportTelegramChatID != 12345 {
		t.Errorf("expected ReportTelegramChatID=12345 (fallback to TelegramChatID), got %d", cfg.ReportTelegramChatID)
	}
	if cfg.AdminChatID != 12345 {
		t.Errorf("expected AdminChatID=12345 (fallback to TelegramChatID), got %d", cfg.AdminChatID)
	}
}

func TestLeggiConfig_Custom(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "custom-token")
	t.Setenv("TELEGRAM_CHAT_ID", "99999")
	t.Setenv("LINEA_TRENO", "FL1")
	t.Setenv("STAZIONE_ORIGINE", "Roma Termini")
	t.Setenv("STAZIONE_DESTINAZIONE", "Milano Centrale")
	t.Setenv("RITARDO_SOGLIA_MINUTI", "5")
	t.Setenv("INTERVALLO_POLLING_SECONDI", "60")
	t.Setenv("TIPI_TRENO", "R,RV")
	t.Setenv("INVIO_REPORT_MENSILE", "false")
	t.Setenv("REPORT_TELEGRAM_CHAT_ID", "77777")
	t.Setenv("SQLITE_DB_PATH", "/tmp/test.sqlite")

	cfg, err := LeggiConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.TelegramToken != "custom-token" {
		t.Errorf("expected token custom-token, got %s", cfg.TelegramToken)
	}
	if cfg.TelegramChatID != 99999 {
		t.Errorf("expected TelegramChatID=99999, got %d", cfg.TelegramChatID)
	}
	if cfg.LineaTreno != "FL1" {
		t.Errorf("expected LineaTreno=FL1, got %s", cfg.LineaTreno)
	}
	if cfg.RitardoSogliaMinuti != 5 {
		t.Errorf("expected RitardoSogliaMinuti=5, got %d", cfg.RitardoSogliaMinuti)
	}
	if cfg.IntervalloPolling != 60*time.Second {
		t.Errorf("expected IntervalloPolling=60s, got %v", cfg.IntervalloPolling)
	}
	if len(cfg.TipiTrenoAccettati) != 2 {
		t.Errorf("expected 2 train types, got %d: %v", len(cfg.TipiTrenoAccettati), cfg.TipiTrenoAccettati)
	}
	if cfg.ReportMensileAbilitato {
		t.Error("expected ReportMensileAbilitato=false")
	}
	if cfg.ReportTelegramChatID != 77777 {
		t.Errorf("expected ReportTelegramChatID=77777, got %d", cfg.ReportTelegramChatID)
	}
	if cfg.SQLiteDBPath != "/tmp/test.sqlite" {
		t.Errorf("expected SQLiteDBPath=/tmp/test.sqlite, got %s", cfg.SQLiteDBPath)
	}
}

func TestLeggiConfig_AdminChatID(t *testing.T) {
	t.Run("defaults to TelegramChatID", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "tok")
		t.Setenv("TELEGRAM_CHAT_ID", "11111")
		t.Setenv("ADMIN_CHAT_ID", "")

		cfg, err := LeggiConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.AdminChatID != 11111 {
			t.Errorf("expected AdminChatID=11111 (fallback), got %d", cfg.AdminChatID)
		}
	})

	t.Run("custom value", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "tok")
		t.Setenv("TELEGRAM_CHAT_ID", "11111")
		t.Setenv("ADMIN_CHAT_ID", "-9876543")

		cfg, err := LeggiConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.AdminChatID != -9876543 {
			t.Errorf("expected AdminChatID=-9876543, got %d", cfg.AdminChatID)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		t.Setenv("TELEGRAM_BOT_TOKEN", "tok")
		t.Setenv("TELEGRAM_CHAT_ID", "11111")
		t.Setenv("ADMIN_CHAT_ID", "not-a-number")

		_, err := LeggiConfig()
		if err == nil {
			t.Fatal("expected error for invalid ADMIN_CHAT_ID")
		}
	})
}

func TestLeggiConfig_InvalidReportChatID(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_CHAT_ID", "12345")
	t.Setenv("REPORT_TELEGRAM_CHAT_ID", "not-a-number")

	_, err := LeggiConfig()
	if err == nil {
		t.Fatal("expected error when REPORT_TELEGRAM_CHAT_ID is invalid")
	}
}

func TestBooleano(t *testing.T) {
	cases := []struct {
		value    string
		fallback bool
		expected bool
	}{
		{"1", false, true},
		{"true", false, true},
		{"vero", false, true},
		{"yes", false, true},
		{"y", false, true},
		{"on", false, true},
		{"0", true, false},
		{"false", true, false},
		{"falso", true, false},
		{"no", true, false},
		{"n", true, false},
		{"off", true, false},
		{"", true, true},
		{"", false, false},
		{"invalid", true, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.value+"_fallback_"+func() string {
			if tc.fallback {
				return "true"
			}
			return "false"
		}(), func(t *testing.T) {
			const key = "TEST_BOOL_VAR_CONFIG"
			t.Setenv(key, tc.value)
			got := booleano(key, tc.fallback)
			if got != tc.expected {
				t.Errorf("booleano(%q, %v) = %v, want %v", tc.value, tc.fallback, got, tc.expected)
			}
		})
	}
}

func TestSplitVirgola(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"R,RV,RE", []string{"R", "RV", "RE"}},
		{"R", []string{"R"}},
		{" R , RV ", []string{"R", "RV"}},
		{"", nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := splitVirgola(tc.input)
			if len(got) != len(tc.expected) {
				t.Errorf("splitVirgola(%q) = %v (len %d), want %v (len %d)",
					tc.input, got, len(got), tc.expected, len(tc.expected))
				return
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("splitVirgola(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestInteri(t *testing.T) {
	cases := []struct {
		value    string
		fallback int
		expected int
	}{
		{"42", 0, 42},
		{"", 99, 99},
		{"invalid", 5, 5},
		{"0", 10, 0},
		{"-1", 0, -1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.value, func(t *testing.T) {
			const key = "TEST_INT_VAR_CONFIG"
			t.Setenv(key, tc.value)
			got := interi(key, tc.fallback)
			if got != tc.expected {
				t.Errorf("interi(%q, %d) = %d, want %d", tc.value, tc.fallback, got, tc.expected)
			}
		})
	}
}
