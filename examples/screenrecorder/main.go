package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os/signal"
	"syscall"

	"github.com/asticode/go-astiav"
	"github.com/kbinani/screenshot"
)

func main() {
	fmt.Println("Starting Screen recording...")
	// Get the screen resolution
	bounds := screenshot.GetDisplayBounds(0)
	StartScreenRecording("output.mp4", bounds.Dx(), bounds.Dy(), 30)
}

func StartScreenRecording(filename string, width, height, fps int) {
	SetupFFmpeg()

	// create output format context (output file container)
	outputCtx, err := astiav.AllocOutputFormatContext(nil, "", filename)
	if err != nil {
		fmt.Println("Error creating output format context:", err)
		return
	}
	defer outputCtx.Free()

	// create an Encoder Codec and Encoder Codec Context
	enc, encCtx, err := NewH264EncoderCodec(width, height, fps, outputCtx.BitRate())
	if err != nil {
		fmt.Println("Error creating encoder codec context:", err)
		return
	}
	defer encCtx.Free()

	// add a new video stream to the output context
	videoStream := outputCtx.NewStream(enc)
	if videoStream == nil {
		fmt.Println("Error creating new video stream")
		return
	}

	// open codec context
	if err = encCtx.Open(enc, nil); err != nil {
		fmt.Println("Error opening codec context:", err)
		return
	}

	// update video stream params from codec context
	if err = videoStream.CodecParameters().FromCodecContext(encCtx); err != nil {
		fmt.Println("Error updating video stream params:", err)
		return
	}

	// set stream timebase
	videoStream.SetTimeBase(encCtx.TimeBase())

	// write output format ctx as streams have been configured
	// as we are writing to a file we need to provide an io context
	var ioCtx *astiav.IOContext
	if ioCtx, err = astiav.OpenIOContext(filename, astiav.NewIOContextFlags(astiav.IOContextFlagWrite)); err != nil {
		fmt.Println("Error opening IO context:", err)
		return
	}
	defer ioCtx.Free()
	// assign the io context to the output context
	outputCtx.SetPb(ioCtx)
	// write output format header
	if err = outputCtx.WriteHeader(nil); err != nil {
		fmt.Println("Error writing header:", err)
		return
	}
	// write trailer when done
	defer func() {
		if err = outputCtx.WriteTrailer(); err != nil {
			fmt.Println("Error writing trailer:", err)
			return
		}
		// close io context
		fmt.Println("Stopping Screen recording!")
	}()

	// start capturing
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stop()

	// make a channel to recv images
	imgChan := make(chan *image.RGBA)

	// Start Screen Capture
	go func() {
		if err := StartScreenCap(ctx, fps, imgChan); err != nil {
			fmt.Println("Error capturing screen:", err)
			return
		}
	}()

	// PTS Increment
	// for a 90kHz timebase freq, and 60fps
	// PTS = Frame Number × (90*1000 ÷ 60) = 1 * (90000 ÷ 60) = 1500
	pts := int64((90 * 1000) / fps)
	frameNumber := int64(0)

	// encode and write frames
	for {
		select {
		case <-ctx.Done():
			return
		case img := <-imgChan:
			// create frame from image
			frame, err := ImageRGBAtoAVFrame(img)
			if err != nil {
				fmt.Println("error creating frame from image:", err)
				defer ctx.Done()
				return
			}

			// set frame pts
			frame.SetPts(frameNumber * pts)

			// send frame for encoding
			if err := encCtx.SendFrame(frame); err != nil {
				fmt.Println("error sending frame for encoding:", err)
				defer ctx.Done()
				return
			}

			// increment frame number
			frameNumber++

			// create a packet to store encoded data
			packet := astiav.AllocPacket()
			if packet == nil {
				fmt.Println("error allocating packet")
				defer ctx.Done()
				return
			}
			defer packet.Free()

			// encode packet
			for {
				err := encCtx.ReceivePacket(packet)
				if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
					break
				} else if err != nil {
					fmt.Println("error encoding packet:", err)
					defer ctx.Done()
					return
				}

				// rescale packet timestamp from codec timebase to stream timebase
				packet.RescaleTs(encCtx.TimeBase(), videoStream.TimeBase())

				// write packet to output context
				if err = outputCtx.WriteFrame(packet); err != nil {
					fmt.Println("error writing packet:", err)
					defer ctx.Done()
					return
				}
			}
		}
	}
}
