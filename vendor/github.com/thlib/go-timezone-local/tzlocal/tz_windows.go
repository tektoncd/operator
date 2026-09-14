package tzlocal

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

const tzKey = `SYSTEM\CurrentControlSet\Control\TimeZoneInformation`
const tzKeyVal = "TimeZoneKeyName"
const tzDynamicDayLightDisabledKeyVal = "DynamicDaylightTimeDisabled"

// LocalTZ obtains the name of the time zone Windows is configured to use. Returns the corresponding IANA standard name
func LocalTZ() (string, error) {
	winTZname, dstOff, err := localTZInfo()
	if err != nil {
		return "", err
	}

	// Get the IANA time zone from the set time zone.
	timezone, ok := WinTZtoIANA[winTZname]
	if !ok {
		return "", fmt.Errorf("could not find IANA tz name for set time zone %q", winTZname)
	}

	// If daylight saving time adjustments are disabled, return a fixed-offset
	// zone instead of one that applies the IANA zone's DST transitions.
	if dstOff {
		location, err := time.LoadLocation(timezone)
		if err != nil {
			return "", fmt.Errorf("time.LoadLocation() returned error for IANA timeZone: %v, error: %v", timezone, err.Error())
		}
		hasDst, stdOffset, _ := getDstInfo(location)

		// The DST is turned off in the Windows configuration,
		// but this timezone doesn't have DST, so it doesn't matter.
		if !hasDst {
			return timezone, nil
		}

		if stdOffset == nil {
			return "", fmt.Errorf("%s claims to not have a non-DST time", winTZname)
		}

		// Can't convert this to an hourly offset.
		if *stdOffset%3600 != 0 {
			return "", fmt.Errorf("cannot support disabling DST in the %s zone", winTZname)
		}

		// This has whole hours as offset.
		// Return GMT offset as Etc/GMT+offset
		return fmt.Sprintf("Etc/GMT%+.0f", float64(-*stdOffset)/3600), nil
	}

	return timezone, nil
}

// localTZInfo obtains the Windows time zone name and whether daylight saving
// time is disabled, falling back from the Windows API to tzutil and registry.
func localTZInfo() (string, bool, error) {
	winTZname, dstOff, errDynamic := localTZfromDynamicTimeZoneInformation()
	if errDynamic == nil {
		return winTZname, dstOff, nil
	}

	winTZname, errTzUtil := localTZfromTzutil()
	if errTzUtil == nil {
		const dstOffSuffix = "_dstoff"
		dstOff = strings.HasSuffix(winTZname, dstOffSuffix)
		return strings.TrimSuffix(winTZname, dstOffSuffix), dstOff, nil
	}

	winTZname, dstOff, errReg := localTZfromReg()
	if errReg == nil {
		return winTZname, dstOff, nil
	}

	return "", false, fmt.Errorf("failed to read timezone from Windows API (%v), tzutil (%v), and registry (%v)", errDynamic, errTzUtil, errReg)
}

// localTZfromReg obtains the time zone Windows is configured to use from registry.
func localTZfromReg() (string, bool, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, tzKey, registry.QUERY_VALUE)
	if err != nil {
		return "", false, err
	}
	defer k.Close()

	winTZname, _, err := k.GetStringValue(tzKeyVal)
	if err != nil {
		return "", false, err
	}

	// Get the `DynamicDaylightTimeDisabled` key value, which indicates if DST is enabled or disabled.
	dstOff, _, err := k.GetIntegerValue(tzDynamicDayLightDisabledKeyVal)
	if err != nil {
		dstOff = 0 // Assume DST is not disabled if the value cannot be read
	}

	return strings.TrimSpace(winTZname), dstOff == 1, nil
}

// getDstInfo determines if the timezone observes DST and retrieves the standard and DST offsets.
func getDstInfo(location *time.Location) (bool, *int, *int) {
	var hasDst bool
	var stdOffset, dstOffset *int

	now := time.Now()
	year := now.Year()
	for _, dt := range []time.Time{time.Date(year, 1, 1, 0, 0, 0, 0, location), time.Date(year, 6, 1, 0, 0, 0, 0, location)} {
		_, offset := dt.Zone()
		if dt.IsDST() {
			dstOffset = &offset
			hasDst = true
		} else {
			stdOffset = &offset
		}
	}

	return hasDst, stdOffset, dstOffset
}
