package main

import "testing"

func TestValidateYouTubeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "watch url", value: "https://www.youtube.com/watch?v=abc123"},
		{name: "short url", value: "https://youtu.be/abc123"},
		{name: "music subdomain", value: "https://music.youtube.com/watch?v=abc123"},
		{name: "nocookie", value: "https://www.youtube-nocookie.com/embed/abc123"},
		{name: "http rejected", value: "http://www.youtube.com/watch?v=abc123", wantErr: true},
		{name: "non youtube rejected", value: "https://example.com/watch?v=abc123", wantErr: true},
		{name: "lookalike rejected", value: "https://youtube.com.example.com/watch?v=abc123", wantErr: true},
		{name: "file scheme rejected", value: "file:///etc/passwd", wantErr: true},
		{name: "control char rejected", value: "https://www.youtube.com/watch?v=abc123\n--help", wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateYouTubeURL(test.value)
			if test.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCleanURLsDedupesAndValidates(t *testing.T) {
	t.Parallel()

	urls, err := cleanURLs([]string{
		" https://www.youtube.com/watch?v=abc123 ",
		"https://www.youtube.com/watch?v=abc123",
		"https://youtu.be/xyz789",
		"",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 unique URLs, got %d: %#v", len(urls), urls)
	}
}

func TestValidateAudioFormat(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"mp3", "m4a", "opus", "wav", "flac"} {
		if err := validateAudioFormat(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}
	if err := validateAudioFormat("../../../bad"); err == nil {
		t.Fatal("expected unsafe audio format to be rejected")
	}
}

func TestValidateOutputDir(t *testing.T) {
	t.Parallel()

	if err := validateOutputDir("/"); err == nil {
		t.Fatal("expected filesystem root to be rejected")
	}
	if err := validateOutputDir("downloads"); err != nil {
		t.Fatalf("expected relative downloads dir to be valid: %v", err)
	}
}
