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

	s.workers[serverID] = client
	// if there is one worker, it means this client is the first in. Make it leader.
	if len(s.workers) == 1 {
		// wait for leader hangs until the server responds that it is a leader
		// basically there is an election process that needs to end before we
		// can start the assignment process.
		s.leaderID = client.ServerID
		return waitForLeader(client)
	} else {
		// otherwise we want to tell them to join the leader.
		return s.connectToLeader(client)
	}
}

func (s *BalancerServer) RemoveClient(serverID string) error {
	if _, ok := s.workers[serverID]; ok {
		delete(s.workers, serverID)
	}
	if s.leaderID == serverID {
		s.leaderID = ""
	}
	return nil
}

func (s *BalancerServer) NewLeadership(serverID, grpcAddr, raftAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.leaderID = serverID

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
	leader, ok := s.workers[s.leaderID]
	if !ok {
		log.Printf("backing off waiting for leadership claim\n")
		time.Sleep(time.Second)
		return s.connectToLeader(c)
	}

	resp, err := leader.WorkerClient.RaftState(context.Background(), &workerApi.RaftStateRequest{})
	// TODO it is possible that this would loop forever. Maybe should implement a finite backoff.
	if err != nil || resp.State != "Leader" {
		log.Printf("need a timeout since my request for leader info failed, or I am asking the wrong server.\n")
		time.Sleep(time.Second)
		return s.connectToLeader(c)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := leader.Join(ctx, &workerApi.JoinRequest{
		ServerAddr: c.RaftAddr,
		ServerId:   c.ServerID,
	}); err != nil {
		return err
	}
	s.workers[c.ServerID] = c
	return nil
}

type Memberlist struct {
	Leader    string
	Followers []string
}

func (s *BalancerServer) GetMembers() *Memberlist {
	if len(s.workers) == 0 {
		return nil
	}

	ret := &Memberlist{}
	ret.Leader = s.leaderID
	for _, w := range s.workers {
		if w.ServerID == s.leaderID {
			continue
		}
		ret.Followers = append(ret.Followers, w.ServerID)
	}
	return ret
}
