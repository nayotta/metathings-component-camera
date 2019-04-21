package camera_driver

import (
	"context"
	"math/rand"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/cbroglie/mustache"
)

/*
 * Driver: ffmpeg_simple
 *   for livego rtmp server, generate random live id.
 * Example Options:
 *   driver:
 *     name: ffmpeg_simple
 *     ffmpeg_file: ffmpeg  // ffmpeg file path, default: ffmpeg.
 *     video_input:
 *       format: <format>  // file format, like `v4l2` etc.
 *       file: <path>  // file path, like `/dev/video0` etc.
 *       frame_size: <width>x<height>  // frame size, like `640x480`.
 *       frame_rate: <rate>  // frame rate, like `30`.
 *       codec:
 *         name: <codec>  // video encoder, like `h264_omx` for rasberry pi.
 *         bit_rate: <rate>  // video bitrate, like `2000k`.
 *         extra: [ ... ]  // list of extra arguments for codec.
 *     output:
 *       format: <format>  // file format, like `flv` etc.
 *       file: <path>  // file path, like `rtmp://rtmp-server:1935/path`.
 *     // ffmpeg_command: <template-string>  // template string for ffmpeg, default: constant variable `FFMPEG_SIMPLE_CAMERA_DRIVER_DEFAULT_TEMPLATE`.
 */

const (
	FFMPEG_SIMPLE_CAMERA_DRIVER_DEAFULT_TEMPLATE    = `{{ffmpeg_file}} -y -f {{video_input_format}} -i {{video_input_file}} -s {{video_input_frame_size}} -r {{video_input_frame_rate}} -c:v {{video_input_codec_name}} -b:v {{video_input_codec_bit_rate}} {{video_input_codec_extra}} -f {{output_format}} {{output_file}}`
	FFMPEG_SIMPLE_CAMERA_DRIVER_DEAFULT_FFMPEG_FILE = `ffmepg`
)

type FFmpegSimpleCameraDriver struct {
	op_mtx      *sync.Mutex
	cancel_func context.CancelFunc

	opt *CameraDriverOption
	st  *CameraDriverState
}

const _LIVEID_LETTERS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func random_strings(n int) string {
	buf := make([]byte, n)
	for i := 0; i < n; i++ {
		buf[i] = _LIVEID_LETTERS[rand.Intn(n)]
	}
	return string(buf)
}

func (d *FFmpegSimpleCameraDriver) parse_ffmpeg_command() (string, error) {
	cmd_opt := map[string]interface{}{}

	if val := d.opt.GetString("ffmpeg_file"); val != "" {
		cmd_opt["ffmpeg_file"] = val
	} else {
		cmd_opt["ffmpeg_file"] = FFMPEG_SIMPLE_CAMERA_DRIVER_DEAFULT_FFMPEG_FILE
	}

	video_input := d.opt.Sub("video_input")
	if video_input == nil {
		return "", ErrInvalidConfig
	}

	if val := video_input.GetString("format"); val != "" {
		cmd_opt["video_input_format"] = val
	} else {
		return "", ErrInvalidConfig
	}

	if val := video_input.GetString("file"); val != "" {
		cmd_opt["video_input_file"] = val
	} else {
		return "", ErrInvalidConfig
	}

	if val := video_input.GetString("frame_size"); val != "" {
		cmd_opt["video_input_frame_size"] = val
	} else {
		return "", ErrInvalidConfig
	}

	if val := video_input.GetString("frame_rate"); val != "" {
		cmd_opt["video_input_frame_rate"] = val
	} else {
		return "", ErrInvalidConfig
	}

	video_input_codec := video_input.Sub("codec")
	if video_input_codec == nil {
		return "", ErrInvalidConfig
	}

	if val := video_input_codec.GetString("name"); val != "" {
		cmd_opt["video_input_codec_name"] = val
	} else {
		return "", ErrInvalidConfig
	}

	if val := video_input_codec.GetString("bit_rate"); val != "" {
		cmd_opt["video_input_codec_bit_rate"] = val
	} else {
		return "", ErrInvalidConfig
	}

	if val := video_input_codec.GetStringSlice("extra"); val != nil {
		cmd_opt["video_input_codec_extra"] = strings.Join(val, " ")
	} else {
		return "", ErrInvalidConfig
	}

	output := d.opt.Sub("output")
	if output == nil {
		return "", ErrInvalidConfig
	}

	if val := output.GetString("format"); val != "" {
		cmd_opt["output_format"] = val
	} else {
		return "", ErrInvalidConfig
	}

	if val := output.GetString("file"); val != "" {
		cmd_opt["output_file"] = path.Join(val, random_strings(64))
	} else {
		return "", ErrInvalidConfig
	}

	var temp string
	if val := d.opt.GetString("ffmpeg_command"); val != "" {
		temp = val
	} else {
		temp = FFMPEG_SIMPLE_CAMERA_DRIVER_DEAFULT_FFMPEG_FILE
	}

	cmd_str, err := mustache.Render(temp, cmd_opt)
	if err != nil {
		return "", err
	}

	return cmd_str, nil
}

func (d *FFmpegSimpleCameraDriver) start_ffmpeg(ctx context.Context, cmd_str string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", cmd_str)
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

func (d *FFmpegSimpleCameraDriver) Start() error {
	d.op_mtx.Lock()
	defer d.op_mtx.Unlock()

	if d.st == CAMERA_DRIVER_STATE_ON {
		return ErrNotStartable
	}

	ctx := context.TODO()
	ctx, cfn := context.WithCancel(ctx)
	d.cancel_func = cfn

	cmd_str, err := d.parse_ffmpeg_command()
	if err != nil {
		return err
	}

	cmd, err := d.start_ffmpeg(ctx, cmd_str)
	if err != nil {
		return err
	}

	d.st = CAMERA_DRIVER_STATE_ON
	go func() {
		cmd.Wait()

		d.op_mtx.Lock()
		defer d.op_mtx.Unlock()

		if d.st == CAMERA_DRIVER_STATE_OFF {
			return
		}

		d.reset()
	}()

	return nil
}

func (d *FFmpegSimpleCameraDriver) reset() {
	if d.cancel_func != nil {
		d.cancel_func()
	}
	d.cancel_func = nil
	d.st = CAMERA_DRIVER_STATE_OFF
}

func (d *FFmpegSimpleCameraDriver) Stop() error {
	d.op_mtx.Lock()
	defer d.op_mtx.Unlock()

	if d.st == CAMERA_DRIVER_STATE_OFF {
		return ErrNotStoppable
	}

	d.reset()

	return nil
}

func (d *FFmpegSimpleCameraDriver) State() *CameraDriverState {
	return d.st
}

func NewFFmpegSimpleCameraDriver(opt *CameraDriverOption) (CameraDriver, error) {
	drv := &FFmpegSimpleCameraDriver{
		op_mtx: new(sync.Mutex),
		opt:    opt,
		st:     CAMERA_DRIVER_STATE_OFF,
	}

	return drv, nil
}

var register_ffmpeg_simple_camera_driver_once sync.Once

func init() {
	register_ffmpeg_simple_camera_driver_once.Do(func() {
		register_camera_driver_factory("ffmpeg_simple", NewFFmpegSimpleCameraDriver)
	})
}
