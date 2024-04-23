package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
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

	_ = fs.Parse(os.Args[1:])

	logger.Init(loggerConfig)

	ctx := context.Background()

	kittenService := kitten.New(kittenConfig, unsplash.Service{}, tenor.Service{}, nil, nil, nil, "")

	if len(*input) == 0 {
		slog.ErrorContext(ctx, "input filename is required")
		os.Exit(1)
	}

	if len(*output) == 0 {
		slog.ErrorContext(ctx, "output filename is required")
		os.Exit(1)
	}

	if len(*caption) == 0 {
		slog.ErrorContext(ctx, "caption is required")
		os.Exit(1)
	}

	inputFile, err := os.OpenFile(*input, os.O_RDONLY, mode)
	logger.FatalfOnErr(ctx, err, "open input")

	defer func() {
		if closeErr := inputFile.Close(); closeErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "close input file", slog.Any("error", err))
		}
	}()

	outputFile, err := os.OpenFile(*output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	logger.FatalfOnErr(ctx, err, "create output")

	defer func() {
		if closeErr := outputFile.Close(); closeErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "close output file", slog.Any("error", err))
		}
	}()

	if filepath.Ext(*input) == ".gif" {
		err = generateGif(ctx, kittenService, inputFile, outputFile, *caption)
	} else {
		err = generateImage(ctx, kittenService, inputFile, outputFile, *caption)
	}

	logger.FatalfOnErr(ctx, err, "generate")
}

func generateGif(ctx context.Context, kittenService kitten.Service, input, output *os.File, caption string) error {
	inputContent, err := gif.DecodeAll(input)
	if err != nil {
		return fmt.Errorf("decode gif: %w", err)
	}

	outputContent, err := kittenService.CaptionGif(ctx, inputContent, caption)
	if err != nil {
		return fmt.Errorf("caption gif: %w", err)
	}

	return gif.EncodeAll(output, outputContent)
}

func generateImage(ctx context.Context, kittenService kitten.Service, input, output *os.File, caption string) error {
	inputContent, _, err := image.Decode(input)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	outputContent, err := kittenService.CaptionImage(ctx, inputContent, caption)
	if err != nil {
		return fmt.Errorf("caption image: %w", err)
	}

	return jpeg.Encode(output, outputContent, &jpeg.Options{Quality: 80})
}
