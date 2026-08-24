package cloud

import (
	"net/http"
	"testing"
)

func TestDeprecatedProviderPlaybackOverrideKeysAreIgnored(t *testing.T) {
	cd2 := newCloudDrive2(map[string]any{"url": "http://example.test/dav", "force_302": "true"}, http.DefaultClient)
	if !cd2.proxy {
		t.Fatalf("clouddrive2 should keep safe proxy mode; force_302 is deprecated")
	}
}

func TestUnsupportedProvider(t *testing.T) {
	if _, err := New("dropbox", nil, nil); err != ErrUnsupported {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
	if _, err := New("quark", nil, nil); err != ErrUnsupported {
		t.Fatalf("quark should be unsupported, got %v", err)
	}
	if IsCloudType("quark") {
		t.Fatal("quark should not be an active cloud provider")
	}
}
