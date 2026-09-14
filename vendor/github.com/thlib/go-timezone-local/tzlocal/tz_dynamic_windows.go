package tzlocal

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const timeZoneIDInvalid = 0xffffffff

// dynamicTimeZoneInformation matches the Windows DYNAMIC_TIME_ZONE_INFORMATION
// structure. It is declared here because the project's version of x/sys/windows
// does not expose it.
type dynamicTimeZoneInformation struct {
	Bias                        int32
	StandardName                [32]uint16
	StandardDate                syscall.Systemtime
	StandardBias                int32
	DaylightName                [32]uint16
	DaylightDate                syscall.Systemtime
	DaylightBias                int32
	TimeZoneKeyName             [128]uint16
	DynamicDaylightTimeDisabled uint8
}

var getDynamicTimeZoneInformation = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDynamicTimeZoneInformation")

// localTZfromDynamicTimeZoneInformation obtains the Windows time zone key name
// and whether dynamic daylight saving time is disabled. The procedure is
// resolved at runtime because it is not available before Windows Vista.
func localTZfromDynamicTimeZoneInformation() (string, bool, error) {
	if err := getDynamicTimeZoneInformation.Find(); err != nil {
		return "", false, err
	}

	var info dynamicTimeZoneInformation
	result, _, callErr := getDynamicTimeZoneInformation.Call(uintptr(unsafe.Pointer(&info)))
	if uint32(result) == timeZoneIDInvalid {
		if callErr != syscall.Errno(0) {
			return "", false, callErr
		}
		return "", false, syscall.EINVAL
	}

	name := strings.TrimSpace(syscall.UTF16ToString(info.TimeZoneKeyName[:]))
	if name == "" {
		return "", false, fmt.Errorf("GetDynamicTimeZoneInformation returned an empty time zone key name")
	}

	return name, info.DynamicDaylightTimeDisabled != 0, nil
}
