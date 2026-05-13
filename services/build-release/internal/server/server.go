package server

import (
	"context"
	"errors"
	"fmt"

	buildreleasev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/buildrelease/v1"
	"github.com/quarkloop/services/build-release/pkg/buildrelease"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	buildreleasev1.UnimplementedBuildReleaseServiceServer
	runner *buildrelease.Runner
}

func New(runner *buildrelease.Runner) (*Server, error) {
	if runner == nil {
		return nil, fmt.Errorf("build-release runner is required")
	}
	return &Server{runner: runner}, nil
}

func (s *Server) Release(ctx context.Context, req *buildreleasev1.ReleaseRequest) (*buildreleasev1.ReleaseResponse, error) {
	result, err := s.runner.Release(ctx, buildrelease.ReleaseRequest{
		WorkingDir:  req.GetWorkingDir(),
		ConfigPath:  req.GetConfigPath(),
		Version:     req.GetVersion(),
		Parallelism: int(req.GetParallelism()),
		SkipTests:   req.GetSkipTests(),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &buildreleasev1.ReleaseResponse{
		Success:    result.Success,
		Message:    result.Message,
		Version:    result.Version,
		ReleaseDir: result.ReleaseDir,
		Artifacts:  artifactsToProto(result.Artifacts),
	}, nil
}

func (s *Server) DryRun(ctx context.Context, req *buildreleasev1.DryRunRequest) (*buildreleasev1.DryRunResponse, error) {
	result, err := s.runner.DryRun(ctx, buildrelease.DryRunRequest{
		WorkingDir:  req.GetWorkingDir(),
		ConfigPath:  req.GetConfigPath(),
		Version:     req.GetVersion(),
		Parallelism: int(req.GetParallelism()),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &buildreleasev1.DryRunResponse{
		Version: result.Version,
		Planned: artifactsToProto(result.Planned),
	}, nil
}

func (s *Server) Init(ctx context.Context, req *buildreleasev1.InitRequest) (*buildreleasev1.InitResponse, error) {
	result, err := s.runner.Init(ctx, buildrelease.InitRequest{
		WorkingDir: req.GetWorkingDir(),
		Overwrite:  req.GetOverwrite(),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &buildreleasev1.InitResponse{
		ConfigPath: result.ConfigPath,
		Created:    result.Created,
	}, nil
}

func artifactsToProto(in []buildrelease.Artifact) []*buildreleasev1.Artifact {
	out := make([]*buildreleasev1.Artifact, 0, len(in))
	for _, artifact := range in {
		out = append(out, &buildreleasev1.Artifact{
			BuildName:      artifact.BuildName,
			Os:             artifact.Target.OS,
			Arch:           artifact.Target.Arch,
			Arm:            artifact.Target.ARM,
			Filename:       artifact.Filename,
			ArchiveName:    artifact.ArchiveName,
			Checksum:       artifact.Checksum,
			Size:           artifact.Size,
			DurationMillis: artifact.Duration.Milliseconds(),
			Error:          artifact.Error,
		})
	}
	return out
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	default:
		return status.Error(codes.InvalidArgument, err.Error())
	}
}
