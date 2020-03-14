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

	"google.golang.org/grpc"

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/sandbox"
)

type remoteSandboxClient struct {
	name   string
	client api.StoreClient
}

var _ sandbox.Store = &remoteSandboxClient{}

func newSandboxClient(conn *grpc.ClientConn, name string) *remoteSandboxClient {
	return &remoteSandboxClient{
		name:   name,
		client: api.NewStoreClient(conn),
	}
}

func (r *remoteSandboxClient) Start(ctx context.Context, createInfo *sandbox.CreateOpts) (*sandbox.Info, error) {
	req := api.StartSandboxRequest{
		Name:       r.name,
		ID:         createInfo.ID,
		Labels:     createInfo.Labels,
		Extensions: createInfo.Extensions,
	}

	specAny, err := sandbox.SpecToAny(createInfo.RuntimeSpec)
	if err != nil {
		return nil, err
	}

	req.RuntimeSpec = specAny

	resp, err := r.client.Start(ctx, &req)
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	info := sandbox.Info{
		ID:         resp.Info.ID,
		Labels:     resp.Info.Labels,
		CreatedAt:  resp.Info.CreatedAt,
		UpdatedAt:  resp.Info.UpdatedAt,
		Descriptor: resp.Info.Descriptor_,
		Extensions: resp.Info.Extensions,
	}

	info.RuntimeSpec, err = sandbox.AnyToSpec(resp.Info.RuntimeSpec)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func (r *remoteSandboxClient) Stop(ctx context.Context, id string) error {
	req := api.StopSandboxRequest{
		Name: r.name,
		ID:   id,
	}

	_, err := r.client.Stop(ctx, &req)
	if err != nil {
		return errdefs.FromGRPC(err)
	}

	return err
}

func (r *remoteSandboxClient) Update(ctx context.Context, info *sandbox.CreateOpts, fieldpaths ...string) error {
	req := api.UpdateSandboxRequest{
		Name:       r.name,
		ID:         info.ID,
		Fields:     fieldpaths,
		Labels:     info.Labels,
		Extensions: info.Extensions,
	}

	spec, err := sandbox.SpecToAny(info.RuntimeSpec)
	if err != nil {
		return err
	}

	req.RuntimeSpec = spec

	if _, err := r.client.Update(ctx, &req); err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (r *remoteSandboxClient) Status(ctx context.Context, id string) (sandbox.Status, error) {
	req := api.StatusSandboxRequest{
		Name: r.name,
		ID:   id,
	}

	resp, err := r.client.Status(ctx, &req)
	if err != nil {
		return sandbox.Status{}, errdefs.FromGRPC(err)
	}

	return sandbox.Status{
		ID:      resp.ID,
		PID:     resp.Pid,
		State:   sandbox.State(resp.State),
		Version: resp.Version,
		Extra:   resp.Extra,
	}, nil
}

func (r *remoteSandboxClient) Info(ctx context.Context, id string) (*sandbox.Info, error) {
	req := api.InfoSandboxRequest{
		Name: r.name,
		ID:   id,
	}

	resp, err := r.client.Info(ctx, &req)
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	return r.makeInfo(resp.Info)
}

func (r *remoteSandboxClient) List(ctx context.Context, filter ...string) ([]*sandbox.Info, error) {
	req := api.ListSandboxRequest{
		Name:    r.name,
		Filters: filter,
	}

	resp, err := r.client.List(ctx, &req)
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	out := make([]*sandbox.Info, len(resp.Info))
	for i, p := range resp.Info {
		out[i], err = r.makeInfo(p)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (r *remoteSandboxClient) Delete(ctx context.Context, id string) error {
	req := api.DeleteSandboxRequest{
		Name: r.name,
		ID:   id,
	}

	if _, err := r.client.Delete(ctx, &req); err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

func (remoteSandboxClient) makeInfo(in *api.Info) (*sandbox.Info, error) {
	info := sandbox.Info{
		ID:         in.ID,
		Labels:     in.Labels,
		CreatedAt:  in.CreatedAt,
		UpdatedAt:  in.UpdatedAt,
		Descriptor: in.Descriptor_,
		Extensions: in.Extensions,
	}

	spec, err := sandbox.AnyToSpec(in.RuntimeSpec)
	if err != nil {
		return nil, err
	}

	info.RuntimeSpec = spec
	return &info, nil
}
