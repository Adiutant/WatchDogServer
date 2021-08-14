package httpserver

import (
	"regexp"
	"time"
)

var STATUS_BODY = "^P,\\d,[:lower:]]{3,}"
var STATUS_TIME = "\\d\\d\\d\\d-\\d{1,2}-\\d{1,2}T\\d{1,2}:\\d{1,2}:\\d{1,2}"
var TIME_TEMPLATE = "2006-01-02T15:04:05"

func GetStatusBody(status string) string {
	reg := regexp.MustCompile(STATUS_BODY)
	statusbody := reg.FindString(status)
	return statusbody
}
func GetStatusTime(status string) string {
	reg := regexp.MustCompile(STATUS_TIME)
	statustime := reg.FindString(status)
	return statustime
}
func GetStatusTimeOffsetInMinutes(status string) int {
	timeString := GetStatusTime(status)
	timeParced, _ := time.Parse(TIME_TEMPLATE, timeString)
	offset := time.Since(timeParced)
	return int(offset.Minutes())
}
