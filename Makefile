compile:
	mkdir -p build
	GOOS=linux   GOARCH=amd64 go build -trimpath -o ./build/doccurator ./cmd/doccurator
	GOOS=windows GOARCH=amd64 go build -trimpath -o ./build/doccurator.exe ./cmd/doccurator
	./generate-markdowns
install: compile
	sudo cp ./build/doccurator /usr/local/bin/doccurator
coverage:
	go test -coverprofile=build/coverage.out -coverpkg=./... ./... && go tool cover -html=build/coverage.out
