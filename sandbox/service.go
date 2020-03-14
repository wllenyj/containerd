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

type CreateInfo struct {
	// ID uniquely identifies the sandbox ID
	ID string
	// RuntimeSpec is the configuration to use for the sandbox
	RuntimeSpec *runtime.Spec
	// Labels are extra configuration parameters needed to create a sandbox
	Labels map[string]string
	// Extensions stores client-specified metadata
	Extensions map[string]types.Any
}

// State is a sandbox state
type State string

const (
	StateStarting State = "starting"
	StateStarted  State = "started"
	StateReady    State = "ready"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
)

// Status is a runtime info of a sandbox
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
	Extra map[string]string
}

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
	// Annotations is extra information added by implementations during sandbox creation
	Annotations map[string]string
	// Extensions stores client-specified metadata
	Extensions map[string]types.Any
}

// Service interface to be used by sandbox implementations
type Service interface {
	Start(ctx context.Context, info *CreateInfo) (*Info, error)
	Stop(ctx context.Context, id string) error
	Update(ctx context.Context, info *CreateInfo, fieldpaths ...string) error
	Status(ctx context.Context, id string) (Status, error)
	Delete(ctx context.Context, id string) error
}

// Store defines metadata storage to be used by containerd and client interface for sandbox API
type Store interface {
	Start(ctx context.Context, info *CreateInfo) (*Info, error)
	Stop(ctx context.Context, id string) error
	Update(ctx context.Context, info *CreateInfo, fieldpaths ...string) error
	Status(ctx context.Context, id string) (Status, error)
	Info(ctx context.Context, id string) (*Info, error)
	List(ctx context.Context, filter ...string) ([]*Info, error)
	Delete(ctx context.Context, id string) error
}
