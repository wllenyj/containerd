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

package erofs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/plugin"
	"github.com/davecgh/go-spew/spew"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func init() {
	plugin.Register(&plugin.Registration{
		Type: plugin.DiffPlugin,
		ID:   "linux-erofs",
		Requires: []plugin.Type{
			plugin.MetadataPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			md, err := ic.Get(plugin.MetadataPlugin)
			if err != nil {
				return nil, err
			}

			ic.Meta.Platforms = append(ic.Meta.Platforms, platforms.DefaultSpec())
			cs := md.(*metadata.DB).ContentStore()

			return &diffPlugin{
				store: cs,
			}, nil
		},
	})
}

//type ErofsApplier interface {
//	diff.Applier
//	diff.Comparer
//}

type diffPlugin struct {
	store content.Provider
}

//func NewErofsDiffer(store content.Store) (

func (s *diffPlugin) Compare(ctx context.Context, lower, upper []mount.Mount, opts ...diff.Opt) (d ocispec.Descriptor, err error) {
	log.G(ctx).Infof("======================> Compare")
	return ocispec.Descriptor{}, errdefs.ErrNotImplemented
}

func (s *diffPlugin) Apply(ctx context.Context, desc ocispec.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (d ocispec.Descriptor, err error) {
	log.G(ctx).Infof("======================> Apply: %s", spew.Sdump(mounts))
	var emptyDesc = ocispec.Descriptor{}
	if len(mounts) != 1 || !hasErofsFlags(mounts[0].Options) {
		return emptyDesc, errdefs.ErrNotImplemented
	}
	log.G(ctx).Infof("======================> Apply: ok")

	var config diff.ApplyConfig
	for _, o := range opts {
		if err := o(ctx, desc, &config); err != nil {
			return emptyDesc, fmt.Errorf("failed to apply config opt: %w", err)
		}
	}

	ra, err := s.store.ReaderAt(ctx, desc)
	if err != nil {
		return emptyDesc, fmt.Errorf("failed to get reader from content store: %w", err)
	}
	defer ra.Close()

	var processors []diff.StreamProcessor
	processor := diff.NewProcessorChain(desc.MediaType, content.NewReader(ra))
	processors = append(processors, processor)
	for {
		if processor, err = diff.GetProcessor(ctx, processor, config.ProcessorPayloads); err != nil {
			return emptyDesc, fmt.Errorf("failed to get stream processor for %s: %w", desc.MediaType, err)
		}
		processors = append(processors, processor)
		if processor.MediaType() == ocispec.MediaTypeImageLayer {
			break
		}
	}
	defer processor.Close()

	digester := digest.Canonical.Digester()
	rc := &readCounter{
		r: io.TeeReader(processor, digester.Hash()),
	}

	// TODO: Need temp Mount?
	//if err = mount.WithTempMount(ctx, mounts, func(root string) error {
	if err := createTar(ctx, mounts[0].Source, rc); err != nil {
		return emptyDesc, err
	}
	if err := createMeta(ctx, getBlobId(string(desc.Digest)), mounts[0].Source); err != nil {
		return emptyDesc, err
	}
	//}); err != nil {
	//	return emptyDesc, err
	//}

	// Read any trailing data
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return emptyDesc, err
	}

	for _, p := range processors {
		if ep, ok := p.(interface {
			Err() error
		}); ok {
			if err := ep.Err(); err != nil {
				return emptyDesc, err
			}
		}
	}

	return ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Size:      rc.c,
		Digest:    digester.Digest(),
	}, nil
}

func getBlobId(desc string) string {
	if strings.HasPrefix(desc, "sha256:") && len(desc) == 71 {
		return desc[7:]
	}
	return desc
}

type readCounter struct {
	r io.Reader
	c int64
}

func (rc *readCounter) Read(p []byte) (n int, err error) {
	n, err = rc.r.Read(p)
	rc.c += int64(n)
	return
}

func createTar(ctx context.Context, root string, reader io.Reader) error {
	path := getTarPath(root)
	file, err := archive.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, err = archive.CopyBuffered(ctx, file, reader)
	if err1 := file.Close(); err == nil {
		err = err1
	}
	if err != nil {
		return err
	}
	return err
}

func createMeta(ctx context.Context, blobId, root string) error {
	path := DetectErofsUtils()
	if path == "" {
		return fmt.Errorf("failed to find erofs utils.")
	}
	return ExecCommand(ctx, path, "create", "--blob-id", blobId, "-B", getMetadataPath(root), "-t", "tar-tarfs", "-D", root, getTarPath(root))
}

func ExecCommand(ctx context.Context, name string, arg ...string) error {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Env = os.Environ()

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to run command: %s: %s", err, errBuf.String())
	}
	return nil
}

func getTarPath(root string) string {
	return filepath.Join(root, "tar")
}

func getMetadataPath(root string) string {
	return filepath.Join(root, "metadata")
}

func hasErofsFlags(opts []string) bool {
	for _, o := range opts {
		if o == "erofs" {
			return true
		}
	}
	return false
}

func DetectErofsUtils() string {
	path, err := exec.LookPath("nydus-image")
	if err != nil {
		log.L.WithError(err).Debug("nydus-img not found")
		return ""
	}
	return path
}
