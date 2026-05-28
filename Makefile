BINARY := gohole
CMD     := ./cmd/gohole
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -w -s"

.PHONY: build run test clean install docker lint deps

build:
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BINARY) $(CMD)

run: build
	sudo ./$(BINARY) -config config.yaml

# Run on a high port (no sudo needed) for development
dev: build
	@echo "Starting in dev mode (port 5353 for DNS, 8080 for dashboard)"
	@sed 's/0.0.0.0:53/0.0.0.0:5353/' config.yaml > /tmp/gohole-dev.yaml
	./$(BINARY) -config /tmp/gohole-dev.yaml

test:
	go test ./... -v -race -count=1

clean:
	rm -f $(BINARY) gohole.db

install: build
	sudo useradd -r -s /sbin/nologin gohole 2>/dev/null || true
	sudo mkdir -p /opt/gohole /etc/gohole /var/lib/gohole
	sudo cp $(BINARY) /opt/gohole/
	sudo cp config.yaml /etc/gohole/config.yaml
	sudo chown -R gohole:gohole /var/lib/gohole
	sudo cp deployments/systemd/gohole.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable gohole
	@echo "Run: sudo systemctl start gohole"

docker:
	docker build -f deployments/docker/Dockerfile -t gohole:$(VERSION) .

docker-run: docker
	docker run -d \
		--name gohole \
		--network host \
		-v $(PWD)/data:/app/data \
		-v $(PWD)/config.yaml:/app/config.yaml:ro \
		gohole:$(VERSION)

lint:
	golangci-lint run ./...

deps:
	go mod tidy
	go mod download

# Test DNS resolution against running instance
test-dns:
	@echo "Testing allowed domain..."
	dig @127.0.0.1 -p 5353 google.com A
	@echo "Testing blocked domain..."
	dig @127.0.0.1 -p 5353 doubleclick.net A
