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

type storeServer struct {
	store Store
}

var _ api.StoreServer = &storeServer{}

func NewStore(store Store) api.StoreServer {
	return &storeServer{store: store}
}

func (s *storeServer) Start(ctx context.Context, req *api.StoreStartRequest) (*api.StoreStartResponse, error) {
	in, err := instanceFromProto(req.Instance)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	out, err := s.store.Start(ctx, in)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to start sandbox")
	}

	proto, err := instanceToProto(&out)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.StoreStartResponse{Instance: proto}, nil
}

func (s *storeServer) Stop(ctx context.Context, req *api.StoreStopRequest) (*api.StoreStopResponse, error) {
	err := s.store.Stop(ctx, req.ID)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to stop sandbox")
	}

	return &api.StoreStopResponse{}, nil
}

func (s *storeServer) Update(ctx context.Context, req *api.StoreUpdateRequest) (*api.StoreUpdateResponse, error) {
	in, err := instanceFromProto(req.Instance)
	if err != nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrInvalidArgument, "failed to parse request: %v", err)
	}

	out, err := s.store.Update(ctx, in, req.Fields...)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to update sandbox instance %q", req.Instance.ID)
	}

	proto, err := instanceToProto(&out)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.StoreUpdateResponse{Instance: proto}, nil
}

func (s *storeServer) Find(ctx context.Context, req *api.StoreFindRequest) (*api.StoreFindResponse, error) {
	out, err := s.store.Find(ctx, req.ID)
	if err != nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrNotFound, "sandbox instance %q not found: %v", req.ID, err)
	}

	proto, err := instanceToProto(&out)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &api.StoreFindResponse{Info: proto}, nil
}

func (s *storeServer) Status(ctx context.Context, req *api.StoreStatusRequest) (*api.StoreStatusResponse, error) {
	status, err := s.store.Status(ctx, req.ID)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to get status for sandbox instance %q", req.ID)
	}

	proto := statusToProto(status)
	return &api.StoreStatusResponse{Status: proto}, nil
}

func (s *storeServer) List(ctx context.Context, req *api.StoreListRequest) (*api.StoreListResponse, error) {
	list, err := s.store.List(ctx, req.Filters...)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to list sandbox instances")
	}

	resp := &api.StoreListResponse{
		Info: make([]*api.Instance, len(list)),
	}

	for i, instance := range list {
		out, err := instanceToProto(instance)
		if err != nil {
			return nil, errdefs.ToGRPC(err)
		}

		resp.Info[i] = out
	}

	return resp, nil
}

func (s *storeServer) Delete(ctx context.Context, req *api.StoreDeleteRequest) (*api.StoreDeleteResponse, error) {
	err := s.store.Delete(ctx, req.ID)
	if err != nil {
		return nil, errdefs.ToGRPCf(err, "failed to delete sandbox instance %q", req.ID)
	}

	return &api.StoreDeleteResponse{}, nil
}
