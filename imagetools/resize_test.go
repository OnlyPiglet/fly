package imagetools

import (
	"testing"
)

func TestProgressiveImageProcessor_CreatePlaceholder(t *testing.T) {
	//processor := &ProgressiveImageProcessor{
	//	qualityLevels: []QualityLevel{
	//		{"thumbnail", 150, 150, 60},
	//		{"low", 400, 400, 70},
	//		{"medium", 800, 800, 85},
	//		{"high", 1200, 1200, 95},
	//	},
	//}
	//
	//reader, _ := os.Open("/Users/onlypiglet/Desktop/r3rgHk_1748078109_NewTestCount2Channel3Image_w3072_h2048_fn0.jpg")
	//
	//src, err := imaging.Decode(reader)
	//if err != nil {
	//	log.Fatalf("Failed to decode image: %v", err)
	//}
	//
	//// 生成不同质量级别的字节流
	//for _, level := range processor.qualityLevels {
	//	processedBytes, err := processor.EncodeToBytes(src, level)
	//	if err != nil {
	//		log.Printf("Failed to process %s quality: %v", level.Name, err)
	//		continue
	//	}
	//
	//	file, err := os.Create(fmt.Sprintf("./%s.jpeg", level.Name))
	//	file.Write(processedBytes)
	//	file.Close()
	//
	//	fmt.Printf("Generated %s quality: %d bytes\n", level.Name, len(processedBytes))
	//
	//	// 这里你可以将 processedBytes 保存到数据库、发送到网络等
	//}
}
