package main

import (
	"fmt"
	"time"
)

func getRelativeTimeDiff(t1, t2 time.Time) string {
	t1 = t1.Local()
	t2 = t2.Local()
	diff := t1.Sub(t2)
	if diff.Seconds() < 60 {
		return fmt.Sprintf("%d sec ago", int(diff.Seconds()))
	}
	if diff.Minutes() < 60 {
		return fmt.Sprintf("%d min ago", int(diff.Minutes()))
	}
	if diff.Hours() < 24 {
		return fmt.Sprintf("%d h ago", int(diff.Hours()))
	}
	if diff.Hours() < 24*30 {
		return fmt.Sprintf("%d d ago", int(diff.Hours()/24))
	}
	if diff.Hours() < 24*30*12 {
		return fmt.Sprintf("%d mon ago", int(diff.Hours()/(24*30)))
	}
	return fmt.Sprintf("%d years ago", int(diff.Hours()/(24*30*12)))
}
