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

	"github.com/containerd/containerd/containers"
)

// WithSandboxID appends a sandbox descriptor to a runtime (a metadata object that runtime implementations can use
// to run containers inside of sandbox instance with the given ID).
func WithSandboxID(name, id string) NewContainerOpts {
	return func(ctx context.Context, client *Client, c *containers.Container) error {
		sandbox, err := client.LoadSandbox(ctx, name, id)
		if err != nil {
			return err
		}

		descriptor, err := sandbox.Descriptor(ctx)
		if err != nil {
			return err
		}

		// TODO: should we use a dedicated field in 'CreateTaskRequest' to pass sandbox descriptor to runtime?
		c.Runtime.Options = descriptor
		return nil
	}
}
