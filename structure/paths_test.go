package structure

import (
	"reflect"
	"testing"
)

func TestNormalizeRelativePaths(t *testing.T) {
	t.Run("normalizes separators and redundant components", func(t *testing.T) {
		got, err := NormalizeRelativePaths([]string{`assets\components`, "references/./generated"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"assets/components", "references/generated"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("NormalizeRelativePaths() = %#v, want %#v", got, want)
		}
	})

	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "Unix absolute", path: "/assets/components"},
		{name: "Windows drive absolute", path: `C:\assets\components`},
		{name: "Windows UNC absolute", path: `\\server\share\components`},
		{name: "escapes root", path: "../components"},
		{name: "escapes root after clean", path: "assets/../../components"},
	} {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			if _, err := NormalizeRelativePaths([]string{tt.path}); err == nil {
				t.Fatalf("NormalizeRelativePaths(%q) unexpectedly succeeded", tt.path)
			}
		})
	}
}
