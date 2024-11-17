package main

import (
	"errors"
	"image"
	"log"
	"strings"

	"github.com/asticode/go-astiav"
)

// SetupFFmpeg sets up the ffmpeg logging
func SetupFFmpeg() {
	// Set Log Level
	astiav.SetLogLevel(astiav.LogLevelDebug)
	// Set Logging Callback
	astiav.SetLogCallback(func(c astiav.Classer, l astiav.LogLevel, fmt, msg string) {
		var cs string
		if c != nil {
			if cl := c.Class(); cl != nil {
				cs = " - class: " + cl.String()
			}
		}
		log.Printf("ffmpeg log: %s%s - level: %d\n", strings.TrimSpace(msg), cs, l)
	})
}

// ImageRGBAtoAVFrame Converts an image.RGBA to an astiav.Frame
func ImageRGBAtoAVFrame(img *image.RGBA) (*astiav.Frame, error) {
	// create a source frame
	srcFrame := astiav.AllocFrame()
	// clean memory for it when this function exits
	defer srcFrame.Free()

	// set the frame data
	srcFrame.SetWidth(img.Bounds().Dx())
	srcFrame.SetHeight(img.Bounds().Dy())
	srcFrame.SetPixelFormat(astiav.PixelFormatRgba)

	// copy the image into the frame
	// allocate frame buffer
	if err := srcFrame.AllocBuffer(1); err != nil {
		log.Println("Error allocating srcFrame frame buffer:", err)
		return nil, err
	}
	// make frame writable
	if err := srcFrame.MakeWritable(); err != nil {
		log.Println("Error making srcFrame frame writable:", err)
		return nil, err
	}
	// copy image into frame
	if err := srcFrame.Data().FromImage(img); err != nil {
		log.Println("Error copying image into frame:", err)
		return nil, err
	}

	// To Convert RGBA Frame to YUV Frame we will use a SwsContext
	// create a colored frame
	coloredFrame := astiav.AllocFrame()
	// !! DO NOT DEFER coloredFrame.Free() here, because we will return it
	//    we don't want to free memory for it here, or it will be set to null
	//    free it in a higher level function after using it

	// set frame data
	coloredFrame.SetWidth(srcFrame.Width())
	coloredFrame.SetHeight(srcFrame.Height())
	coloredFrame.SetPixelFormat(astiav.PixelFormatYuv420P)

	// allocate buffer to colored frame
	if err := coloredFrame.AllocBuffer(1); err != nil {
		log.Println("Error allocating colored frame buffer:", err)
		return nil, err
	}

	// create a sws context (Software Scaler Context)
	swsCtx, err := astiav.CreateSoftwareScaleContext(
		// source width, height first
		srcFrame.Width(),
		srcFrame.Height(),
		// source pixel format (usually is srcFrame.PixelFormat(), but here we know it)
		astiav.PixelFormatRgba,
		// scaled frame width, height
		coloredFrame.Width(),
		coloredFrame.Height(),
		// scaled pixel format
		astiav.PixelFormatYuv420P,
		// scaling flags (algorithm)
		astiav.NewSoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagBilinear),
	)
	if err != nil {
		log.Println("Error creating sws context:", err)
		return nil, err
	}
	// clean memory for it when this function exits
	defer swsCtx.Free()

	// scale the frame
	if err := swsCtx.ScaleFrame(srcFrame, coloredFrame); err != nil {
		log.Println("Error scaling frame:", err)
		return nil, err
	}

	return coloredFrame, nil
}

// NewH264EncoderCodec Creates a new H264 Encoder Codec and Encoder Codec Context
func NewH264EncoderCodec(width, height, fps int, bitrate int64) (*astiav.Codec, *astiav.CodecContext, error) {
	// find h264 encoder
	// for NVENC (GPU Codec) use astiav.FindEncoderByName("h264_nvenc")
	codec := astiav.FindEncoder(astiav.CodecIDH264)
	if codec == nil {
		log.Println("Error finding h264 encoder")
		return nil, nil, errors.New("error finding h264 encoder")
	}

	// allocate codec context
	codecCtx := astiav.AllocCodecContext(codec)
	if codecCtx == nil {
		log.Println("Error allocating codec context")
		return nil, nil, errors.New("error allocating codec context")
	}
	// !! DO NOT DEFER codecCtx.Free() here, because we will return it
	//    we don't want to free memory for it here, or it will be set to null
	//    free it in a higher level function after using it

	// set codec context parameters
	codecCtx.SetWidth(width)
	codecCtx.SetHeight(height)
	// set frame rate (here, Rationale is 60/1 = 60fps)
	codecCtx.SetFramerate(astiav.NewRational(fps, 1))
	// set time base (here, Rationale is 90kHz = 1/90000)
	codecCtx.SetTimeBase(astiav.NewRational(1, 90*1000))
	// set pixel format to yuv420p
	codecCtx.SetPixelFormat(astiav.PixelFormatYuv420P)
	// set bitrate
	codecCtx.SetBitRate(bitrate)
	// set global header flag
	codecCtx.SetFlags(codecCtx.Flags().Add(astiav.CodecContextFlagGlobalHeader))

	// return
	return codec, codecCtx, nil
}
