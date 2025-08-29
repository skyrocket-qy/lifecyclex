cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

bk:
	git add .
	git commit -m update
	git push