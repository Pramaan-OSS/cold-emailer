package oauth

import "testing"

func TestAppNameFollowsTheNamingRules(t *testing.T) {
	if got, err := appName("  Acme   Sync ", ""); err != nil || got != "Acme Sync" {
		t.Fatalf("appName = %q, %v", got, err)
	}
	for _, bad := range []string{"", "Verify at evil.example", "https://acme.com", "Acme ‮ppa", "<b>Acme</b>"} {
		if _, err := appName(bad, ""); err == nil {
			t.Errorf("appName(%q) accepted", bad)
		}
	}
}

func TestAppWebsite(t *testing.T) {
	for _, ok := range []string{"", "https://acme.com", "http://acme.com/path"} {
		if _, err := appWebsite(ok); err != nil {
			t.Errorf("appWebsite(%q) refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "data:text/html,x", "acme.com", "https://user:pw@acme.com", "ftp://acme.com"} {
		if _, err := appWebsite(bad); err == nil {
			t.Errorf("appWebsite(%q) accepted", bad)
		}
	}
}

func TestAppNameRefusesThePlatformName(t *testing.T) {
	for _, bad := range []string{"ColdEmail", "ColdEmail Support", "connected for coldemail", "C o l d E m a i l", "Cold-email", "C0IdEmaiI", "ＣｏｌｄＥｍａｉｌ"} {
		if _, err := appName(bad, ""); err == nil {
			t.Errorf("appName(%q) accepted", bad)
		}
	}
	for _, ok := range []string{"Acme Sync", "Warm Leads", "Swarm"} {
		if _, err := appName(ok, ""); err != nil {
			t.Errorf("appName(%q) refused: %v", ok, err)
		}
	}
	if _, err := appName("ColdEmail Sync", "ColdEmail Sync"); err != nil {
		t.Errorf("an unchanged existing name was refused: %v", err)
	}
	if _, err := appName("ColdEmail Sync 2", "ColdEmail Sync"); err == nil {
		t.Error("a changed name naming the platform was accepted")
	}
}

func TestDCRClientNameDropsThePlatformName(t *testing.T) {
	cases := map[string]string{
		"Example Client (coldemail)":       "Example Client",
		"Example Desktop [ColdEmail prod]": "Example Desktop",
		"Example Desktop":                  "Example Desktop",
		"Example Client (work)":            "Example Client (work)",
		"ColdEmail Support":                dcrFallbackName,
		"ColdEmail (Example Client)":       dcrFallbackName,
		"":                                 dcrFallbackName,
		"https://coldemail.example":        dcrFallbackName,
	}
	for in, want := range cases {
		if got := dcrClientName(in); got != want {
			t.Errorf("dcrClientName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := ImportedName("ColdEmail Sync"); got != "Imported app" {
		t.Errorf("ImportedName kept the platform name: %q", got)
	}
}
