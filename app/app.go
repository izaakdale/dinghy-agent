package app

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/izaakdale/dinghy-agent/cluster"
	"github.com/kelseyhightower/envconfig"
)

var spec Specification

type Specification struct {
	GRPCAddr   net.IP `envconfig:"GRPC_ADDR"`
	GRCPPort   int    `envconfig:"GRPC_PORT"`
	clusterCfg cluster.Specification
}

type App struct {
}

func New() *App {
	err := envconfig.Process("", &spec)
	if err != nil {
		panic(err)
	}
	err = envconfig.Process("", &spec.clusterCfg)
	if err != nil {
		panic(err)
	}
	return &App{}
}

func (a *App) Run() error {
	node, evCh, err := cluster.SetupNode(
		spec.clusterCfg.BindAddr,      // BIND defines where the agent listens for incoming connections
		spec.clusterCfg.BindPort,      // in k8s this would be the ip and port of the pod/container
		spec.clusterCfg.AdvertiseAddr, // ADVERTISE defines where the agent is reachable
		spec.clusterCfg.AdvertisePort, // in k8s this correlates to the cluster ip service
		spec.clusterCfg.Name,          // NAME must be unique, which is not possible for replicas with env vars. Uniqueness handled in setup
		spec.GRCPPort,
	)
	if err != nil {
		return err
	}
	defer node.Leave()

	// signal channel for the os/k8s
	shCh := make(chan os.Signal, 2)
	signal.Notify(shCh, os.Interrupt, syscall.SIGTERM)

Loop:
	for {
		select {
		// shutdown
		case <-shCh:
			break Loop
		// serf event channel
		case ev := <-evCh:
			cluster.HandleSerfEvent(ev, node)
			// grpc error
			// case err := <-errCh:
			// 	return err
		}
	}
	return nil
}
