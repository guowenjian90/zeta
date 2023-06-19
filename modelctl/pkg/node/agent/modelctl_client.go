/*
Copyright 2022 The CentaurusAI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	modelctl "CentaurusAI/zeta/modelctl/grpc"
	"CentaurusAI/zeta/modelctl/pkg/util"

	"context"
	"io"
	"time"

	"github.com/go-logr/logr"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type ClientConfiguration struct {
	endpoint       string
	connTimeout    time.Duration
	callTimeout    time.Duration
	maxRecvMsgSize int
}

const (
	DefaultConnTimeout    = 5 * time.Second
	DefaultCallTimeout    = 2 * time.Second
	DefaultMaxRecvMsgSize = 16 * 1024
)

func NewClientConfiguration(endpoint string) *ClientConfiguration {
	return &ClientConfiguration{
		endpoint:       endpoint,
		connTimeout:    DefaultConnTimeout,
		callTimeout:    DefaultCallTimeout,
		maxRecvMsgSize: DefaultMaxRecvMsgSize,
	}
}

type ModelCtlClient interface {
	Start()
	Stop()
	PutMessage(message *modelctl.NodeMessage) error
	GetMessage(receiver string, channel chan *modelctl.NodeMessage) error
}

type modelCtlClient struct {
	identifier       *modelctl.NodeIdentifier
	log              logr.Logger
	stop             bool
	config           *ClientConfiguration
	conn             *grpc.ClientConn
	service          modelctl.ModelCtlServiceClient
	getMessageClient modelctl.ModelCtlService_GetMessageClient
	receivers        map[string]chan *modelctl.NodeMessage
}

func (f *modelCtlClient) GetMessage(receiver string, channel chan *modelctl.NodeMessage) error {
	f.receivers[receiver] = channel
	return nil
}

func (f *modelCtlClient) PutMessage(message *modelctl.NodeMessage) error {
	if f.service == nil {
		if err := f.connect(); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.config.callTimeout)
	defer cancel()
	opts := grpc.EmptyCallOption{}
	_, err := f.service.PutMessage(ctx, message, opts)
	return err
}

func (f *modelCtlClient) disconnect() error {
	return f.conn.Close()
}

func (f *modelCtlClient) connect() error {
	connect := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), f.config.connTimeout)
		defer cancel()

		opts := []grpc_retry.CallOption{
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(100 * time.Millisecond)),
			grpc_retry.WithCodes(codes.NotFound, codes.Aborted, codes.Unavailable, codes.DataLoss, codes.Unknown),
		}
		conn, err := grpc.DialContext(
			ctx,
			f.config.endpoint,
			grpc.WithBlock(),
			grpc.WithInsecure(),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(f.config.maxRecvMsgSize)),
			grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)),
		)
		if err != nil {
			return err
		}

		f.conn = conn
		f.service = modelctl.NewModelCtlServiceClient(conn)
		return nil
	}

	err := util.BackoffExec(2*time.Second, 10*time.Second, 30*time.Second, 1.7, connect)
	return err
}

func (f *modelCtlClient) initGetMessageClient(ctx context.Context, identifier *modelctl.NodeIdentifier) error {
	if f.conn == nil {
		err := f.connect()
		if err != nil {
			return err
		}
	}
	gclient, err := f.service.GetMessage(ctx, identifier)
	if err != nil {
		return err
	}
	f.getMessageClient = gclient
	return nil
}

// should exec in a go routine, recvMessage loop forever until it's old stop
// it receive message and dispatch it to receivers' channel registered by GetMessage
func (f *modelCtlClient) recvMessage() {
	for {
		if f.stop {
			break
		}

		ctx := context.Background()
		if f.getMessageClient == nil {
			err := f.initGetMessageClient(ctx, f.identifier)
			if err != nil {
				f.log.Error(err, "Failed to init ModelCtl GetMessage client", "endpoint", f.config.endpoint)
				time.Sleep(2 * time.Second)
				continue
			}
		}

		msg, err := f.getMessageClient.Recv()
		if err == io.EOF {
			f.log.Error(err, "ModelCtl closed stream at server side, reset to get a new stream client")
			f.getMessageClient = nil
			continue
		}

		if err != nil {
			f.log.Error(err, "Receive message failed with unexpected error, reset to get a new stream client")
			f.getMessageClient = nil
			continue
		}

		panicReceivers := []string{}
		for n, v := range f.receivers {
			func() {
				defer func() {
					if err := recover(); err != nil {
						f.log.Info("Send message panic occurred", "error", err)
						panicReceivers = append(panicReceivers, n)
					}
				}()
				v <- msg
			}()
		}

		for _, n := range panicReceivers {
			delete(f.receivers, n)
		}
	}
}

// Stop disconnect with model ctl server
func (f *modelCtlClient) Stop() {
	f.stop = true
	f.disconnect()
}

// Start recive message from model ctl server
func (f *modelCtlClient) Start() {
	go f.recvMessage()
}

var _ ModelCtlClient = &modelCtlClient{}

func NewModelCtlClient(log logr.Logger, identifier *modelctl.NodeIdentifier, config *ClientConfiguration) *modelCtlClient {
	f := &modelCtlClient{
		log:              log,
		identifier:       identifier,
		stop:             false,
		config:           config,
		conn:             nil,
		service:          nil,
		getMessageClient: nil,
		receivers:        map[string]chan *modelctl.NodeMessage{},
	}
	return f
}
