package main

import "testing"

func TestLoadConfigRequires(t *testing.T) {
	full := map[string]string{"INVITE_KEY": "a", "COOKIE_KEY": "b", "ERGO_URL": "http://ergo:8097"}
	for _, missing := range []string{"", "INVITE_KEY", "COOKIE_KEY", "ERGO_URL"} {
		for k, v := range full {
			if k == missing {
				v = ""
			}
			t.Setenv(k, v)
		}
		_, err := loadConfig()
		if (err == nil) != (missing == "") {
			t.Errorf("missing %q: err %v", missing, err)
		}
	}
}
