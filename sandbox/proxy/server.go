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

package proxy

import (
	"context"

	"github.com/containerd/typeurl"
	"github.com/gogo/protobuf/types"
	runtime "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/sandbox"
)

type proxyServer struct {
	srv sandbox.Service
}

func FromService(service sandbox.Service) api.SandboxServer {
	return &proxyServer{srv: service}
}

var _ api.SandboxServer = &proxyServer{}

func (p *proxyServer) Start(ctx context.Context, req *api.StartSandboxRequest) (*api.StartSandboxResponse, error) {
	spec, err := anyToSpec(req.RuntimeSpec)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	createInfo := sandbox.CreateInfo{
		ID:          req.ID,
		RuntimeSpec: spec,
		Labels:      req.Labels,
		Extensions:  req.Extensions,
	}

	info, err := p.srv.Start(ctx, &createInfo)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.StartSandboxResponse{
		ID: info.ID,
	}, nil
}

func (p *proxyServer) Stop(ctx context.Context, req *api.StopSandboxRequest) (*api.StopSandboxResponse, error) {
	if err := p.srv.Stop(ctx, req.ID); err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.StopSandboxResponse{}, nil
}

func (p *proxyServer) Update(ctx context.Context, req *api.UpdateSandboxRequest) (*api.UpdateSandboxResponse, error) {
	info := &sandbox.CreateInfo{
		ID:         req.ID,
		Labels:     req.Labels,
		Extensions: req.Extensions,
	}

	if req.RuntimeSpec != nil {
		spec, err := anyToSpec(req.RuntimeSpec)
		if err != nil {
			return nil, errdefs.ToGRPC(err)
		}

		info.RuntimeSpec = spec
	}

	err := p.srv.Update(ctx, info, req.Fields...)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.UpdateSandboxResponse{}, nil
}

func (p *proxyServer) Status(ctx context.Context, req *api.StatusSandboxRequest) (*api.StatusSandboxResponse, error) {
	status, err := p.srv.Status(ctx, req.ID)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.StatusSandboxResponse{
		ID:      status.ID,
		Pid:     status.PID,
		State:   string(status.State),
		Version: status.Version,
		Extra:   status.Extra,
	}, nil
}

func (p *proxyServer) Delete(ctx context.Context, req *api.DeleteSandboxRequest) (*api.DeleteSandboxResponse, error) {
	if err := p.srv.Delete(ctx, req.ID); err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.DeleteSandboxResponse{}, nil
}

func anyToSpec(any *types.Any) (*runtime.Spec, error) {
	if any == nil {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "empty runtime spec")
	}

	raw, err := typeurl.UnmarshalAny(any)
	if err != nil {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "failed to unmarshal runtime spec")
	}

	spec, ok := raw.(*runtime.Spec)
	if !ok {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "invalid runtime spec structure")
	}

	return spec, nil
}
