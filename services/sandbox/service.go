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
			return newRouter(ss), nil
		},
	})
}

type router struct {
	ss map[string]api.StoreServer
}

var _ api.StoreServer = &router{}

func newRouter(ss map[string]sandbox.Store) *router {
	srv := &router{
		ss: make(map[string]api.StoreServer),
	}

	for k, v := range ss {
		srv.ss[k] = sandbox.NewStore(v)
	}

	return srv
}

func (s *router) Register(srv *grpc.Server) error {
	api.RegisterStoreServer(srv, s)
	return nil
}

func (s *router) Start(ctx context.Context, req *api.StoreStartRequest) (*api.StoreStartResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("instance", req.Instance).Debug("start sandbox")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	return svc.Start(ctx, req)
}

func (s *router) Stop(ctx context.Context, req *api.StoreStopRequest) (*api.StoreStopResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("stop sandbox")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	return svc.Stop(ctx, req)
}

func (s *router) Update(ctx context.Context, req *api.StoreUpdateRequest) (*api.StoreUpdateResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("instance", req.Instance).Debug("update sandbox")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	return svc.Update(ctx, req)
}

func (s *router) Find(ctx context.Context, req *api.StoreFindRequest) (*api.StoreFindResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("sandbox info")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	return svc.Find(ctx, req)
}

func (s *router) Status(ctx context.Context, req *api.StoreStatusRequest) (*api.StoreStatusResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("sandbox status")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	return svc.Status(ctx, req)
}

func (s *router) List(ctx context.Context, req *api.StoreListRequest) (*api.StoreListResponse, error) {
	log.G(ctx).WithField("name", req.Name).Debug("list sandboxes")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	return svc.List(ctx, req)
}

func (s *router) Delete(ctx context.Context, req *api.StoreDeleteRequest) (*api.StoreDeleteResponse, error) {
	log.G(ctx).WithField("name", req.Name).WithField("sandbox_id", req.ID).Debug("delete sandbox")

	svc, err := s.find(req.Name)
	if err != nil {
		return nil, err
	}

	return svc.Delete(ctx, req)
}

func (s *router) find(name string) (api.StoreServer, error) {
	svc, ok := s.ss[name]
	if !ok {
		return nil, errdefs.ToGRPCf(errdefs.ErrInvalidArgument, "sandbox %q not loaded", name)
	}

	return svc, nil
}
