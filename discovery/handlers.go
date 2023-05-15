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

func handleJoin(m serf.Member) error {
	log.Printf("member joined %s @ %s\n", m.Name, m.Addr)

	tag, ok := m.Tags["test"]
	if !ok {
		log.Printf("no tags\n")
	}
	log.Printf("---- %+v ----\n", tag)

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
