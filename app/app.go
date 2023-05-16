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
	"github.com/izaakdale/dinghy-agent/internal/worker"
	"github.com/kelseyhightower/envconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var spec Specification

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

type App struct {
}

func New() *App {
	if err := envconfig.Process("", &spec); err != nil {
		panic(err)
	}
	return &App{}
}

func (a *App) Run() {
	log.Printf("hello, my name is %s\n", spec.Name)

	gAddr := fmt.Sprintf("%s:%d", spec.GRPCAddr, spec.GRPCPort)
	ln, err := net.Listen("tcp", gAddr)
	if err != nil {
		log.Fatalf("failed to start up grpc listener: %v", err)
	}

	balance := &worker.Balancer{
		Leader:    nil,
		Followers: []*worker.Client{},
	}

	gsrv := grpc.NewServer()
	reflection.Register(gsrv)

	srv := server.New(balance)
	v1.RegisterAgentServer(gsrv, srv)

	errCh := make(chan error)
	go func(ch chan error) {
		ch <- gsrv.Serve(ln)
	}(errCh)

	node, evCh, err := discovery.NewMembership(
		spec.BindAddr,
		spec.BindPort, // BIND defines where the agent listen for incomming connection
		spec.AdvertiseAddr,
		spec.AdvertisePort, // ADVERTISE defines where the agent is reachable, often it the same as BIND
		spec.ClusterAddr,
		spec.ClusterPort, // CLUSTER is the address of a first agent
		spec.Name,
	) // NAME must be unique in a cluster
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
			discovery.HandleSerfEvent(e, node, balance)
		case err := <-errCh:
			log.Fatalf("grpc server errored: %v", err)
		}
	}
}
