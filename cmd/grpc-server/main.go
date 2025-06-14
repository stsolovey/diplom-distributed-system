package main

import (
	"log"
	"net"

	grpcservice "github.com/stsolovey/diplom-distributed-system/internal/grpc"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", "localhost:50052")
	if err != nil {
		log.Printf("Failed to listen: %v", err)

		return
	}

	server := grpc.NewServer()

	// Регистрируем наш сервис
	ingestServer := grpcservice.NewIngestServer("http://localhost:8081")
	grpcservice.RegisterIngestServiceServer(server, ingestServer)

	log.Println("gRPC server listening on localhost:50052")

	if err := server.Serve(lis); err != nil {
		log.Printf("Failed to serve: %v", err)
	}
}
