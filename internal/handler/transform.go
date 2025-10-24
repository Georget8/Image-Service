package handler

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"image-service/internal/cache"
	"image-service/internal/processor"
)

type Handler struct {
	cache        *cache.Cache
	processor    *processor.Processor
	maxImageSize int64
}

func NewHandler(c *cache.Cache, p *processor.Processor, maxSize int64) *Handler {
	return &Handler{
		cache:        c,
		processor:    p,
		maxImageSize: maxSize,
	}
}

func (h *Handler) Transform(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	imageURL := query.Get("url")
	width, _ := strconv.Atoi(query.Get("w"))
	height, _ := strconv.Atoi(query.Get("h"))
	fit := query.Get("fit")
	if fit == "" {
		fit = "cover"
	}
	format := query.Get("f")
	if format == "" {
		format = "jpeg"
	}
	quality, _ := strconv.Atoi(query.Get("q"))
	if quality <= 0 || quality > 100 {
		quality = 80
	}
	crop := query.Get("crop")
	blur, _ := strconv.Atoi(query.Get("blur"))

	// Advanced parameters
	sharpen, _ := strconv.ParseFloat(query.Get("sharpen"), 64)
	brightness, _ := strconv.ParseFloat(query.Get("brightness"), 64)
	contrast, _ := strconv.ParseFloat(query.Get("contrast"), 64)
	if contrast == 0 {
		contrast = 1.0
	}
	saturation, _ := strconv.ParseFloat(query.Get("saturation"), 64)
	if saturation == 0 {
		saturation = 1.0
	}
	autoOptim := query.Get("auto") == "true" || query.Get("auto") == "1"
	grayscale := query.Get("grayscale") == "true" || query.Get("bw") == "true"
	flip := query.Get("flip")
	rotate, _ := strconv.Atoi(query.Get("rotate"))
	background := query.Get("bg")
	strip := query.Get("strip") != "false"

	cacheKey := h.generateCacheKey(
		imageURL, width, height, fit, format, quality, crop, blur,
		sharpen, brightness, contrast, saturation, autoOptim, grayscale, flip, rotate, background, strip,
	)

	// Check cache
	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		contentType := h.getContentType(format)
		// Check if cached data is SVG
		if h.isSVG(cached) {
			contentType = "image/svg+xml"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Cache", "HIT")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Write(cached)
		return
	}

	// Download image
	imageData, err := h.downloadImage(imageURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to download image: %v", err), http.StatusBadGateway)
		return
	}

	if int64(len(imageData)) > h.maxImageSize {
		http.Error(w, "Image too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Check if input is SVG
	if h.isSVG(imageData) {
		// SVG detected - return as-is (no transformations)
		go func() {
			bgCtx := context.Background()
			h.cache.Set(bgCtx, cacheKey, imageData)
		}()

		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("X-Cache", "MISS")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Write(imageData)
		return
	}

	// Process non-SVG images
	opts := processor.TransformOptions{
		Width:      width,
		Height:     height,
		Fit:        fit,
		Format:     format,
		Quality:    quality,
		Crop:       crop,
		Blur:       blur,
		Sharpen:    sharpen,
		Brightness: brightness,
		Contrast:   contrast,
		Saturation: saturation,
		AutoOptim:  autoOptim,
		Grayscale:  grayscale,
		Flip:       flip,
		Rotate:     rotate,
		Background: background,
		Strip:      strip,
	}

	transformed, err := h.processor.Transform(imageData, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to transform image: %v", err), http.StatusInternalServerError)
		return
	}

	go func() {
		bgCtx := context.Background()
		h.cache.Set(bgCtx, cacheKey, transformed)
	}()

	w.Header().Set("Content-Type", h.getContentType(format))
	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Write(transformed)
}

func (h *Handler) downloadImage(imageURL string) ([]byte, error) {
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		return nil, err
	}
	baseURL := parsedURL.Scheme + "://" + parsedURL.Host

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, err
	}

	// Comprehensive browser headers
	req.Header.Set("X-Image-Service", "railway-transform-v1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("Origin", baseURL)
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "image")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	limitReader := io.LimitReader(resp.Body, h.maxImageSize+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (h *Handler) isSVG(data []byte) bool {
	if len(data) < 5 {
		return false
	}

	// Check first 500 bytes for SVG signatures
	checkLength := 500
	if len(data) < checkLength {
		checkLength = len(data)
	}

	prefix := strings.ToLower(string(data[0:checkLength]))

	// Check for common SVG patterns
	return strings.Contains(prefix, "<svg") ||
		strings.Contains(prefix, "<!doctype svg") ||
		(strings.Contains(prefix, "<?xml") && strings.Contains(prefix, "svg"))
}

func (h *Handler) generateCacheKey(imageURL string, w, ht int, fit, format string, quality int, crop string, blur int,
	sharpen, brightness, contrast, saturation float64, autoOptim, grayscale bool, flip string, rotate int, bg string, strip bool) string {
	data := fmt.Sprintf("%s:%d:%d:%s:%s:%d:%s:%d:%.2f:%.2f:%.2f:%.2f:%t:%t:%s:%d:%s:%t",
		imageURL, w, ht, fit, format, quality, crop, blur,
		sharpen, brightness, contrast, saturation, autoOptim, grayscale, flip, rotate, bg, strip)
	hashBytes := md5.Sum([]byte(data))
	return hex.EncodeToString(hashBytes[:])
}

func (h *Handler) getContentType(format string) string {
	switch format {
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	case "png":
		return "image/png"
	case "svg":
		return "image/svg+xml"
	default:
		return "image/jpeg"
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}
