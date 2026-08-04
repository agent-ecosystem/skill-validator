package structure

import (
	"reflect"
	"testing"
)

func TestNormalizeRelativePaths(t *testing.T) {
	t.Run("normalizes separators and redundant components", func(t *testing.T) {
		got, err := NormalizeRelativePaths([]string{`generated\site`, "docs/./generated"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"generated/site", "docs/generated"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("NormalizeRelativePaths() = %#v, want %#v", got, want)
		}
	})

	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "Unix absolute", path: "/site"},
		{name: "Windows drive absolute", path: `C:\site`},
		{name: "Windows UNC absolute", path: `\\server\share\site`},
		{name: "escapes root", path: "../site"},
		{name: "escapes root after clean", path: "generated/../../site"},
	} {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			if _, err := NormalizeRelativePaths([]string{tt.path}); err == nil {
				t.Fatalf("NormalizeRelativePaths(%q) unexpectedly succeeded", tt.path)
			}
		})
	}
}
