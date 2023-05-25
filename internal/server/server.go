package server

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	v1 "github.com/izaakdale/dinghy-agent/api/v1"

	workerApi "github.com/izaakdale/dinghy-worker/api/v1"
)

var ErrNoServers = errors.New("no servers to carry out request")

var _ v1.AgentServer = (*BalancingServer)(nil)

type BalancingServer struct {
	v1.UnimplementedAgentServer
	mu        sync.Mutex
	leader    *Client
	followers []*Client
	current   uint64
}

type Client struct {
	ServerID string
	GRPCAddr string
	RaftAddr string
	workerApi.WorkerClient
}

func New() *BalancingServer {
	return &BalancingServer{
		leader:    nil,
		followers: []*Client{},
	}
}

func (s *BalancingServer) Insert(ctx context.Context, request *v1.InsertRequest) (*v1.InsertResponse, error) {
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

func (s *BalancingServer) Delete(ctx context.Context, request *v1.DeleteRequest) (*v1.DeleteResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *BalancingServer) Fetch(ctx context.Context, request *v1.FetchRequest) (*v1.FetchResponse, error) {
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

func (s *BalancingServer) Memberlist(context.Context, *v1.MemberlistRequest) (*v1.MemberlistResponse, error) {
	members := s.GetMembers()
	if members == nil {
		return &v1.MemberlistResponse{}, nil
	}

	return &v1.MemberlistResponse{
		Leader:    members.Leader,
		Followers: members.Followers,
	}, nil
}

func (b *BalancingServer) nextFollower() (*Client, error) {
	switch {
	case len(b.followers) == 0 && b.leader == nil:
		return nil, ErrNoServers
	case len(b.followers) == 0:
		return b.leader, nil
	}

	cur := atomic.AddUint64(&b.current, uint64(1))
	len := uint64(len(b.followers))
	idx := int(cur % len)
	if b.followers[idx] == nil {
		return b.nextFollower()
	}
	return b.followers[idx], nil
}
