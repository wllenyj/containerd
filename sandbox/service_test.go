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
	"reflect"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestInstanceExtension(t *testing.T) {
	var (
		instance = Instance{}
		ext      = &specs.Spec{Version: "123"}
	)

	err := instance.AddExtension("test", ext)
	if err != nil {
		t.Fatal(err)
	}

	out := &specs.Spec{}
	err = instance.Extension("test", out)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ext, out) {
		t.Fatalf("types are not equal %+v != %+v", ext, out)
	}
}
