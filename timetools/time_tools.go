package timetools

import (
	"fmt"
	"sync"
	"time"
)

// 缓存已加载的时区位置
var locationCache = make(map[string]*time.Location)
var cacheMutex sync.RWMutex

// GetCurrentTimeInTimezone 获取当前时间在指定时区的表示（高性能版本）
// targetTimezone: 目标时区名称（如 "America/New_York"）
// 返回: 目标时区的时间字符串和时区信息
func GetCurrentTimeInTimezone(targetTimezone string) (string, string, error) {
	// 获取当前时间（UTC）
	now := time.Now().UTC()

	// 获取时区位置（使用缓存）
	loc, err := getCachedLocation(targetTimezone)
	if err != nil {
		return "", "", err
	}

	// 转换为目标时区时间
	targetTime := now.In(loc)

	// 格式化为字符串
	targetTimeStr := targetTime.Format("2006-01-02 15:04:05")
	timezoneAbbr := targetTime.Format("MST")

	return targetTimeStr, timezoneAbbr, nil
}

// getCachedLocation 获取缓存的时区位置
func getCachedLocation(timezone string) (*time.Location, error) {
	// 先尝试读锁读取缓存
	cacheMutex.RLock()
	if loc, found := locationCache[timezone]; found {
		cacheMutex.RUnlock()
		return loc, nil
	}
	cacheMutex.RUnlock()

	// 未命中缓存，获取写锁
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// 双重检查，避免其他goroutine已经加载
	if loc, found := locationCache[timezone]; found {
		return loc, nil
	}

	// 加载时区
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("无法加载时区 '%s': %v", timezone, err)
	}

	// 存入缓存
	locationCache[timezone] = loc
	return loc, nil
}
