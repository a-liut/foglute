package config

import (
	"fmt"
)

const (
	FoglutePackageName = "foglute.aliut.com"
	LongitudeLabelName = "longitude"
	LatitudeLabelName  = "latitude"
	IotCapsLabelName   = "iot_caps"
	SecCapsLabelName   = "sec_caps"
	HwCapsLabelName    = "hw_caps"
)

var LongitudeLabel string
var LatitudeLabel string
var IotLabel string
var SecLabel string
var HwCapsLabel string

func init() {
	LongitudeLabel = fmt.Sprintf("%s/%s", FoglutePackageName, LongitudeLabelName)
	LatitudeLabel = fmt.Sprintf("%s/%s", FoglutePackageName, LatitudeLabelName)
	IotLabel = fmt.Sprintf("%s/%s", FoglutePackageName, IotCapsLabelName)
	SecLabel = fmt.Sprintf("%s/%s", FoglutePackageName, SecCapsLabelName)
	HwCapsLabel = fmt.Sprintf("%s/%s", FoglutePackageName, HwCapsLabelName)
}
