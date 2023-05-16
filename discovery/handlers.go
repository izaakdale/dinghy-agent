package discovery

import (
	"log"

	"github.com/hashicorp/serf/serf"
)

func HandleSerfEvent(e serf.Event, node *serf.Serf) {
	switch e.EventType() {
	case serf.EventMemberJoin:
		for _, member := range e.(serf.MemberEvent).Members {
			if isLocal(node, member) {
				continue
			}
			err := handleJoin(member)
			if err != nil {
				log.Printf("error handling member join: %e", err)
			}
		}
	case serf.EventMemberLeave, serf.EventMemberFailed:
		for _, member := range e.(serf.MemberEvent).Members {
			if isLocal(node, member) {
				continue
			}
			handleLeave(member)
		}
	case serf.EventMemberUpdate:
		for _, member := range e.(serf.MemberEvent).Members {
			if isLocal(node, member) {
				continue
			}
			handleUpdate(member)
		}
	}
}

type WorkerNode struct {
	Name     string
	GRPCAddr string
	RaftAddr string
}

func handleJoin(m serf.Member) error {
	log.Printf("member joined %s @ %s\n", m.Name, m.Addr)

	tag, _ := m.Tags["name"]
	tag2, _ := m.Tags["grpc_addr"]
	tag3, _ := m.Tags["raft_addr"]

	w := WorkerNode{
		Name:     tag,
		GRPCAddr: tag2,
		RaftAddr: tag3,
	}
	log.Printf("%+v\n", w)

	return nil
}

func isLocal(c *serf.Serf, m serf.Member) bool {
	return c.LocalMember().Name == m.Name
}

func handleLeave(m serf.Member) {
	log.Printf("member leaving %s @ %s\n", m.Name, m.Addr)
}

func handleUpdate(m serf.Member) {
	log.Printf("receiving update from %s @ %s\n", m.Name, m.Addr)
}
