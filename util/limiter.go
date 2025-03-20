package util

import (
	"golang.org/x/time/rate"
	"rhyus-golang/conf"
	"sync"
	"time"
)

var GlobalLimiter = rate.NewLimiter(rate.Every(1*60*time.Second), conf.Conf.SessionGlobalLimiter)
var visitors = make(map[string]*rate.Limiter)
var mu sync.Mutex

func GetApiKeyLimiter(apikey string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := visitors[apikey]; !exists {
		visitors[apikey] = rate.NewLimiter(rate.Every(1*60*time.Second), conf.Conf.SessionApikeyLimiter)
	}
	return visitors[apikey]
}
