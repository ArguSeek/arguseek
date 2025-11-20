.PHONY: help build test lint clean version tag release release-clean deploy-dev deploy-prod deploy-dev-dry deploy-prod-dry build-deploy install install-user

# Default target
help:
	@echo "Available commands:"
	@echo "  build         - Build the server binary"
	@echo "  build-deploy  - Build the deploy tool"
	@echo "  install       - Install to /usr/local/bin/arguseek (requires sudo)"
	@echo "  install-user  - Install to ~/bin/arguseek (no sudo required)"
	@echo "  test          - Run tests"
	@echo "  lint          - Run linter"
	@echo "  clean         - Clean build artifacts"
	@echo ""
	@echo "Release commands:"
	@echo "  version       - Show current version from git tags"
	@echo "  tag           - Create a new version tag (interactive)"
	@echo "  release       - Build release binaries and create GitHub release"
	@echo "  release-clean - Clean release artifacts"
	@echo ""
	@echo "Deployment commands:"
	@echo "  deploy-dev    - Deploy to development environment"
	@echo "  deploy-dev-dry- Preview development deployment"
	@echo "  deploy-prod   - Deploy to production environment"
	@echo "  deploy-prod-dry- Preview production deployment"
	@echo ""
	@echo "Local development:"
	@echo "  dev           - Run local development server"

# Build targets
build:
	go build -o bin/server ./cmd/server

build-deploy:
	go build -o bin/deploy cmd/deploy/main.go
	chmod +x bin/deploy

# Install targets
install: build
	@echo "Installing arguseek to /usr/local/bin..."
	sudo cp bin/server /usr/local/bin/arguseek
	@echo "✓ ArguSeek installed globally as 'arguseek'"
	@echo "  Run 'arguseek' or 'arguseek -http' from anywhere"

install-user: build
	@echo "Installing arguseek to ~/bin..."
	mkdir -p ~/bin
	cp bin/server ~/bin/arguseek
	@echo "✓ ArguSeek installed to ~/bin/arguseek"
	@echo "  Make sure ~/bin is in your PATH"
	@echo "  Add to ~/.zshrc or ~/.bashrc: export PATH=\"\$$HOME/bin:\$$PATH\""

# Test and quality
test:
	go test -v ./...

test-coverage:
	go test -v -cover ./...

lint:
	golangci-lint run

# Clean
clean:
	rm -rf bin/

# Release targets
version:
	@CURRENT_VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "no tags yet"); \
	echo "Current version: $$CURRENT_VERSION"

tag:
	@# Get current version
	@CURRENT_VERSION=$$(git describe --tags --abbrev=0 2>/dev/null); \
	if [ -z "$$CURRENT_VERSION" ]; then \
		echo "No existing tags found. Creating first tag..."; \
		CURRENT_VERSION="v0.0.0"; \
	fi; \
	echo "Current version: $$CURRENT_VERSION"; \
	echo ""; \
	\
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: Working directory is not clean."; \
		echo "Please commit or stash changes before tagging."; \
		exit 1; \
	fi; \
	\
	CURRENT_BRANCH=$$(git branch --show-current); \
	if [ "$$CURRENT_BRANCH" != "main" ] && [ "$$CURRENT_BRANCH" != "master" ]; then \
		echo "Warning: You are not on main/master branch (current: $$CURRENT_BRANCH)"; \
		read -p "Continue anyway? [y/N] " -n 1 -r; \
		echo ""; \
		if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then \
			echo "Aborted."; \
			exit 1; \
		fi; \
	fi; \
	\
	echo "Bump version:"; \
	echo "  1) patch (bug fixes)"; \
	echo "  2) minor (new features)"; \
	echo "  3) major (breaking changes)"; \
	echo "  4) custom"; \
	read -p "Choice [1-4]: " choice; \
	\
	VERSION_PARTS=$$(echo $$CURRENT_VERSION | sed 's/v//'); \
	MAJOR=$$(echo $$VERSION_PARTS | cut -d. -f1); \
	MINOR=$$(echo $$VERSION_PARTS | cut -d. -f2); \
	PATCH=$$(echo $$VERSION_PARTS | cut -d. -f3); \
	\
	case $$choice in \
		1) PATCH=$$((PATCH + 1)); ;; \
		2) MINOR=$$((MINOR + 1)); PATCH=0; ;; \
		3) MAJOR=$$((MAJOR + 1)); MINOR=0; PATCH=0; ;; \
		4) read -p "Enter new version (without 'v' prefix): " CUSTOM_VERSION; \
		   NEW_VERSION="v$$CUSTOM_VERSION"; \
		   echo ""; \
		   read -p "Create tag $$NEW_VERSION? [y/N] " -n 1 -r; \
		   echo ""; \
		   if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then \
		       echo "Aborted."; \
		       exit 1; \
		   fi; \
		   git tag -a "$$NEW_VERSION" -m "Release $$NEW_VERSION"; \
		   echo "✓ Created tag $$NEW_VERSION"; \
		   echo ""; \
		   echo "Next steps:"; \
		   echo "  git push origin $$NEW_VERSION  # Push tag to remote"; \
		   echo "  make release                    # Build and create GitHub release"; \
		   exit 0; \
		   ;; \
		*) echo "Invalid choice"; exit 1; ;; \
	esac; \
	\
	NEW_VERSION="v$$MAJOR.$$MINOR.$$PATCH"; \
	echo ""; \
	echo "Will create tag: $$NEW_VERSION"; \
	read -p "Proceed? [y/N] " -n 1 -r; \
	echo ""; \
	if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then \
		echo "Aborted."; \
		exit 1; \
	fi; \
	\
	git tag -a "$$NEW_VERSION" -m "Release $$NEW_VERSION"; \
	echo "✓ Created tag $$NEW_VERSION"; \
	echo ""; \
	echo "Next steps:"; \
	echo "  git push origin $$NEW_VERSION  # Push tag to remote"; \
	echo "  make release                    # Build and create GitHub release"

release:
	@echo "Building release binaries..."
	@# Get version from git tags (single source of truth)
	@VERSION=$$(git describe --tags --abbrev=0 2>/dev/null); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: No git tags found."; \
		echo "Recommended: make tag    # Interactive semver bump"; \
		echo "Manual:      git tag -a v1.0.0 -m 'Release v1.0.0'"; \
		exit 1; \
	fi; \
	echo "Building version: $$VERSION"; \
	\
	if ! git diff-index --quiet HEAD --; then \
		echo "Error: Tracked files have uncommitted changes. Commit or stash changes first."; \
		exit 1; \
	fi; \
	\
	mkdir -p dist; \
	# IMPORTANT: ldflags path must match Dockerfile and internal/version package
	LDFLAGS="-X arguseek/internal/version.injectedVersion=$$VERSION"; \
	\
	echo "Building linux/amd64..."; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$$LDFLAGS" -o dist/arguseek-$$VERSION-linux-amd64 ./cmd/server; \
	if ! strings dist/arguseek-$$VERSION-linux-amd64 | grep -q "$$VERSION"; then \
		echo "Error: Version injection failed for linux-amd64"; \
		exit 1; \
	fi; \
	tar -czf dist/arguseek-$$VERSION-linux-amd64.tar.gz -C dist arguseek-$$VERSION-linux-amd64; \
	\
	echo "Building linux/arm64..."; \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$$LDFLAGS" -o dist/arguseek-$$VERSION-linux-arm64 ./cmd/server; \
	if ! strings dist/arguseek-$$VERSION-linux-arm64 | grep -q "$$VERSION"; then \
		echo "Error: Version injection failed for linux-arm64"; \
		exit 1; \
	fi; \
	tar -czf dist/arguseek-$$VERSION-linux-arm64.tar.gz -C dist arguseek-$$VERSION-linux-arm64; \
	\
	echo "Building darwin/amd64..."; \
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$$LDFLAGS" -o dist/arguseek-$$VERSION-darwin-amd64 ./cmd/server; \
	if ! strings dist/arguseek-$$VERSION-darwin-amd64 | grep -q "$$VERSION"; then \
		echo "Error: Version injection failed for darwin-amd64"; \
		exit 1; \
	fi; \
	tar -czf dist/arguseek-$$VERSION-darwin-amd64.tar.gz -C dist arguseek-$$VERSION-darwin-amd64; \
	\
	echo "Building darwin/arm64..."; \
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$$LDFLAGS" -o dist/arguseek-$$VERSION-darwin-arm64 ./cmd/server; \
	if ! strings dist/arguseek-$$VERSION-darwin-arm64 | grep -q "$$VERSION"; then \
		echo "Error: Version injection failed for darwin-arm64"; \
		exit 1; \
	fi; \
	tar -czf dist/arguseek-$$VERSION-darwin-arm64.tar.gz -C dist arguseek-$$VERSION-darwin-arm64; \
	\
	echo "Building windows/amd64..."; \
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$$LDFLAGS" -o dist/arguseek-$$VERSION-windows-amd64.exe ./cmd/server; \
	if ! strings dist/arguseek-$$VERSION-windows-amd64.exe | grep -q "$$VERSION"; then \
		echo "Error: Version injection failed for windows-amd64"; \
		exit 1; \
	fi; \
	cd dist && zip -q arguseek-$$VERSION-windows-amd64.zip arguseek-$$VERSION-windows-amd64.exe && cd ..; \
	\
	echo "Generating checksums..."; \
	cd dist && shasum -a 256 *.tar.gz *.zip > checksums.txt && cd ..; \
	\
	echo ""; \
	echo "✓ Release build complete!"; \
	echo "  Version: $$VERSION"; \
	echo "  Output: dist/"; \
	echo ""; \
	ls -lh dist/; \
	\
	echo ""; \
	if ! command -v gh &> /dev/null; then \
		echo "⚠ GitHub CLI (gh) not found. Skipping GitHub release creation."; \
		echo "  Install: https://cli.github.com/"; \
		echo "  To create release manually:"; \
		echo "    git push origin $$VERSION"; \
		echo "    gh release create $$VERSION dist/*.tar.gz dist/*.zip dist/checksums.txt"; \
	else \
		echo "Creating GitHub release..."; \
		if ! git ls-remote --tags origin | grep -q "$$VERSION"; then \
			echo "⚠ Tag $$VERSION not found on remote. Pushing tag..."; \
			git push origin $$VERSION || { \
				echo "Error: Failed to push tag. Push manually with: git push origin $$VERSION"; \
				exit 1; \
			}; \
		fi; \
		\
		PREV_TAG=$$(git describe --tags --abbrev=0 $$VERSION^ 2>/dev/null || echo ""); \
		if [ -n "$$PREV_TAG" ]; then \
			echo "Generating release notes from $$PREV_TAG to $$VERSION..."; \
			git log $$PREV_TAG..$$VERSION --pretty=format:"- %s" --no-merges > /tmp/arguseek-release-notes.txt; \
		else \
			echo "No previous tag found. Using initial release notes."; \
			echo "Initial release" > /tmp/arguseek-release-notes.txt; \
		fi; \
		\
		gh release create "$$VERSION" \
			dist/*.tar.gz dist/*.zip dist/checksums.txt \
			--title "Release $$VERSION" \
			--notes-file /tmp/arguseek-release-notes.txt \
			--verify-tag || { \
			echo "Error: Failed to create GitHub release."; \
			echo "Create manually with: gh release create $$VERSION dist/*.tar.gz dist/*.zip dist/checksums.txt"; \
			exit 1; \
		}; \
		\
		echo ""; \
		echo "✓ GitHub release created successfully!"; \
		REPO_URL=$$(git config --get remote.origin.url | sed 's/\.git$$//' | sed 's|git@github.com:|https://github.com/|'); \
		echo "  View at: $$REPO_URL/releases/tag/$$VERSION"; \
	fi

release-clean:
	@echo "Cleaning release artifacts..."
	rm -rf dist/
	@echo "✓ Release artifacts cleaned"

# Deployment commands
deploy-dev: ensure-deploy
	./bin/deploy dev

deploy-dev-dry: ensure-deploy
	./bin/deploy dev --dry-run

deploy-prod: ensure-deploy
	./bin/deploy prod

deploy-prod-dry: ensure-deploy
	./bin/deploy prod --dry-run

# Local development
dev:
	@echo "Starting local development server..."
	@echo "Make sure to set environment variables: GOOGLE_API_KEY, GOOGLE_CSE_ID, GCP_PROJECT_ID"
	DEBUG=true go run ./cmd/server

# Check if deploy binary exists, build if not
ensure-deploy:
	@if [ ! -f bin/deploy ]; then make build-deploy; fi