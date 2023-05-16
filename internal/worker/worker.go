package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	v1 "github.com/izaakdale/dinghy-worker/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	ServerID string
	GRPCAddr string
	RaftAddr string
	v1.WorkerClient
}

type Balancer struct {
	mu        sync.Mutex
	Leader    *Client
	Followers []*Client
}

func (b *Balancer) AddClient(c *Client) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.Leader != nil {
		b.Followers = append(b.Followers, c)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		log.Printf("%+v\n", b.Leader)
		_, err := b.Leader.Join(ctx, &v1.JoinRequest{
			ServerAddr: c.RaftAddr,
			ServerId:   c.ServerID,
		})
		if err != nil {
			return err
		}
		return nil
	}
	b.Leader = c
	return nil
}

func (b *Balancer) RemoveNode(serverID string) error {
	if b.Leader.ServerID == serverID {
		b.Leader = nil
	}

	for i, f := range b.Followers {
		if f.ServerID == serverID {
			b.Followers = append(b.Followers[:i], b.Followers[i+1:]...)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		resp, err := f.RaftState(ctx, &v1.RaftStateRequest{})
		if err != nil {
			return err
		}
		if resp.State == "Leader" {
			b.Leader = f
		}
	}

	return nil
}

func New(serverID, grpcAddr, raftAddr string) (*Client, error) {
	conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s", grpcAddr)
	}
	client := v1.NewWorkerClient(conn)
	return &Client{
		ServerID:     serverID,
		GRPCAddr:     grpcAddr,
		RaftAddr:     raftAddr,
		WorkerClient: client,
	}, nil
}
