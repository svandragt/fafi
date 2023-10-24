package sander

var Debug = false

func UpdateDebugState() {
	isDebug := GetArgFromEnvWithDefault("FAFI_DEBUG", "0")
	Debug = isDebug != "0"
}
