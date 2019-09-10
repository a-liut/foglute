BIN_FOLDER = bin
BIN_NAME = foglute

GOARM=7
GOARCH=arm

main: setup
	go build -o $(BIN_FOLDER)/$(BIN_NAME) cmd/foglute/main.go

arm: setup
	GOARM=$(GOARM)	GOARCH=$(GOARCH) go build -o $(BIN_FOLDER)/$(BIN_NAME) cmd/foglute/main.go

.PHONY: setup
setup:
	mkdir -p $(BIN_FOLDER)

.PHONY: clean
clean:
	@rm -rf $(BIN_FOLDER)