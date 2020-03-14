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
	"github.com/containerd/typeurl"
	"github.com/gogo/protobuf/types"
	runtime "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	"github.com/containerd/containerd/errdefs"
)

func SpecToAny(spec *runtime.Spec) (*types.Any, error) {
	any, err := typeurl.MarshalAny(spec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal runtime spec")
	}

	return any, nil
}

func AnyToSpec(any *types.Any) (*runtime.Spec, error) {
	if any == nil {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "empty runtime spec")
	}

	raw, err := typeurl.UnmarshalAny(any)
	if err != nil {
		return nil, errors.Wrapf(errdefs.ErrInvalidArgument, "failed to unmarshal runtime spec: %v", err)
	}

	spec, ok := raw.(*runtime.Spec)
	if !ok {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "unexpected runtime spec structure")
	}

	return spec, nil
}
