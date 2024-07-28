package main

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/sirupsen/logrus"
)

func InitializeCache(limit int) (*lru.Cache, error) {
	cache, err := lru.NewWithEvict(limit, func(key interface{}, value interface{}) {
		logrus.Infof("Evicted from cache: %v", key)
	})
	if err != nil {
		return nil, err
	}
	return cache, nil
}
