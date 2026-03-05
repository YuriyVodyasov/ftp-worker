package keycloak

import (
	"sync"
	"time"
)

type store struct {
	value        *resp
	createdAt    time.Time
	refCreatedAt time.Time
	mu           sync.RWMutex
}

func (s *store) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.value == nil
}

func (s *store) Set(value *resp, isRefresh bool) {
	s.mu.Lock()

	s.createdAt = time.Now()

	if !isRefresh {
		s.refCreatedAt = s.createdAt
	}

	s.value = value

	s.mu.Unlock()
}

func (s *store) TokenIsValid() bool {
	s.mu.RLock()

	t := s.createdAt.Add(time.Second * time.Duration(s.value.ExpiresIn))

	s.mu.RUnlock()

	return time.Now().Before(t)
}

func (s *store) RefreshTokenIsValid() bool {
	s.mu.RLock()

	t := s.refCreatedAt.Add(time.Second * time.Duration(s.value.RefreshExpiresIn))

	s.mu.RUnlock()

	return time.Now().Before(t)
}

func (s *store) GetToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.value.AccessToken
}

func (s *store) GetRefreshToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.value.RefreshToken
}
