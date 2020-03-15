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
	"time"

	"github.com/gogo/protobuf/types"
	runtime "github.com/opencontainers/runtime-spec/specs-go"
)

const DescriptorExtensionName = "io.containerd.ext/sandbox/descriptor"

// Descriptor is an implementation agnostic connection handle returned from `Start` routine.
// This object should contain connection information to be passed to shim implementation, so
// they can communicate with sandboxes (for instance - socket path, API address, ports, etc).
// When running a container (but after starting a sandbox), this descriptor can be passed along
// with other container configuration via `containerd.WithSandboxID` and `containerd.WithSandboxDescriptor`.
type Descriptor = types.Any

// Controller interface to be implemented by sandbox proxy plugins to manage sandbox instances.
type Controller interface {
	// Start creates and runs a new sandbox instance.
	// Clients may configure sandbox environment via runtime spec, labels, and extensions.
	// Returned `Descriptor` represents an implementation specific connection object that
	// will be passed to a runtime for communcating with sandbox instance.
	Start(ctx context.Context, info *CreateOpts) (Descriptor, error)
	// Stop stops a sandbox instance identified by id.
	Stop(ctx context.Context, id string) error
	// Update changes configuration of already running sandbox instance.
	Update(ctx context.Context, info *CreateOpts, fieldpaths ...string) error
	// Status returns a runtime status of a sandbox status identified by id.
	Status(ctx context.Context, id string) (Status, error)
	// Delete completely deletes sandbox instance from metadata store.
	Delete(ctx context.Context, id string) error
}

// Store defines metadata storage and sandbox API interface for containerd clients.
// metadata packages proxies client calls to a proxy plugins that implement `Controller` interface.
type Store interface {
	Start(ctx context.Context, info *CreateOpts) (*Info, error)
	Stop(ctx context.Context, id string) error
	Update(ctx context.Context, info *CreateOpts, fieldpaths ...string) error
	Status(ctx context.Context, id string) (Status, error)
	Info(ctx context.Context, id string) (*Info, error)
	List(ctx context.Context, filter ...string) ([]*Info, error)
	Delete(ctx context.Context, id string) error
}

// CreateOpts represents sandbox creation parameters
type CreateOpts struct {
	// ID uniquely identifies the sandbox ID
	ID string
	// RuntimeSpec is the configuration to use for the sandbox
	RuntimeSpec *runtime.Spec
	// Labels are extra configuration parameters needed to create a sandbox
	Labels map[string]string
	// Extensions stores client-specified metadata
	Extensions map[string]types.Any
}

// State is current state of a sandbox (reported by `Status` call)
type State string

const (
	StateStarting State = "starting"
	StateStarted  State = "started"
	StateReady    State = "ready"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
)

// Status is a structure describing current sandbox instance status
type Status struct {
	// ID is a sandbox identifier
	ID string
	// PID is a process ID of the sandbox process (if any)
	PID uint32
	// State is current sandbox state
	State State
	// Version represents sandbox version
	Version string
	// Extra is additional information that might be included by sandbox implementations
	Extra map[string]types.Any
}

// Info describes a sandbox instance
type Info struct {
	// ID uniquely identifies the sandbox ID
	ID string
	// RuntimeSpec is the configuration to use for the sandbox
	RuntimeSpec *runtime.Spec
	// Labels are extra configuration parameters used as input to create a sandbox
	Labels map[string]string
	// CreatedAt is the time at which the sandbox was created.
	CreatedAt time.Time
	// UpdatedAt is the time at which the sandbox was updated.
	UpdatedAt time.Time
	// Descriptor is an object that describes how to communicate with a sandbox instance from a shim
	Descriptor Descriptor
	// Extensions stores client-specified metadata
	Extensions map[string]types.Any
}
