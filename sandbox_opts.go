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

	"github.com/containerd/typeurl"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"

	"github.com/containerd/containerd/sandbox"
)

type NewSandboxOpt func(ctx context.Context, client *Client, instance *sandbox.Instance) error

type DelSandboxOpt func(ctx context.Context, client *Client, instance *sandbox.Instance) error

func WithSandboxSpec(spec *sandbox.Spec) NewSandboxOpt {
	return func(ctx context.Context, client *Client, instance *sandbox.Instance) error {
		instance.Spec = spec
		return nil
	}
}

func WithSandboxExtension(name string, ext interface{}) NewSandboxOpt {
	return func(ctx context.Context, client *Client, instance *sandbox.Instance) error {
		if instance.Extensions == nil {
			instance.Extensions = make(map[string]types.Any)
		}

		any, err := typeurl.MarshalAny(ext)
		if err != nil {
			return errors.Wrap(err, "failed to marshal sandbox extension")
		}

		instance.Extensions[name] = *any
		return nil
	}
}

func WithSandboxLabels(labels map[string]string) NewSandboxOpt {
	return func(ctx context.Context, client *Client, opinstance *sandbox.Instance) error {
		if opinstance.Labels == nil {
			opinstance.Labels = make(map[string]string)
		}

		for k, v := range labels {
			opinstance.Labels[k] = v
		}

		return nil
	}
}
