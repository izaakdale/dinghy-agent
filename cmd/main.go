package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"os"

	v1 "github.com/izaakdale/dinghy-agent/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	crt, err := tls.LoadX509KeyPair(os.Getenv("CLIENT_CRT"), os.Getenv("CLIENT_KEY"))
	if err != nil {
		log.Fatalf("1: %+v\n", err)
	}

	fCa, err := os.OpenFile(os.Getenv("ROOT_CA"), os.O_RDONLY, os.ModeTemporary)
	if err != nil {
		log.Fatalf("2: %+v\n", err)
	}
	pemBytes, err := io.ReadAll(fCa)
	if err != nil {
		log.Fatalf("3: %+v\n", err)
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(pemBytes)
	if !ok {
		panic("error appending certs from pem")
	}

	conn, err := grpc.Dial("test.tester:443", grpc.WithTransportCredentials(
		credentials.NewTLS(
			&tls.Config{
				Certificates: []tls.Certificate{crt},
				RootCAs:      caCertPool,
			},
		),
	))
	if err != nil {
		panic(err)
	}

	client := v1.NewAgentClient(conn)

	// s := struct {
	// 	Name string `json:"name,omitempty"`
	// 	Age  int    `json:"age,omitempty"`
	// }{
	// 	"izaak",
	// 	30,
	// }

	// bytes, err := json.Marshal(s)
	// if err != nil {
	// 	panic(err)
	// }

	_, err = client.Insert(context.Background(), &v1.InsertRequest{
		Key:   ("hugo"),
		Value: "bytes",
	})
	if err != nil {
		panic(err)
	}

	fResp, err := client.Fetch(context.Background(), &v1.FetchRequest{
		Key: ("hugo"),
	})
	if err != nil {
		panic(err)
	}

	log.Printf("%+v\n", fResp.Value)
}
