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

package sandbox

import (
	"context"

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
)

type controllerServer struct {
	ctrl Controller
}

func FromService(service Controller) api.ControllerServer {
	return &controllerServer{ctrl: service}
}

var _ api.ControllerServer = &controllerServer{}

func (p *controllerServer) Start(ctx context.Context, req *api.ControllerStartRequest) (*api.ControllerStartResponse, error) {
	in, err := instanceFromProto(req.Instance)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	out, err := p.ctrl.Start(ctx, in)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	proto, err := instanceToProto(&out)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.ControllerStartResponse{Instance: proto}, nil
}

func (p *controllerServer) Stop(ctx context.Context, req *api.ControllerStopRequest) (*api.ControllerStopResponse, error) {
	in, err := instanceFromProto(req.Instance)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	out, err := p.ctrl.Stop(ctx, in)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	proto, err := instanceToProto(&out)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.ControllerStopResponse{Instance: proto}, nil
}

func (p *controllerServer) Update(ctx context.Context, req *api.ControllerUpdateRequest) (*api.ControllerUpdateResponse, error) {
	in, err := instanceFromProto(req.Instance)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	out, err := p.ctrl.Update(ctx, in, req.Fields...)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	proto, err := instanceToProto(&out)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.ControllerUpdateResponse{Instance: proto}, nil
}

func (p *controllerServer) Status(ctx context.Context, req *api.ControllerStatusRequest) (*api.ControllerStatusResponse, error) {
	in, err := instanceFromProto(req.Instance)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	status, err := p.ctrl.Status(ctx, in)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	proto := statusToProto(status)
	return &api.ControllerStatusResponse{Status: proto}, nil
}

func (p *controllerServer) Delete(ctx context.Context, req *api.ControllerDeleteRequest) (*api.ControllerDeleteResponse, error) {
	in, err := instanceFromProto(req.Instance)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	if err := p.ctrl.Delete(ctx, in); err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.ControllerDeleteResponse{}, nil
}
