.PHONY: build build-frontend build-backend test clean

build: build-frontend build-backend

build-frontend:
	cd web && npm install && npm run build

build-backend:
	go build -o flashyspeed ./cmd/flashyspeed

test:
	go test ./...

clean:
	rm -f flashyspeed
	rm -rf web/dist
