
run: 
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