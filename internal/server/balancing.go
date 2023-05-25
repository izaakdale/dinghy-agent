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

func (b *BalancingServer) AddClient(serverID, grpcAddr, raftAddr string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

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

	if b.leader != nil {
		b.followers = append(b.followers, client)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := b.leader.Join(ctx, &workerApi.JoinRequest{
			ServerAddr: client.RaftAddr,
			ServerId:   client.ServerID,
		})
		if err != nil {
			return err
		}
		return nil
	}
	b.leader = client
	return connectToLeader(b.leader)
}

func (s *BalancingServer) RemoveClient(serverID string) error {
	if s.leader != nil && s.leader.ServerID == serverID {
		s.leader = nil
	}

	for i, f := range s.followers {
		if f.ServerID == serverID {
			s.followers = append(s.followers[:i], s.followers[i+1:]...)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		resp, err := f.RaftState(ctx, &workerApi.RaftStateRequest{})
		if err != nil {
			return err
		}
		if resp.State == "Leader" {
			s.leader = f
		}
	}

	return nil
}

func connectToLeader(c *Client) error {
	resp, err := c.RaftState(context.Background(), &workerApi.RaftStateRequest{})
	if err != nil {
		return err
	}

	if resp.State != "Leader" {
		log.Printf("waiting for first client through the door to announce leadership.\n")
		time.Sleep(time.Second)
		connectToLeader(c)
	}

	return nil
}

func (s *BalancingServer) GetMembers() *Memberlist {
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
