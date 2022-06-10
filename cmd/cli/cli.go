package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kitten/pkg/giphy"
	"github.com/ViBiOh/kitten/pkg/kitten"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

const mode = 0o600

func main() {
	fs := flag.NewFlagSet("kitten-cli", flag.ExitOnError)

	loggerConfig := logger.Flags(fs, "logger")
	kittenConfig := kitten.Flags(fs, "")

	input := fs.String("input", "", "input file")
	caption := fs.String("caption", "", "caption text")
	output := fs.String("output", "", "output file")

	logger.Fatal(fs.Parse(os.Args[1:]))

	logger.Global(logger.New(loggerConfig))
	defer logger.Close()

	kittenApp := kitten.New(kittenConfig, unsplash.App{}, giphy.App{}, nil, nil, "")

	if len(*input) == 0 {
		logger.Fatal(errors.New("input filename is required"))
	}

	if len(*output) == 0 {
		logger.Fatal(errors.New("output filename is required"))
	}

	if len(*caption) == 0 {
		logger.Fatal(errors.New("caption is required"))
	}

	inputFile, err := os.OpenFile(*input, os.O_RDONLY, mode)
	logger.Fatal(err)
	defer func() {
		if closeErr := inputFile.Close(); closeErr != nil {
			logger.Warn("unable to close input file: %s", err)
		}
	}()

	outputFile, err := os.OpenFile(*output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	logger.Fatal(err)
	defer func() {
		if closeErr := outputFile.Close(); closeErr != nil {
			logger.Warn("unable to close output file: %s", err)
		}
	}()

	if filepath.Ext(*input) == ".gif" {
		logger.Fatal(generateGif(kittenApp, inputFile, outputFile, *caption))
	} else {
		logger.Fatal(generateImage(kittenApp, inputFile, outputFile, *caption))
	}
}

func generateGif(kittenApp kitten.App, input, output *os.File, caption string) error {
	inputContent, err := gif.DecodeAll(input)
	if err != nil {
		return fmt.Errorf("unable to decode gif: %s", err)
	}

	outputContent, err := kittenApp.CaptionGif(context.Background(), inputContent, caption)
	if err != nil {
		return fmt.Errorf("unable to caption gif: %s", err)
	}

	return gif.EncodeAll(output, outputContent)
}

func generateImage(kittenApp kitten.App, input, output *os.File, caption string) error {
	inputContent, _, err := image.Decode(input)
	if err != nil {
		return fmt.Errorf("unable to decode image: %s", err)
	}

	outputContent, err := kittenApp.CaptionImage(context.Background(), inputContent, caption)
	if err != nil {
		return fmt.Errorf("unable to caption image: %s", err)
	}

	return jpeg.Encode(output, outputContent, &jpeg.Options{Quality: 80})
}
