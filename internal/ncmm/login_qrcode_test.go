package ncmm

import (
	"testing"

	"github.com/3899/ncmm/pkg/log"
)

func TestQRCodeCommandOutputFlag(t *testing.T) {
	cmd := qrcode(&Login{}, (*log.Logger)(nil))
	if err := cmd.ParseFlags([]string{"-o", "custom.json"}); err != nil {
		t.Fatal(err)
	}
	got, err := cmd.Flags().GetString("output")
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom.json" {
		t.Fatalf("output flag = %q; want custom.json", got)
	}
}
