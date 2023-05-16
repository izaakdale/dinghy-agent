
run: 
	GRPC_ADDR=127.0.0.1 \
	GRPC_PORT=5000 \
	BIND_ADDR=127.0.0.1 \
	BIND_PORT=7777 \
	ADVERTISE_ADDR=127.0.0.1 \
	ADVERTISE_PORT=7777 \
	CLUSTER_ADDR=127.0.0.1 \
	CLUSTER_PORT=7777 \
	NAME=agent \
	go run .

docker:
	docker build -t dinghy-agent .

up:
	kubectl apply -k deploy/
dn:
	kubectl delete -k deploy/

.PHONY: gproto
gproto:
	protoc api/v1/*.proto \
	--go_out=. \
	--go-grpc_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_opt=paths=source_relative \
	--proto_path=.