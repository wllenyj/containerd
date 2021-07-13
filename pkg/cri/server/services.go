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

package server

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
	runtime_alpha "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	criconfig "github.com/containerd/containerd/pkg/cri/config"
	"github.com/containerd/containerd/plugin"
)

// grpcServices are all the grpc services provided by cri containerd.
type grpcServices interface {
	runtime.RuntimeServiceServer
	runtime.ImageServiceServer
}

type grpcAlphaServices interface {
	runtime_alpha.RuntimeServiceServer
	runtime_alpha.ImageServiceServer
}

// CRIPlugin is the interface implement different CRI implementions
type CRIPlugin interface {
	// need implement recover
	Run() error
	// used by Grpc service
	Initialized() bool
	// io.Closer is used by containerd to gracefully stop cri service.
	io.Closer
	grpcServices
}

// CRIService is the interface implement CRI remote service server.
type CRIService interface {
	plugin.Service
	CRIPlugin
}

type criServices struct {
	// config contains all configurations.
	config *criconfig.Config
	// default service
	c *criService
}

// NewCRIServices returns a new instance of CRIService
func NewCRIServices(config criconfig.Config, client *containerd.Client) (CRIService, error) {
	c, err := newCRIService(&config, client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CRI service")
	}
	cs := &criServices{
		config: &config,
		c:      c,
	}
	return cs, nil
}

// Register registers all required services onto a specific grpc server.
// This is used by containerd cri plugin.
func (c *criServices) Register(s *grpc.Server) error {
	return c.register(s)
}

// RegisterTCP register all required services onto a GRPC server on TCP.
// This is used by containerd CRI plugin.
func (c *criServices) RegisterTCP(s *grpc.Server) error {
	if !c.config.DisableTCPService {
		return c.register(s)
	}
	return nil
}

// implement CRIPlugin Initialized interface
func (c *criServices) Initialized() bool {
	return c.c.Initialized()
}

// Run starts the CRI service.
func (c *criServices) Run() error {
	return c.c.Run()
}

// Close stops the CRI service.
func (c *criServices) Close() error {
	return c.c.Close()
}

func (c *criServices) register(s *grpc.Server) error {
	instrumented := newInstrumentedService(c)
	runtime.RegisterRuntimeServiceServer(s, instrumented)
	runtime.RegisterImageServiceServer(s, instrumented)
	instrumentedAlpha := newInstrumentedAlphaService(c)
	runtime_alpha.RegisterRuntimeServiceServer(s, instrumentedAlpha)
	runtime_alpha.RegisterImageServiceServer(s, instrumentedAlpha)
	return nil
}

func (c *criServices) RunPodSandbox(ctx context.Context, r *runtime.RunPodSandboxRequest) (resp *runtime.RunPodSandboxResponse, retErr error) {
	return c.c.RunPodSandbox(ctx, r)
}

func (c *criServices) ListPodSandbox(ctx context.Context, r *runtime.ListPodSandboxRequest) (resp *runtime.ListPodSandboxResponse, retErr error) {
	return c.c.ListPodSandbox(ctx, r)
}

func (c *criServices) PodSandboxStatus(ctx context.Context, r *runtime.PodSandboxStatusRequest) (resp *runtime.PodSandboxStatusResponse, retErr error) {
	return c.c.PodSandboxStatus(ctx, r)
}

func (c *criServices) StopPodSandbox(ctx context.Context, r *runtime.StopPodSandboxRequest) (_ *runtime.StopPodSandboxResponse, err error) {
	return c.c.StopPodSandbox(ctx, r)
}

func (c *criServices) RemovePodSandbox(ctx context.Context, r *runtime.RemovePodSandboxRequest) (resp *runtime.RemovePodSandboxResponse, retErr error) {
	return c.c.RemovePodSandbox(ctx, r)
}

func (c *criServices) PortForward(ctx context.Context, r *runtime.PortForwardRequest) (res *runtime.PortForwardResponse, err error) {
	return c.c.PortForward(ctx, r)
}

func (c *criServices) CreateContainer(ctx context.Context, r *runtime.CreateContainerRequest) (resp *runtime.CreateContainerResponse, retErr error) {
	return c.c.CreateContainer(ctx, r)
}

func (c *criServices) StartContainer(ctx context.Context, r *runtime.StartContainerRequest) (_ *runtime.StartContainerResponse, err error) {
	return c.c.StartContainer(ctx, r)
}

func (c *criServices) ListContainers(ctx context.Context, r *runtime.ListContainersRequest) (resp *runtime.ListContainersResponse, retErr error) {
	return c.c.ListContainers(ctx, r)
}

func (c *criServices) ContainerStatus(ctx context.Context, r *runtime.ContainerStatusRequest) (res *runtime.ContainerStatusResponse, err error) {
	return c.c.ContainerStatus(ctx, r)
}

func (c *criServices) StopContainer(ctx context.Context, r *runtime.StopContainerRequest) (res *runtime.StopContainerResponse, err error) {
	return c.c.StopContainer(ctx, r)
}

func (c *criServices) RemoveContainer(ctx context.Context, r *runtime.RemoveContainerRequest) (resp *runtime.RemoveContainerResponse, retErr error) {
	return c.c.RemoveContainer(ctx, r)
}

func (c *criServices) ExecSync(ctx context.Context, r *runtime.ExecSyncRequest) (res *runtime.ExecSyncResponse, err error) {
	return c.c.ExecSync(ctx, r)
}

func (c *criServices) Exec(ctx context.Context, r *runtime.ExecRequest) (res *runtime.ExecResponse, err error) {
	return c.c.Exec(ctx, r)
}

func (c *criServices) Attach(ctx context.Context, r *runtime.AttachRequest) (res *runtime.AttachResponse, err error) {
	return c.c.Attach(ctx, r)
}

func (c *criServices) UpdateContainerResources(ctx context.Context, r *runtime.UpdateContainerResourcesRequest) (res *runtime.UpdateContainerResourcesResponse, err error) {
	return c.c.UpdateContainerResources(ctx, r)
}

func (c *criServices) PullImage(ctx context.Context, r *runtime.PullImageRequest) (res *runtime.PullImageResponse, err error) {
	return c.c.PullImage(ctx, r)
}

func (c *criServices) ListImages(ctx context.Context, r *runtime.ListImagesRequest) (res *runtime.ListImagesResponse, err error) {
	return c.c.ListImages(ctx, r)
}

func (c *criServices) ImageStatus(ctx context.Context, r *runtime.ImageStatusRequest) (res *runtime.ImageStatusResponse, err error) {
	return c.c.ImageStatus(ctx, r)
}

func (c *criServices) RemoveImage(ctx context.Context, r *runtime.RemoveImageRequest) (_ *runtime.RemoveImageResponse, err error) {
	return c.c.RemoveImage(ctx, r)
}

func (c *criServices) ImageFsInfo(ctx context.Context, r *runtime.ImageFsInfoRequest) (res *runtime.ImageFsInfoResponse, err error) {
	return c.c.ImageFsInfo(ctx, r)
}

func (c *criServices) ContainerStats(ctx context.Context, r *runtime.ContainerStatsRequest) (res *runtime.ContainerStatsResponse, err error) {
	return c.c.ContainerStats(ctx, r)
}

func (c *criServices) ListContainerStats(ctx context.Context, r *runtime.ListContainerStatsRequest) (resp *runtime.ListContainerStatsResponse, retErr error) {
	return c.c.ListContainerStats(ctx, r)
}

func (c *criServices) Status(ctx context.Context, r *runtime.StatusRequest) (res *runtime.StatusResponse, err error) {
	return c.c.Status(ctx, r)
}

func (c *criServices) Version(ctx context.Context, r *runtime.VersionRequest) (res *runtime.VersionResponse, err error) {
	return c.c.Version(ctx, r)
}

func (c *criServices) UpdateRuntimeConfig(ctx context.Context, r *runtime.UpdateRuntimeConfigRequest) (res *runtime.UpdateRuntimeConfigResponse, err error) {
	return c.c.UpdateRuntimeConfig(ctx, r)
}

func (c *criServices) ReopenContainerLog(ctx context.Context, r *runtime.ReopenContainerLogRequest) (res *runtime.ReopenContainerLogResponse, err error) {
	return c.c.ReopenContainerLog(ctx, r)
}
