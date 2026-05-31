package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const appName = "youtube-downloader"
const version = "0.1.0"
const maxURLFileBytes int64 = 1 << 20
const maxConcurrentDownloads = 8
const elevenLabsAPIBase = "https://api.elevenlabs.io/v1"
const geminiAPIBase = "https://generativelanguage.googleapis.com"

type config struct {
	outputDir     string
	quality       string
	urlFile       string
	playlist      bool
	update        bool
	openDir       bool
	dryRun        bool
	dubAfter      bool
	dubEngine     string
	concurrent    int
	subtitles     bool
	autoSubtitles bool
	subtitleLangs string
	audioFormat   string
	urls          []string
}

type dubConfig struct {
	outputDir  string
	targetLang string
	sourceURL  string
	name       string
}

type geminiDubConfig struct {
	outputDir  string
	targetLang string
	sourceURL  string
}

type geminiFolderDubConfig struct {
	inputDir   string
	outputDir  string
	targetLang string
}

type localDubConfig struct {
	outputDir  string
	targetLang string
	sourceURL  string
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
	if err := loadDefaultEnvFiles(); err != nil {
		return err
	}

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
	case "dub":
		cfg, err := parseDubFlags(args[1:])
		if err != nil {
			return err
		}
		return dub(cfg)
	case "dub-gemini":
		cfg, err := parseGeminiDubFlags(args[1:])
		if err != nil {
			return err
		}
		return dubWithGemini(cfg)
	case "dub-local":
		cfg, err := parseLocalDubFlags(args[1:])
		if err != nil {
			return err
		}
		return dubWithLocalAI(cfg)
	case "dub-downloaded":
		cfg, err := parseGeminiFolderDubFlags(args[1:])
		if err != nil {
			return err
		}
		return dubDownloadedWithGemini(cfg)
	case "dub-downloaded-local":
		cfg, err := parseGeminiFolderDubFlags(args[1:])
		if err != nil {
			return err
		}
		return dubDownloadedWithLocalAI(cfg)
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
		outputDir:     defaultDownloadDir(),
		quality:       "best",
		audioFormat:   "mp3",
		concurrent:    1,
		subtitleLangs: "pt.*,pt-BR,pt",
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
	fs.BoolVar(&cfg.dubAfter, "dub-after", false, "offer dubbing after download")
	fs.StringVar(&cfg.dubEngine, "dub-engine", "", "dubbing engine after download: auto, gemini, local")
	fs.IntVar(&cfg.concurrent, "concurrent", cfg.concurrent, "number of videos to download at the same time")
	fs.IntVar(&cfg.concurrent, "j", cfg.concurrent, "number of videos to download at the same time")
	fs.BoolVar(&cfg.subtitles, "subtitles", false, "download and embed Portuguese subtitles when available")
	fs.BoolVar(&cfg.subtitles, "subs", false, "download and embed Portuguese subtitles when available")
	fs.BoolVar(&cfg.autoSubtitles, "auto-subtitles", false, "include auto-generated subtitles when manual subtitles are unavailable")
	fs.StringVar(&cfg.subtitleLangs, "subtitle-langs", cfg.subtitleLangs, "yt-dlp subtitle language selector")
	fs.StringVar(&cfg.audioFormat, "audio-format", cfg.audioFormat, "audio format when quality is audio")
	fs.Usage = printHelp

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	cfg.urls = fs.Args()
	cfg.outputDir = expandHome(cfg.outputDir)
	return normalizeConfig(cfg)
}

func parseDubFlags(args []string) (dubConfig, error) {
	cfg := dubConfig{
		outputDir:  defaultDownloadDir(),
		targetLang: "pt",
	}

	fs := flag.NewFlagSet(appName+" dub", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.StringVar(&cfg.outputDir, "output", cfg.outputDir, "directory for dubbed videos")
	fs.StringVar(&cfg.outputDir, "o", cfg.outputDir, "directory for dubbed videos")
	fs.StringVar(&cfg.targetLang, "to", cfg.targetLang, "target language code for dubbing")
	fs.StringVar(&cfg.targetLang, "t", cfg.targetLang, "target language code for dubbing")
	fs.Usage = printHelp

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		return cfg, errors.New("dub requires exactly one YouTube URL")
	}

	cfg.outputDir = expandHome(strings.TrimSpace(cfg.outputDir))
	cfg.targetLang = strings.TrimSpace(cfg.targetLang)
	cfg.sourceURL = strings.TrimSpace(remaining[0])
	cfg.name = "YouTube dub"
	if err := validateOutputDir(cfg.outputDir); err != nil {
		return cfg, err
	}
	if err := validateLanguageCode(cfg.targetLang); err != nil {
		return cfg, err
	}
	if err := validateYouTubeURL(cfg.sourceURL); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func parseGeminiDubFlags(args []string) (geminiDubConfig, error) {
	cfg := geminiDubConfig{
		outputDir:  defaultDownloadDir(),
		targetLang: "pt-BR",
	}

	fs := flag.NewFlagSet(appName+" dub-gemini", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.StringVar(&cfg.outputDir, "output", cfg.outputDir, "directory for dubbed videos")
	fs.StringVar(&cfg.outputDir, "o", cfg.outputDir, "directory for dubbed videos")
	fs.StringVar(&cfg.targetLang, "to", cfg.targetLang, "target language code for dubbing")
	fs.StringVar(&cfg.targetLang, "t", cfg.targetLang, "target language code for dubbing")
	fs.Usage = printHelp

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		return cfg, errors.New("dub-gemini requires exactly one YouTube URL")
	}

	cfg.outputDir = expandHome(strings.TrimSpace(cfg.outputDir))
	cfg.targetLang = strings.TrimSpace(cfg.targetLang)
	cfg.sourceURL = strings.TrimSpace(remaining[0])
	if err := validateOutputDir(cfg.outputDir); err != nil {
		return cfg, err
	}
	if err := validateLanguageCode(cfg.targetLang); err != nil {
		return cfg, err
	}
	if err := validateYouTubeURL(cfg.sourceURL); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func parseLocalDubFlags(args []string) (localDubConfig, error) {
	cfg := localDubConfig{
		outputDir:  defaultDownloadDir(),
		targetLang: "pt-BR",
	}

	fs := flag.NewFlagSet(appName+" dub-local", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.StringVar(&cfg.outputDir, "output", cfg.outputDir, "directory for dubbed videos")
	fs.StringVar(&cfg.outputDir, "o", cfg.outputDir, "directory for dubbed videos")
	fs.StringVar(&cfg.targetLang, "to", cfg.targetLang, "target language code for dubbing")
	fs.StringVar(&cfg.targetLang, "t", cfg.targetLang, "target language code for dubbing")
	fs.Usage = printHelp

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		return cfg, errors.New("dub-local requires exactly one YouTube URL")
	}

	cfg.outputDir = expandHome(strings.TrimSpace(cfg.outputDir))
	cfg.targetLang = strings.TrimSpace(cfg.targetLang)
	cfg.sourceURL = strings.TrimSpace(remaining[0])
	if err := validateOutputDir(cfg.outputDir); err != nil {
		return cfg, err
	}
	if err := validateLanguageCode(cfg.targetLang); err != nil {
		return cfg, err
	}
	if err := validateYouTubeURL(cfg.sourceURL); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func parseGeminiFolderDubFlags(args []string) (geminiFolderDubConfig, error) {
	cfg := geminiFolderDubConfig{
		inputDir:   defaultDownloadDir(),
		outputDir:  defaultDownloadDir(),
		targetLang: "pt-BR",
	}

	fs := flag.NewFlagSet(appName+" dub-downloaded", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.StringVar(&cfg.inputDir, "input", cfg.inputDir, "folder with downloaded videos")
	fs.StringVar(&cfg.inputDir, "i", cfg.inputDir, "folder with downloaded videos")
	fs.StringVar(&cfg.outputDir, "output", cfg.outputDir, "directory for dubbed videos")
	fs.StringVar(&cfg.outputDir, "o", cfg.outputDir, "directory for dubbed videos")
	fs.StringVar(&cfg.targetLang, "to", cfg.targetLang, "target language code for dubbing")
	fs.StringVar(&cfg.targetLang, "t", cfg.targetLang, "target language code for dubbing")
	fs.Usage = printHelp

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if len(fs.Args()) != 0 {
		return cfg, errors.New("dub-downloaded does not accept URLs; use dub-gemini to download and dub one URL")
	}

	cfg.inputDir = expandHome(strings.TrimSpace(cfg.inputDir))
	cfg.outputDir = expandHome(strings.TrimSpace(cfg.outputDir))
	cfg.targetLang = strings.TrimSpace(cfg.targetLang)
	if cfg.inputDir == "" {
		return cfg, errors.New("input directory cannot be empty")
	}
	if err := validateOutputDir(cfg.outputDir); err != nil {
		return cfg, err
	}
	if err := validateLanguageCode(cfg.targetLang); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func normalizeConfig(cfg config) (config, error) {
	cfg.outputDir = strings.TrimSpace(cfg.outputDir)
	cfg.quality = strings.TrimSpace(cfg.quality)
	cfg.audioFormat = strings.TrimSpace(cfg.audioFormat)
	cfg.dubEngine = strings.ToLower(strings.TrimSpace(cfg.dubEngine))
	cfg.subtitleLangs = strings.TrimSpace(cfg.subtitleLangs)

	if cfg.outputDir == "" {
		return cfg, errors.New("output directory cannot be empty")
	}
	if err := validateOutputDir(cfg.outputDir); err != nil {
		return cfg, err
	}
	if cfg.quality == "" {
		return cfg, errors.New("quality cannot be empty")
	}
	if cfg.audioFormat == "" {
		return cfg, errors.New("audio format cannot be empty")
	}
	if err := validateAudioFormat(cfg.audioFormat); err != nil {
		return cfg, err
	}
	if err := validateConcurrentDownloads(cfg.concurrent); err != nil {
		return cfg, err
	}
	if err := validateDubEngine(cfg.dubEngine); err != nil {
		return cfg, err
	}
	if cfg.autoSubtitles {
		cfg.subtitles = true
	}
	if cfg.subtitles {
		if err := validateSubtitleLangs(cfg.subtitleLangs); err != nil {
			return cfg, err
		}
	}
	if cfg.urlFile != "" {
		urls, err := readURLFile(cfg.urlFile)
		if err != nil {
			return cfg, err
		}
		cfg.urls = append(cfg.urls, urls...)
	}
	urls, err := cleanURLs(cfg.urls)
	if err != nil {
		return cfg, err
	}
	cfg.urls = urls
	return cfg, nil
}

func interactive() error {
	reader := bufio.NewReader(os.Stdin)
	printHeader()

	switch choose(reader, "What do you want to do?", []string{
		"Download videos",
		"Download and dub",
		"Dub downloaded videos",
		"Dub one video",
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
		cfg.dubAfter = true
		cfg.dubEngine = "auto"
		if err := promptDownload(reader, &cfg); err != nil {
			return err
		}
		cfg.dubAfter = true
		return download(cfg, false)
	case 3:
		return promptDubDownloaded(reader)
	case 4:
		return promptDubOne(reader)
	case 5:
		cfg := defaultConfig()
		if err := promptURLs(reader, &cfg); err != nil {
			return err
		}
		cfg.playlist = promptYesNo(reader, "Treat playlist URLs as playlists?", false)
		return download(cfg, true)
	case 6:
		return check()
	case 7:
		return streamCommand("yt-dlp", "-U")
	default:
		fmt.Println("Bye.")
		return nil
	}
}

func defaultConfig() config {
	return config{
		outputDir:     defaultDownloadDir(),
		quality:       "best",
		audioFormat:   "mp3",
		concurrent:    1,
		subtitleLangs: "pt.*,pt-BR,pt",
		openDir:       runtime.GOOS == "darwin",
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
	if cfg.quality != "audio" {
		cfg.subtitles = promptYesNo(reader, "Download and embed Portuguese subtitles?", false)
		if cfg.subtitles {
			cfg.autoSubtitles = promptYesNo(reader, "Use auto-generated subtitles if needed?", true)
			cfg.subtitleLangs = promptText(reader, "Subtitle languages", cfg.subtitleLangs)
		}
	}
	cfg.outputDir = expandHome(promptText(reader, "Output folder", cfg.outputDir))
	cfg.playlist = promptYesNo(reader, "Download full playlist when URL is a playlist?", false)
	cfg.concurrent = promptInt(reader, "Simultaneous downloads", cfg.concurrent, 1, maxConcurrentDownloads)
	cfg.openDir = promptYesNo(reader, "Open folder in Finder when done?", cfg.openDir)
	cfg.update = promptYesNo(reader, "Update yt-dlp before download?", false)
	cfg.dryRun = promptYesNo(reader, "Preview command only?", false)
	if cfg.dubAfter {
		cfg.dubEngine = "auto"
	} else {
		cfg.dubAfter = promptYesNo(reader, "Offer dubbing after download?", false)
	}
	return nil
}

func promptDubOne(reader *bufio.Reader) error {
	urls, err := promptURLList(reader, 1)
	if err != nil {
		return err
	}
	if len(urls) == 0 {
		return errors.New("no YouTube URL provided")
	}
	targetLang := promptText(reader, "Target language", "pt-BR")
	outputDir := expandHome(promptText(reader, "Output folder", defaultDownloadDir()))

	switch choose(reader, "Dubbing engine", []string{
		"Gemini API",
		"Local AI",
		"ElevenLabs",
	}, 1) {
	case 1:
		return dubWithGemini(geminiDubConfig{outputDir: outputDir, targetLang: targetLang, sourceURL: urls[0]})
	case 2:
		return dubWithLocalAI(localDubConfig{outputDir: outputDir, targetLang: targetLang, sourceURL: urls[0]})
	default:
		if targetLang == "pt-BR" {
			targetLang = "pt"
		}
		return dub(dubConfig{outputDir: outputDir, targetLang: targetLang, sourceURL: urls[0], name: "YouTube dub"})
	}
}

func promptDubDownloaded(reader *bufio.Reader) error {
	inputDir := expandHome(promptText(reader, "Input folder", defaultDownloadDir()))
	outputDir := expandHome(promptText(reader, "Output folder", inputDir))
	targetLang := promptText(reader, "Target language", "pt-BR")

	return offerDubbingWithEngineChoice(reader, inputDir, outputDir, targetLang)
}

func offerDubbingForDownloadedVideos(reader *bufio.Reader, dir, engine string) error {
	videos, err := findDownloadedVideos(dir)
	if err != nil {
		return err
	}
	if len(videos) == 0 {
		printHint("No newly downloaded videos need dubbing.")
		return nil
	}
	printSection("Downloaded videos ready for dubbing")
	printField("folder", dir)
	printField("videos", strconv.Itoa(len(videos)))

	if reader == nil {
		reader = bufio.NewReader(os.Stdin)
	}
	if engine == "" && !promptYesNo(reader, "Dub these videos now?", true) {
		return nil
	}
	targetLang := promptText(reader, "Target language", "pt-BR")
	return offerDubbingWithEngine(reader, dir, dir, targetLang, engine)
}

func offerDubbingWithEngineChoice(reader *bufio.Reader, inputDir, outputDir, targetLang string) error {
	return offerDubbingWithEngine(reader, inputDir, outputDir, targetLang, "")
}

func offerDubbingWithEngine(reader *bufio.Reader, inputDir, outputDir, targetLang, engine string) error {
	engine = normalizeDubEngine(engine)
	switch engine {
	case "gemini":
		return dubDownloadedWithGemini(geminiFolderDubConfig{inputDir: inputDir, outputDir: outputDir, targetLang: targetLang})
	case "local":
		return dubDownloadedWithLocalAI(geminiFolderDubConfig{inputDir: inputDir, outputDir: outputDir, targetLang: targetLang})
	}

	if preferred := preferredDubEngine(); preferred != "" {
		printField("engine", preferred)
		return offerDubbingWithEngine(reader, inputDir, outputDir, targetLang, preferred)
	}

	switch choose(reader, "Dubbing engine", []string{
		"Gemini API",
		"Local AI",
	}, 1) {
	case 1:
		return dubDownloadedWithGemini(geminiFolderDubConfig{inputDir: inputDir, outputDir: outputDir, targetLang: targetLang})
	default:
		return dubDownloadedWithLocalAI(geminiFolderDubConfig{inputDir: inputDir, outputDir: outputDir, targetLang: targetLang})
	}
}

func preferredDubEngine() string {
	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != "" {
		return "gemini"
	}
	if localDubbingLooksConfigured() {
		return "local"
	}
	return ""
}

func normalizeDubEngine(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "auto" {
		return ""
	}
	return value
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

	urls, err := promptURLList(reader, 0)
	if err != nil {
		return err
	}
	cfg.urls = append(cfg.urls, urls...)
	urls, err = cleanURLs(cfg.urls)
	if err != nil {
		return err
	}
	cfg.urls = urls
	return nil
}

func promptURLList(reader *bufio.Reader, max int) ([]string, error) {
	message := "Paste URLs one per line. Press Enter on an empty line when done."
	if max == 1 {
		message = "Paste one YouTube URL."
	}
	printHint(message)
	var urls []string
	for {
		fmt.Printf("%s ", promptLabel("URL"))
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		urls = append(urls, line)
		if max > 0 && len(urls) >= max {
			break
		}
	}
	urls, err := cleanURLs(urls)
	if err != nil {
		return nil, err
	}
	return urls, nil
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

	if cfg.concurrent > 1 && !listFormats && len(cfg.urls) > 1 {
		if err := downloadConcurrently(cfg); err != nil {
			return err
		}
	} else if err := streamCommand("yt-dlp", args...); err != nil {
		return err
	}
	if cfg.dubAfter && !listFormats && cfg.quality != "audio" {
		if err := offerDubbingForDownloadedVideos(nil, cfg.outputDir, cfg.dubEngine); err != nil {
			return err
		}
	}
	if cfg.openDir && runtime.GOOS == "darwin" && !listFormats {
		return exec.Command("open", cfg.outputDir).Run()
	}
	return nil
}

func downloadConcurrently(cfg config) error {
	workers := cfg.concurrent
	if workers > len(cfg.urls) {
		workers = len(cfg.urls)
	}

	jobs := make(chan string)
	results := make(chan error, len(cfg.urls))
	for worker := 1; worker <= workers; worker++ {
		go func(worker int) {
			for videoURL := range jobs {
				one := cfg
				one.urls = []string{videoURL}
				printStep(fmt.Sprintf("worker %d starting %s", worker, videoURL))
				if err := streamCommand("yt-dlp", buildYTDLPArgs(one, false)...); err != nil {
					results <- fmt.Errorf("download %q: %w", videoURL, err)
					continue
				}
				printStep(fmt.Sprintf("worker %d finished %s", worker, videoURL))
				results <- nil
			}
		}(worker)
	}

	for _, videoURL := range cfg.urls {
		jobs <- videoURL
	}
	close(jobs)

	var failures []error
	for range cfg.urls {
		if err := <-results; err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d parallel download(s) failed: %w", len(failures), failures[0])
	}
	return nil
}

func dub(cfg dubConfig) error {
	apiKey := strings.TrimSpace(os.Getenv("ELEVENLABS_API_KEY"))
	if apiKey == "" {
		return errors.New("set ELEVENLABS_API_KEY before dubbing")
	}
	if err := os.MkdirAll(cfg.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Minute}
	printSection("Dubbing ready")
	printField("folder", cfg.outputDir)
	printField("target", cfg.targetLang)
	fmt.Println()

	job, err := createDubbingJob(client, apiKey, cfg)
	if err != nil {
		return err
	}
	printField("dubbing id", job.DubbingID)
	if job.ExpectedDurationSec > 0 {
		printField("duration", fmt.Sprintf("%.0fs", job.ExpectedDurationSec))
	}

	if err := waitForDubbing(client, apiKey, job.DubbingID); err != nil {
		return err
	}

	outPath, err := downloadDubbedMedia(client, apiKey, job.DubbingID, cfg.targetLang, cfg.outputDir)
	if err != nil {
		return err
	}
	printStep("saved " + outPath)
	return nil
}

func dubWithGemini(cfg geminiDubConfig) error {
	apiKey, err := requireGeminiDubbingDeps(true)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	workDir, err := os.MkdirTemp("", appName+"-dub-*")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	printSection("Gemini dubbing ready")
	printField("folder", cfg.outputDir)
	printField("target", cfg.targetLang)
	fmt.Println()

	videoPath, err := downloadVideoForDubbing(cfg.sourceURL, workDir)
	if err != nil {
		return err
	}

	_, err = dubVideoFileWithGemini(apiKey, cfg.targetLang, cfg.outputDir, videoPath)
	return err
}

func dubDownloadedWithGemini(cfg geminiFolderDubConfig) error {
	apiKey, err := requireGeminiDubbingDeps(false)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	videos, err := findDownloadedVideos(cfg.inputDir)
	if err != nil {
		return err
	}
	if len(videos) == 0 {
		return fmt.Errorf("no downloaded videos found in %s", cfg.inputDir)
	}

	printSection("Gemini dubbing downloaded videos")
	printField("input", cfg.inputDir)
	printField("output", cfg.outputDir)
	printField("target", cfg.targetLang)
	printField("videos", strconv.Itoa(len(videos)))
	fmt.Println()

	var failures []error
	for index, videoPath := range videos {
		printStep(fmt.Sprintf("[%d/%d] %s", index+1, len(videos), videoPath))
		if _, err := dubVideoFileWithGemini(apiKey, cfg.targetLang, cfg.outputDir, videoPath); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", videoPath, err))
			fmt.Println(danger("failed"), err)
			continue
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d video(s) failed to dub: %w", len(failures), failures[0])
	}
	return nil
}

func dubWithLocalAI(cfg localDubConfig) error {
	stack, err := requireLocalDubbingDeps(true)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	workDir, err := os.MkdirTemp("", appName+"-local-dub-*")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	printSection("Local AI dubbing ready")
	printField("folder", cfg.outputDir)
	printField("target", cfg.targetLang)
	printField("asr", stack.whisper)
	printField("llm", stack.ollamaModel)
	printField("tts", stack.piperVoice)
	fmt.Println()

	videoPath, err := downloadVideoForDubbing(cfg.sourceURL, workDir)
	if err != nil {
		return err
	}
	_, err = dubVideoFileWithLocalAI(stack, cfg.targetLang, cfg.outputDir, videoPath)
	return err
}

func dubDownloadedWithLocalAI(cfg geminiFolderDubConfig) error {
	stack, err := requireLocalDubbingDeps(false)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	videos, err := findDownloadedVideos(cfg.inputDir)
	if err != nil {
		return err
	}
	if len(videos) == 0 {
		return fmt.Errorf("no downloaded videos found in %s", cfg.inputDir)
	}

	printSection("Local AI dubbing downloaded videos")
	printField("input", cfg.inputDir)
	printField("output", cfg.outputDir)
	printField("target", cfg.targetLang)
	printField("videos", strconv.Itoa(len(videos)))
	fmt.Println()

	var failures []error
	for index, videoPath := range videos {
		printStep(fmt.Sprintf("[%d/%d] %s", index+1, len(videos), videoPath))
		if _, err := dubVideoFileWithLocalAI(stack, cfg.targetLang, cfg.outputDir, videoPath); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", videoPath, err))
			fmt.Println(danger("failed"), err)
			continue
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d video(s) failed to dub locally: %w", len(failures), failures[0])
	}
	return nil
}

type localDubbingStack struct {
	whisper     string
	ollama      string
	ollamaModel string
	piper       string
	piperVoice  string
}

func requireGeminiDubbingDeps(needsYTDLP bool) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		return "", errors.New("set GEMINI_API_KEY before using Gemini dubbing")
	}
	if needsYTDLP {
		if _, err := exec.LookPath("yt-dlp"); err != nil {
			return "", errors.New("yt-dlp is not installed or not on PATH")
		}
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return "", errors.New("ffmpeg is not installed or not on PATH")
	}
	return apiKey, nil
}

func requireLocalDubbingDeps(needsYTDLP bool) (localDubbingStack, error) {
	var stack localDubbingStack
	var err error
	if needsYTDLP {
		if _, err := exec.LookPath("yt-dlp"); err != nil {
			return stack, errors.New("yt-dlp is not installed or not on PATH")
		}
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return stack, errors.New("ffmpeg is not installed or not on PATH")
	}

	stack.whisper, err = commandFromEnvOrPath("WHISPER_BIN", "whisper", "whisper-cli")
	if err != nil {
		return stack, errors.New("install Whisper locally or set WHISPER_BIN in .env")
	}
	stack.ollama, err = commandFromEnvOrPath("OLLAMA_BIN", "ollama")
	if err != nil {
		return stack, errors.New("install Ollama locally or set OLLAMA_BIN in .env")
	}
	stack.ollamaModel = strings.TrimSpace(os.Getenv("OLLAMA_MODEL"))
	if stack.ollamaModel == "" {
		stack.ollamaModel = "llama3.1:8b"
	}
	stack.piper, err = commandFromEnvOrPath("PIPER_BIN", "piper")
	if err != nil {
		return stack, errors.New("install Piper locally or set PIPER_BIN in .env")
	}
	stack.piperVoice = expandHome(strings.TrimSpace(os.Getenv("PIPER_VOICE")))
	if stack.piperVoice == "" {
		return stack, errors.New("set PIPER_VOICE in .env to a Piper .onnx voice model")
	}
	if info, err := os.Stat(stack.piperVoice); err != nil || info.IsDir() {
		return stack, fmt.Errorf("PIPER_VOICE must point to a .onnx voice model: %s", stack.piperVoice)
	}
	return stack, nil
}

func localDubbingLooksConfigured() bool {
	if strings.TrimSpace(os.Getenv("PIPER_VOICE")) == "" {
		return false
	}
	if _, err := commandFromEnvOrPath("WHISPER_BIN", "whisper", "whisper-cli"); err != nil {
		return false
	}
	if _, err := commandFromEnvOrPath("OLLAMA_BIN", "ollama"); err != nil {
		return false
	}
	if _, err := commandFromEnvOrPath("PIPER_BIN", "piper"); err != nil {
		return false
	}
	return true
}

func commandFromEnvOrPath(envName string, names ...string) (string, error) {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return expandHome(value), nil
	}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("none of %s found", strings.Join(names, ", "))
}

func dubVideoFileWithGemini(apiKey, targetLang, outputDir, videoPath string) (string, error) {
	workDir, err := os.MkdirTemp("", appName+"-dub-*")
	if err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	audioPath := filepath.Join(workDir, "source-audio.mp3")
	if err := extractAudioForGemini(videoPath, audioPath); err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	printStep("uploading audio to Gemini")
	file, err := uploadGeminiFile(client, apiKey, audioPath, "audio/mpeg", safeFilePart(videoBaseName(videoPath)))
	if err != nil {
		return "", err
	}

	printStep("creating translated narration")
	script, err := createGeminiDubScript(client, apiKey, file.URI, file.MimeType, targetLang)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(script) == "" {
		return "", errors.New("Gemini returned an empty narration script")
	}

	printStep("generating dubbed voice")
	pcm, err := generateGeminiSpeech(client, apiKey, script)
	if err != nil {
		return "", err
	}
	voicePath := filepath.Join(workDir, "dubbed-voice.wav")
	if err := writeWAVFile(voicePath, pcm, 1, 24000, 2); err != nil {
		return "", err
	}

	outPath := filepath.Join(outputDir, fmt.Sprintf("%s.%s.dub.%d.mp4", safeFilePart(videoBaseName(videoPath)), safeFilePart(targetLang), time.Now().Unix()))
	if err := muxDubbedVideo(videoPath, voicePath, outPath); err != nil {
		return "", err
	}
	printStep("saved " + outPath)
	return outPath, nil
}

func dubVideoFileWithLocalAI(stack localDubbingStack, targetLang, outputDir, videoPath string) (string, error) {
	workDir, err := os.MkdirTemp("", appName+"-local-dub-*")
	if err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	audioPath := filepath.Join(workDir, "source-audio.wav")
	if err := extractAudioForLocalAI(videoPath, audioPath); err != nil {
		return "", err
	}

	printStep("transcribing with Whisper")
	transcript, err := transcribeWithWhisper(stack.whisper, audioPath, workDir)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(transcript) == "" {
		return "", errors.New("Whisper returned an empty transcript")
	}

	printStep("translating with Ollama")
	script, err := translateWithOllama(stack.ollama, stack.ollamaModel, transcript, targetLang)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(script) == "" {
		return "", errors.New("Ollama returned an empty translation")
	}

	printStep("generating voice with Piper")
	voicePath := filepath.Join(workDir, "dubbed-voice.wav")
	if err := synthesizeWithPiper(stack.piper, stack.piperVoice, script, voicePath); err != nil {
		return "", err
	}

	outPath := filepath.Join(outputDir, fmt.Sprintf("%s.%s.local-dub.%d.mp4", safeFilePart(videoBaseName(videoPath)), safeFilePart(targetLang), time.Now().Unix()))
	if err := muxDubbedVideo(videoPath, voicePath, outPath); err != nil {
		return "", err
	}
	printStep("saved " + outPath)
	return outPath, nil
}

type createDubbingResponse struct {
	DubbingID           string  `json:"dubbing_id"`
	ExpectedDurationSec float64 `json:"expected_duration_sec"`
}

type geminiFile struct {
	Name     string `json:"name"`
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
}

type geminiUploadResponse struct {
	File geminiFile `json:"file"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text       string            `json:"text"`
				InlineData *geminiInlineData `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type getDubbingResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func createDubbingJob(client *http.Client, apiKey string, cfg dubConfig) (createDubbingResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"name":        cfg.name,
		"source_url":  cfg.sourceURL,
		"source_lang": "auto",
		"target_lang": cfg.targetLang,
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return createDubbingResponse{}, fmt.Errorf("prepare dubbing request: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return createDubbingResponse{}, fmt.Errorf("prepare dubbing request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, elevenLabsAPIBase+"/dubbing", &body)
	if err != nil {
		return createDubbingResponse{}, err
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var result createDubbingResponse
	if err := doJSON(client, req, &result); err != nil {
		return result, err
	}
	if result.DubbingID == "" {
		return result, errors.New("ElevenLabs response did not include a dubbing_id")
	}
	return result, nil
}

func waitForDubbing(client *http.Client, apiKey, dubbingID string) error {
	deadline := time.Now().Add(3 * time.Hour)
	for {
		status, err := getDubbingStatus(client, apiKey, dubbingID)
		if err != nil {
			return err
		}
		switch strings.ToLower(status.Status) {
		case "dubbed":
			printStep("status dubbed")
			return nil
		case "failed":
			if status.Error != "" {
				return fmt.Errorf("dubbing failed: %s", status.Error)
			}
			return errors.New("dubbing failed")
		default:
			printStep("status " + status.Status)
		}
		if time.Now().After(deadline) {
			return errors.New("dubbing timed out after 3 hours")
		}
		time.Sleep(5 * time.Second)
	}
}

func getDubbingStatus(client *http.Client, apiKey, dubbingID string) (getDubbingResponse, error) {
	req, err := http.NewRequest(http.MethodGet, elevenLabsAPIBase+"/dubbing/"+url.PathEscape(dubbingID), nil)
	if err != nil {
		return getDubbingResponse{}, err
	}
	req.Header.Set("xi-api-key", apiKey)

	var result getDubbingResponse
	if err := doJSON(client, req, &result); err != nil {
		return result, err
	}
	if result.Status == "" {
		return result, errors.New("ElevenLabs response did not include a status")
	}
	return result, nil
}

func downloadDubbedMedia(client *http.Client, apiKey, dubbingID, languageCode, outputDir string) (string, error) {
	endpoint := fmt.Sprintf("%s/dubbing/%s/audio/%s", elevenLabsAPIBase, url.PathEscape(dubbingID), url.PathEscape(languageCode))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("xi-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download dubbed media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("download dubbed media: ElevenLabs returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}

	ext := extensionForContentType(resp.Header.Get("Content-Type"))
	outPath := filepath.Join(outputDir, fmt.Sprintf("dub-%s-%s%s", safeFilePart(dubbingID), safeFilePart(languageCode), ext))
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create dubbed media file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("write dubbed media file: %w", err)
	}
	return outPath, nil
}

func doJSON(client *http.Client, req *http.Request, target any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ElevenLabs returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode ElevenLabs response: %w", err)
	}
	return nil
}

func downloadVideoForDubbing(sourceURL, workDir string) (string, error) {
	printStep("downloading source video")
	args := append(ytdlpJSRuntimeArgs(),
		"--no-playlist",
		"--restrict-filenames",
		"--windows-filenames",
		"--merge-output-format", "mp4",
		"-f", "bestvideo+bestaudio/best",
		"-o", filepath.Join(workDir, "source.%(ext)s"),
		sourceURL,
	)
	if err := streamCommand("yt-dlp", args...); err != nil {
		return "", err
	}

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return "", fmt.Errorf("inspect downloaded video: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "source.") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".mp4", ".mkv", ".webm", ".mov":
			return filepath.Join(workDir, entry.Name()), nil
		}
	}
	return "", errors.New("yt-dlp did not produce a source video")
}

func findDownloadedVideos(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read downloaded videos folder: %w", err)
	}

	var videos []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isDubbedOutputName(name) || !isVideoFile(name) {
			continue
		}
		if hasExistingDubbedOutput(entries, name) {
			continue
		}
		videos = append(videos, filepath.Join(dir, name))
	}
	return videos, nil
}

func hasExistingDubbedOutput(entries []os.DirEntry, sourceName string) bool {
	sourceBase := safeFilePart(videoBaseName(sourceName))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isDubbedOutputName(name) {
			continue
		}
		if strings.HasPrefix(safeFilePart(videoBaseName(name)), sourceBase) {
			return true
		}
	}
	return false
}

func isVideoFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mkv", ".webm", ".mov":
		return true
	default:
		return false
	}
}

func isDubbedOutputName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, ".dub.") || strings.HasPrefix(lower, "gemini-dub-") || strings.HasPrefix(lower, "dub-")
}

func videoBaseName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func extractAudioForGemini(videoPath, audioPath string) error {
	printStep("extracting audio")
	return streamCommand("ffmpeg", "-y", "-i", videoPath, "-vn", "-ac", "1", "-ar", "16000", "-b:a", "48k", audioPath)
}

func extractAudioForLocalAI(videoPath, audioPath string) error {
	printStep("extracting audio")
	return streamCommand("ffmpeg", "-y", "-i", videoPath, "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", audioPath)
}

func transcribeWithWhisper(whisperBin, audioPath, outputDir string) (string, error) {
	if strings.HasSuffix(filepath.Base(whisperBin), "whisper-cli") {
		return transcribeWithWhisperCPP(whisperBin, audioPath, outputDir)
	}
	return transcribeWithOpenAIWhisper(whisperBin, audioPath, outputDir)
}

func transcribeWithOpenAIWhisper(whisperBin, audioPath, outputDir string) (string, error) {
	args := []string{
		audioPath,
		"--model", envOrDefault("WHISPER_MODEL", "large-v3"),
		"--output_format", "txt",
		"--output_dir", outputDir,
	}
	if lang := strings.TrimSpace(os.Getenv("WHISPER_LANGUAGE")); lang != "" {
		args = append(args, "--language", lang)
	}
	if err := streamCommand(whisperBin, args...); err != nil {
		return "", err
	}
	return readFirstTextFile(outputDir)
}

func transcribeWithWhisperCPP(whisperBin, audioPath, outputDir string) (string, error) {
	model := expandHome(strings.TrimSpace(os.Getenv("WHISPER_MODEL")))
	if model == "" {
		return "", errors.New("set WHISPER_MODEL in .env to a whisper.cpp model path when using whisper-cli")
	}
	outputBase := filepath.Join(outputDir, "transcript")
	args := []string{"-m", model, "-f", audioPath, "-otxt", "-of", outputBase}
	if lang := strings.TrimSpace(os.Getenv("WHISPER_LANGUAGE")); lang != "" {
		args = append(args, "-l", lang)
	}
	if err := streamCommand(whisperBin, args...); err != nil {
		return "", err
	}
	data, err := os.ReadFile(outputBase + ".txt")
	if err != nil {
		return "", fmt.Errorf("read whisper transcript: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func translateWithOllama(ollamaBin, model, transcript, targetLang string) (string, error) {
	prompt := fmt.Sprintf(`Translate this transcript into %s as a natural spoken dubbing script.
Return only the translated narration.
Do not include markdown, timestamps, labels, notes, or explanations.

Transcript:
%s`, targetLang, transcript)

	cmd := exec.Command(ollamaBin, "run", model)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ollama failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func synthesizeWithPiper(piperBin, voiceModel, text, outputPath string) error {
	cmd := exec.Command(piperBin, "--model", voiceModel, "--output_file", outputPath)
	cmd.Stdin = strings.NewReader(text)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("piper failed: %w", err)
	}
	return nil
}

func readFirstTextFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read transcript directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".txt" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return "", fmt.Errorf("read transcript: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", errors.New("Whisper did not produce a .txt transcript")
}

func uploadGeminiFile(client *http.Client, apiKey, path, mimeType, displayName string) (geminiFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return geminiFile{}, fmt.Errorf("read upload file: %w", err)
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(path))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	meta, err := json.Marshal(map[string]any{
		"file": map[string]string{"display_name": displayName},
	})
	if err != nil {
		return geminiFile{}, err
	}

	startReq, err := http.NewRequest(http.MethodPost, geminiAPIBase+"/upload/v1beta/files", bytes.NewReader(meta))
	if err != nil {
		return geminiFile{}, err
	}
	startReq.Header.Set("x-goog-api-key", apiKey)
	startReq.Header.Set("X-Goog-Upload-Protocol", "resumable")
	startReq.Header.Set("X-Goog-Upload-Command", "start")
	startReq.Header.Set("X-Goog-Upload-Header-Content-Length", strconv.Itoa(len(data)))
	startReq.Header.Set("X-Goog-Upload-Header-Content-Type", mimeType)
	startReq.Header.Set("Content-Type", "application/json")

	startResp, err := client.Do(startReq)
	if err != nil {
		return geminiFile{}, fmt.Errorf("start Gemini upload: %w", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode < 200 || startResp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(startResp.Body, 4096))
		return geminiFile{}, fmt.Errorf("start Gemini upload: Gemini returned %s: %s", startResp.Status, strings.TrimSpace(string(message)))
	}
	uploadURL := startResp.Header.Get("X-Goog-Upload-URL")
	if uploadURL == "" {
		return geminiFile{}, errors.New("Gemini upload response did not include X-Goog-Upload-URL")
	}

	uploadReq, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		return geminiFile{}, err
	}
	uploadReq.Header.Set("Content-Length", strconv.Itoa(len(data)))
	uploadReq.Header.Set("X-Goog-Upload-Offset", "0")
	uploadReq.Header.Set("X-Goog-Upload-Command", "upload, finalize")

	var result geminiUploadResponse
	if err := doJSON(client, uploadReq, &result); err != nil {
		return geminiFile{}, err
	}
	if result.File.URI == "" {
		return geminiFile{}, errors.New("Gemini upload response did not include a file URI")
	}
	if result.File.MimeType == "" {
		result.File.MimeType = mimeType
	}
	return result.File, nil
}

func createGeminiDubScript(client *http.Client, apiKey, fileURI, mimeType, targetLang string) (string, error) {
	prompt := fmt.Sprintf(`Transcribe the spoken content from this audio and translate it into %s.
Return only a natural spoken narration script in %s.
Do not include markdown, timestamps, labels, notes, speaker names, or explanations.
Keep the narration faithful to the original meaning and suitable for text-to-speech.`, targetLang, targetLang)

	body := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{
				{"file_data": map[string]string{"mime_type": mimeType, "file_uri": fileURI}},
				{"text": prompt},
			},
		}},
		"generationConfig": map[string]any{
			"temperature":     0.2,
			"maxOutputTokens": 12000,
		},
	}

	var result geminiGenerateResponse
	if err := geminiGenerate(client, apiKey, "gemini-3.5-flash", body, &result); err != nil {
		return "", err
	}
	return geminiText(result), nil
}

func generateGeminiSpeech(client *http.Client, apiKey, script string) ([]byte, error) {
	body := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]string{{
				"text": "Read this translated narration naturally and clearly:\n\n" + script,
			}},
		}},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]any{
				"voiceConfig": map[string]any{
					"prebuiltVoiceConfig": map[string]string{"voiceName": "Kore"},
				},
			},
		},
	}

	var result geminiGenerateResponse
	if err := geminiGenerate(client, apiKey, "gemini-3.1-flash-tts-preview", body, &result); err != nil {
		return nil, err
	}
	for _, candidate := range result.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || part.InlineData.Data == "" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
			if err != nil {
				return nil, fmt.Errorf("decode Gemini speech audio: %w", err)
			}
			return data, nil
		}
	}
	return nil, errors.New("Gemini TTS response did not include audio")
}

func geminiGenerate(client *http.Client, apiKey, model string, body any, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", geminiAPIBase, url.PathEscape(model))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("x-goog-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	return doJSON(client, req, target)
}

func geminiText(result geminiGenerateResponse) string {
	var parts []string
	for _, candidate := range result.Candidates {
		for _, part := range candidate.Content.Parts {
			text := strings.TrimSpace(part.Text)
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func writeWAVFile(path string, pcm []byte, channels, sampleRate, sampleWidth int) error {
	var out bytes.Buffer
	dataSize := uint32(len(pcm))
	byteRate := uint32(sampleRate * channels * sampleWidth)
	blockAlign := uint16(channels * sampleWidth)
	bitsPerSample := uint16(sampleWidth * 8)

	out.WriteString("RIFF")
	if err := binary.Write(&out, binary.LittleEndian, uint32(36)+dataSize); err != nil {
		return err
	}
	out.WriteString("WAVEfmt ")
	if err := binary.Write(&out, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(&out, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(&out, binary.LittleEndian, uint16(channels)); err != nil {
		return err
	}
	if err := binary.Write(&out, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(&out, binary.LittleEndian, byteRate); err != nil {
		return err
	}
	if err := binary.Write(&out, binary.LittleEndian, blockAlign); err != nil {
		return err
	}
	if err := binary.Write(&out, binary.LittleEndian, bitsPerSample); err != nil {
		return err
	}
	out.WriteString("data")
	if err := binary.Write(&out, binary.LittleEndian, dataSize); err != nil {
		return err
	}
	out.Write(pcm)

	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write wav file: %w", err)
	}
	return nil
}

func muxDubbedVideo(videoPath, voicePath, outputPath string) error {
	printStep("creating dubbed video")
	return streamCommand("ffmpeg", "-y", "-i", videoPath, "-i", voicePath, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-movflags", "+faststart", outputPath)
}

func buildYTDLPArgs(cfg config, listFormats bool) []string {
	playlistFlag := "--no-playlist"
	if cfg.playlist {
		playlistFlag = "--yes-playlist"
	}
	if listFormats {
		return append(append(ytdlpJSRuntimeArgs(), "--list-formats", playlistFlag), cfg.urls...)
	}

	args := append(ytdlpJSRuntimeArgs(),
		"--ignore-errors",
		"--continue",
		"--no-overwrites",
		"--restrict-filenames",
		"--windows-filenames",
		"--embed-metadata",
		"-o", filepath.Join(cfg.outputDir, "%(title).200s [%(id)s].%(ext)s"),
		playlistFlag,
	)

	if cfg.quality == "audio" {
		args = append(args, "-f", "bestaudio/best", "-x", "--audio-format", cfg.audioFormat)
	} else {
		args = append(args, "--embed-thumbnail", "--merge-output-format", "mp4", "-f", formatForQuality(cfg.quality))
		if cfg.subtitles {
			args = append(args, "--write-subs", "--embed-subs", "--sub-langs", cfg.subtitleLangs)
			if cfg.autoSubtitles {
				args = append(args, "--write-auto-subs")
			}
		}
	}
	return append(args, cfg.urls...)
}

func ytdlpJSRuntimeArgs() []string {
	runtime := strings.TrimSpace(os.Getenv("YTDLP_JS_RUNTIME"))
	if runtime == "none" || runtime == "off" {
		return nil
	}
	if runtime == "" {
		runtime = detectYTDLPJSRuntime()
	}
	if runtime == "" {
		return nil
	}
	return []string{"--js-runtimes", runtime}
}

func detectYTDLPJSRuntime() string {
	for _, candidate := range []string{"deno", "node"} {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return candidate + ":" + path
		}
	}
	return ""
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
	printSection("Download")

	ytDLPPath, ytDLPErr := exec.LookPath("yt-dlp")
	printCheck("yt-dlp", ytDLPPath, ytDLPErr, true)
	if ytDLPErr == nil {
		version, _ := commandOutput("yt-dlp", "--version")
		printField("version", strings.TrimSpace(version))
	}
	if runtime := envOrDefault("YTDLP_JS_RUNTIME", detectYTDLPJSRuntime()); runtime != "" {
		printField("js runtime", runtime)
	} else {
		printStatus("missing", "js runtime", "optional; YouTube may miss formats")
	}

	ffmpegPath, ffmpegErr := exec.LookPath("ffmpeg")
	printCheck("ffmpeg", ffmpegPath, ffmpegErr, false)

	printSection("API dubbing")
	printEnvCheck("GEMINI_API_KEY", os.Getenv("GEMINI_API_KEY") != "", false)
	printEnvCheck("ELEVENLABS_API_KEY", os.Getenv("ELEVENLABS_API_KEY") != "", false)

	printSection("Local AI dubbing")
	printToolCheck("whisper", "WHISPER_BIN", "whisper", "whisper-cli")
	printToolCheck("ollama", "OLLAMA_BIN", "ollama")
	printToolCheck("piper", "PIPER_BIN", "piper")
	printEnvCheck("OLLAMA_MODEL", os.Getenv("OLLAMA_MODEL") != "", false)
	printFileEnvCheck("PIPER_VOICE", os.Getenv("PIPER_VOICE"), true)

	if ytDLPErr != nil {
		return errors.New("install yt-dlp before downloading")
	}
	if ffmpegErr != nil {
		printHint("Tip: install ffmpeg for video/audio merging, subtitles, and dubbing.")
	}
	return nil
}

func readURLFile(path string) ([]string, error) {
	path = expandHome(path)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect URL file: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("URL file path points to a directory")
	}
	if info.Size() > maxURLFileBytes {
		return nil, fmt.Errorf("URL file is too large: max %d bytes", maxURLFileBytes)
	}

	file, err := os.Open(path)
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

func loadDefaultEnvFiles() error {
	candidates := []string{".env"}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, ".env"),
			filepath.Join(filepath.Dir(exeDir), ".env"),
		)
	}

	seen := map[string]bool{}
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		if err := loadEnvFile(clean); err != nil {
			return err
		}
	}
	return nil
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid env line: %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			return fmt.Errorf("invalid env line: %q", line)
		}
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set env %s: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read env file: %w", err)
	}
	return nil
}

func choose(reader *bufio.Reader, title string, options []string, defaultChoice int) int {
	printSection(title)
	for i, option := range options {
		marker := " "
		if i+1 == defaultChoice {
			marker = ">"
		}
		fmt.Printf("  %s %s %s\n", accent(marker), dim(strconv.Itoa(i+1)+"."), option)
	}
	for {
		fmt.Printf("%s ", promptLabel(fmt.Sprintf("Choose [%d]", defaultChoice)))
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return defaultChoice
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= len(options) {
			return n
		}
		printHint("Please choose a number from the list.")
	}
}

func promptText(reader *bufio.Reader, label, current string) string {
	fmt.Printf("%s ", promptLabel(label+" ["+current+"]"))
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return current
	}
	return line
}

func promptInt(reader *bufio.Reader, label string, current, min, max int) int {
	for {
		fmt.Printf("%s ", promptLabel(fmt.Sprintf("%s [%d]", label, current)))
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return current
		}
		value, err := strconv.Atoi(line)
		if err == nil && value >= min && value <= max {
			return value
		}
		printHint(fmt.Sprintf("Please choose a number from %d to %d.", min, max))
	}
}

func promptYesNo(reader *bufio.Reader, label string, current bool) bool {
	defaultLabel := "n"
	if current {
		defaultLabel = "y"
	}
	for {
		fmt.Printf("%s ", promptLabel(label+" ["+defaultLabel+"]"))
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
		printHint("Please answer y or n.")
	}
}

func clipboardText() string {
	out, err := commandOutput("pbpaste")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func cleanURLs(values []string) ([]string, error) {
	seen := map[string]bool{}
	var urls []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		if err := validateYouTubeURL(value); err != nil {
			return nil, err
		}
		seen[value] = true
		urls = append(urls, value)
	}
	return urls, nil
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func validateYouTubeURL(value string) error {
	if strings.ContainsAny(value, "\x00\r\n\t") {
		return fmt.Errorf("URL contains unsafe control characters: %q", value)
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", value, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("only https URLs are allowed: %q", value)
	}
	if !isAllowedYouTubeHost(parsed.Hostname()) {
		return fmt.Errorf("only YouTube URLs are allowed by default: %q", value)
	}
	return nil
}

func isAllowedYouTubeHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	switch host {
	case "youtu.be", "youtube.com", "youtube-nocookie.com":
		return true
	}
	return strings.HasSuffix(host, ".youtube.com") || strings.HasSuffix(host, ".youtube-nocookie.com")
}

func validateAudioFormat(value string) error {
	switch strings.ToLower(value) {
	case "aac", "alac", "flac", "m4a", "mp3", "opus", "vorbis", "wav":
		return nil
	default:
		return fmt.Errorf("unsupported audio format %q", value)
	}
}

func validateSubtitleLangs(value string) error {
	if value == "" {
		return errors.New("subtitle languages cannot be empty")
	}
	if strings.ContainsAny(value, "\x00\r\n\t") {
		return fmt.Errorf("subtitle languages contain unsafe control characters: %q", value)
	}
	return nil
}

func validateConcurrentDownloads(value int) error {
	if value < 1 || value > maxConcurrentDownloads {
		return fmt.Errorf("concurrent downloads must be between 1 and %d", maxConcurrentDownloads)
	}
	return nil
}

func validateDubEngine(value string) error {
	switch value {
	case "", "auto", "gemini", "local":
		return nil
	default:
		return fmt.Errorf("unsupported dub engine %q; use auto, gemini, or local", value)
	}
}

func validateLanguageCode(value string) error {
	if value == "" {
		return errors.New("target language cannot be empty")
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' {
			continue
		}
		return fmt.Errorf("target language must be an ISO language code: %q", value)
	}
	return nil
}

func validateOutputDir(path string) error {
	clean := filepath.Clean(path)
	if clean == "." {
		return nil
	}
	if clean == string(filepath.Separator) {
		return errors.New("refusing to use filesystem root as output directory")
	}
	home, err := os.UserHomeDir()
	if err == nil && clean == filepath.Clean(home) {
		return errors.New("refusing to use home directory itself as output directory")
	}
	return nil
}

func extensionForContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch contentType {
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	default:
		return ".mp4"
	}
}

func safeFilePart(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "file"
	}
	return builder.String()
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

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func printHeader() {
	fmt.Println(accent("youtube-downloader"))
	fmt.Println(dim("  download | subtitles | dubbing | local ai"))
}

func printHelp() {
	printHeader()
	fmt.Println()
	fmt.Println(bold("Usage"))
	fmt.Printf("  %s                          interactive mode\n", appName)
	fmt.Printf("  %s download [options] URL   download videos\n", appName)
	fmt.Printf("  %s formats URL              list available formats\n", appName)
	fmt.Printf("  %s check                    check dependencies\n", appName)
	fmt.Printf("  %s update                   update yt-dlp\n", appName)
	fmt.Printf("  %s version                  print version\n", appName)
	fmt.Println()
	fmt.Println(bold("Dubbing"))
	fmt.Printf("  %s dub-gemini [opts] URL    download and dub with Gemini\n", appName)
	fmt.Printf("  %s dub-downloaded [opts]    dub downloaded videos with Gemini\n", appName)
	fmt.Printf("  %s dub-local [opts] URL     download and dub with local AI\n", appName)
	fmt.Printf("  %s dub-downloaded-local     dub downloaded videos locally\n", appName)
	fmt.Printf("  %s dub [options] URL        dub with ElevenLabs\n", appName)
	fmt.Println()
	fmt.Println(bold("Download Options"))
	fmt.Println("  -q, --quality VALUE       best, 1080, 720, audio, or yt-dlp format")
	fmt.Println("  -o, --output DIR          download folder")
	fmt.Println("  -f, --file FILE           read URLs from file")
	fmt.Println("  -p, --playlist            download full playlists")
	fmt.Println("  -j, --concurrent NUM      videos to download at the same time. Default: 1")
	fmt.Println("      --open                open folder in Finder")
	fmt.Println("      --dry-run             preview yt-dlp command")
	fmt.Println("      --dub-after           offer dubbing after download")
	fmt.Println("      --dub-engine VALUE    auto, gemini, or local. Default: auto")
	fmt.Println()
	fmt.Println(bold("Media Options"))
	fmt.Println("      --audio-format VALUE  mp3, m4a, opus, wav, etc. Default: mp3")
	fmt.Println("      --subtitles           download and embed Portuguese subtitles")
	fmt.Println("      --auto-subtitles      include auto-generated subtitles")
	fmt.Println("      --subtitle-langs VAL  yt-dlp subtitle languages. Default: pt.*,pt-BR,pt")
	fmt.Println()
	fmt.Println(bold("Dubbing Options"))
	fmt.Println("  -i, --input DIR           folder with downloaded videos. Default: ~/Downloads/YouTube")
	fmt.Println("  -o, --output DIR          folder for dubbed videos")
	fmt.Println("  -t, --to VALUE            target language for dub. Default: pt-BR")
}

func printRunSummary(cfg config, listFormats bool, args []string) {
	if listFormats {
		printSection("Listing formats")
	} else {
		printSection("Download ready")
		printField("folder", cfg.outputDir)
		printField("quality", cfg.quality)
		if cfg.concurrent > 1 && len(cfg.urls) > 1 {
			printField("parallel", strconv.Itoa(cfg.concurrent))
		}
		if cfg.subtitles && cfg.quality != "audio" {
			mode := "manual"
			if cfg.autoSubtitles {
				mode = "manual + auto"
			}
			printField("subs", cfg.subtitleLangs+" ("+mode+")")
		}
	}
	printField("urls", strconv.Itoa(len(cfg.urls)))
	fmt.Println(dim("  $ yt-dlp " + shellQuoteArgs(args)))
	fmt.Println()
}

func printCheck(name, path string, err error, required bool) {
	label := "optional"
	if required {
		label = "required"
	}
	if err != nil {
		printStatus("missing", name, "("+label+")")
		return
	}
	printStatus("ok", name, path)
}

func printToolCheck(label, envName string, names ...string) {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		printStatus("ok", label, value+" from "+envName)
		return
	}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			printStatus("ok", label, path)
			return
		}
	}
	printStatus("missing", label, "optional")
}

func printEnvCheck(name string, ok bool, required bool) {
	label := "optional"
	if required {
		label = "required"
	}
	if !ok {
		printStatus("missing", name, "("+label+")")
		return
	}
	printStatus("ok", name, "set")
}

func printFileEnvCheck(name, value string, required bool) {
	value = expandHome(strings.TrimSpace(value))
	if value == "" {
		printEnvCheck(name, false, required)
		return
	}
	info, err := os.Stat(value)
	if err != nil || info.IsDir() {
		printStatus("missing", name, value)
		return
	}
	printStatus("ok", name, value)
}

func printError(err error) {
	fmt.Fprintln(os.Stderr, danger("error"), err)
}

func bold(value string) string {
	return color(value, "1")
}

func accent(value string) string {
	return color(value, "35")
}

func success(value string) string {
	return color(value, "32")
}

func danger(value string) string {
	return color(value, "31")
}

func dim(value string) string {
	return color(value, "2")
}

func promptLabel(value string) string {
	return accent(">") + " " + bold(value) + dim(":")
}

func printSection(title string) {
	fmt.Println()
	fmt.Println(accent(title))
}

func printHint(value string) {
	fmt.Println(dim("  " + value))
}

func printStep(value string) {
	fmt.Println(accent(">") + " " + value)
}

func printField(label, value string) {
	fmt.Printf("  %-10s %s\n", dim(label), value)
}

func printStatus(state, label, detail string) {
	badge := success("[ok]")
	if state != "ok" {
		badge = danger("[missing]")
	}
	fmt.Printf("  %-10s %-18s %s\n", badge, label, dim(detail))
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
