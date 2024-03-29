package app

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	v1 "github.com/izaakdale/dinghy-agent/api/v1"
	"github.com/izaakdale/dinghy-agent/internal/discovery"
	"github.com/izaakdale/dinghy-agent/internal/server"
	"github.com/kelseyhightower/envconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Specification struct {
	GRPCAddr      string `envconfig:"GRPC_ADDR"`
	GRPCPort      int    `envconfig:"GRPC_PORT"`
	BindAddr      string `envconfig:"BIND_ADDR"`
	BindPort      string `envconfig:"BIND_PORT"`
	AdvertiseAddr string `envconfig:"ADVERTISE_ADDR"`
	AdvertisePort string `envconfig:"ADVERTISE_PORT"`
	ClusterAddr   string `envconfig:"CLUSTER_ADDR"`
	ClusterPort   string `envconfig:"CLUSTER_PORT"`
	Name          string `envconfig:"NAME"`
}

func Run() {
	var spec Specification
	if err := envconfig.Process("", &spec); err != nil {
		panic(err)
	}

	log.Printf("hello, my name is %s\n", spec.Name)

	gAddr := fmt.Sprintf("%s:%d", spec.GRPCAddr, spec.GRPCPort)
	log.Printf("grpc_addr: %s\n", gAddr)
	ln, err := net.Listen("tcp", gAddr)
	if err != nil {
		log.Fatalf("failed to start up grpc listener: %v", err)
	}

	gsrv := grpc.NewServer()
	reflection.Register(gsrv)

	srv := server.New()
	v1.RegisterAgentServer(gsrv, srv)

	errCh := make(chan error)
	go func(ch chan error) {
		ch <- gsrv.Serve(ln)
	}(errCh)

	node, evCh, err := discovery.NewMembership(
		spec.BindAddr,
		spec.BindPort, // BIND defines where the agent listens for incoming connection, e.g. the pod IP
		spec.AdvertiseAddr,
		spec.AdvertisePort, // ADVERTISE defines where the agent is reachable, e.g. the service IP
		spec.ClusterAddr,
		spec.ClusterPort, // CLUSTER is the address of a first agent, e.g. the service IP, since all servers are reachable here
		spec.Name,
	)
	defer node.Leave()
	if err != nil {
		log.Fatal(err)
	}

	shCh := make(chan os.Signal, 2)
	signal.Notify(shCh, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-shCh:
			err := node.Leave()
			if err != nil {
				log.Fatalf("error leaving cluster %v", err)
			}
			os.Exit(1)
		case e := <-evCh:
			discovery.HandleSerfEvent(e, node, srv)
		case err := <-errCh:
			log.Fatalf("grpc server errored: %v", err)
		}
	}
}
