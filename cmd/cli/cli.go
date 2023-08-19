package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"log"
	"log/slog"
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

	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	logger.Init(loggerConfig)

	ctx := context.Background()

	kittenApp := kitten.New(kittenConfig, unsplash.App{}, tenor.App{}, nil, nil, nil, "")

	if len(*input) == 0 {
		slog.Error("input filename is required")
		os.Exit(1)
	}

	if len(*output) == 0 {
		slog.Error("output filename is required")
		os.Exit(1)
	}

	if len(*caption) == 0 {
		slog.Error("caption is required")
		os.Exit(1)
	}

	inputFile, err := os.OpenFile(*input, os.O_RDONLY, mode)
	if err != nil {
		slog.Error("open input", "err", err)
		os.Exit(1)
	}

	defer func() {
		if closeErr := inputFile.Close(); closeErr != nil {
			slog.Warn("close input file", "err", err)
		}
	}()

	outputFile, err := os.OpenFile(*output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		slog.Error("create output", "err", err)
		os.Exit(1)
	}

	defer func() {
		if closeErr := outputFile.Close(); closeErr != nil {
			slog.Warn("close output file", "err", err)
		}
	}()

	if filepath.Ext(*input) == ".gif" {
		err = generateGif(ctx, kittenApp, inputFile, outputFile, *caption)
	} else {
		err = generateImage(ctx, kittenApp, inputFile, outputFile, *caption)
	}

	if err != nil {
		slog.Error("generate", "err", err)
		os.Exit(1)
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
