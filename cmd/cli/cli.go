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

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kitten/pkg/kitten"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

const mode = 0o600

func main() {
	fs := flag.NewFlagSet("kitten-cli", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	loggerConfig := logger.Flags(fs, "logger")
	kittenConfig := kitten.Flags(fs, "")

	input := fs.String("input", "", "input file")
	caption := fs.String("caption", "", "caption text")
	output := fs.String("output", "", "output file")

	logger.Fatal(fs.Parse(os.Args[1:]))

	logger.Global(logger.New(loggerConfig))
	defer logger.Close()

	ctx := context.Background()

	kittenApp := kitten.New(kittenConfig, unsplash.App{}, tenor.App{}, nil, nil, nil, "")

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
			logger.Warn("close input file: %s", err)
		}
	}()

	outputFile, err := os.OpenFile(*output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	logger.Fatal(err)
	defer func() {
		if closeErr := outputFile.Close(); closeErr != nil {
			logger.Warn("close output file: %s", err)
		}
	}()

	if filepath.Ext(*input) == ".gif" {
		logger.Fatal(generateGif(ctx, kittenApp, inputFile, outputFile, *caption))
	} else {
		logger.Fatal(generateImage(ctx, kittenApp, inputFile, outputFile, *caption))
	}
}

func generateGif(ctx context.Context, kittenApp kitten.App, input, output *os.File, caption string) error {
	inputContent, err := gif.DecodeAll(input)
	if err != nil {
		return fmt.Errorf("decode gif: %w", err)
	}

	outputContent, err := kittenApp.CaptionGif(ctx, inputContent, caption)
	if err != nil {
		return fmt.Errorf("caption gif: %w", err)
	}

	return gif.EncodeAll(output, outputContent)
}

func generateImage(ctx context.Context, kittenApp kitten.App, input, output *os.File, caption string) error {
	inputContent, _, err := image.Decode(input)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	outputContent, err := kittenApp.CaptionImage(ctx, inputContent, caption)
	if err != nil {
		return fmt.Errorf("caption image: %w", err)
	}

	return jpeg.Encode(output, outputContent, &jpeg.Options{Quality: 80})
}
