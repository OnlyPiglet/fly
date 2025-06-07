package imagetools

import (
	"bytes"
	"fmt"
	"github.com/disintegration/imaging"
	"image"
	"io"
	"path/filepath"
)

type ImageProcessLevel = string

const (
	Thumbnail ImageProcessLevel = "thumbnail"
	Low       ImageProcessLevel = "low"
	Medium    ImageProcessLevel = "medium"
	High      ImageProcessLevel = "high"
)

type ProgressiveImageProcessor struct {
	qualityLevels []QualityLevel
}

var DefaultProgressiveImageProcessor = &ProgressiveImageProcessor{
	qualityLevels: []QualityLevel{
		{Thumbnail, 150, 150, 60},
		{Low, 400, 400, 70},
		{Medium, 800, 800, 85},
		{High, 1200, 1200, 95},
	},
}

type QualityLevel struct {
	Name        string
	Width       int
	Height      int
	JPEGQuality int
}

// ProcessImageFromBytes 从字节流处理图像
func (p *ProgressiveImageProcessor) ProcessImageFromBytes(imageData []byte, outputDir string, baseName string) error {
	// 从字节流创建 Reader
	reader := bytes.NewReader(imageData)

	// 使用 Decode 函数加载图像
	src, err := imaging.Decode(reader)
	if err != nil {
		return fmt.Errorf("failed to decode image from bytes: %v", err)
	}

	// 为每个质量级别生成图像
	for _, level := range p.qualityLevels {
		if err := p.generateQualityLevel(src, level, outputDir, baseName); err != nil {
			return fmt.Errorf("failed to generate %s quality: %v", level.Name, err)
		}
	}

	return nil
}

// ProcessImageFromReader 从任意 io.Reader 处理图像
func (p *ProgressiveImageProcessor) ProcessImageFromReader(reader io.Reader, outputDir string, baseName string) error {
	// 直接从 Reader 解码
	src, err := imaging.Decode(reader)
	if err != nil {
		return fmt.Errorf("failed to decode image from reader: %v", err)
	}

	// 处理图像
	for _, level := range p.qualityLevels {
		if err := p.generateQualityLevel(src, level, outputDir, baseName); err != nil {
			return fmt.Errorf("failed to generate %s quality: %v", level.Name, err)
		}
	}

	return nil
}

// EncodeToBytes 将处理后的图像编码为字节流
func (p *ProgressiveImageProcessor) EncodeToBytes(src image.Image, level QualityLevel) ([]byte, error) {
	// 调整图像尺寸
	resized := imaging.Fit(src, level.Width, level.Height, imaging.Lanczos)

	// 应用锐化
	if level.Width < 800 {
		resized = imaging.Sharpen(resized, 0.5)
	}

	// 编码为字节流
	var buf bytes.Buffer
	err := imaging.Encode(&buf, resized, imaging.JPEG, imaging.JPEGQuality(level.JPEGQuality))
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %v", err)
	}

	return buf.Bytes(), nil
}

func (p *ProgressiveImageProcessor) generateQualityLevel(src image.Image, level QualityLevel, outputDir, baseName string) error {
	// 调整图像尺寸，保持宽高比
	resized := imaging.Fit(src, level.Width, level.Height, imaging.Lanczos)

	// 应用轻微的锐化以补偿缩放损失
	if level.Width < 800 {
		resized = imaging.Sharpen(resized, 0.5)
	}

	// 生成输出文件名
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.jpg", baseName, level.Name))

	// 保存为 JPEG，使用指定质量
	err := imaging.Save(resized, outputPath, imaging.JPEGQuality(level.JPEGQuality))
	if err != nil {
		return fmt.Errorf("failed to save image: %v", err)
	}

	fmt.Printf("Generated %s: %s\n", level.Name, outputPath)
	return nil
}

// GenerateWebPVersions 生成 WebP 格式版本（如果需要更好的压缩）
func (p *ProgressiveImageProcessor) GenerateWebPVersions(inputPath string, outputDir string) error {
	src, err := imaging.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %v", err)
	}

	baseName := filepath.Base(inputPath)
	nameWithoutExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]

	// 生成不同尺寸的 PNG 版本（可以后续转换为 WebP）
	for _, level := range p.qualityLevels {
		resized := imaging.Fit(src, level.Width, level.Height, imaging.Lanczos)
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.png", nameWithoutExt, level.Name))

		// 使用最佳压缩级别
		err := imaging.Save(resized, outputPath, imaging.PNGCompressionLevel(9))
		if err != nil {
			return fmt.Errorf("failed to save PNG: %v", err)
		}
	}

	return nil
}

// CompressAnyToJpegImage 将任何其他格式的图片 压缩图片质量的方式，减少图片大小，同时设置格式为 JPEG
func (p *ProgressiveImageProcessor) CompressAnyToJpegImage(inputImg io.Reader, level QualityLevel) (io.Reader, error) {
	src, err := imaging.Decode(inputImg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %s", err.Error())
	}
	processedBytes, err := DefaultProgressiveImageProcessor.EncodeToBytes(src, level)
	if err != nil {
		return nil, fmt.Errorf("failed to process %s quality: %s", level.Name, err.Error())
	}
	return bytes.NewReader(processedBytes), nil
}
