package attachments

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"voice-rooms/internal/attachments/archive"
	imageprocessor "voice-rooms/internal/attachments/image"
	videoProcessor "voice-rooms/internal/attachments/video"
)

type ProcessedFile struct {
	Kind        Kind
	Mime        string
	DisplayName string
	Size        int64
	Path        string
}

type Processor struct {
	MaxInputMedia  int64
	MaxArchive     int64
	MaxOutputMedia int64
	FFmpeg         videoProcessor.Runner
}

func (p Processor) Process(ctx context.Context, src io.Reader, originalName string, inputSize int64) (ProcessedFile, error) {
	if inputSize <= 0 || inputSize > p.MaxInputMedia && inputSize > p.MaxArchive {
		return ProcessedFile{}, ErrTooLarge
	}
	input, err := os.CreateTemp("", "threaden-upload-*")
	if err != nil {
		return ProcessedFile{}, fmt.Errorf("create upload temp file: %w", err)
	}
	inputPath := input.Name()
	defer os.Remove(inputPath)
	if _, err := io.Copy(input, io.LimitReader(src, maxInput(p)+1)); err != nil {
		_ = input.Close()
		return ProcessedFile{}, err
	}
	if err := input.Close(); err != nil {
		return ProcessedFile{}, err
	}
	info, err := os.Stat(inputPath)
	if err != nil {
		return ProcessedFile{}, err
	}
	actualSize := info.Size()
	if actualSize <= 0 || actualSize > p.MaxInputMedia && actualSize > p.MaxArchive {
		return ProcessedFile{}, ErrTooLarge
	}
	file, err := os.Open(inputPath)
	if err != nil {
		return ProcessedFile{}, err
	}
	kind, mimeType, detectErr := Detect(file, originalName)
	_ = file.Close()
	if detectErr != nil {
		return ProcessedFile{}, detectErr
	}
	name := SanitizeName(originalName)
	if kind == string(KindArchive) {
		if actualSize > p.MaxArchive || archive.Validate(inputPath, p.MaxArchive, archive.MaxExpandedBytes) != nil {
			return ProcessedFile{}, ErrInvalidFormat
		}
		output, err := os.CreateTemp("", "threaden-archive-*")
		if err != nil {
			return ProcessedFile{}, err
		}
		outputPath := output.Name()
		if err := copyFile(inputPath, output); err != nil {
			_ = output.Close()
			_ = os.Remove(outputPath)
			return ProcessedFile{}, err
		}
		if err := output.Close(); err != nil {
			_ = os.Remove(outputPath)
			return ProcessedFile{}, err
		}
		return ProcessedFile{Kind: KindArchive, Mime: mimeType, DisplayName: name, Size: actualSize, Path: outputPath}, nil
	}
	if kind == string(KindImage) {
		result, err := imageprocessor.ProcessFile(inputPath, uint64(p.MaxOutputMedia))
		if err != nil {
			return ProcessedFile{}, err
		}
		return ProcessedFile{Kind: KindImage, Mime: result.Mime, DisplayName: name, Size: result.Size, Path: result.Path}, nil
	}
	if kind == string(KindVideo) {
		output, err := videoProcessor.NewTempOutput(filepath.Dir(inputPath))
		if err != nil {
			return ProcessedFile{}, err
		}
		size, err := p.FFmpeg.Process(ctx, inputPath, output, p.MaxOutputMedia)
		if err != nil {
			_ = os.Remove(output)
			return ProcessedFile{}, err
		}
		return ProcessedFile{Kind: KindVideo, Mime: mimeType, DisplayName: name, Size: size, Path: output}, nil
	}
	return ProcessedFile{}, ErrUnsupportedFormat
}

func copyFile(path string, output *os.File) error {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	_, err = io.Copy(output, input)
	return err
}

func maxInput(p Processor) int64 {
	if p.MaxInputMedia > p.MaxArchive {
		return p.MaxInputMedia
	}
	return p.MaxArchive
}
