package cluster

import (
	"log"
	"strconv"

	"github.com/hashicorp/serf/serf"
	"github.com/pkg/errors"
)

func Setup(bindAddr, bindPort, advertiseAddr, advertisePort, clusterAddr, clusterPort, name string) (*serf.Serf, chan serf.Event, error) {
	conf := serf.DefaultConfig()
	conf.Init()
	conf.MemberlistConfig.AdvertiseAddr = advertiseAddr
	conf.MemberlistConfig.AdvertisePort, _ = strconv.Atoi(advertisePort)
	conf.MemberlistConfig.BindAddr = bindAddr
	conf.MemberlistConfig.BindPort, _ = strconv.Atoi(bindPort)
	conf.MemberlistConfig.ProtocolVersion = 3 // Version 3 enable the ability to bind different port for each agent
	conf.NodeName = name

	evCh := make(chan serf.Event)
	conf.EventCh = evCh

	cluster, err := serf.Create(conf)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Couldn't create cluster")
	}

	_, err = cluster.Join([]string{clusterAddr + ":" + clusterPort}, true)
	if err != nil {
		log.Printf("Couldn't join cluster, starting own: %v\n", err)
	}

	return cluster, evCh, nil
}
