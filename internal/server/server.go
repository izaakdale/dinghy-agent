package server

import (
	"context"

	v1 "github.com/izaakdale/dinghy-agent/api/v1"
	"github.com/izaakdale/dinghy-agent/internal/worker"
)

var _ v1.AgentServer = (*Server)(nil)

type Server struct {
	v1.UnimplementedAgentServer
	balance *worker.Balancer
}

func New(b *worker.Balancer) *Server {
	return &Server{balance: b}
}

func (s *Server) Insert(ctx context.Context, request *v1.InsertRequest) (*v1.InsertResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) Delete(ctx context.Context, request *v1.DeleteRequest) (*v1.DeleteResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) Fetch(ctx context.Context, request *v1.FetchRequest) (*v1.FetchResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) Memberlist(context.Context, *v1.MemberlistRequest) (*v1.MemberlistResponse, error) {
	var followers []string
	if s.balance.Leader == nil {
		return &v1.MemberlistResponse{}, nil
	}

	for _, f := range s.balance.Followers {
		followers = append(followers, f.ServerID)
	}
	return &v1.MemberlistResponse{
		Leader:    s.balance.Leader.ServerID,
		Followers: followers,
	}, nil
}
