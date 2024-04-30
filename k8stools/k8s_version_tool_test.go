package k8stools

import "testing"

type testData struct {
	Input    string
	Excepted uint32
}

func TestVersioTool(t *testing.T) {

	tests := []testData{
		{
			"v1.18.3",
			118,
		},
		{
			"v1.18",
			118,
		}, {
			"v1.28.3",
			128,
		}, {
			"v1.28",
			128,
		},
		{
			"1.18.3",
			118,
		},
		{
			"1.18",
			118,
		},
		{
			"1.28.3",
			128,
		},
		{
			" 1.28",
			128,
		},
	}

	for i, _ := range tests {
		version, err := NewK8sVersion(tests[i].Input)
		if err != nil {
			panic(err)
		}
		if version.VersionUint != tests[i].Excepted {
			panic("version mismatch")
		}
	}

}
