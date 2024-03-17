gen:
	protoc -I ./proto  --go_out ./proto --go_opt paths=source_relative --go-grpc_out ./proto --go-grpc_opt paths=source_relative  --grpc-gateway_out ./proto --grpc-gateway_opt paths=source_relative  ./proto/bin.proto
build:
	go get -u all
	go mod tidy
	go build main.go
clean:
	go mod tidy
	go get -u
