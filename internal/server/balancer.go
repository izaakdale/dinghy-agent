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

	switch {
	case s.leader == nil && len(s.followers) == 0:
		// this case is a true leadership assigment.
		log.Printf("true leadership assignment\n")
		s.leader = client
		return waitForLeader(client)
	case s.leader == nil:
		// leader is nil but followers is not, probably a leader failure.
		log.Printf("leader is nil, followers is not.\n")
		// unlock the mutex to allow the leader to claim its ownership via serf
		s.mu.Unlock()
		return s.connectToLeader(client)
	case s.leader != nil:
		// leader is not nil so add a follower
		log.Printf("connecting to leader\n")
		return s.connectToLeader(client)
	}
	return nil
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
		s.followers[s.leader.ServerID] = s.leader
		s.leader = c
		return nil
	}

	// this should never happen
	if s.leader == nil {
		log.Printf("I thought this would NEVER happen... check it out!\n")
		return s.AddClient(serverID, grpcAddr, raftAddr)
	}

	for _, f := range s.followers {
		if _, err := s.leader.Join(context.TODO(), &workerApi.JoinRequest{ServerId: f.ServerID, ServerAddr: f.RaftAddr}); err != nil {
			return err
		}
	}

	return nil
}

func waitForLeader(c *Client) error {
	log.Printf("getting raft state from new client\n")
	resp, err := c.RaftState(context.Background(), &workerApi.RaftStateRequest{})
	if err != nil {
		return err
	}

	log.Printf("raft state: %s\n", resp.State)
	if resp.State != "Leader" {
		log.Printf("waiting for first client through the door to announce leadership.\n")
		time.Sleep(time.Second)
		waitForLeader(c)
	}
	return nil
}

func (s *BalancerServer) connectToLeader(c *Client) error {
	if s.leader == nil {
		log.Printf("backing off waiting for leadership claim\n")
		time.Sleep(time.Second)
		return s.connectToLeader(c)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := s.leader.Join(ctx, &workerApi.JoinRequest{
		ServerAddr: c.RaftAddr,
		ServerId:   c.ServerID,
	}); err != nil {
		return err
	}
	s.followers[c.ServerID] = c
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
