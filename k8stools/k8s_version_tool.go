package k8stools

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	V118 = "1.18"
	V128 = "1.28"
)

type K8sVersion struct {
	VersionStr     string
	MainVersionStr string
	VersionUint    uint32
}

func NewK8sVersion(version string) (*K8sVersion, error) {
	v := strings.ToLower(version)
	if strings.HasPrefix(v, "v") {
		v = v[1:]
	}
	mvs, uv, err := formatK8sVersionFromStr(strings.Trim(v, " "))
	if err != nil {
		return nil, err
	}
	return &K8sVersion{
		VersionStr:     v,
		VersionUint:    uv,
		MainVersionStr: mvs,
	}, nil
}

func formatK8sVersionFromStr(version string) (string, uint32, error) {
	// 按点号（.）分割版本字符串
	versionParts := strings.Split(version, ".")

	if len(versionParts) < 2 {
		return "", 0, fmt.Errorf("version can't be a valid k8s verison %s", version)
	}
	mainVersionString := fmt.Sprintf("%s%s", versionParts[0], versionParts[1])

	// 将版本字符串转换为整数
	versionInt, err := strconv.Atoi(mainVersionString)
	if err != nil {
		return "", 0, err
	}

	return mainVersionString, uint32(versionInt), nil

}

func (k *K8sVersion) NotAfter(c *K8sVersion) bool {
	return k.VersionUint <= c.VersionUint
}

func (k *K8sVersion) NotBefore(c *K8sVersion) bool {
	return k.VersionUint >= c.VersionUint
}

func (k *K8sVersion) MainVerion() string {
	return k.MainVersionStr
}
