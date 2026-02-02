.PHONY: build clean test lint

BINARY_DIR := bin

build: $(BINARY_DIR)/webhook $(BINARY_DIR)/cleanup

$(BINARY_DIR)/webhook: cmd/webhook/main.go internal/**/*.go
	go build -o $@ ./cmd/webhook

$(BINARY_DIR)/cleanup: cmd/cleanup/main.go internal/**/*.go
	go build -o $@ ./cmd/cleanup

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf $(BINARY_DIR)

deploy: build
	scp $(BINARY_DIR)/webhook $(BINARY_DIR)/cleanup runner-host:/usr/local/bin/
	scp deploy/webhook.service deploy/cleanup.service deploy/cleanup.timer runner-host:/etc/systemd/system/
	ssh runner-host 'systemctl daemon-reload && systemctl restart webhook && systemctl enable --now cleanup.timer'
