package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	v1 "github.com/izaakdale/dinghy-agent/api/v1"

	workerApi "github.com/izaakdale/dinghy-worker/api/v1"
)

var ErrNoServers = errors.New("no servers to carry out request")

var _ v1.AgentServer = (*BalancerServer)(nil)

type BalancerServer struct {
	v1.UnimplementedAgentServer
	mu              sync.Mutex
	workers         map[string]*Client
	leaderID        string
	currentWorkerID string
}

type Client struct {
	ServerID string
	GRPCAddr string
	RaftAddr string
	workerApi.WorkerClient
}

func New() *BalancerServer {
	return &BalancerServer{
		leaderID: "",
		workers:  make(map[string]*Client),
	}
}

func (b *BalancerServer) HeartbeatHandler(server *workerApi.ServerHeartbeat) {
	log.Printf("received heartbeat from %s\n", server.Name)
	if server.IsLeader && b.leaderID != server.Name {
		b.leaderID = server.Name
	}

	if _, ok := b.workers[server.Name]; !ok {
		log.Printf("received a heartbeat from an unknown server - %s\n", server.Name)
	}
}

func (s *BalancerServer) Insert(ctx context.Context, request *v1.InsertRequest) (*v1.InsertResponse, error) {
	log.Printf("insert served by %s\n", s.leaderID)
	leader, ok := s.workers[s.leaderID]
	if !ok {
		return nil, fmt.Errorf("No registered leader to insert request")
	}
	if leader == nil {
		return nil, ErrNoServers
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := leader.Insert(ctx, &workerApi.InsertRequest{
		Key:   request.Key,
		Value: request.Value,
	})
	if err != nil {
		return nil, err
	}

	return &v1.InsertResponse{}, nil
}

func (s *BalancerServer) Delete(ctx context.Context, request *v1.DeleteRequest) (*v1.DeleteResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *BalancerServer) Fetch(ctx context.Context, request *v1.FetchRequest) (*v1.FetchResponse, error) {
	f, err := s.nextFollower()
	if err != nil {
		return nil, err
	}
	log.Printf("fetch served by %+v\n", f.ServerID)

	resp, err := f.Fetch(ctx, &workerApi.FetchRequest{
		Key: request.Key,
	})
	if err != nil {
		return nil, err
	}

	return &v1.FetchResponse{
		Key:   resp.Key,
		Value: resp.Value,
	}, nil
}

func (s *BalancerServer) Memberlist(context.Context, *v1.MemberlistRequest) (*v1.MemberlistResponse, error) {
	members := s.GetMembers()
	if members == nil {
		return &v1.MemberlistResponse{}, nil
	}

	return &v1.MemberlistResponse{
		Leader:    members.Leader,
		Followers: members.Followers,
	}, nil
}

func (b *BalancerServer) nextFollower() (*Client, error) {
	if len(b.workers) == 0 {
		return nil, ErrNoServers
	}

	for _, c := range b.workers {
		if c.ServerID != b.currentWorkerID && c.ServerID != b.leaderID {
			b.currentWorkerID = c.ServerID
			return c, nil
		}
	}
	// reaching here is technically impossible, but still return nil, poet, know it
	return nil, nil
}
