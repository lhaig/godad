build:
	go build -o bin/godad main.go

compile:
	echo "Compiling for multiple platforms"
	GOOS=darwin GOARCH=amd64 go build -o bin/godad-darwin main.go
	# GOOS=linux GOARCH=amd64 go build -o bin/godad-linux-amd64 main.go
	# GOOS=linux GOARCH=arm64 go build -o bin/godad-linux-arm64 main.go
	GOOS=windows GOARCH=amd64 go build -o bin/godad-windows-amd64.exe main.go

code_vul_scan:
	time gosec ./...

run:
	go run main.go