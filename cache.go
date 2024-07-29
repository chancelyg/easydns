package main

import (
	lru "github.com/hashicorp/golang-lru"
	log "github.com/sirupsen/logrus"
)

func InitializeCache(limit int) (*lru.Cache, error) {
	cache, err := lru.NewWithEvict(limit, func(key interface{}, value interface{}) {
		log.WithFields(log.Fields{"key": key}).Info("evicted cache")
	})
	if err != nil {
		return nil, err
	}
	return cache, nil
}
