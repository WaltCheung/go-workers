package workers

import (
	"time"

	"github.com/sirupsen/logrus"
)

type MiddlewareStats struct{}

func (l *MiddlewareStats) Call(queue string, message *Msg, next func() CallResult) (result CallResult) {
	defer func() {
		if e := recover(); e != nil || result.Err != nil {
			incrementStats(message.Logger, "failed")
			if e != nil {
				panic(e)
			}
		}
	}()

	result = next()

	incrementStats(message.Logger, "processed")

	return
}

func incrementStats(logger *logrus.Entry, metric string) {
	conn := Config.Pool.Get()
	defer conn.Close()

	today := time.Now().UTC().Format("2006-01-02")

	conn.Send("incr", Config.Namespace+"stat:"+metric)
	conn.Send("incr", Config.Namespace+"stat:"+metric+":"+today)

	conn.Flush()
	if _, err := conn.Receive(); err != nil {
		logger.Println("couldn't save stats:", err)
	}
}
