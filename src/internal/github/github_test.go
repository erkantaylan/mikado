package github

import "testing"

func TestParseRef(t *testing.T) {
	good := map[string]string{
		"studio/game#140":                            "studio/game#140",
		" Studio-Games/Winter.Update#26 ":            "Studio-Games/Winter.Update#26",
		"https://github.com/studio/my.repo/issues/7": "studio/my.repo#7",
		"https://github.com/studio/game/pull/9":      "studio/game#9",
	}
	for in, want := range good {
		r, err := ParseRef(in)
		if err != nil || r.String() != want {
			t.Errorf("ParseRef(%q) = %v, %v; want %s", in, r, err, want)
		}
	}
	for _, bad := range []string{"game#1", "studio/game", "studio/game#0", "studio/game#x", "-studio/game#1", `a"b/c#1`} {
		if _, err := ParseRef(bad); err == nil {
			t.Errorf("ParseRef(%q) accepted", bad)
		}
	}
	if r, _ := ParseRef("Studio/Game#1"); r.Key() != "studio/game#1" {
		t.Errorf("Key = %q", r.Key())
	}
}
