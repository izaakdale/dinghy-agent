package discovery

import (
	"fmt"
	"log"

	"github.com/hashicorp/serf/serf"
	"github.com/izaakdale/dinghy-agent/internal/server"
	v1 "github.com/izaakdale/dinghy-worker/api/v1"
	"google.golang.org/protobuf/proto"
)

func HandleSerfEvent(e serf.Event, node *serf.Serf, srv *server.BalancerServer) {
	switch e.EventType() {
	case serf.EventMemberJoin:
		for _, member := range e.(serf.MemberEvent).Members {
			m := member
			if isLocal(node, m) {
				continue
			}
			// so that long joins don't block the serf events we run concurrently
			go func() {
				err := handleJoin(m, srv)
				if err != nil {
					log.Printf("error handling member join: %s", err.Error())
				}
			}()
		}
	case serf.EventMemberLeave, serf.EventMemberFailed:
		for _, member := range e.(serf.MemberEvent).Members {
			if isLocal(node, member) {
				continue
			}
			err := handleLeave(member, srv)
			if err != nil {
				log.Printf("error handling member leave: %s", err.Error())
			}
		}
	case serf.EventUser:
		err := handleCustomEvent(e.(serf.UserEvent), srv)
		if err != nil {
			log.Printf("error handling custom event: %s", err.Error())
		}
	}
}

func handleJoin(m serf.Member, srv *server.BalancerServer) error {
	log.Printf("member joined %s @ %s\n", m.Name, m.Addr)

	// pre-emtively adding types for expansion of agent
	typeTag, ok := m.Tags["type"]
	if !ok {
		return fmt.Errorf("no type tag for incoming node")
	}
	if typeTag != "worker" {
		// handle potential "agent" join here
		return nil
	}

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

	if err := srv.AddClient(nameTag, grpcTag, raftTag); err != nil {
		return err
	}
	return nil
}

func isLocal(c *serf.Serf, m serf.Member) bool {
	return c.LocalMember().Name == m.Name
}

func handleLeave(m serf.Member, srv *server.BalancerServer) error {
	log.Printf("member leaving %s @ %s\n", m.Name, m.Addr)
	err := srv.RemoveClient(m.Name)
	if err != nil {
		return err
	}
	return nil
}

func handleCustomEvent(e serf.UserEvent, srv *server.BalancerServer) error {
	var node v1.ServerHeartbeat
	if err := proto.Unmarshal(e.Payload, &node); err != nil {
		return err
	}
	srv.HeartbeatHandler(&node)
	return nil
}
