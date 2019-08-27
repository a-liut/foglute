BIN_FOLDER = bin
BIN_NAME = foglute

GOARM=7
GOARCH=arm

main:
	mkdir -p $(BIN_FOLDER)
	GOARM=$(GOARM)	GOARCH=$(GOARCH) go build -o $(BIN_FOLDER)/$(BIN_NAME) cmd/foglute/main.go

clean:
	@rm -rf $(BIN_FOLDER)