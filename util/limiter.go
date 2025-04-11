package util

import (
	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/time/rate"
	"math"
	"rhyus-golang/conf"
	"time"
)

var GlobalLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(conf.Conf.SessionGlobalLimiter)), conf.Conf.SessionGlobalLimiter)
var visitors, _ = lru.New[string, *rate.Limiter](conf.Conf.SessionApikeyLimiterCacheSize)

func GetApiKeyLimiter(apikey string) *rate.Limiter {

	limiter, ok := visitors.Get(apikey)
	if ok {
		return limiter
	} else {
		limiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(conf.Conf.SessionApikeyLimiter)), conf.Conf.SessionApikeyLimiter)
		visitors.Add(apikey, limiter)
		return limiter
	}
}

var NormalBandwidth = math.Round(float64(conf.Conf.MaxBandwidth*1024) * 0.95)
var FastNormalBandwidth = math.Round(float64(conf.Conf.MaxBandwidth*1024) * 0.95 * 0.1)
var slowBandwidth = math.Round(float64(conf.Conf.MaxBandwidth*1024) * 0.05)
var NormalQueueLimiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(NormalBandwidth)), int(NormalBandwidth))
var FastQueueLimiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(NormalBandwidth)), int(NormalBandwidth))
var SlowQueueLimiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(slowBandwidth)), int(slowBandwidth))
