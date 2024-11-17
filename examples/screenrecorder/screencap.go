package main

import (
	"context"
	"image"
	"log"
	"time"
	
	"github.com/kbinani/screenshot"
)

// StartScreenCap Starts Streaming Screenshots to an image.RGBA channel
func StartScreenCap(ctx context.Context, fps int, imgChan chan<- *image.RGBA) error {
	// Calculate the time to wait between each frame
	waitTime := time.Second / time.Duration(fps)

	// Get Screen Resolution
	res := screenshot.GetDisplayBounds(0)

	// create a fps ticker
	ticker := time.NewTicker(waitTime)
	defer ticker.Stop()

	// Start the screenshot loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Screenshot Stream Stopped")
			return nil
		case <-ticker.C:
			img, err := screenshot.CaptureRect(res)
			if err != nil {
				log.Println("Error capturing screenshot:", err)
				return err
			}

			// forwards the image to the channel
			imgChan <- img
		}
	}
}
