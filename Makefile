.PHONY: build build-version build-demo lint lint-fix lint-docs vet clean check check-demo release preview e2e e2e-gen e2e-update nix-deps

build:
	go build -o lazyjira ./cmd/lazyjira

build-demo:
	go build -tags demo -o lazyjira ./cmd/lazyjira

build-version:
	go build -ldflags "-s -w -X main.version=$$(git rev-parse --short HEAD)" -o lazyjira ./cmd/lazyjira

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

lint-docs:
	npx --yes markdownlint-cli README.md CHANGELOG.md docs/*.md --disable MD001 MD013 MD024 MD033 MD040 MD041 MD060

vet:
	go vet ./...

clean:
	rm -f lazyjira

check: lint vet build

release:
	@test -n "$(VERSION)" || (echo "Usage: make release VERSION=2.7.0" && exit 1)
	keepachangelog release $(VERSION)
	sed -i '' 's/^pkgver=.*/pkgver=$(VERSION)/' aur/lazyjira-git/PKGBUILD
	git add CHANGELOG.md aur/lazyjira-git/PKGBUILD
	git commit -m "release v$(VERSION)"
	git tag v$(VERSION)
	@echo "Tagged v$(VERSION). Push with: git push && git push --tags"

check-demo:
	golangci-lint run --build-tags demo ./...
	go vet -tags demo ./...
	go build -tags demo -o lazyjira ./cmd/lazyjira

preview: build-demo e2e-gen
	@vhs -q e2e/tapes/00_preview.tape & vhs -q e2e/tapes/00_preview_vertical.tape & wait
	@rm -f e2e/tapes/*.tape

e2e: build-demo e2e-gen
	@pids=""; fail=0; \
	for tape in e2e/tapes/*.tape; do \
		echo "Running $$tape..."; \
		vhs -q $$tape & pids="$$pids $$!"; \
	done; \
	for pid in $$pids; do \
		wait $$pid || fail=1; \
	done; \
	if [ $$fail -eq 1 ]; then echo "SOME TAPES FAILED" && exit 1; fi
	@rm -f e2e/tapes/*.tape
	@echo "All tapes passed."

e2e-gen:
	@./e2e/tape.sh generate-all

nix-deps:
	gomod2nix generate

e2e-update: build-demo e2e-gen
	@pids=""; \
	for tape in e2e/tapes/*.tape; do \
		echo "Running $$tape..."; \
		vhs -q $$tape & pids="$$pids $$!"; \
	done; \
	for pid in $$pids; do wait $$pid; done
	@rm -f e2e/tapes/*.tape
	@echo "Golden files updated. Review with: git diff e2e/golden/"
