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

	"github.com/pkg/errors"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/sandbox"
)

func WithSandboxDescriptor(descriptor sandbox.Descriptor) NewContainerOpts {
	return func(ctx context.Context, client *Client, c *containers.Container) error {
		c.Extensions[sandbox.DescriptorExtensionName] = descriptor
		return nil
	}
}

func WithSandboxID(name, id string) NewContainerOpts {
	return func(ctx context.Context, client *Client, c *containers.Container) error {
		store := client.SandboxService(name)
		// Retrieve descriptor that shim can use for communicating with a sandbox intance
		info, err := store.Info(ctx, id)
		if err != nil {
			return errors.Wrap(err, "failed to query sandbox descriptor")
		}

		c.Extensions[sandbox.DescriptorExtensionName] = info.Descriptor
		return nil
	}
}
