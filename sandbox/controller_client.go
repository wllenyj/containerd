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
)

// controllerClient is a client used by containerd to communicate with a sandbox proxy plugin
type controllerClient struct {
	client api.ControllerClient
}

var _ Controller = &controllerClient{}

func NewController(client api.ControllerClient) Controller {
	return &controllerClient{client: client}
}

func (p *controllerClient) Start(ctx context.Context, instance Instance) (Instance, error) {
	inst, err := instanceToProto(&instance)
	if err != nil {
		return Instance{}, err
	}

	resp, err := p.client.Start(ctx, &api.ControllerStartRequest{Instance: inst})
	if err != nil {
		return Instance{}, err
	}

	return instanceFromProto(resp.Instance)
}

func (p *controllerClient) Stop(ctx context.Context, instance Instance) (Instance, error) {
	inst, err := instanceToProto(&instance)
	if err != nil {
		return Instance{}, err
	}

	resp, err := p.client.Stop(ctx, &api.ControllerStopRequest{Instance: inst})
	if err != nil {
		return Instance{}, err
	}

	return instanceFromProto(resp.Instance)
}

func (p *controllerClient) Update(ctx context.Context, instance Instance, fieldpaths ...string) (Instance, error) {
	inst, err := instanceToProto(&instance)
	if err != nil {
		return Instance{}, err
	}

	resp, err := p.client.Update(ctx, &api.ControllerUpdateRequest{
		Instance: inst,
		Fields:   fieldpaths,
	})
	if err != nil {
		return Instance{}, err
	}

	return instanceFromProto(resp.Instance)
}

func (p *controllerClient) Status(ctx context.Context, instance Instance) (Status, error) {
	inst, err := instanceToProto(&instance)
	if err != nil {
		return Status{}, err
	}

	resp, err := p.client.Status(ctx, &api.ControllerStatusRequest{
		Instance: inst,
	})
	if err != nil {
		return Status{}, err
	}

	return statusFromProto(resp.Status), nil
}

func (p *controllerClient) Delete(ctx context.Context, instance Instance) error {
	inst, err := instanceToProto(&instance)
	if err != nil {
		return err
	}

	if _, err = p.client.Delete(ctx, &api.ControllerDeleteRequest{
		Instance: inst,
	}); err != nil {
		return err
	}

	return nil
}
