package discovery

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/hashicorp/serf/serf"
	"github.com/pkg/errors"
)

func NewMembership(bindAddr, bindPort, advertiseAddr, advertisePort, clusterAddr, clusterPort, name string) (*serf.Serf, chan serf.Event, error) {
	conf := serf.DefaultConfig()
	conf.Init()

	// since in k8s you will want to advertise the cluster ip service which changes,
	// we will enter the name in the format <svc-name>.<namespace>.svc.cluster.local to resolve the ip
	res, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%s", advertiseAddr, advertisePort))
	if err != nil {
		return nil, nil, err
	}
	conf.MemberlistConfig.AdvertiseAddr = res.IP.String()
	conf.MemberlistConfig.AdvertisePort = res.Port

	conf.MemberlistConfig.BindAddr = bindAddr
	conf.MemberlistConfig.BindPort, _ = strconv.Atoi(bindPort)
	conf.MemberlistConfig.ProtocolVersion = 3 // Version 3 enable the ability to bind different port for each agent

	// prevent annoying serf and memberlist logs
	conf.MemberlistConfig.Logger = log.New(io.Discard, "", log.Flags())
	conf.Logger = log.New(io.Discard, "", log.Flags())
	conf.NodeName = name

	evCh := make(chan serf.Event)
	conf.EventCh = evCh

	cluster, err := serf.Create(conf)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Couldn't create cluster")
	}

	_, err = cluster.Join([]string{clusterAddr + ":" + clusterPort}, true)
	if err != nil {
		log.Printf("Couldn't join the cluster specified. Starting own.\n")
	}

	return cluster, evCh, nil
}
