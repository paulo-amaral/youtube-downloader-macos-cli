package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestBuildYTDLPArgsWithPortugueseSubtitles(t *testing.T) {
	t.Setenv("YTDLP_JS_RUNTIME", "off")

	args := buildYTDLPArgs(config{
		outputDir:     "downloads",
		quality:       "1080",
		subtitles:     true,
		autoSubtitles: true,
		subtitleLangs: "pt.*,pt-BR,pt",
		urls:          []string{"https://www.youtube.com/watch?v=abc123"},
	}, false)

	wantArgs := []string{"--write-subs", "--embed-subs", "--sub-langs", "pt.*,pt-BR,pt", "--write-auto-subs"}
	for _, want := range wantArgs {
		if !hasArg(args, want) {
			t.Fatalf("expected args to include %q: %#v", want, args)
		}
	}
}

func TestBuildYTDLPArgsWithJSRuntime(t *testing.T) {
	t.Setenv("YTDLP_JS_RUNTIME", "node:/usr/local/bin/node")

	args := buildYTDLPArgs(config{
		outputDir: "downloads",
		quality:   "best",
		urls:      []string{"https://www.youtube.com/watch?v=abc123"},
	}, false)

	if !hasArg(args, "--js-runtimes") || !hasArg(args, "node:/usr/local/bin/node") {
		t.Fatalf("expected JS runtime args: %#v", args)
	}
}

func TestValidateSubtitleLangs(t *testing.T) {
	t.Parallel()

	if err := validateSubtitleLangs("pt.*,pt-BR,pt"); err != nil {
		t.Fatalf("expected subtitle languages to be valid: %v", err)
	}
	if err := validateSubtitleLangs("pt\n--help"); err == nil {
		t.Fatal("expected unsafe subtitle languages to be rejected")
	}
}

func TestParseDownloadFlagsWithConcurrentDownloads(t *testing.T) {
	t.Parallel()

	cfg, err := parseDownloadFlags([]string{
		"--concurrent", "3",
		"https://www.youtube.com/watch?v=abc123",
		"https://youtu.be/xyz789",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.concurrent != 3 {
		t.Fatalf("expected 3 concurrent downloads, got %d", cfg.concurrent)
	}
}

func TestParseDownloadFlagsWithDubAfter(t *testing.T) {
	t.Parallel()

	cfg, err := parseDownloadFlags([]string{
		"--dub-after",
		"https://www.youtube.com/watch?v=abc123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.dubAfter {
		t.Fatal("expected dub-after to be enabled")
	}
}

func TestParseDownloadFlagsWithDubEngine(t *testing.T) {
	t.Parallel()

	cfg, err := parseDownloadFlags([]string{
		"--dub-after",
		"--dub-engine", "gemini",
		"https://www.youtube.com/watch?v=abc123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.dubEngine != "gemini" {
		t.Fatalf("expected gemini engine, got %q", cfg.dubEngine)
	}
}

func TestValidateDubEngine(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "auto", "gemini", "local"} {
		if err := validateDubEngine(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}
	if err := validateDubEngine("bad"); err == nil {
		t.Fatal("expected unsupported engine to be rejected")
	}
}

func TestValidateConcurrentDownloads(t *testing.T) {
	t.Parallel()

	for _, value := range []int{1, 4, maxConcurrentDownloads} {
		if err := validateConcurrentDownloads(value); err != nil {
			t.Fatalf("expected %d to be valid: %v", value, err)
		}
	}
	for _, value := range []int{0, maxConcurrentDownloads + 1} {
		if err := validateConcurrentDownloads(value); err == nil {
			t.Fatalf("expected %d to be rejected", value)
		}
	}
}

func TestParseDubFlags(t *testing.T) {
	t.Parallel()

	cfg, err := parseDubFlags([]string{
		"--output", "downloads",
		"--to", "es",
		"https://www.youtube.com/watch?v=abc123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.outputDir != "downloads" {
		t.Fatalf("expected output dir downloads, got %q", cfg.outputDir)
	}
	if cfg.targetLang != "es" {
		t.Fatalf("expected target lang es, got %q", cfg.targetLang)
	}
	if cfg.sourceURL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("unexpected source URL: %q", cfg.sourceURL)
	}
}

func TestParseGeminiDubFlags(t *testing.T) {
	t.Parallel()

	cfg, err := parseGeminiDubFlags([]string{
		"--output", "downloads",
		"--to", "pt-BR",
		"https://www.youtube.com/watch?v=abc123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.outputDir != "downloads" {
		t.Fatalf("expected output dir downloads, got %q", cfg.outputDir)
	}
	if cfg.targetLang != "pt-BR" {
		t.Fatalf("expected target lang pt-BR, got %q", cfg.targetLang)
	}
	if cfg.sourceURL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("unexpected source URL: %q", cfg.sourceURL)
	}
}

func TestParseLocalDubFlags(t *testing.T) {
	t.Parallel()

	cfg, err := parseLocalDubFlags([]string{
		"--output", "downloads",
		"--to", "pt-BR",
		"https://www.youtube.com/watch?v=abc123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.outputDir != "downloads" {
		t.Fatalf("expected output dir downloads, got %q", cfg.outputDir)
	}
	if cfg.targetLang != "pt-BR" {
		t.Fatalf("expected target lang pt-BR, got %q", cfg.targetLang)
	}
	if cfg.sourceURL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("unexpected source URL: %q", cfg.sourceURL)
	}
}

func TestParseGeminiFolderDubFlags(t *testing.T) {
	t.Parallel()

	cfg, err := parseGeminiFolderDubFlags([]string{
		"--input", "downloads",
		"--output", "dubbed",
		"--to", "pt-BR",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.inputDir != "downloads" {
		t.Fatalf("expected input dir downloads, got %q", cfg.inputDir)
	}
	if cfg.outputDir != "dubbed" {
		t.Fatalf("expected output dir dubbed, got %q", cfg.outputDir)
	}
	if cfg.targetLang != "pt-BR" {
		t.Fatalf("expected target lang pt-BR, got %q", cfg.targetLang)
	}
}

func TestValidateLanguageCode(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"pt", "en", "pt-BR"} {
		if err := validateLanguageCode(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}
	if err := validateLanguageCode("pt\n--help"); err == nil {
		t.Fatal("expected unsafe language code to be rejected")
	}
}

func TestFindDownloadedVideosSkipsDubbedOutputs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := []string{
		"video.mp4",
		"clip.webm",
		"already_done [abc123].mp4",
		"already_doneabc123.pt-BR.dub.123.mp4",
		"clip.pt-BR.dub.123.mp4",
		"gemini-dub-pt-BR-123.mp4",
		"notes.txt",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write test file: %v", err)
		}
	}

	videos, err := findDownloadedVideos(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(videos) != 1 {
		t.Fatalf("expected 1 video, got %d: %#v", len(videos), videos)
	}
}

func TestReadFirstTextFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "transcript.txt"), []byte(" hello "), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	got, err := readFirstTextFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected trimmed transcript, got %q", got)
	}
}

func TestGeminiText(t *testing.T) {
	t.Parallel()

	got := geminiText(geminiGenerateResponse{
		Candidates: []struct {
			Content struct {
				Parts []struct {
					Text       string            `json:"text"`
					InlineData *geminiInlineData `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		}{
			{
				Content: struct {
					Parts []struct {
						Text       string            `json:"text"`
						InlineData *geminiInlineData `json:"inlineData"`
					} `json:"parts"`
				}{
					Parts: []struct {
						Text       string            `json:"text"`
						InlineData *geminiInlineData `json:"inlineData"`
					}{
						{Text: " Olá "},
						{Text: "mundo"},
					},
				},
			},
		},
	})
	if got != "Olá\nmundo" {
		t.Fatalf("unexpected Gemini text: %q", got)
	}
}

func TestExtensionForContentType(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"video/mp4":              ".mp4",
		"audio/mpeg":             ".mp3",
		"video/mp4; charset=utf": ".mp4",
		"":                       ".mp4",
	}
	for contentType, want := range tests {
		if got := extensionForContentType(contentType); got != want {
			t.Fatalf("expected %q for %q, got %q", want, contentType, got)
		}
	}
}

func TestSafeFilePart(t *testing.T) {
	t.Parallel()

	if got := safeFilePart("abc/123.pt-BR"); got != "abc123pt-BR" {
		t.Fatalf("unexpected safe file part: %q", got)
	}
}

func TestLoadEnvFile(t *testing.T) {
	t.Setenv("TEST_ENV_KEY", "")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("TEST_ENV_KEY=\"loaded\"\n# ignored\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv("TEST_ENV_KEY"); got != "loaded" {
		t.Fatalf("expected env value loaded, got %q", got)
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

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
