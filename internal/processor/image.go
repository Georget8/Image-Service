package processor

import (
    "fmt"

    "github.com/davidbyttow/govips/v2/vips"
)

type TransformOptions struct {
    Width   int
    Height  int
    Fit     string
    Format  string
    Quality int
    Crop    string
    Blur    int
}

type Processor struct{}

func NewProcessor() *Processor {
    vips.Startup(&vips.Config{
        ConcurrencyLevel: 4,
    })
    return &Processor{}
}

func (p *Processor) Transform(imageData []byte, opts TransformOptions) ([]byte, error) {
    img, err := vips.NewImageFromBuffer(imageData)
    if err != nil {
        return nil, fmt.Errorf("failed to load image: %w", err)
    }
    defer img.Close()

    if err := img.AutoRotate(); err != nil {
        return nil, fmt.Errorf("failed to auto-rotate: %w", err)
    }

    if opts.Width > 0 || opts.Height > 0 {
        interest := vips.InterestingNone
        if opts.Fit == "cover" {
            interest = vips.InterestingCentre
        }

        if err := img.Thumbnail(opts.Width, opts.Height, interest); err != nil {
            return nil, fmt.Errorf("failed to resize: %w", err)
        }
    }

    if opts.Crop != "" {
        var x, y, w, h int
        if _, err := fmt.Sscanf(opts.Crop, "%d,%d,%d,%d", &x, &y, &w, &h); err == nil {
            if err := img.ExtractArea(x, y, w, h); err != nil {
                return nil, fmt.Errorf("failed to crop: %w", err)
            }
        }
    }

    if opts.Blur > 0 {
        if err := img.GaussianBlur(float64(opts.Blur)); err != nil {
            return nil, fmt.Errorf("failed to blur: %w", err)
        }
    }

    quality := opts.Quality
    if quality <= 0 {
        quality = 80
    }

    var output []byte
    switch opts.Format {
    case "webp":
        params := vips.NewWebpExportParams()
        params.Quality = quality
        params.StripMetadata = true
        output, _, err = img.ExportWebp(params)
    case "avif":
        params := vips.NewAvifExportParams()
        params.Quality = quality
        params.StripMetadata = true
        output, _, err = img.ExportAvif(params)
    case "png":
        params := vips.NewPngExportParams()
        params.StripMetadata = true
        output, _, err = img.ExportPng(params)
    default:
        params := vips.NewJpegExportParams()
        params.Quality = quality
        params.StripMetadata = true
        output, _, err = img.ExportJpeg(params)
    }

    if err != nil {
        return nil, fmt.Errorf("failed to export image: %w", err)
    }

    return output, nil
}

func (p *Processor) Shutdown() {
    vips.Shutdown()
}