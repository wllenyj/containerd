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

	"github.com/gogo/protobuf/types"
)

// Controller is an interface to manage a runtime sandbox instance (runtimes's SandboxManager)
// SandboxRuntime is responsible for the sandbox instances lifecycle management.
// When running the traditional containerd shim, the workflow looks as follows:
// For each new task we're about to run:
// 	1. Invoke `shim_binary --start` to obtain `TaskService` address (printed in stdout)
// 	2. Call TaskService.RunContainer(id=1)
// 	3. Exec `shim_binary --delete` to stop shim
// 	4. Exec `shim_binary --start` again to obtain another `TaskService` address
// 	5. TaskService.RunContainer(id=2)
// 	6. Exec `shim_binary --delete` to stop shim
//
// When running in sandbox mode, shim must implement `SandboxService`. In sandbox mode shim lifetimes are managed manually.
// 	1. Client calls `client.Controller.Start()` to launch new shim and create sandbox process
// 	2. Run containers with `shim.TaskService.RunContainer(id=1)` and another one `shim.TaskService.RunContainer(id=2)`
//	3. ... usual container lifecycle calls to `shim.TaskService`
// 	4. Client calls shim to stop the VM with `client.SandboxService.Shutdown()`
//  5. Shim implementation will perform cleanup similar to regular task service (e.g. shutdown, clean, and `shim_binary --delete`)
type Controller interface {
	// Start will start new sandbox instance.
	// containerd will run new shim runtime instance and will invoke Start to create a sandbox process.
	// This routine must be invoked before scheduling containers on this instance.
	// Once started clients may run containers via Task service (additionally specifying sandbox id the container will belong to).
	Start(ctx context.Context, sandboxID string) error
	// Shutdown deletes and cleans all tasks and sandbox instance.
	Shutdown(ctx context.Context, sandboxID string) error
	// Pause will freeze running sandbox instance.
	// Shim implementations may return ErrNotImplemented if this is out of scope of a given sandbox.
	Pause(ctx context.Context, sandboxID string) error
	// Resume will unfreeze previously paused sandbox instance
	Resume(ctx context.Context, sandboxID string) error
	// Ping is a lightweight API call to check whether sandbox instance is still alive.
	// This should not involve any complex logic and containerd will not debug log it as it might be called quite often.
	Ping(ctx context.Context, sandboxID string) error
	// Status will query sandbox process status
	Status(ctx context.Context, sandboxID string) (*types.Any, error)
}
