package app

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/izaakdale/dinghy-agent/discovery"
	"github.com/kelseyhightower/envconfig"
)

var spec Specification

type Specification struct {
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
			discovery.HandleSerfEvent(e, node)
		}
	}
}
