package util

import "testing"

func TestCodeBlockStrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "backtick fence",
			input: "before\n```go\ncode here\n```\nafter",
			want:  "before\n\nafter",
		},
		{
			name:  "tilde fence",
			input: "before\n~~~python\ncode here\n~~~\nafter",
			want:  "before\n\nafter",
		},
		{
			name:  "info string with punctuation",
			input: "before\n```c++\nint x;\n```\nmiddle\n```shell-session\n$ ls\n```\nafter",
			want:  "before\n\nmiddle\n\nafter",
		},
		{
			name:  "info string with attributes",
			input: "before\n``` bash title=\"run.sh\"\n./run.sh\n```\nafter",
			want:  "before\n\nafter",
		},
		{
			name:  "no fence",
			input: "just plain text",
			want:  "just plain text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CodeBlockStrip.ReplaceAllString(tt.input, "")
			if got != tt.want {
				t.Errorf("CodeBlockStrip:\n  got:  %q\n  want: %q", got, tt.want)
			}
		})
	}
}

func TestInlineCodeStrip(t *testing.T) {
	input := "use `foo` and `bar` here"
	want := "use  and  here"
	got := InlineCodeStrip.ReplaceAllString(input, "")
	if got != want {
		t.Errorf("InlineCodeStrip:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestCodeBlockPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantN    int
		wantBody string
	}{
		{
			name:     "backtick fence",
			input:    "```go\nfmt.Println()\n```",
			wantN:    1,
			wantBody: "fmt.Println()\n",
		},
		{
			name:     "tilde fence",
			input:    "~~~python\nprint('hi')\n~~~",
			wantN:    1,
			wantBody: "print('hi')\n",
		},
		{
			name:     "info string with punctuation",
			input:    "```objective-c\n[obj run];\n```\nprose\n```c++\nint x;\n```",
			wantN:    2,
			wantBody: "[obj run];\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := CodeBlockPattern.FindAllStringSubmatch(tt.input, -1)
			if len(matches) != tt.wantN {
				t.Fatalf("expected %d matches, got %d", tt.wantN, len(matches))
			}
			if matches[0][1] != tt.wantBody {
				t.Errorf("body:\n  got:  %q\n  want: %q", matches[0][1], tt.wantBody)
			}
		})
	}
}
