package ncmm

import (
	"reflect"
	"testing"

	"github.com/3899/ncmm/config"
)

func TestConfiguredFansGroupIDsFallbackDefault(t *testing.T) {
	got := configuredFansGroupIDs(nil)
	want := []string{defaultFansGroupId}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuredFansGroupIDs(nil) = %#v, want %#v", got, want)
	}

	got = configuredFansGroupIDs(&config.FansGroupConf{})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuredFansGroupIDs(empty) = %#v, want %#v", got, want)
	}
}

func TestConfiguredFansGroupIDsUsesConfiguredUniqueList(t *testing.T) {
	got := configuredFansGroupIDs(&config.FansGroupConf{
		GroupIDs: config.StringOrSlice{"111", "222", "111", "", " 333 "},
	})
	want := []string{"111", "222", "333"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuredFansGroupIDs(configured) = %#v, want %#v", got, want)
	}
}
