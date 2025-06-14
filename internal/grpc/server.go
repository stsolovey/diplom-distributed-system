package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/stsolovey/diplom-distributed-system/internal/client"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type IngestServer struct {
	UnimplementedIngestServiceServer
	processorClient *client.ProcessorClient
}

func NewIngestServer(processorURL string) *IngestServer {
	return &IngestServer{
		processorClient: client.NewProcessorClient(processorURL),
	}
}

func (s *IngestServer) Ingest(ctx context.Context, req *IngestRequest) (*IngestResponse, error) {
	msg := &models.DataMessage{
		Id:        uuid.New().String(),
		Timestamp: time.Now().Unix(),
		Source:    req.GetSource(),
		Payload:   req.GetData(),
		Metadata:  req.GetMetadata(),
	}

	if err := s.processorClient.SendMessage(ctx, msg); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to process: %v", err)
	}

	return &IngestResponse{
		MessageId: msg.GetId(),
		Status:    "accepted",
	}, nil
}

func (s *IngestServer) IngestStream(stream grpc.ClientStreamingServer[IngestRequest, IngestResponse]) error {
	var processed int32

	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("stream send and close failed: %w", stream.SendAndClose(&IngestResponse{
				MessageId: fmt.Sprintf("batch-%d", processed),
				Status:    "completed",
			}))
		}

		if err != nil {
			return fmt.Errorf("stream receive failed: %w", err)
		}

		msg := &models.DataMessage{
			Id:        uuid.New().String(),
			Timestamp: time.Now().Unix(),
			Source:    req.GetSource(),
			Payload:   req.GetData(),
			Metadata:  req.GetMetadata(),
		}

		if err := s.processorClient.SendMessage(stream.Context(), msg); err != nil {
			return status.Errorf(codes.Internal, "failed to process message %d: %v", processed, err)
		}

		processed++
	}
}
