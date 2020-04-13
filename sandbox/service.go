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

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/typeurl"
	"github.com/gogo/protobuf/types"
	runtime "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// Spec is a specification to use for creating sandbox instances.
// TODO: this should be a "sandbox-spec" instead of "runtime-spec", using runtime one as a proof of concept.
type Spec = runtime.Spec

// Descriptor is a metadata object to be passed to runtime implementations in order to run containers inside of
//a sandbox instance.
type Descriptor *types.Any

// Controller interface to be implemented by sandbox proxy plugins to manage sandbox instances.
type Controller interface {
	// Start creates and runs a new sandbox instance.
	// Clients may configure sandbox environment via runtime spec, labels, and extensions.
	Start(ctx context.Context, instance Instance) (Instance, error)
	// Stop stops a sandbox instance identified by id.
	Stop(ctx context.Context, instance Instance) (Instance, error)
	// Update changes configuration of already running sandbox instance.
	Update(ctx context.Context, instance Instance, fieldpaths ...string) (Instance, error)
	// Status returns a runtime status of a sandbox status identified by id.
	Status(ctx context.Context, instance Instance) (Status, error)
	// Delete completely deletes sandbox instance from metadata store.
	Delete(ctx context.Context, instance Instance) error
}

// Store defines metadata storage and sandbox API interface for containerd clients.
// metadata packages proxies client calls to a proxy plugins that implement `Controller` interface.
type Store interface {
	Start(ctx context.Context, instance Instance) (Instance, error)
	Stop(ctx context.Context, id string) error
	Update(ctx context.Context, instance Instance, fieldpaths ...string) (Instance, error)
	Status(ctx context.Context, id string) (Status, error)
	Find(ctx context.Context, id string) (Instance, error)
	List(ctx context.Context, filter ...string) ([]*Instance, error)
	Delete(ctx context.Context, id string) error
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

// Instance is a sandbox instance object to be passed to controller plugin
type Instance struct {
	// ID uniquely identifies the sandbox ID
	ID string
	// Spec is the configuration that was used to create this sandbox instance
	Spec *Spec
	// Labels are extra configuration parameters used as input to create a sandbox
	Labels map[string]string
	// CreatedAt is the time at which the sandbox was created.
	CreatedAt time.Time
	// UpdatedAt is the time at which the sandbox was updated.
	UpdatedAt time.Time
	// Extensions store client-specified metadata
	Extensions map[string]types.Any
}

func (i *Instance) ToDescriptor() (Descriptor, error) {
	proto, err := instanceToProto(i)
	if err != nil {
		return nil, err
	}

	d := api.Descriptor{Instance: proto}
	return toAny(d)
}

// AddExtension appends an extension to sandbox instance's extension list.
// Extension type must be registered within `typeurl` package.
func (i *Instance) AddExtension(name string, ext interface{}) error {
	if i.Extensions == nil {
		i.Extensions = make(map[string]types.Any)
	}

	any, err := typeurl.MarshalAny(ext)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal extension %q", name)
	}

	i.Extensions[name] = *any
	return nil
}

// Extension unmarshals instance's extension by 'name'
func (i *Instance) Extension(name string, out interface{}) error {
	any, ok := i.Extensions[name]
	if !ok {
		return errors.Wrapf(errdefs.ErrNotFound, "extension %q not found", name)
	}

	return typeurl.UnmarshalTo(&any, out)
}
