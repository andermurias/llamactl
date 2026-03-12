package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckLatest_NewVersionAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Release{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/example/releases/v2.0.0",
		})
	}))
	defer srv.Close()

	rel, hasUpdate, err := checkLatestFromURL("v1.0.0", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !hasUpdate {
		t.Error("expected hasUpdate=true")
	}
	if rel.TagName != "v2.0.0" {
		t.Errorf("unexpected tag: %s", rel.TagName)
	}
}

func TestCheckLatest_AlreadyUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Release{TagName: "v1.0.0"})
	}))
	defer srv.Close()

	_, hasUpdate, err := checkLatestFromURL("v1.0.0", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if hasUpdate {
		t.Error("expected hasUpdate=false when versions match")
	}
}

func TestCheckLatest_DevVersion(t *testing.T) {
	_, hasUpdate, err := CheckLatest("dev")
	if err != nil {
		t.Fatal(err)
	}
	if hasUpdate {
		t.Error("dev builds should never report an update")
	}
}

func TestNormalize(t *testing.T) {
	if normalize("v1.2.3") != "1.2.3" {
		t.Error("normalize should strip leading v")
	}
	if normalize("1.2.3") != "1.2.3" {
		t.Error("normalize should leave no-v versions alone")
	}
}
