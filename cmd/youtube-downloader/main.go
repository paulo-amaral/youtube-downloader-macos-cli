package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const appName = "youtube-downloader"
const version = "0.1.0"

type config struct {
	outputDir   string
	quality     string
	urlFile     string
	playlist    bool
	update      bool
	openDir     bool
	dryRun      bool
	audioFormat string
	urls        []string
}

type commandMode string

const (
	modeDownload commandMode = "download"
	modeFormats  commandMode = "formats"
	modeCheck    commandMode = "check"
	modeUpdate   commandMode = "update"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		printError(err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return interactive()
	}

	switch args[0] {
	case "download", "dl":
		cfg, err := parseDownloadFlags(args[1:])
		if err != nil {
			return err
		}
		return download(cfg, false)
	case "formats", "fmt":
		cfg, err := parseDownloadFlags(args[1:])
		if err != nil {
			return err
		}
		return download(cfg, true)
	case "check", "doctor":
		return check()
	case "update":
		return streamCommand("yt-dlp", "-U")
	case "version", "-v", "--version":
		fmt.Printf("%s %s\n", appName, version)
		return nil
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		cfg, err := parseDownloadFlags(args)
		if err != nil {
			return err
		}
		return download(cfg, false)
	}
}

func parseDownloadFlags(args []string) (config, error) {
	cfg := config{
		outputDir:   defaultDownloadDir(),
		quality:     "best",
		audioFormat: "mp3",
	}

	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.StringVar(&cfg.outputDir, "output", cfg.outputDir, "directory for downloads")
	fs.StringVar(&cfg.outputDir, "o", cfg.outputDir, "directory for downloads")
	fs.StringVar(&cfg.quality, "quality", cfg.quality, "best, 2160, 1440, 1080, 720, 480, 360, audio, or yt-dlp format")
	fs.StringVar(&cfg.quality, "q", cfg.quality, "best, 2160, 1440, 1080, 720, 480, 360, audio, or yt-dlp format")
	fs.StringVar(&cfg.urlFile, "file", "", "read URLs from a text file")
	fs.StringVar(&cfg.urlFile, "f", "", "read URLs from a text file")
	fs.BoolVar(&cfg.playlist, "playlist", false, "download full playlist")
	fs.BoolVar(&cfg.playlist, "p", false, "download full playlist")
	fs.BoolVar(&cfg.update, "update", false, "update yt-dlp before downloading")
	fs.BoolVar(&cfg.update, "u", false, "update yt-dlp before downloading")
	fs.BoolVar(&cfg.openDir, "open", false, "open output folder in Finder after downloading")
	fs.BoolVar(&cfg.dryRun, "dry-run", false, "print the yt-dlp command without running it")
	fs.StringVar(&cfg.audioFormat, "audio-format", cfg.audioFormat, "audio format when quality is audio")
	fs.Usage = printHelp

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	cfg.urls = fs.Args()
	cfg.outputDir = expandHome(cfg.outputDir)
	return normalizeConfig(cfg)
}

func normalizeConfig(cfg config) (config, error) {
	cfg.outputDir = strings.TrimSpace(cfg.outputDir)
	cfg.quality = strings.TrimSpace(cfg.quality)
	cfg.audioFormat = strings.TrimSpace(cfg.audioFormat)

	if cfg.outputDir == "" {
		return cfg, errors.New("output directory cannot be empty")
	}
	if cfg.quality == "" {
		return cfg, errors.New("quality cannot be empty")
	}
	if cfg.audioFormat == "" {
		return cfg, errors.New("audio format cannot be empty")
	}
	if cfg.urlFile != "" {
		urls, err := readURLFile(cfg.urlFile)
		if err != nil {
			return cfg, err
		}
		cfg.urls = append(cfg.urls, urls...)
	}
	cfg.urls = cleanURLs(cfg.urls)
	return cfg, nil
}

func interactive() error {
	reader := bufio.NewReader(os.Stdin)
	printHeader()

	switch choose(reader, "What do you want to do?", []string{
		"Download videos",
		"List formats for a video",
		"Check dependencies",
		"Update yt-dlp",
		"Quit",
	}, 1) {
	case 1:
		cfg := defaultConfig()
		if err := promptDownload(reader, &cfg); err != nil {
			return err
		}
		return download(cfg, false)
	case 2:
		cfg := defaultConfig()
		if err := promptURLs(reader, &cfg); err != nil {
			return err
		}
		cfg.playlist = promptYesNo(reader, "Treat playlist URLs as playlists?", false)
		return download(cfg, true)
	case 3:
		return check()
	case 4:
		return streamCommand("yt-dlp", "-U")
	default:
		fmt.Println("Bye.")
		return nil
	}
}

func defaultConfig() config {
	return config{
		outputDir:   defaultDownloadDir(),
		quality:     "best",
		audioFormat: "mp3",
		openDir:     runtime.GOOS == "darwin",
	}
}

func promptDownload(reader *bufio.Reader, cfg *config) error {
	if err := promptURLs(reader, cfg); err != nil {
		return err
	}
	if len(cfg.urls) == 0 {
		return errors.New("no YouTube URLs provided")
	}

	cfg.quality = promptQuality(reader)
	if cfg.quality == "audio" {
		cfg.audioFormat = promptText(reader, "Audio format", cfg.audioFormat)
	}
	cfg.outputDir = expandHome(promptText(reader, "Output folder", cfg.outputDir))
	cfg.playlist = promptYesNo(reader, "Download full playlist when URL is a playlist?", false)
	cfg.openDir = promptYesNo(reader, "Open folder in Finder when done?", cfg.openDir)
	cfg.update = promptYesNo(reader, "Update yt-dlp before download?", false)
	cfg.dryRun = promptYesNo(reader, "Preview command only?", false)
	return nil
}

func promptURLs(reader *bufio.Reader, cfg *config) error {
	if runtime.GOOS == "darwin" {
		clip := clipboardText()
		if looksLikeURL(clip) && promptYesNo(reader, "Use URL from clipboard?", true) {
			cfg.urls = append(cfg.urls, clip)
		}
	}

	if len(cfg.urls) > 0 && !promptYesNo(reader, "Add more URLs?", false) {
		return nil
	}

	fmt.Println(dim("Paste URLs one per line. Press Enter on an empty line when done."))
	for {
		fmt.Print(bold("URL: "))
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		cfg.urls = append(cfg.urls, line)
	}
	cfg.urls = cleanURLs(cfg.urls)
	return nil
}

func promptQuality(reader *bufio.Reader) string {
	options := []string{
		"best video",
		"2160p / 4K",
		"1440p",
		"1080p",
		"720p",
		"audio only",
		"custom yt-dlp format",
	}
	switch choose(reader, "Choose quality", options, 1) {
	case 1:
		return "best"
	case 2:
		return "2160"
	case 3:
		return "1440"
	case 4:
		return "1080"
	case 5:
		return "720"
	case 6:
		return "audio"
	default:
		return promptText(reader, "yt-dlp format", "bestvideo+bestaudio/best")
	}
}

func download(cfg config, listFormats bool) error {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return errors.New("yt-dlp is not installed or not on PATH")
	}
	if len(cfg.urls) == 0 {
		return errors.New("no YouTube URLs provided")
	}
	if cfg.update {
		if err := streamCommand("yt-dlp", "-U"); err != nil {
			return err
		}
	}
	args := buildYTDLPArgs(cfg, listFormats)
	printRunSummary(cfg, listFormats, args)
	if cfg.dryRun {
		return nil
	}
	if !listFormats {
		if err := os.MkdirAll(cfg.outputDir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	if err := streamCommand("yt-dlp", args...); err != nil {
		return err
	}
	if cfg.openDir && runtime.GOOS == "darwin" && !listFormats {
		return exec.Command("open", cfg.outputDir).Run()
	}
	return nil
}

func buildYTDLPArgs(cfg config, listFormats bool) []string {
	playlistFlag := "--no-playlist"
	if cfg.playlist {
		playlistFlag = "--yes-playlist"
	}
	if listFormats {
		return append([]string{"--list-formats", playlistFlag}, cfg.urls...)
	}

	args := []string{
		"--ignore-errors",
		"--continue",
		"--no-overwrites",
		"--restrict-filenames",
		"--windows-filenames",
		"--embed-metadata",
		"-o", filepath.Join(cfg.outputDir, "%(title).200s [%(id)s].%(ext)s"),
		playlistFlag,
	}

	if cfg.quality == "audio" {
		args = append(args, "-f", "bestaudio/best", "-x", "--audio-format", cfg.audioFormat)
	} else {
		args = append(args, "--embed-thumbnail", "--merge-output-format", "mp4", "-f", formatForQuality(cfg.quality))
	}
	return append(args, cfg.urls...)
}

func formatForQuality(quality string) string {
	switch quality {
	case "best":
		return "bestvideo+bestaudio/best"
	case "2160", "1440", "1080", "720", "480", "360":
		return fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best[height<=%s]/best", quality, quality)
	default:
		return quality
	}
}

func check() error {
	printHeader()
	fmt.Println(bold("Dependency check"))

	ytDLPPath, ytDLPErr := exec.LookPath("yt-dlp")
	printCheck("yt-dlp", ytDLPPath, ytDLPErr, true)
	if ytDLPErr == nil {
		version, _ := commandOutput("yt-dlp", "--version")
		fmt.Println(dim("  version: " + strings.TrimSpace(version)))
	}

	ffmpegPath, ffmpegErr := exec.LookPath("ffmpeg")
	printCheck("ffmpeg", ffmpegPath, ffmpegErr, false)

	if ytDLPErr != nil {
		return errors.New("install yt-dlp before downloading")
	}
	if ffmpegErr != nil {
		fmt.Println(dim("Tip: install ffmpeg for best video/audio merging and thumbnails."))
	}
	return nil
}

func readURLFile(path string) ([]string, error) {
	file, err := os.Open(expandHome(path))
	if err != nil {
		return nil, fmt.Errorf("open URL file: %w", err)
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read URL file: %w", err)
	}
	return urls, nil
}

func streamCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func commandOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func choose(reader *bufio.Reader, title string, options []string, defaultChoice int) int {
	fmt.Println()
	fmt.Println(bold(title))
	for i, option := range options {
		fmt.Printf("  %d. %s\n", i+1, option)
	}
	for {
		fmt.Printf("Choose [%d]: ", defaultChoice)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return defaultChoice
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= len(options) {
			return n
		}
		fmt.Println("Please choose a number from the list.")
	}
}

func promptText(reader *bufio.Reader, label, current string) string {
	fmt.Printf("%s [%s]: ", bold(label), current)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return current
	}
	return line
}

func promptYesNo(reader *bufio.Reader, label string, current bool) bool {
	defaultLabel := "n"
	if current {
		defaultLabel = "y"
	}
	for {
		fmt.Printf("%s [%s]: ", bold(label), defaultLabel)
		line, _ := reader.ReadString('\n')
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" {
			return current
		}
		switch line {
		case "y", "yes", "s", "sim":
			return true
		case "n", "no", "nao", "não":
			return false
		}
		fmt.Println("Please answer y or n.")
	}
}

func clipboardText() string {
	out, err := commandOutput("pbpaste")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func cleanURLs(values []string) []string {
	seen := map[string]bool{}
	var urls []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		urls = append(urls, value)
	}
	return urls
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "downloads"
	}
	return filepath.Join(home, "Downloads", "YouTube")
}

func expandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func printHeader() {
	fmt.Println(color("YouTube Downloader", "36"))
	fmt.Println(dim("Fast downloads with yt-dlp, tuned for macOS."))
}

func printHelp() {
	printHeader()
	fmt.Println()
	fmt.Println(bold("Usage"))
	fmt.Printf("  %s                         interactive mode\n", appName)
	fmt.Printf("  %s download [options] URL  download videos\n", appName)
	fmt.Printf("  %s formats URL             list available formats\n", appName)
	fmt.Printf("  %s check                   check dependencies\n", appName)
	fmt.Printf("  %s update                  update yt-dlp\n", appName)
	fmt.Printf("  %s version                 print version\n", appName)
	fmt.Println()
	fmt.Println(bold("Options"))
	fmt.Println("  -q, --quality VALUE       best, 1080, 720, audio, or yt-dlp format")
	fmt.Println("  -o, --output DIR          download folder")
	fmt.Println("  -f, --file FILE           read URLs from file")
	fmt.Println("  -p, --playlist            download full playlists")
	fmt.Println("      --audio-format VALUE  mp3, m4a, opus, wav, etc. Default: mp3")
	fmt.Println("      --open                open folder in Finder")
	fmt.Println("      --dry-run             preview yt-dlp command")
}

func printRunSummary(cfg config, listFormats bool, args []string) {
	fmt.Println()
	if listFormats {
		fmt.Println(bold("Listing formats"))
	} else {
		fmt.Println(bold("Download ready"))
		fmt.Println("Folder:  " + cfg.outputDir)
		fmt.Println("Quality: " + cfg.quality)
	}
	fmt.Printf("URLs:    %d\n", len(cfg.urls))
	fmt.Println(dim("Command: yt-dlp " + shellQuoteArgs(args)))
	fmt.Println()
}

func printCheck(name, path string, err error, required bool) {
	label := "optional"
	if required {
		label = "required"
	}
	if err != nil {
		fmt.Printf("%s %s %s\n", color("missing", "31"), name, dim("("+label+")"))
		return
	}
	fmt.Printf("%s %s %s\n", color("ok", "32"), name, dim(path))
}

func printError(err error) {
	fmt.Fprintln(os.Stderr, color("Error:", "31"), err)
}

func bold(value string) string {
	return color(value, "1")
}

func dim(value string) string {
	return color(value, "2")
}

func color(value, code string) string {
	if os.Getenv("NO_COLOR") != "" {
		return value
	}
	return "\033[" + code + "m" + value + "\033[0m"
}

func shellQuoteArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"'&?[]()") {
			quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}
