package ffmpeg

/*
#cgo pkg-config: libavformat libavcodec libavutil libswresample

#include <libavformat/avformat.h>
#include <libavutil/error.h>
#include <libswresample/swresample.h>

int averror(int c) {
	return AVERROR(c);
}

int channelMap[] = {0, 1, 2, 3, 6, 7, -1, -1};
*/
import "C"

import (
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"
)

var initialize sync.Once

type FFmpeg struct {
	formatCtx        *C.struct_AVFormatContext
	streams          []*C.AVStream
	audioStreamIndex int
	sampleFormat     C.enum_AVSampleFormat
	resampler        *C.struct_SwrContext
	remapper         *C.struct_SwrContext

	// State for reading
	readStarted    bool
	frame          *C.AVFrame
	resampledFrame *C.AVFrame
	remappedFrame  *C.AVFrame
}

type AudioFrame struct {
	Data     []byte
	Position time.Duration
}

func durationToBase(stream *C.AVStream, position int64) int64 {
	return (position * int64(stream.time_base.den)) / (1e9 * int64(stream.time_base.num))
}

func baseToDuration(stream *C.AVStream, position int64) int64 {
	return (position * 1e9 * int64(stream.time_base.num)) / int64(stream.time_base.den)
}

func Create(file string, maxChannels int) (*FFmpeg, error) {
	initialize.Do(func() {
		C.av_register_all()
		C.av_log_set_level(C.AV_LOG_WARNING)
	})

	success := false

	// Open file
	cFile := C.CString(file)
	defer C.free(unsafe.Pointer(cFile))
	var formatCtx *C.struct_AVFormatContext
	if err := C.avformat_open_input(&formatCtx, cFile, nil, nil); err != 0 {
		return nil, avError("open input", err)
	}
	defer func() {
		if !success {
			C.avformat_close_input(&formatCtx)
		}
	}()

	if err := C.avformat_find_stream_info(formatCtx, nil); err != 0 {
		return nil, avError("find stream info", err)
	}

	// C.av_dump_format(formatCtx, 0, cFile, 0)

	streams := (*[1 << 20]*C.AVStream)(unsafe.Pointer(formatCtx.streams))[:formatCtx.nb_streams:formatCtx.nb_streams]
	ret := C.av_find_best_stream(formatCtx, C.AVMEDIA_TYPE_AUDIO, -1, -1, nil, 0)
	if ret < 0 {
		return nil, avError("find audio stream", ret)
	}
	audioStreamIndex := int(ret)

	// Open decoder
	stream := streams[audioStreamIndex]
	codec := C.avcodec_find_decoder(stream.codec.codec_id)
	if codec == nil {
		return nil, fmt.Errorf("Unsupported codec: %v", stream.codec.codec_id)
	}

	var options *C.AVDictionary
	if err := C.avcodec_open2(stream.codec, codec, &options); err != 0 {
		return nil, avError("open codec", err)
	}

	// Determine sample format
	var sampleFormat C.enum_AVSampleFormat
	switch stream.codec.sample_fmt {
	case C.AV_SAMPLE_FMT_U8, C.AV_SAMPLE_FMT_S16, C.AV_SAMPLE_FMT_U8P, C.AV_SAMPLE_FMT_S16P:
		sampleFormat = C.AV_SAMPLE_FMT_S16
	default:
		sampleFormat = C.AV_SAMPLE_FMT_S32
	}

	// Initialize helper state
	frame := C.av_frame_alloc()

	// Determine the output settings
	resampledFrame := C.av_frame_alloc()
	resampledFrame.channel_layout = C.AV_CH_LAYOUT_STEREO
	if maxChannels >= 8 {
		resampledFrame.channel_layout = C.AV_CH_LAYOUT_7POINT1
	} else if maxChannels >= 6 {
		resampledFrame.channel_layout = C.AV_CH_LAYOUT_5POINT1
	}
	resampledFrame.sample_rate = stream.codec.sample_rate
	resampledFrame.format = C.int(sampleFormat)
	resampler := C.swr_alloc_set_opts(nil,
		C.int64_t(resampledFrame.channel_layout),
		int32(resampledFrame.format),
		resampledFrame.sample_rate,
		C.int64_t(stream.codec.channel_layout),
		stream.codec.sample_fmt,
		stream.codec.sample_rate,
		0, nil)

	// Determine remapping
	remappedFrame := C.av_frame_alloc()
	remappedFrame.channel_layout = resampledFrame.channel_layout
	remappedFrame.sample_rate = resampledFrame.sample_rate
	remappedFrame.format = resampledFrame.format
	remapChannels :=
		C.av_get_channel_layout_nb_channels(resampledFrame.channel_layout) == 8 &&
			C.av_get_channel_layout_nb_channels(stream.codec.channel_layout) == 6 &&
			(stream.codec.channel_layout&C.AV_CH_SIDE_LEFT) != 0 &&
			(stream.codec.channel_layout&C.AV_CH_BACK_LEFT) == 0
	var remapper *C.struct_SwrContext
	if remapChannels {
		remapper = C.swr_alloc_set_opts(nil,
			C.int64_t(resampledFrame.channel_layout),
			int32(resampledFrame.format),
			resampledFrame.sample_rate,
			C.int64_t(resampledFrame.channel_layout),
			int32(resampledFrame.format),
			resampledFrame.sample_rate,
			0, nil)
	}

	// Debug
	log.Printf("Input: %v %v %v %v",
		C.GoString(stream.codec.codec.name),
		C.GoString(C.av_get_sample_fmt_name(int32(stream.codec.sample_fmt))),
		C.av_get_channel_layout_nb_channels(stream.codec.channel_layout),
		stream.codec.sample_rate)
	log.Printf("Output: %v %v %v %v (remap: %v)",
		C.GoString(stream.codec.codec.name),
		C.GoString(C.av_get_sample_fmt_name(int32(resampledFrame.format))),
		C.av_get_channel_layout_nb_channels(resampledFrame.channel_layout),
		resampledFrame.sample_rate,
		remapper != nil)

	success = true
	return &FFmpeg{
		formatCtx:        formatCtx,
		streams:          streams,
		audioStreamIndex: audioStreamIndex,
		sampleFormat:     sampleFormat,
		resampler:        resampler,
		remapper:         remapper,

		frame:          frame,
		resampledFrame: resampledFrame,
		remappedFrame:  remappedFrame,
	}, nil
}

func (f *FFmpeg) Close() {
	if f.remapper != nil {
		C.swr_free(&f.remapper)
	}
	C.av_frame_free(&f.remappedFrame)
	C.swr_free(&f.resampler)
	C.av_frame_free(&f.resampledFrame)
	C.av_frame_free(&f.frame)
	C.avcodec_close(f.audioStream().codec)
	C.avformat_close_input(&f.formatCtx)
}

func (f *FFmpeg) SampleRate() int {
	return int(f.resampledFrame.sample_rate)
}

func (f *FFmpeg) BytesPerSample() int {
	return int(C.av_get_bytes_per_sample(int32(f.sampleFormat)))
}

func (f *FFmpeg) NumChannels() int {
	return int(C.av_get_channel_layout_nb_channels(f.resampledFrame.channel_layout))
}

func (f *FFmpeg) audioStream() *C.struct_AVStream {
	return f.streams[f.audioStreamIndex]
}

func (f *FFmpeg) ReadAudioFrame() (*AudioFrame, error) {
	stream := f.audioStream()
	resampledFrame := f.resampledFrame
	remappedFrame := f.remappedFrame
	frame := f.frame
	resampler := f.resampler
	remapper := f.remapper

	// Initialize reader
	if !f.readStarted {
		if err := C.swr_init(resampler); err != 0 {
			return nil, avError("initialize resampler", err)
		}
		if remapper != nil {
			if err := C.swr_set_channel_mapping(remapper, &C.channelMap[0]); err != 0 {
				return nil, avError("set channel mapping", err)
			}
			if err := C.swr_init(remapper); err != 0 {
				return nil, avError("initialize remapper", err)
			}
		}
		f.readStarted = true
	}

	// Read a packet until we have a frame
	for {
		var packet C.AVPacket
		for {
			if err := C.av_read_frame(f.formatCtx, &packet); err != 0 {
				if err == C.AVERROR_EOF {
					return nil, nil
				} else {
					return nil, avError("read packet", err)
				}
			}

			if packet.stream_index == C.int(f.audioStreamIndex) {
				break
			} else {
				C.av_packet_unref(&packet)
			}
		}
		defer C.av_packet_unref(&packet)

		// Decode a frame
		if err := C.avcodec_send_packet(stream.codec, &packet); err != 0 {
			return nil, avError("send packet", err)
		}

		err := C.avcodec_receive_frame(stream.codec, frame)
		if err == C.averror(C.EAGAIN) {
			continue
		} else if err != 0 {
			return nil, avError("receive frame", err)
		} else {
			break
		}
	}

	// Convert frame
	if err := C.swr_convert_frame(resampler, resampledFrame, frame); err != 0 {
		return nil, avError("resample frame", err)
	}
	outFrame := resampledFrame
	if remapper != nil {
		if err := C.swr_convert_frame(remapper, remappedFrame, resampledFrame); err != 0 {
			return nil, avError("remap frame", err)
		}
		outFrame = remappedFrame
	}

	numChannels := C.av_get_channel_layout_nb_channels(resampledFrame.channel_layout)
	bytesPerSample := C.av_get_bytes_per_sample(int32(resampledFrame.format))
	lineSize := outFrame.nb_samples * bytesPerSample * numChannels
	return &AudioFrame{
		Data:     C.GoBytes(unsafe.Pointer(*outFrame.extended_data), lineSize),
		Position: time.Duration(baseToDuration(stream, int64(frame.pts))),
	}, nil
}

func (f *FFmpeg) Seek(position time.Duration) error {
	log.Printf("Seeking to %v", position)
	if err := C.av_seek_frame(f.formatCtx, -1, C.int64_t(position/1000), 0); err != 0 {
		return avError("seek", err)
	}
	return nil
}

func avError(message string, err C.int) error {
	// Use av_err2str instead?
	buf := make([]C.char, C.AV_ERROR_MAX_STRING_SIZE)
	if C.av_strerror(err, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))) != 0 {
		return fmt.Errorf("%s: Unknown error", message)
	}
	return fmt.Errorf("%s (%v): %s", message, err, C.GoString((*C.char)(unsafe.Pointer(&buf[0]))))
}
