package gofmt

import (
	"fmt"
	"os"
	"testing"

	"github.com/qiniu/x/xlog"
)

func TestFormatGofmt(t *testing.T) {
	content, err := os.ReadFile("../../../../testdata/gofmt_test.txt")
	if err != nil {
		fmt.Println("无法读取文件:", err)
		return
	}
	result, _ := formatGofmtOutput(&xlog.Logger{}, content)
	for key, value := range result {
		fmt.Printf("filename : %s \n ", key)
		for _, v := range value {
			fmt.Println("message: \n", v)
		}
	}
	fmt.Println(result)
}
