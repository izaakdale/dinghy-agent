package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	agentApi "github.com/izaakdale/dinghy-agent/api/v1"
	workerApi "github.com/izaakdale/dinghy-worker/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ErrNoServers = errors.New("no servers to carry out request")

type Client struct {
	ServerID string
	GRPCAddr string
	RaftAddr string
	workerApi.WorkerClient
}

type Balancer struct {
	mu        sync.Mutex
	leader    *Client
	followers []*Client
	current   uint64
}

func (b *Balancer) AddClient(serverID, grpcAddr, raftAddr string) error {
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
		log.Printf("%+v\n", b.leader)
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

func connectToLeader(c *Client) error {
	resp, err := c.RaftState(context.Background(), &workerApi.RaftStateRequest{})
	if err != nil {
		return err
	}

	if resp.State != "Leader" {
		log.Printf("backing off for a second since I thought it was the leader\n")
		time.Sleep(time.Second)
		connectToLeader(c)
	}

	return nil
}

type Memberlist struct {
	Leader    string
	Followers []string
}

func (b *Balancer) GetMembers() *Memberlist {
	if b.leader == nil && len(b.followers) == 0 {
		// find new leader or return nil for true missing servers
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	var followers []string
	for _, f := range b.followers {
		followers = append(followers, f.ServerID)
	}

	return &Memberlist{
		Leader:    b.leader.ServerID,
		Followers: followers,
	}
}

func (b *Balancer) RemoveNode(serverID string) error {
	if b.leader != nil && b.leader.ServerID == serverID {
		b.leader = nil
	}

	for i, f := range b.followers {
		if f.ServerID == serverID {
			b.followers = append(b.followers[:i], b.followers[i+1:]...)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		resp, err := f.RaftState(ctx, &workerApi.RaftStateRequest{})
		if err != nil {
			return err
		}
		if resp.State == "Leader" {
			b.leader = f
		}
	}

	return nil
}

func NewBalancer() *Balancer {
	return &Balancer{
		leader:    nil,
		followers: []*Client{},
	}
}

func (b *Balancer) ForwardFetch(ctx context.Context, request *agentApi.FetchRequest) (*agentApi.FetchResponse, error) {
	f, err := b.nextFollower()
	if err != nil {
		return nil, err
	}
	log.Printf("------- fetch served by %+v -------\n", f.ServerID)

	resp, err := f.Fetch(ctx, &workerApi.FetchRequest{
		Key: request.Key,
	})
	if err != nil {
		return nil, err
	}

	return &agentApi.FetchResponse{
		Key:   resp.Key,
		Value: resp.Value,
	}, nil
}
func (b *Balancer) ForwardInsert(ctx context.Context, request *agentApi.InsertRequest) (*agentApi.InsertResponse, error) {
	if b.leader == nil {
		return nil, ErrNoServers
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := b.leader.Insert(ctx, &workerApi.InsertRequest{
		Key:   request.Key,
		Value: request.Value,
	})
	if err != nil {
		return nil, err
	}

	return &agentApi.InsertResponse{}, nil
}

func (b *Balancer) nextFollower() (*Client, error) {
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
