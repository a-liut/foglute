BIN_FOLDER = bin
BIN_NAME = foglute

main:
	mkdir -p $(BIN_FOLDER)
	go build -o $(BIN_FOLDER)/$(BIN_NAME) cmd/foglute/main.go

clean:
	@rm -rf $(BIN_FOLDER)