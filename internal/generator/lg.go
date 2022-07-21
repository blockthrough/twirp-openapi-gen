package generator

import "log"

type Lg struct {
	verbose bool
}

func (l *Lg) logd(format string, args ...interface{}) {
	if !l.verbose {
		return
	}
	log.Printf(format, args...)
}

func (l *Lg) log(format string, args ...interface{}) {
	log.Printf(format, args...)
}
