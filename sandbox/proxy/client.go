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
	"time"

	"github.com/containerd/typeurl"
	"github.com/pkg/errors"

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/sandbox"
)

type proxyClient struct {
	name   string
	client api.SandboxClient
}

func NewClient(client api.SandboxClient, name string) sandbox.Service {
	return &proxyClient{client: client, name: name}
}

func (p *proxyClient) Start(ctx context.Context, info *sandbox.CreateInfo) (*sandbox.Info, error) {
	specAny, err := typeurl.MarshalAny(info.RuntimeSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal runtime spec")
	}

	req := &api.StartSandboxRequest{
		Name:        p.name,
		ID:          info.ID,
		Labels:      info.Labels,
		RuntimeSpec: specAny,
		Extensions:  info.Extensions,
	}

	resp, err := p.client.Start(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start sandbox")
	}

	return &sandbox.Info{
		ID:          resp.ID,
		RuntimeSpec: info.RuntimeSpec,
		Labels:      req.Labels,
		Extensions:  req.Extensions,
		CreatedAt:   time.Time{},
		UpdatedAt:   time.Time{},
		Annotations: nil,
	}, nil
}

func (p *proxyClient) Stop(ctx context.Context, id string) error {
	_, err := p.client.Stop(ctx, &api.StopSandboxRequest{
		Name: p.name,
		ID:   id,
	})

	if err != nil {
		return errors.Wrap(err, "failed to stop sandbox")
	}

	return nil
}

func (p *proxyClient) Update(ctx context.Context, info *sandbox.CreateInfo, fieldpaths ...string) error {
	req := api.UpdateSandboxRequest{
		Name:       p.name,
		ID:         info.ID,
		Fields:     fieldpaths,
		Labels:     info.Labels,
		Extensions: info.Extensions,
	}

	if info.RuntimeSpec != nil {
		any, err := typeurl.MarshalAny(info.RuntimeSpec)
		if err != nil {
			return errors.Wrap(err, "failed to marshal runtime spec")
		}

		req.RuntimeSpec = any
	}

	_, err := p.client.Update(ctx, &req)
	if err != nil {
		return errors.Wrap(err, "failed to update sandbox")
	}

	return nil
}

func (p *proxyClient) Status(ctx context.Context, id string) (sandbox.Status, error) {
	resp, err := p.client.Status(ctx, &api.StatusSandboxRequest{
		Name: p.name,
		ID:   id,
	})

	if err != nil {
		return sandbox.Status{}, errors.Wrapf(err, "failed to get sandbox %q status", id)
	}

	return sandbox.Status{
		ID:      resp.ID,
		PID:     resp.Pid,
		State:   sandbox.State(resp.State),
		Version: resp.Version,
		Extra:   resp.Extra,
	}, nil
}

func (p *proxyClient) Delete(ctx context.Context, id string) error {
	_, err := p.client.Delete(ctx, &api.DeleteSandboxRequest{
		Name: p.name,
		ID:   id,
	})

	if err != nil {
		return errors.Wrapf(err, "failed to delete sandbox %q", id)
	}

	return nil
}
