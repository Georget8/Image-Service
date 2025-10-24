package processor

import (
	"fmt"

	"github.com/davidbyttow/govips/v2/vips"
)

type TransformOptions struct {
	Width      int
	Height     int
	Fit        string // cover, contain, fill
	Format     string // jpeg, webp, avif, png
	Quality    int
	Crop       string // "x,y,width,height"
	Blur       int
	Sharpen    float64 // 0-10 (default: 0, recommended: 1-3)
	Brightness float64 // -100 to 100 (0 = no change)
	Contrast   float64 // 0.5 to 2.0 (1.0 = no change)
	Saturation float64 // 0.0 to 3.0 (1.0 = no change)
	AutoOptim  bool    // Auto-optimize for web
	Grayscale  bool    // Convert to grayscale
	Flip       string  // "h" (horizontal), "v" (vertical), "both"
	Rotate     int     // 90, 180, 270 degrees
	Background string  // hex color for padding (e.g., "ffffff")
	Strip      bool    // Strip all metadata (default: true)
}

type Processor struct{}

func NewProcessor() *Processor {
	vips.Startup(&vips.Config{
		ConcurrencyLevel: 8,
		MaxCacheSize:     200,
		MaxCacheMem:      100 * 1024 * 1024,
		MaxCacheFiles:    500,
	})
	return &Processor{}
}

func (p *Processor) Transform(imageData []byte, opts TransformOptions) ([]byte, error) {
	// Load image
	img, err := vips.NewImageFromBuffer(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to load image: %w", err)
	}
	defer img.Close()

	// Auto-rotate based on EXIF
	if err := img.AutoRotate(); err != nil {
		return nil, fmt.Errorf("failed to auto-rotate: %w", err)
	}

	// Manual rotation
	if opts.Rotate > 0 {
		angle := vips.Angle0
		switch opts.Rotate {
		case 90:
			angle = vips.Angle90
		case 180:
			angle = vips.Angle180
		case 270:
			angle = vips.Angle270
		}
		if err := img.Rotate(angle); err != nil {
			return nil, fmt.Errorf("failed to rotate: %w", err)
		}
	}

	// Flip
	if opts.Flip != "" {
		switch opts.Flip {
		case "h":
			if err := img.Flip(vips.DirectionHorizontal); err != nil {
				return nil, fmt.Errorf("failed to flip: %w", err)
			}
		case "v":
			if err := img.Flip(vips.DirectionVertical); err != nil {
				return nil, fmt.Errorf("failed to flip: %w", err)
			}
		case "both":
			if err := img.Flip(vips.DirectionHorizontal); err != nil {
				return nil, fmt.Errorf("failed to flip: %w", err)
			}
			if err := img.Flip(vips.DirectionVertical); err != nil {
				return nil, fmt.Errorf("failed to flip: %w", err)
			}
		}
	}

	// Resize with smart cropping
	if opts.Width > 0 || opts.Height > 0 {
		interest := vips.InterestingNone
		if opts.Fit == "cover" {
			interest = vips.InterestingCentre // Smart crop to center
		} else if opts.Fit == "attention" {
			interest = vips.InterestingAttention // Crop to most interesting area
		}

		if err := img.Thumbnail(opts.Width, opts.Height, interest); err != nil {
			return nil, fmt.Errorf("failed to resize: %w", err)
		}
	}

	// Manual crop
	if opts.Crop != "" {
		var x, y, w, h int
		if _, err := fmt.Sscanf(opts.Crop, "%d,%d,%d,%d", &x, &y, &w, &h); err == nil {
			if err := img.ExtractArea(x, y, w, h); err != nil {
				return nil, fmt.Errorf("failed to crop: %w", err)
			}
		}
	}

	// Auto-optimization: Smart sharpen + color optimization
	if opts.AutoOptim {
		// Mild sharpen for web display
		if err := img.Sharpen(1.0, 1.0, 1.2); err != nil {
			return nil, fmt.Errorf("failed to auto-sharpen: %w", err)
		}

		// Optimize colors for sRGB (web standard)
		// This is done automatically during export
	}

	// Manual sharpen
	if opts.Sharpen > 0 {
		// sigma: 1.0 (how much blur before sharpening)
		// x1: 1.0 (flat area threshold)
		// m2: opts.Sharpen (sharpening amount)
		if err := img.Sharpen(1.0, 1.0, opts.Sharpen); err != nil {
			return nil, fmt.Errorf("failed to sharpen: %w", err)
		}
	}

	// Blur
	if opts.Blur > 0 {
		if err := img.GaussianBlur(float64(opts.Blur)); err != nil {
			return nil, fmt.Errorf("failed to blur: %w", err)
		}
	}

	// Grayscale
	if opts.Grayscale {
		if err := img.ToColorSpace(vips.InterpretationBW); err != nil {
			return nil, fmt.Errorf("failed to convert to grayscale: %w", err)
		}
	}

	// Brightness adjustment
	if opts.Brightness != 0 {
		// Brightness: -100 to +100
		multiplier := 1.0 + (opts.Brightness / 100.0)
		if err := img.Linear([]float64{multiplier}, []float64{0}); err != nil {
			return nil, fmt.Errorf("failed to adjust brightness: %w", err)
		}
	}

	// Contrast adjustment
	if opts.Contrast != 0 && opts.Contrast != 1.0 {
		// Contrast: 0.5 (low) to 2.0 (high), 1.0 = normal
		offset := 128 * (1 - opts.Contrast)
		if err := img.Linear([]float64{opts.Contrast}, []float64{offset}); err != nil {
			return nil, fmt.Errorf("failed to adjust contrast: %w", err)
		}
	}

	// Saturation adjustment
	if opts.Saturation != 0 && opts.Saturation != 1.0 {
		// Convert to LAB color space for saturation adjustment
		originalSpace := img.Interpretation()

		if err := img.ToColorSpace(vips.InterpretationLAB); err != nil {
			return nil, fmt.Errorf("failed to convert to LAB: %w", err)
		}

		// Multiply a and b channels (chrominance) by saturation factor
		if err := img.Linear(
			[]float64{1.0, opts.Saturation, opts.Saturation},
			[]float64{0, 0, 0},
		); err != nil {
			return nil, fmt.Errorf("failed to adjust saturation: %w", err)
		}

		// Convert back to original color space
		if err := img.ToColorSpace(originalSpace); err != nil {
			return nil, fmt.Errorf("failed to convert back: %w", err)
		}
	}

	// Set quality
	quality := opts.Quality
	if quality <= 0 {
		quality = 80
	}

	// Strip metadata by default (better for privacy and file size)
	stripMetadata := true
	if !opts.Strip {
		stripMetadata = false
	}

	// Export with format-specific optimizations
	var output []byte
	switch opts.Format {
	case "webp":
		params := vips.NewWebpExportParams()
		params.Quality = quality
		params.StripMetadata = stripMetadata
		params.ReductionEffort = 4 // 0-6, higher = better compression, slower
		params.Lossless = quality == 100
		output, _, err = img.ExportWebp(params)

	case "avif":
		params := vips.NewAvifExportParams()
		params.Quality = quality
		params.StripMetadata = stripMetadata
		params.Speed = 6 // 0-8, higher = faster, lower quality
		output, _, err = img.ExportAvif(params)

	case "png":
		params := vips.NewPngExportParams()
		params.StripMetadata = stripMetadata
		params.Compression = 6 // 0-9, higher = better compression
		params.Filter = vips.PngFilterAll
		output, _, err = img.ExportPng(params)

	case "jpg", "jpeg":
		fallthrough
	default:
		params := vips.NewJpegExportParams()
		params.Quality = quality
		params.StripMetadata = stripMetadata
		params.OptimizeCoding = true // Optimize Huffman tables
		params.Interlace = true      // Progressive JPEG
		params.SubsampleMode = vips.VipsForeignSubsampleAuto
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
