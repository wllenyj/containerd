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
	"errors"

	"google.golang.org/grpc"

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/sandbox"
	"github.com/containerd/containerd/services"
)

func init() {
	plugin.Register(&plugin.Registration{
		Type: plugin.GRPCPlugin,
		ID:   "sandboxes",
		Requires: []plugin.Type{
			plugin.ServicePlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			plugins, err := ic.GetByType(plugin.ServicePlugin)
			if err != nil {
				return nil, err
			}
			p, ok := plugins[services.SandboxService]
			if !ok {
				return nil, errors.New("sandbox service not found")
			}
			i, err := p.Instance()
			if err != nil {
				return nil, err
			}
			ss := i.(map[string]sandbox.Store)
			return &service{ss: ss}, nil
		},
	})
}

type service struct {
	ss map[string]sandbox.Store
}

var _ api.StoreServer = &service{}

func (s *service) Register(srv *grpc.Server) error {
	api.RegisterStoreServer(srv, s)
	return nil
}

func (s *service) Start(ctx context.Context, req *api.StartSandboxRequest) (*api.InfoSandboxResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("start sandbox")

	spec, err := sandbox.AnyToSpec(req.RuntimeSpec)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	createInfo := sandbox.CreateOpts{
		ID:          req.ID,
		RuntimeSpec: spec,
		Labels:      req.Labels,
		Extensions:  req.Extensions,
	}

	info, err := svc.Start(ctx, &createInfo)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to start sandbox")
	}

	return &api.InfoSandboxResponse{
		Info: &api.Info{
			ID:          info.ID,
			Labels:      info.Labels,
			CreatedAt:   info.CreatedAt,
			UpdatedAt:   info.UpdatedAt,
			Extensions:  info.Extensions,
			Descriptor_: info.Descriptor,
		},
	}, nil
}

func (s *service) Stop(ctx context.Context, req *api.StopSandboxRequest) (*api.StopSandboxResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("stop sandbox")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	if err := svc.Stop(ctx, req.ID); err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to stop sandbox")
	}

	return &api.StopSandboxResponse{}, nil
}

func (s *service) Update(ctx context.Context, req *api.UpdateSandboxRequest) (*api.UpdateSandboxResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("update sandbox")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	createInfo := sandbox.CreateOpts{
		ID:          req.ID,
		RuntimeSpec: nil,
		Labels:      req.Labels,
		Extensions:  req.Extensions,
	}

	if req.RuntimeSpec != nil {
		spec, err := sandbox.AnyToSpec(req.RuntimeSpec)
		if err != nil {
			return nil, err
		}

		createInfo.RuntimeSpec = spec
	}

	if err := svc.Update(ctx, &createInfo, req.Fields...); err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to udpate sandbox %q", req.ID)
	}

	return &api.UpdateSandboxResponse{}, nil
}

func (s *service) Info(ctx context.Context, req *api.InfoSandboxRequest) (*api.InfoSandboxResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("sandbox info")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	info, err := svc.Info(ctx, req.ID)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to get sandbox %q status", req.ID)
	}

	spec, err := sandbox.SpecToAny(info.RuntimeSpec)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.InfoSandboxResponse{
		Info: &api.Info{
			ID:          info.ID,
			Labels:      info.Labels,
			RuntimeSpec: spec,
			CreatedAt:   info.CreatedAt,
			UpdatedAt:   info.UpdatedAt,
		},
	}, nil
}

func (s *service) Status(ctx context.Context, req *api.StatusSandboxRequest) (*api.StatusSandboxResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("sandbox status")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	status, err := svc.Status(ctx, req.ID)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to get sandbox %q status", req.ID)
	}

	return &api.StatusSandboxResponse{
		ID:      status.ID,
		Pid:     status.PID,
		State:   string(status.State),
		Version: status.Version,
		Extra:   status.Extra,
	}, nil
}

func (s *service) List(ctx context.Context, req *api.ListSandboxRequest) (*api.ListSandboxResponse, error) {
	log.G(ctx).WithField("name", req.Name).Debug("list sandboxes")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	list, err := svc.List(ctx, req.Filters...)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to list sandboxes")
	}

	var (
		resp = api.ListSandboxResponse{}
	)

	resp.Info = make([]*api.Info, len(list))

	for i, item := range list {
		info := api.Info{
			ID:          item.ID,
			Labels:      item.Labels,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
			Descriptor_: item.Descriptor,
			Extensions:  item.Extensions,
		}

		info.RuntimeSpec, err = sandbox.SpecToAny(item.RuntimeSpec)
		if err != nil {
			return nil, errdefs.ToGRPC(err)
		}

		resp.Info[i] = &info
	}

	return &resp, nil
}

func (s *service) Delete(ctx context.Context, req *api.DeleteSandboxRequest) (*api.DeleteSandboxResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("delete sandbox")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	if err := svc.Delete(ctx, req.ID); err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to delete sandbox %q", req.ID)
	}

	return &api.DeleteSandboxResponse{}, nil
}

func (s *service) find(name string) (sandbox.Store, error) {
	svc, ok := s.ss[name]
	if !ok {
		return nil, errdefs.ToGRPCf(errdefs.ErrInvalidArgument, "sandbox %q not loaded", name)
	}

	return svc, nil
}
