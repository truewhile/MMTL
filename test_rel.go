package main
import (
	"context"
	"fmt"
	"github.com/ShukeBta/MMTL/internal/service"
	"github.com/ShukeBta/MMTL/internal/service/cloud115"
	"go.uber.org/zap"
)
func main() {
	crypto := service.NewCryptoService("test-secret", zap.NewNop())
	// Let's test with a mock RemoteFileDetail
	d := &cloud115.RemoteFileDetail{
		FileId: "3251154147730910635",
		FileName: "出包王女",
		Paths: []struct {
			FileId string 
			Name   string 
		}{
			{FileId: "0", Name: "根目录"},
			{FileId: "3238787832374488117", Name: "影视库"},
			{FileId: "3238787913223892116", Name: "动漫"},
		},
	}
	fmt.Println("RelativePath when rootCID is 3238787832374488117:", d.RelativePath("3238787832374488117"))
}
