.PHONY: build build-version build-demo lint lint-fix vet clean check check-demo e2e e2e-gen e2e-update

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

vet:
	go vet ./...

clean:
	rm -f lazyjira

check: lint vet build

check-demo:
	golangci-lint run --build-tags demo ./...
	go vet -tags demo ./...
	go build -tags demo -o lazyjira ./cmd/lazyjira

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
	@echo "All tapes passed."

e2e-gen:
	@for src in e2e/tapes/*.tape.sh; do \
		[ -f "$$src" ] || continue; \
		dst="$${src%.sh}"; \
		./e2e/tape.sh "$$src" > "$$dst"; \
	done

e2e-update: build-demo e2e-gen
	@pids=""; \
	for tape in e2e/tapes/*.tape; do \
		echo "Running $$tape..."; \
		vhs -q $$tape & pids="$$pids $$!"; \
	done; \
	for pid in $$pids; do wait $$pid; done
	@echo "Golden files updated. Review with: git diff e2e/golden/"
