/*
   Copyright The containerd Authors.

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

package containerd

import (
	"context"

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
	sb "github.com/containerd/containerd/sandbox"
	"github.com/gogo/protobuf/types"
)

// sandboxRemoteController is a low level GRPC client for containerd's sandbox controller service
type sandboxRemoteController struct {
	client api.ControllerClient
}

var _ sb.Controller = (*sandboxRemoteController)(nil)

// NewSandboxRemoteController creates client for sandbox controller
func NewSandboxRemoteController(client api.ControllerClient) sb.Controller {
	return &sandboxRemoteController{client: client}
}

func (s *sandboxRemoteController) Start(ctx context.Context, sandboxID string) error {
	if _, err := s.client.Start(ctx, &api.ControllerStartRequest{
		SandboxID: sandboxID,
	}); err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (s *sandboxRemoteController) Shutdown(ctx context.Context, sandboxID string) error {
	_, err := s.client.Shutdown(ctx, &api.ControllerShutdownRequest{SandboxID: sandboxID})
	if err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (s *sandboxRemoteController) Pause(ctx context.Context, sandboxID string) error {
	_, err := s.client.Pause(ctx, &api.ControllerPauseRequest{SandboxID: sandboxID})
	if err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (s *sandboxRemoteController) Resume(ctx context.Context, sandboxID string) error {
	_, err := s.client.Resume(ctx, &api.ControllerResumeRequest{SandboxID: sandboxID})
	if err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (s *sandboxRemoteController) Ping(ctx context.Context, sandboxID string) error {
	if _, err := s.client.Ping(ctx, &api.ControllerPingRequest{SandboxID: sandboxID}); err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (s *sandboxRemoteController) Status(ctx context.Context, sandboxID string) (*types.Any, error) {
	resp, err := s.client.Status(ctx, &api.ControllerStatusRequest{SandboxID: sandboxID})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	return resp.Status, nil
}
