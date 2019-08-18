BIN_FOLDER = bin
BIN_NAME = fogluted

main:
	mkdir -p $(BIN_FOLDER)
	go build -o $(BIN_FOLDER)/$(BIN_NAME) cmd/fogluted/main.go

clean:
	@rm -rf $(BIN_FOLDER)