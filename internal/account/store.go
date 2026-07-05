package account

import "sync"

// Store is a simple in-memory, thread-safe collection of accounts.
type Store struct {
	mu       sync.Mutex
	accounts map[string]*Account
}

func NewStore() *Store {
	return &Store{
		accounts: make(map[string]*Account),
	}
}

func (s *Store) Create(acc *Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[acc.ID] = acc
}

func (s *Store) Get(id string) (*Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accounts[id]
	return acc, ok
}
