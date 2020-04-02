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

	"google.golang.org/grpc"

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
)

type remoteStoreClient struct {
	name   string
	client api.StoreClient
}

var _ Store = &remoteStoreClient{}

func NewRemoteStore(conn *grpc.ClientConn, name string) Store {
	return &remoteStoreClient{
		name:   name,
		client: api.NewStoreClient(conn),
	}
}

func (r *remoteStoreClient) Start(ctx context.Context, instance Instance) (Instance, error) {
	proto, err := instanceToProto(&instance)
	if err != nil {
		return Instance{}, err
	}

	req := api.StoreStartRequest{
		Name:     r.name,
		Instance: proto,
	}

	resp, err := r.client.Start(ctx, &req)
	if err != nil {
		return Instance{}, errdefs.FromGRPC(err)
	}

	return instanceFromProto(resp.Instance)
}

func (r *remoteStoreClient) Stop(ctx context.Context, id string) error {
	if _, err := r.client.Stop(ctx, &api.StoreStopRequest{
		Name: r.name,
		ID:   id,
	}); err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (r *remoteStoreClient) Update(ctx context.Context, instance Instance, fieldpaths ...string) (Instance, error) {
	proto, err := instanceToProto(&instance)
	if err != nil {
		return Instance{}, err
	}

	req := api.StoreUpdateRequest{
		Name:     r.name,
		Instance: proto,
		Fields:   fieldpaths,
	}

	resp, err := r.client.Update(ctx, &req)
	if err != nil {
		return Instance{}, errdefs.FromGRPC(err)
	}

	return instanceFromProto(resp.Instance)
}

func (r *remoteStoreClient) Status(ctx context.Context, id string) (Status, error) {
	resp, err := r.client.Status(ctx, &api.StoreStatusRequest{
		Name: r.name,
		ID:   id,
	})

	if err != nil {
		return Status{}, errdefs.FromGRPC(err)
	}

	status := statusFromProto(resp.Status)
	return status, nil
}

func (r *remoteStoreClient) Find(ctx context.Context, id string) (Instance, error) {
	resp, err := r.client.Find(ctx, &api.StoreFindRequest{
		Name: r.name,
		ID:   id,
	})

	if err != nil {
		return Instance{}, errdefs.FromGRPC(err)
	}

	return instanceFromProto(resp.Info)
}

func (r *remoteStoreClient) List(ctx context.Context, filter ...string) ([]*Instance, error) {
	resp, err := r.client.List(ctx, &api.StoreListRequest{
		Name:    r.name,
		Filters: filter,
	})

	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	list := make([]*Instance, len(resp.Info))
	for i, item := range resp.Info {
		inst, err := instanceFromProto(item)
		if err != nil {
			return nil, err
		}

		list[i] = &inst
	}

	return list, nil
}

func (r *remoteStoreClient) Delete(ctx context.Context, id string) error {
	if _, err := r.client.Delete(ctx, &api.StoreDeleteRequest{
		Name: r.name,
		ID:   id,
	}); err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}
