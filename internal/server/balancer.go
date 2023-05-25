package server

import (
	"context"
	"fmt"
	"log"
	"time"

	workerApi "github.com/izaakdale/dinghy-worker/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Memberlist struct {
	Leader    string
	Followers []string
}

func (s *BalancerServer) AddClient(serverID, grpcAddr, raftAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("adding client\n")

	conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to %s", grpcAddr)
	}
	worker := workerApi.NewWorkerClient(conn)
	client := &Client{
		ServerID:     serverID,
		GRPCAddr:     grpcAddr,
		RaftAddr:     raftAddr,
		WorkerClient: worker,
	}
	// TODO check if both leader and followers are nil to assign new leader

	if s.leader != nil {
		log.Printf("leader not nil\n")
		s.followers[client.ServerID] = client
		// b.followers = append(b.followers, client)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := s.leader.Join(ctx, &workerApi.JoinRequest{
			ServerAddr: client.RaftAddr,
			ServerId:   client.ServerID,
		})
		if err != nil {
			return err
		}
		return nil
	}
	log.Printf("adding new leader\n")
	s.leader = client
	return connectToLeader(s.leader)
}

func (s *BalancerServer) RemoveClient(serverID string) error {
	if s.leader != nil && s.leader.ServerID == serverID {
		s.leader = nil
	}

	if _, ok := s.followers[serverID]; ok {
		// leaving server lives in map
		delete(s.followers, serverID)
	} // else do nothing it doesn't exist anyway!

	return nil
}

func (s *BalancerServer) NewLeadership(serverID, grpcAddr, raftAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("new leadership triggered\n")

	// if new leader connection lives in followers
	if c, ok := s.followers[serverID]; ok {
		log.Printf("new leader is in followers\n")
		// can just assign if leader is nil
		if s.leader == nil {
			s.leader = c
			return nil
		}

		log.Printf("current leader was not nil, so need to swap\n")
		// whack leader in follows, and new leadership claim in leader
		s.followers[serverID] = s.leader
		s.leader = c
		return nil
	}

	log.Printf("adding brand new connection\n")
	// add new connection.
	return s.AddClient(serverID, grpcAddr, raftAddr)
}

func connectToLeader(c *Client) error {
	log.Printf("getting raft state from new client\n")
	resp, err := c.RaftState(context.Background(), &workerApi.RaftStateRequest{})
	if err != nil {
		return err
	}

	log.Printf("raft state: %s\n", resp.State)
	if resp.State != "Leader" {
		log.Printf("waiting for first client through the door to announce leadership.\n")
		time.Sleep(time.Second)
		connectToLeader(c)
	}

	return nil
}

func (s *BalancerServer) GetMembers() *Memberlist {
	if s.leader == nil && len(s.followers) == 0 {
		// find new leader or return nil for true missing servers
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var followers []string
	for _, f := range s.followers {
		followers = append(followers, f.ServerID)
	}

	ret := &Memberlist{}
	if s.leader != nil {
		ret.Leader = s.leader.ServerID
	}
	ret.Followers = followers

	return ret
}
