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
	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/typeurl"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
)

func instanceToProto(i *Instance) (*api.Instance, error) {
	if i == nil {
		return nil, errdefs.ErrInvalidArgument
	}

	out := &api.Instance{
		ID:         i.ID,
		Labels:     i.Labels,
		CreatedAt:  i.CreatedAt,
		UpdatedAt:  i.UpdatedAt,
		Extensions: i.Extensions,
	}

	if i.Spec != nil {
		spec, err := toAny(i.Spec)
		if err != nil {
			return nil, err
		}

		out.Spec = spec
	}

	return out, nil
}

func instanceFromProto(proto *api.Instance) (Instance, error) {
	if proto == nil {
		return Instance{}, errdefs.ErrInvalidArgument
	}

	out := Instance{
		ID:         proto.ID,
		Labels:     proto.Labels,
		CreatedAt:  proto.CreatedAt,
		UpdatedAt:  proto.UpdatedAt,
		Extensions: proto.Extensions,
	}

	if proto.Spec != nil {
		spec, err := anyToSpec(proto.Spec)
		if err != nil {
			return Instance{}, err
		}

		out.Spec = spec
	}

	return out, nil
}

func statusFromProto(proto *api.Status) Status {
	return Status{
		ID:      proto.ID,
		PID:     proto.Pid,
		State:   State(proto.State),
		Version: proto.Version,
		Extra:   proto.Extra,
	}
}

func statusToProto(status Status) *api.Status {
	return &api.Status{
		ID:      status.ID,
		Pid:     status.PID,
		State:   string(status.State),
		Version: status.Version,
		Extra:   status.Extra,
	}
}

func toAny(v interface{}) (*types.Any, error) {
	any, err := typeurl.MarshalAny(v)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal struct to any")
	}

	return any, nil
}

func fromAny(any *types.Any) (interface{}, error) {
	if any == nil {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "any can't be empty")
	}

	raw, err := typeurl.UnmarshalAny(any)
	if err != nil {
		return nil, errors.Wrapf(errdefs.ErrInvalidArgument, "failed to unmarshal any: %v", err)
	}

	return raw, nil
}

func anyToSpec(any *types.Any) (*Spec, error) {
	raw, err := fromAny(any)
	if err != nil {
		return nil, err
	}

	spec, ok := raw.(*Spec)
	if !ok {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "unexpected sandbox spec structure")
	}

	return spec, nil
}
