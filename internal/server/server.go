package server

import (
	"context"
	"errors"
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
	mu        sync.Mutex
	leader    *Client
	followers map[string]*Client
	current   string
}

type Client struct {
	ServerID string
	GRPCAddr string
	RaftAddr string
	workerApi.WorkerClient
}

func New() *BalancerServer {
	return &BalancerServer{
		leader:    nil,
		followers: make(map[string]*Client),
	}
}

func (s *BalancerServer) Insert(ctx context.Context, request *v1.InsertRequest) (*v1.InsertResponse, error) {
	if s.leader == nil {
		return nil, ErrNoServers
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := s.leader.Insert(ctx, &workerApi.InsertRequest{
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
	switch {
	case len(b.followers) == 0 && b.leader == nil:
		return nil, ErrNoServers
	case len(b.followers) == 0:
		return b.leader, nil
	}

	// TODO simplify/merge two loops even though its not needed really.

	// if current is empty, just return any old server
	if b.current == "" {
		for _, c := range b.followers {
			b.current = c.ServerID
			return c, nil
		}
	}

	// current not empty, return first server that doesn't match, then assign current
	for _, c := range b.followers {
		if c.ServerID != b.current {
			b.current = c.ServerID
			return c, nil
		}
	}
	// reaching here is technically impossible, but still return nil, poet, know it
	return nil, nil
}
