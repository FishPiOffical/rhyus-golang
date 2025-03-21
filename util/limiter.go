package util

import (
	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/time/rate"
	"rhyus-golang/common"
	"rhyus-golang/conf"
	"sync"
	"time"
)

var GlobalLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(conf.Conf.SessionGlobalLimiter)), conf.Conf.SessionGlobalLimiter)
var visitors, _ = lru.New[string, *rate.Limiter](conf.Conf.SessionApikeyLimiterCacheSize)
var mu sync.Mutex

func GetApiKeyLimiter(apikey string) *rate.Limiter {

	limiter, ok := visitors.Get(apikey)
	if ok {
		return limiter
	} else {
		limiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(conf.Conf.SessionApikeyLimiter)), conf.Conf.SessionApikeyLimiter)
		visitors.Add(apikey, limiter)
		common.Log.Info("size {}", visitors.Len())
		return limiter
	}
}
