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
	return s.balance.ForwardInsert(ctx, request)
}

func (s *Server) Delete(ctx context.Context, request *v1.DeleteRequest) (*v1.DeleteResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) Fetch(ctx context.Context, request *v1.FetchRequest) (*v1.FetchResponse, error) {
	return s.balance.ForwardFetch(ctx, request)
}

func (s *Server) Memberlist(context.Context, *v1.MemberlistRequest) (*v1.MemberlistResponse, error) {
	members := s.balance.GetMembers()
	if members == nil {
		return &v1.MemberlistResponse{}, nil
	}

	return &v1.MemberlistResponse{
		Leader:    members.Leader,
		Followers: members.Followers,
	}, nil
}
