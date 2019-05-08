package camera_driver

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	opt_helper "github.com/nayotta/metathings/pkg/common/option"
	log "github.com/sirupsen/logrus"
)

/*
 * Framework: ffmpeg
 * Options:
 *   driver:
 *   ...
 *     framework:
 *       name: ffmpeg
 *       [ binary: ffmpeg ]  // ffmpeg binary file path.
 *       inputs:
 *         0:
 *           format: <format>  // input file format, like `v4l2`.
 *           file: <path>  // file path, like `/dev/video0`, set from camera driver.
 *           [ frame_size: <width>x<height> ]  // frame size, like `640x480`.
 *           [ frame_rate: <rate> ]  // frame rate, like `30`.
 *       outputs:
 *         0:
 *           format: <format>  // output file format, like `flv`.
 *           [ file: <path> ]  // file path, like `rtmp://rtmp-server:port/path`.
 *       video:
 *         codec:
 *           name: <codec>  // video codec, like `h264_omx` for raspberry pi.
 *           [ bit_rate: <rate> ]  // video bitrate, like `2000k`.
 *           [ extra: [ ... ] ]  // list of extra arguments for codec.
 *       audio:
 *         codec:
 *           name: <codec>  // audio codec, like `copy` for copy rtsp to rtmp
 *   ...
 *
 */

const (
	FFMPEG_FRAMEWORK_DEAFULT_BINARY = `ffmpeg`
)

func (f *FFmpegFramework) parse_ffmpeg_command() (string, error) {
	var cmd_str string

	if val := f.opt.GetString("binary"); val != "" {
		cmd_str = val
	} else {
		cmd_str = FFMPEG_FRAMEWORK_DEAFULT_BINARY
	}

	cmd_str += " -y"

	// INPUTS
	inputs := f.opt.Sub("inputs")
	if inputs == nil {
		return "", new_invalid_config_error("inputs")
	}

	for _, k := range inputs.NextKeys() {
		input := inputs.Sub(k)

		if val := input.GetString("format"); val != "" {
			cmd_str += " -f " + val
		} else {
			return "", new_invalid_config_error(fmt.Sprintf("inputs.%v.format", k))
		}

		if val := input.GetString("file"); val != "" {
			cmd_str += " -i " + val
		} else {
			return "", new_invalid_config_error(fmt.Sprintf("inputs.%v.file", k))
		}

		if val := input.GetString("frame_size"); val != "" {
			cmd_str += " -s " + val
		}

		if val := input.GetString("frame_rate"); val != "" {
			cmd_str += " -r " + val
		}
	}

	// VIDEO
	video := f.opt.Sub("video")
	if video == nil {
		return "", new_invalid_config_error("video")
	}

	video_codec := video.Sub("codec")
	if video_codec == nil {
		return "", new_invalid_config_error("video.codec")
	}

	if val := video_codec.GetString("name"); val != "" {
		cmd_str += " -c:v " + val
	} else {
		return "", new_invalid_config_error("video.codec.name")
	}

	if val := video_codec.GetString("bit_rate"); val != "" {
		cmd_str += " -b:v " + val
	}

	if val := video_codec.GetStringSlice("extra"); val != nil {
		cmd_str += " " + strings.Join(val, " ")
	}

	// AUDIO
	audio := f.opt.Sub("audio")
	if audio == nil {
		// disable audio
		cmd_str += " -an"
	} else {
		audio_codec := audio.Sub("codec")
		if audio_codec == nil {
			return "", new_invalid_config_error("audio.codec")
		}

		if val := audio_codec.GetString("name"); val != "" {
			cmd_str += " -c:a " + val
		} else {
			return "", new_invalid_config_error("audio.codec.name")
		}

		if val := audio_codec.GetStringSlice("extra"); val != nil {
			cmd_str += strings.Join(val, " ")
		}
	}

	// OUTPUTS
	outputs := f.opt.Sub("outputs")
	if outputs == nil {
		return "", new_invalid_config_error("outputs")
	}

	for _, k := range outputs.NextKeys() {
		output := outputs.Sub(k)

		if val := output.GetString("format"); val != "" {
			cmd_str += " -f " + val
		} else {
			return "", new_invalid_config_error(fmt.Sprintf("outputs.%v.format", k))
		}

		if val := output.GetString("file"); val != "" {
			cmd_str += " " + val
		} else {
			return "", new_invalid_config_error(fmt.Sprintf("outputs.%v.file", k))
		}
	}

	return cmd_str, nil
}

type FFmpegFramework struct {
	opt *FrameworkOption

	logger log.FieldLogger
	op_mtx *sync.Mutex
	cmd    *exec.Cmd
	cfn    context.CancelFunc
	errchs []chan<- error
}

// NOTE: should be call after `op_mtx` locked!
func (f *FFmpegFramework) send_to_waiting_channels(err error) {
	for _, errch := range f.errchs {
		errch <- err
	}
}

func (f *FFmpegFramework) Start() error {
	f.op_mtx.Lock()
	defer f.op_mtx.Unlock()

	if f.cfn != nil {
		f.logger.WithError(ErrNotStartable).Debugf("ffmpeg not startable")
		return ErrNotStartable
	}

	cmd_str, err := f.parse_ffmpeg_command()
	if err != nil {
		f.logger.WithError(err).Debugf("failed to parse ffmpeg command")
		return err
	}

	ctx := context.TODO()
	ctx, f.cfn = context.WithCancel(ctx)
	f.cmd = exec.CommandContext(ctx, "/bin/bash", "-c", cmd_str)

	err = f.cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		err := f.cmd.Wait()

		f.op_mtx.Lock()
		defer f.op_mtx.Unlock()

		if err != nil {
			f.logger.WithError(err).Debugf("failed to wait command exit")
		}

		f.send_to_waiting_channels(err)
	}()

	f.logger.WithField("cmd", cmd_str).Debugf("ffmpeg start")

	return nil
}

func (f *FFmpegFramework) Stop() error {
	f.op_mtx.Lock()
	defer f.op_mtx.Unlock()

	if f.cfn == nil {
		f.logger.WithError(ErrNotStoppable).Debugf("ffmpeg not stopable")
		return ErrNotStoppable
	}

	f.cfn()
	f.send_to_waiting_channels(nil)

	f.logger.Debugf("ffmpeg stop")

	return nil
}

func (f *FFmpegFramework) Wait() <-chan error {
	f.op_mtx.Lock()
	defer f.op_mtx.Unlock()

	errch := make(chan error)
	f.errchs = append(f.errchs, errch)

	return errch
}

func NewFFmpegFramework(opt *FrameworkOption, args ...interface{}) (Framework, error) {
	var ok bool
	var logger log.FieldLogger

	opt_helper.Setopt(opt_helper.SetoptConds{
		"logger": func(key string, val interface{}) error {
			if logger, ok = val.(log.FieldLogger); !ok {
				return opt_helper.ErrInvalidArguments
			}
			return nil
		},
	})(args...)

	frm := &FFmpegFramework{
		opt:    opt,
		op_mtx: new(sync.Mutex),
		logger: logger,
	}

	frm.logger.Debugf("new ffmpeg framework")

	return frm, nil
}

var register_ffmpeg_framework_once sync.Once

func init() {
	register_ffmpeg_framework_once.Do(func() {
		register_framework_factory("ffmpeg", NewFFmpegFramework)
	})
}
