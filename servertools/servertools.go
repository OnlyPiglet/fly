package servertools

import (
	"errors"
	"fmt"
	"github.com/OnlyPiglet/fly/filetools"
	"github.com/OnlyPiglet/fly/nettools"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"math"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func Round(f float64, n int) float64 {
	pow10_n := math.Pow10(n)
	return math.Trunc((f+0.5/pow10_n)*pow10_n) / pow10_n // TODO +0.5 是为了四舍五入，如果不希望这样去掉这个
}

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)

var (
	//Version           string
	//expectDiskFsTypes = []string{
	//	"apfs", "ext4", "ext3", "ext2", "f2fs", "reiserfs", "jfs", "btrfs",
	//	"fuseblk", "zfs", "simfs", "ntfs", "fat32", "exfat", "xfs", "fuse.rclone",
	//}
	excludeNetInterfaces = []string{
		"lo", "tun", "docker", "veth", "br-", "vmbr", "vnet", "kube",
	}
	//getMacDiskNo = regexp.MustCompile(`\/dev\/disk(\d)s.*`)
)

var (
	netInSpeed, netOutSpeed, netInTransfer, netOutTransfer, lastUpdateNetStats uint64
	cachedBootTime                                                             time.Time
)

// GetHourDiffer 获取相差时间
func GetHourDiffer(startTime, endTime string) int64 {
	var hour int64
	t1, err := time.ParseInLocation("2006-01-02 15:04:05", startTime, time.Local)
	t2, err := time.ParseInLocation("2006-01-02 15:04:05", endTime, time.Local)
	if err == nil && t1.Before(t2) {
		diff := t2.Unix() - t1.Unix() //
		hour = diff / 3600
		return hour
	} else {
		return hour
	}
}

// ServerInfo 获取系统信息
// @Summary 系统信息
// @Description 获取JSON
// @Tags 系统信息
// @Success 200 {object} response.Response "{"code": 200, "data": [...]}"
// @Router /api/v1/server-monitor [get]
// @Security Bearer
func ServerInfo() map[string]interface{} {
	sysInfo, err := host.Info()
	osDic := make(map[string]interface{}, 0)
	osDic["goOs"] = runtime.GOOS
	osDic["arch"] = runtime.GOARCH
	osDic["mem"] = runtime.MemProfileRate
	osDic["compiler"] = runtime.Compiler
	osDic["version"] = runtime.Version()
	osDic["numGoroutine"] = runtime.NumGoroutine()
	osDic["ip"] = nettools.GetLocaHonst()
	osDic["projectDir"] = filetools.GetCurrentPath()
	osDic["hostName"] = sysInfo.Hostname
	osDic["time"] = time.Now().Format("2006-01-02 15:04:05")

	memory, _ := mem.VirtualMemory()
	memDic := make(map[string]interface{}, 0)
	memDic["used"] = memory.Used / MB
	memDic["total"] = memory.Total / MB

	memDic["percent"] = Round(memory.UsedPercent, 2)

	swapDic := make(map[string]interface{}, 0)
	swapDic["used"] = memory.SwapTotal - memory.SwapFree
	swapDic["total"] = memory.SwapTotal

	cpuDic := make(map[string]interface{}, 0)
	cpuDic["cpuInfo"], _ = cpu.Info()
	percent, _ := cpu.Percent(0, false)
	cpuDic["percent"] = Round(percent[0], 2)
	cpuDic["cpuNum"], _ = cpu.Counts(false)

	//服务器磁盘信息
	disklist := make([]disk.UsageStat, 0)
	//所有分区
	var diskTotal, diskUsed, diskUsedPercent float64
	diskInfo, err := disk.Partitions(true)
	if err == nil {
		for _, p := range diskInfo {
			diskDetail, err := disk.Usage(p.Mountpoint)
			if err == nil {
				diskDetail.UsedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", diskDetail.UsedPercent), 64)
				diskDetail.Total = diskDetail.Total / 1024 / 1024
				diskDetail.Used = diskDetail.Used / 1024 / 1024
				diskDetail.Free = diskDetail.Free / 1024 / 1024
				disklist = append(disklist, *diskDetail)

			}
		}
	}

	d, _ := disk.Usage("/")

	diskTotal = float64(d.Total / GB)
	diskUsed = float64(d.Used / GB)
	diskUsedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", d.UsedPercent), 64)

	diskDic := make(map[string]interface{}, 0)
	diskDic["total"] = diskTotal
	diskDic["used"] = diskUsed
	diskDic["percent"] = diskUsedPercent

	bootTime, _ := host.BootTime()
	cachedBootTime = time.Unix(int64(bootTime), 0)

	TrackNetworkSpeed()
	netDic := make(map[string]interface{}, 0)
	netDic["in"] = Round(float64(netInSpeed/KB), 2)
	netDic["out"] = Round(float64(netOutSpeed/KB), 2)
	return map[string]interface{}{
		"code":     200,
		"os":       osDic,
		"mem":      memDic,
		"cpu":      cpuDic,
		"disk":     diskDic,
		"net":      netDic,
		"swap":     swapDic,
		"location": "Aliyun",
		"bootTime": GetHourDiffer(cachedBootTime.Format("2006-01-02 15:04:05"), time.Now().Format("2006-01-02 15:04:05")),
	}
}

func TrackNetworkSpeed() {
	var innerNetInTransfer, innerNetOutTransfer uint64
	nc, err := net.IOCounters(true)
	if err == nil {
		for _, v := range nc {
			if isListContainsStr(excludeNetInterfaces, v.Name) {
				continue
			}
			innerNetInTransfer += v.BytesRecv
			innerNetOutTransfer += v.BytesSent
		}
		now := uint64(time.Now().Unix())
		diff := now - lastUpdateNetStats
		if diff > 0 {
			netInSpeed = (innerNetInTransfer - netInTransfer) / diff
			netOutSpeed = (innerNetOutTransfer - netOutTransfer) / diff
		}
		netInTransfer = innerNetInTransfer
		netOutTransfer = innerNetOutTransfer
		lastUpdateNetStats = now
	}
}

func isListContainsStr(list []string, str string) bool {
	for i := 0; i < len(list); i++ {
		if strings.Contains(str, list[i]) {
			return true
		}
	}
	return false
}

func GetIPv4ByInterfaceName(ifaceName string) (string, error) {
	iface, err := rnet.InterfaceByName(ifaceName)
	if err != nil {
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		var ip rnet.IP
		switch v := addr.(type) {
		case *rnet.IPNet:
			ip = v.IP
		case *rnet.IPAddr:
			ip = v.IP
		}

		// 只返回非 loopback 的 IPv4 地址
		if ip.To4() != nil && !ip.IsLoopback() {
			return ip.String(), nil
		}
	}
	return "", errors.New("no valid IPv4 address found on interface " + ifaceName)
}
