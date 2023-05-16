package discovery

import (
	"fmt"
	"log"

	"github.com/hashicorp/serf/serf"
	"github.com/izaakdale/dinghy-agent/internal/worker"
)

func HandleSerfEvent(e serf.Event, node *serf.Serf, balance *worker.Balancer) {
	switch e.EventType() {
	case serf.EventMemberJoin:
		for _, member := range e.(serf.MemberEvent).Members {
			if isLocal(node, member) {
				continue
			}
			err := handleJoin(member, balance)
			if err != nil {
				log.Printf("error handling member join: %s", err.Error())
			}
		}
	case serf.EventMemberLeave, serf.EventMemberFailed:
		for _, member := range e.(serf.MemberEvent).Members {
			if isLocal(node, member) {
				continue
			}
			err := handleLeave(member, balance)
			if err != nil {
				log.Printf("error handling member leave: %s", err.Error())
			}
		}
	}
}

func handleJoin(m serf.Member, balance *worker.Balancer) error {
	log.Printf("------- member joined %s @ %s -------\n", m.Name, m.Addr)

	nameTag, ok := m.Tags["name"]
	if !ok {
		return fmt.Errorf("no name tag for incoming node")
	}
	grpcTag, ok := m.Tags["grpc_addr"]
	if !ok {
		return fmt.Errorf("no grpc_addr tag for incoming node")
	}
	raftTag, ok := m.Tags["raft_addr"]
	if !ok {
		return fmt.Errorf("no raft_addr tag for incoming node")
	}

	client, err := worker.New(nameTag, grpcTag, raftTag)
	if err != nil {
		return err
	}
	if err := balance.AddClient(client); err != nil {
		return err
	}
	return nil
}

func isLocal(c *serf.Serf, m serf.Member) bool {
	return c.LocalMember().Name == m.Name
}

func handleLeave(m serf.Member, balance *worker.Balancer) error {
	log.Printf("------- member leaving %s @ %s -------\n", m.Name, m.Addr)
	err := balance.RemoveNode(m.Name)
	if err != nil {
		return err
	}
	return nil
}
