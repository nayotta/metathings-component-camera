package camera_service

import (
	"context"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	driver "github.com/nayotta/metathings-component-camera/pkg/camera/driver"
	component "github.com/nayotta/metathings/pkg/component"
)

type CameraService struct {
	module *component.Module
	driver driver.CameraDriver
}

func (cs *CameraService) logger() log.FieldLogger {
	return cs.module.Logger()
}

func (cs *CameraService) HANDLE_GRPC_Start(ctx context.Context, in *any.Any) (*any.Any, error) {
	var err error
	req := &empty.Empty{}

	if err = ptypes.UnmarshalAny(in, req); err != nil {
		return nil, err
	}

	res, err := cs.Start(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := ptypes.MarshalAny(res)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (cs *CameraService) Start(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	err := cs.driver.Start()
	if err != nil {
		cs.logger().WithError(err).Errorf("failed to start camera")
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	cs.logger().Infof("camera started")

	return &empty.Empty{}, nil
}

func (cs *CameraService) HANDLE_GRPC_Stop(ctx context.Context, in *any.Any) (*any.Any, error) {
	var err error
	req := &empty.Empty{}

	if err = ptypes.UnmarshalAny(in, req); err != nil {
		return nil, err
	}

	res, err := cs.Stop(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := ptypes.MarshalAny(res)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (cs *CameraService) Stop(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	err := cs.driver.Stop()
	if err != nil {
		cs.logger().WithError(err).Errorf("failed to stop camera")
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	cs.logger().Infof("camera stop")

	return &empty.Empty{}, nil
}

func (cs *CameraService) InitModuleService(m *component.Module) error {
	var err error

	cs.module = m

	drv_opt := &driver.CameraDriverOption{cs.module.Kernel().Config().Sub("driver").Raw()}
	cs.driver, err = driver.NewCameraDriver(drv_opt.GetString("name"), drv_opt, "logger", cs.logger(), "module", cs.module)
	if err != nil {
		return err
	}
	cs.logger().WithField("driver", drv_opt.GetString("name")).Debugf("init camera driver")

	return nil
}
