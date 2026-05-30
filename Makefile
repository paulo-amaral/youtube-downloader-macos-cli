APP := youtube-downloader
CMD := ./cmd/youtube-downloader
BIN_DIR := bin

.PHONY: help fmt test build security check run clean

help:
	@echo "Targets:"
	@echo "  make fmt     Format Go code"
	@echo "  make test    Run Go tests"
	@echo "  make build   Build $(APP)"
	@echo "  make security Run local security checks"
	@echo "  make check   Run all local checks"
	@echo "  make run     Run the interactive CLI"
	@echo "  make clean   Remove build output"

fmt:
	gofmt -w .

test:
	go test ./...

build:
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BIN_DIR)/$(APP) $(CMD)

security:
	bash scripts/security_check.sh

check: fmt test build security
	bash -n scripts/download_videos.sh

run:
	go run $(CMD)

clean:
	rm -rf $(BIN_DIR)
